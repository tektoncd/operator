/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	rcommon "github.com/tektoncd/operator/pkg/reconciler/common"
)

const (
	// MetricsHTTPPort is the upstream port name for plain-HTTP metrics.
	MetricsHTTPPort = "http-metrics"
	// MetricsHTTPSPort is the renamed port name after mTLS is enabled.
	MetricsHTTPSPort = "https-metrics"

	// metricsServingCertAnnotation is the OpenShift annotation that triggers
	// automatic provisioning of a service serving certificate.
	metricsServingCertAnnotation = "service.beta.openshift.io/serving-cert-secret-name"

	// metricsServingCertVolume and metricsClientCAVolume are the Pod volume names.
	metricsServingCertVolume = "metrics-serving-cert"
	metricsClientCAVolume    = "metrics-client-ca"

	// MetricsTLSMountPath is where the serving-cert Secret is mounted.
	MetricsTLSMountPath = "/etc/metrics-tls"
	// MetricsClientCAMountPath is where the client-CA ConfigMap is mounted.
	MetricsClientCAMountPath = "/etc/metrics-client-ca"

	// Env vars read by knative/pkg prometheus.Server to configure mTLS.
	envMetricsTLSCert       = "METRICS_PROMETHEUS_TLS_CERT"
	envMetricsTLSKey        = "METRICS_PROMETHEUS_TLS_KEY"
	envMetricsTLSClientCA   = "METRICS_PROMETHEUS_TLS_CLIENT_CA_FILE"
	envMetricsTLSClientAuth = "METRICS_PROMETHEUS_TLS_CLIENT_AUTH"

	// Paths inside the Prometheus pod where CMO mounts the scraping credentials.
	// These match the tls-client-certificate-auth scrapeClass definition.
	promCertFile = "/etc/prometheus/secrets/metrics-client-certs/tls.crt"
	promKeyFile  = "/etc/prometheus/secrets/metrics-client-certs/tls.key"
	promCAFile   = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"
)

// MetricsServingCertSecretName returns the conventional TLS Secret name for
// the given Service (i.e. "<serviceName>-metrics-tls").
func MetricsServingCertSecretName(serviceName string) string {
	return serviceName + "-metrics-tls"
}

// AnnotateMetricsServingCert adds the OpenShift serving-cert annotation to the
// named Service so that OpenShift automatically provisions a TLS Secret named
// "<serviceName>-metrics-tls". Call this once per Service that needs its own
// metrics serving certificate. If another transformer already sets this
// annotation on the Service, do not call this — use the existing secret instead.
func AnnotateMetricsServingCert(serviceName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Service" || u.GetName() != serviceName {
			return nil
		}
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[metricsServingCertAnnotation] = MetricsServingCertSecretName(serviceName)
		u.SetAnnotations(annotations)
		return nil
	}
}

// RenameServicePort renames fromPort to toPort on the named Service.
// Use this to align the Service port name with what the ServiceMonitor expects
// (e.g. "http-metrics" → "https-metrics", "prometheus" → "https-prometheus").
func RenameServicePort(serviceName, fromPort, toPort string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Service" || u.GetName() != serviceName {
			return nil
		}
		svc := &corev1.Service{}
		if err := scheme.Scheme.Convert(u, svc, nil); err != nil {
			return err
		}
		renamed := false
		for i, p := range svc.Spec.Ports {
			if p.Name == fromPort {
				svc.Spec.Ports[i].Name = toPort
				renamed = true
			}
		}
		if !renamed {
			return nil
		}
		svc.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   corev1.SchemeGroupVersion.Group,
			Version: corev1.SchemeGroupVersion.Version,
			Kind:    "Service",
		})
		m, err := toUnstructured(svc)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(m.Object)
		return nil
	}
}

// UpdateServiceMonitorForMetricsMTLS transforms a ServiceMonitor (matched by
// name) that was previously scraping over plain HTTP into one that uses mTLS:
//
//   - Renames the endpoint port from fromPort to toPort
//   - Sets scheme to "https"
//   - Injects the standard Prometheus-pod tlsConfig (cert/key/ca file paths +
//     serverName derived from serviceName and targetNamespace)
//
// fromPort and toPort must be consistent with the corresponding RenameServicePort
// call on the backing Service (e.g. MetricsHTTPPort → MetricsHTTPSPort).
func UpdateServiceMonitorForMetricsMTLS(smName, fromPort, toPort, serviceName, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ServiceMonitor" || u.GetName() != smName {
			return nil
		}

		endpoints, found, err := unstructured.NestedSlice(u.Object, "spec", "endpoints")
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		for i, ep := range endpoints {
			epMap, ok := ep.(map[string]interface{})
			if !ok {
				continue
			}
			if epMap["port"] == fromPort {
				epMap["port"] = toPort
			}
			epMap["scheme"] = "https"
			epMap["tlsConfig"] = map[string]interface{}{
				"caFile":     promCAFile,
				"certFile":   promCertFile,
				"keyFile":    promKeyFile,
				"serverName": serviceName + "." + targetNamespace + ".svc",
			}
			endpoints[i] = epMap
		}
		return unstructured.SetNestedSlice(u.Object, endpoints, "spec", "endpoints")
	}
}

// ApplyMetricsTLS is a manifest transformer that injects the serving-cert
// Secret and the metrics-client-ca ConfigMap as volumes into the named
// Deployment or StatefulSet, and sets the METRICS_PROMETHEUS_TLS_* env vars
// on all containers so that the knative prometheus.Server enables mTLS.
//
//   - kind: "Deployment" or "StatefulSet"
//   - name: metadata.name of the resource to patch
//   - secretName: name of the TLS Secret to mount (use MetricsServingCertSecretName)
func ApplyMetricsTLS(kind, name, secretName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != kind || u.GetName() != name {
			return nil
		}

		switch kind {
		case "Deployment":
			return applyMetricsTLSToDeployment(u, secretName)
		case "StatefulSet":
			return applyMetricsTLSToStatefulSet(u, secretName)
		}
		return nil
	}
}

func applyMetricsTLSToDeployment(u *unstructured.Unstructured, secretName string) error {
	d := &appsv1.Deployment{}
	if err := scheme.Scheme.Convert(u, d, nil); err != nil {
		return err
	}

	injectMetricsTLSIntoPodSpec(&d.Spec.Template.Spec, secretName)

	d.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "Deployment",
	})
	m, err := toUnstructured(d)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(m.Object)
	return nil
}

func applyMetricsTLSToStatefulSet(u *unstructured.Unstructured, secretName string) error {
	sts := &appsv1.StatefulSet{}
	if err := scheme.Scheme.Convert(u, sts, nil); err != nil {
		return err
	}

	injectMetricsTLSIntoPodSpec(&sts.Spec.Template.Spec, secretName)

	sts.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   appsv1.SchemeGroupVersion.Group,
		Version: appsv1.SchemeGroupVersion.Version,
		Kind:    "StatefulSet",
	})
	m, err := toUnstructured(sts)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(m.Object)
	return nil
}

// injectMetricsTLSIntoPodSpec adds the TLS volumes and env vars to a PodSpec.
func injectMetricsTLSIntoPodSpec(spec *corev1.PodSpec, secretName string) {
	// Add the serving-cert Secret volume.
	servingCertVol := corev1.Volume{
		Name: metricsServingCertVolume,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	spec.Volumes = rcommon.AddOrReplaceInList(
		spec.Volumes, servingCertVol, func(v corev1.Volume) string { return v.Name },
	)

	// Add the client-CA ConfigMap volume.
	clientCAVol := corev1.Volume{
		Name: metricsClientCAVolume,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: MetricsClientCAConfigMap,
				},
			},
		},
	}
	spec.Volumes = rcommon.AddOrReplaceInList(
		spec.Volumes, clientCAVol, func(v corev1.Volume) string { return v.Name },
	)

	tlsEnvVars := []corev1.EnvVar{
		{Name: envMetricsTLSCert, Value: fmt.Sprintf("%s/tls.crt", MetricsTLSMountPath)},
		{Name: envMetricsTLSKey, Value: fmt.Sprintf("%s/tls.key", MetricsTLSMountPath)},
		{Name: envMetricsTLSClientCA, Value: fmt.Sprintf("%s/%s", MetricsClientCAMountPath, MetricsClientCAKey)},
		{Name: envMetricsTLSClientAuth, Value: "require"},
	}

	for i := range spec.Containers {
		c := &spec.Containers[i]

		// Add serving-cert volume mount.
		c.VolumeMounts = rcommon.AddOrReplaceInList(
			c.VolumeMounts,
			corev1.VolumeMount{
				Name:      metricsServingCertVolume,
				MountPath: MetricsTLSMountPath,
				ReadOnly:  true,
			},
			func(vm corev1.VolumeMount) string { return vm.Name },
		)

		// Add client-CA volume mount.
		c.VolumeMounts = rcommon.AddOrReplaceInList(
			c.VolumeMounts,
			corev1.VolumeMount{
				Name:      metricsClientCAVolume,
				MountPath: MetricsClientCAMountPath,
				ReadOnly:  true,
			},
			func(vm corev1.VolumeMount) string { return vm.Name },
		)

		// Add / replace TLS env vars.
		for _, env := range tlsEnvVars {
			c.Env = rcommon.AddOrReplaceInList(
				c.Env, env, func(e corev1.EnvVar) string { return e.Name },
			)
		}
	}
}
