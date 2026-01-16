/*
Copyright 2021 The Tekton Authors

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
	"context"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// RemoveRunAsUser will remove RunAsUser from all container in a deployment
func RemoveRunAsUser() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		containers := d.Spec.Template.Spec.Containers
		removeRunAsUser(containers)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// RemoveRunAsUserForJob will remove RunAsUser from all container in a job
func RemoveRunAsUserForJob() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Job" {
			return nil
		}

		jb := &batchv1.Job{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, jb)
		if err != nil {
			return err
		}

		containers := jb.Spec.Template.Spec.Containers
		removeRunAsUser(containers)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

func removeRunAsUser(containers []corev1.Container) {
	for i := range containers {
		c := &containers[i]
		if c.SecurityContext != nil {
			// Remove runAsUser
			c.SecurityContext.RunAsUser = nil
		}
	}
}

// RemoveRunAsGroup will remove runAsGroup from all container in a deployment
func RemoveRunAsGroup() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			if c.SecurityContext != nil {
				// Remove runAsGroup
				c.SecurityContext.RunAsGroup = nil
			}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// RemoveFsGroupForDeployment will remove FsGroup in a deployment
func RemoveFsGroupForDeployment() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		if d.Spec.Template.Spec.SecurityContext.FSGroup != nil {
			d.Spec.Template.Spec.SecurityContext.FSGroup = nil
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// RemoveFsGroupForJob will remove FsGroup in a job
func RemoveFsGroupForJob() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Job" {
			return nil
		}

		jb := &batchv1.Job{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, jb)
		if err != nil {
			return err
		}

		if jb.Spec.Template.Spec.SecurityContext.FSGroup != nil {
			jb.Spec.Template.Spec.SecurityContext.FSGroup = nil
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func UpdateServiceMonitorTargetNamespace(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ServiceMonitor" {
			return nil
		}
		nsSelector, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "namespaceSelector")
		if !found || err != nil {
			return err
		}
		nsSelector.(map[string]interface{})["matchNames"].([]interface{})[0] = targetNamespace
		return nil
	}
}

// RemoveRunAsUserForStatefulset will remove RunAsUser from all container in a statefulset
func RemoveRunAsUserForStatefulSet(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" || u.GetName() != name {
			return nil
		}

		sts := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sts)
		if err != nil {
			return err
		}

		containers := sts.Spec.Template.Spec.Containers
		removeRunAsUser(containers)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// RemoveFsGroupForStatefulSet will remove FsGroup in a statefulset
func RemoveFsGroupForStatefulSet(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" || u.GetName() != name {
			return nil
		}

		sts := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sts)
		if err != nil {
			return err
		}

		if sts.Spec.Template.Spec.SecurityContext != nil && sts.Spec.Template.Spec.SecurityContext.FSGroup != nil {
			sts.Spec.Template.Spec.SecurityContext.FSGroup = nil
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// RemoveRunAsGroupForStatefulSet will remove runAsGroup from all container in a statefulset
func RemoveRunAsGroupForStatefulSet(name string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" || u.GetName() != name {
			return nil
		}

		sts := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sts)
		if err != nil {
			return err
		}

		for i := range sts.Spec.Template.Spec.Containers {
			c := &sts.Spec.Template.Spec.Containers[i]
			if c.SecurityContext != nil {
				// Remove runAsGroup
				c.SecurityContext.RunAsGroup = nil
			}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// InjectTLSEnvVarsTransformer creates a manifestival transformer that injects TLS configuration
// as environment variables into Deployments and StatefulSets, and adds a hash annotation to
// trigger pod restarts when the TLS configuration changes.
//
// This transformer reads TLS config from the context (populated by ObserveAndStoreTLSConfig
// at operator startup) and injects it as environment variables:
//   - TLS_MIN_VERSION: e.g., "TLSv1.3"
//   - TLS_CIPHER_SUITES: e.g., "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384"
//   - TLS_CURVE_PREFERENCES: e.g., "P-256,P-384,P-521"
//
// The transformer also adds an annotation "operator.tekton.dev/tls-config-hash" to the pod template,
// which triggers automatic pod restarts when the cluster's TLS profile changes.
//
// Parameters:
//   - ctx: Context containing the TLS config (set by ObserveAndStoreTLSConfig at startup)
//   - containerNames: Optional list of container names to inject into. If empty, injects into all containers.
//
// Usage:
//
//	transformers := []mf.Transformer{
//	    InjectTLSEnvVarsTransformer(ctx, "webhook", "controller"),
//	}
func InjectTLSEnvVarsTransformer(ctx context.Context, containerNames ...string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Only process Deployments and StatefulSets
		kind := u.GetKind()
		if kind != "Deployment" && kind != "StatefulSet" {
			return nil
		}

		// Get TLS config from context
		tlsConfig := GetTLSConfigFromContext(ctx)
		if tlsConfig == nil {
			// TLS config not available (e.g., Kubernetes platform or disabled)
			return nil
		}

		tlsConfigHash := GetTLSConfigHashFromContext(ctx)
		tlsEnvVars := TLSEnvVarsFromConfig(tlsConfig)

		// Convert unstructured to typed object
		var podTemplateSpec *corev1.PodTemplateSpec
		switch kind {
		case "Deployment":
			d := &appsv1.Deployment{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d); err != nil {
				return err
			}
			podTemplateSpec = &d.Spec.Template
			defer func() {
				// Convert back to unstructured
				uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
				if err != nil {
					return
				}
				u.SetUnstructuredContent(uObj)
			}()
		case "StatefulSet":
			s := &appsv1.StatefulSet{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, s); err != nil {
				return err
			}
			podTemplateSpec = &s.Spec.Template
			defer func() {
				// Convert back to unstructured
				uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
				if err != nil {
					return
				}
				u.SetUnstructuredContent(uObj)
			}()
		}

		// Create environment variables
		envVars := []corev1.EnvVar{}
		if tlsEnvVars.MinVersion != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  TLSMinVersionEnvVar,
				Value: tlsEnvVars.MinVersion,
			})
		}
		if tlsEnvVars.CipherSuites != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  TLSCipherSuitesEnvVar,
				Value: tlsEnvVars.CipherSuites,
			})
		}
		if tlsEnvVars.CurvePreferences != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  TLSCurvePreferencesEnvVar,
				Value: tlsEnvVars.CurvePreferences,
			})
		}

		// Inject env vars into containers
		for i, container := range podTemplateSpec.Spec.Containers {
			// If specific containers specified, only inject to those
			if len(containerNames) > 0 {
				found := false
				for _, name := range containerNames {
					if container.Name == name {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			// Merge env vars (replace if exists, append if new)
			podTemplateSpec.Spec.Containers[i].Env = mergeEnvVars(container.Env, envVars)
		}

		// Add hash annotation to pod template to trigger pod restart when TLS config changes
		if podTemplateSpec.Annotations == nil {
			podTemplateSpec.Annotations = make(map[string]string)
		}
		podTemplateSpec.Annotations["operator.tekton.dev/tls-config-hash"] = tlsConfigHash

		return nil
	}
}

// mergeEnvVars merges new environment variables into existing ones
// If an env var with the same name exists, it's replaced; otherwise appended
func mergeEnvVars(existing, new []corev1.EnvVar) []corev1.EnvVar {
	result := make([]corev1.EnvVar, len(existing))
	copy(result, existing)

	for _, newEnv := range new {
		found := false
		for j, existingEnv := range result {
			if existingEnv.Name == newEnv.Name {
				result[j] = newEnv
				found = true
				break
			}
		}
		if !found {
			result = append(result, newEnv)
		}
	}

	return result
}
