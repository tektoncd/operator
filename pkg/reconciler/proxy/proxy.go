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

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/markbates/inflect"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	// user-provided and system CA certificates
	trustedCAConfigMapName   = "config-trusted-cabundle"
	trustedCAConfigMapVolume = "config-trusted-cabundle-volume"
	trustedCAKey             = "ca-bundle.crt"

	// service serving certificates (required to talk to the internal registry)
	serviceCAConfigMapName   = "config-service-cabundle"
	serviceCAConfigMapVolume = "config-service-cabundle-volume"
	serviceCAKey             = "service-ca.crt"
)

// reconciler implements the AdmissionController for resources
type reconciler struct {
	webhook.StatelessAdmissionImpl
	pkgreconciler.LeaderAwareFuncs

	key  types.NamespacedName
	path string

	withContext func(context.Context) context.Context

	client       kubernetes.Interface
	mwhlister    admissionlisters.MutatingWebhookConfigurationLister
	secretlister corelisters.SecretLister

	disallowUnknownFields bool
	secretName            string
}

var _ controller.Reconciler = (*reconciler)(nil)
var _ pkgreconciler.LeaderAware = (*reconciler)(nil)
var _ webhook.AdmissionController = (*reconciler)(nil)
var _ webhook.StatelessAdmissionController = (*reconciler)(nil)

// Reconcile implements controller.Reconciler
func (ac *reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	if !ac.IsLeaderFor(ac.key) {
		logger.Debugf("Skipping key %q, not the leader.", ac.key)
		return nil
	}

	// Look up the webhook secret, and fetch the CA cert bundle.
	secret, err := ac.secretlister.Secrets(system.Namespace()).Get(ac.secretName)
	if err != nil {
		logger.Errorw("Error fetching secret", zap.Error(err))
		return err
	}
	caCert, ok := secret.Data[certresources.CACert]
	if !ok {
		return fmt.Errorf("secret %q is missing %q key", ac.secretName, certresources.CACert)
	}

	// Reconcile the webhook configuration.
	return ac.reconcileMutatingWebhook(ctx, caCert)
}

// Path implements AdmissionController
func (ac *reconciler) Path() string {
	return ac.path
}

// Admit implements AdmissionController
func (ac *reconciler) Admit(ctx context.Context, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if ac.withContext != nil {
		ctx = ac.withContext(ctx)
	}

	logger := logging.FromContext(ctx)
	switch request.Operation {
	case admissionv1.Create:
	default:
		logger.Info("Unhandled webhook operation, letting it through ", request.Operation)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	patchBytes, err := ac.mutate(ctx, request)
	if err != nil {
		return webhook.MakeErrorStatus("mutation failed: %v", err)
	}
	logger.Infof("Kind: %q PatchBytes: %v", request.Kind, string(patchBytes))

	return &admissionv1.AdmissionResponse{
		Patch:   patchBytes,
		Allowed: true,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (ac *reconciler) reconcileMutatingWebhook(ctx context.Context, caCert []byte) error {
	logger := logging.FromContext(ctx)

	plural := strings.ToLower(inflect.Pluralize("Pod"))
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{plural, plural + "/status"},
			},
		},
	}

	configuredWebhook, err := ac.mwhlister.Get(ac.key.Name)
	if err != nil {
		return fmt.Errorf("error retrieving webhook: %w", err)
	}

	webhook := configuredWebhook.DeepCopy()

	// Clear out any previous (bad) OwnerReferences.
	// See: https://github.com/knative/serving/issues/5845
	webhook.OwnerReferences = nil

	for i, wh := range webhook.Webhooks {
		if wh.Name != webhook.Name {
			continue
		}
		webhook.Webhooks[i].Rules = rules
		webhook.Webhooks[i].NamespaceSelector = &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "operator.tekton.dev/disable-proxy",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}, {
				// "control-plane" is added to support Azure's AKS, otherwise the controllers fight.
				// See knative/pkg#1590 for details.
				Key:      "control-plane",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}},
		}
		webhook.Webhooks[i].ObjectSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/managed-by": "tekton-pipelines",
			},
		}
		webhook.Webhooks[i].ClientConfig.CABundle = caCert
		if webhook.Webhooks[i].ClientConfig.Service == nil {
			return fmt.Errorf("missing service reference for webhook: %s", wh.Name)
		}
		webhook.Webhooks[i].ClientConfig.Service.Path = ptr.String(ac.Path())
	}

	if ok, err := kmp.SafeEqual(configuredWebhook, webhook); err != nil {
		return fmt.Errorf("error diffing webhooks: %w", err)
	} else if !ok {
		logger.Info("Updating webhook")
		mwhclient := ac.client.AdmissionregistrationV1().MutatingWebhookConfigurations()
		if _, err := mwhclient.Update(ctx, webhook, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	} else {
		logger.Info("Webhook is valid")
	}
	return nil
}

func (ac *reconciler) mutate(ctx context.Context, req *admissionv1.AdmissionRequest) ([]byte, error) {
	kind := req.Kind
	newBytes := req.Object.Raw
	oldBytes := req.OldObject.Raw
	// Why, oh why are these different types...
	gvk := schema.GroupVersionKind{
		Group:   kind.Group,
		Version: kind.Version,
		Kind:    kind.Kind,
	}

	logger := logging.FromContext(ctx)
	if gvk.Group != "" || gvk.Version != "v1" || gvk.Kind != "Pod" {
		logger.Error("Unhandled kind: ", gvk)
		return nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	// nil values denote absence of `old` (create) or `new` (delete) objects.
	var oldObj, newObj corev1.Pod

	if len(newBytes) != 0 {
		newDecoder := json.NewDecoder(bytes.NewBuffer(newBytes))
		if ac.disallowUnknownFields {
			newDecoder.DisallowUnknownFields()
		}
		if err := newDecoder.Decode(&newObj); err != nil {
			return nil, fmt.Errorf("cannot decode incoming new object: %w", err)
		}
	}
	if len(oldBytes) != 0 {
		oldDecoder := json.NewDecoder(bytes.NewBuffer(oldBytes))
		if ac.disallowUnknownFields {
			oldDecoder.DisallowUnknownFields()
		}
		if err := oldDecoder.Decode(&oldObj); err != nil {
			return nil, fmt.Errorf("cannot decode incoming old object: %w", err)
		}
	}
	var patches duck.JSONPatch

	var err error
	// Skip this step if the type we're dealing with is a duck type, since it is inherently
	// incomplete and this will patch away all of the unspecified fields.
	// Add these before defaulting fields, otherwise defaulting may cause an illegal patch
	// because it expects the round tripped through Golang fields to be present already.
	rtp, err := roundTripPatch(newBytes, newObj)
	if err != nil {
		return nil, fmt.Errorf("cannot create patch for round tripped newBytes: %w", err)
	}
	patches = append(patches, rtp...)

	ctx = apis.WithinCreate(ctx)
	ctx = apis.WithUserInfo(ctx, &req.UserInfo)

	// Default the new object.
	if patches, err = setDefaults(ac.client, ctx, patches, newObj); err != nil {
		logger.Errorw("Failed the resource specific defaulter", zap.Error(err))
		// Return the error message as-is to give the defaulter callback
		// discretion over (our portion of) the message that the user sees.
		return nil, err
	}

	return json.Marshal(patches)
}

// roundTripPatch generates the JSONPatch that corresponds to round tripping the given bytes through
// the Golang type (JSON -> Golang type -> JSON). Because it is not always true that
// bytes == json.Marshal(json.Unmarshal(bytes)).
//
// For example, if bytes did not contain a 'spec' field and the Golang type specifies its 'spec'
// field without omitempty, then by round tripping through the Golang type, we would have added
// `'spec': {}`.
func roundTripPatch(bytes []byte, unmarshalled interface{}) (duck.JSONPatch, error) {
	if unmarshalled == nil {
		return duck.JSONPatch{}, nil
	}
	marshaledBytes, err := json.Marshal(unmarshalled)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal interface: %w", err)
	}
	return jsonpatch.CreatePatch(bytes, marshaledBytes)
}

// setDefaults simply leverages apis.Defaultable to set defaults.
func setDefaults(client kubernetes.Interface, ctx context.Context, patches duck.JSONPatch, pod corev1.Pod) (duck.JSONPatch, error) {
	before, after := pod.DeepCopyObject(), pod

	var proxyEnv = []corev1.EnvVar{{
		Name:  "HTTPS_PROXY",
		Value: os.Getenv("HTTPS_PROXY"),
	}, {
		Name:  "HTTP_PROXY",
		Value: os.Getenv("HTTP_PROXY"),
	}, {
		Name:  "NO_PROXY",
		Value: os.Getenv("NO_PROXY"),
	}}

	if after.Spec.Containers != nil {
		for i, container := range after.Spec.Containers {
			newEnvs := updateAndMergeEnv(container.Env, proxyEnv)
			after.Spec.Containers[i].Env = newEnvs
		}
	}

	exist, err := checkConfigMapExist(client, ctx, after.Namespace, trustedCAConfigMapName)
	if err != nil {
		return nil, err
	}
	if exist {
		after = updateVolume(after, trustedCAConfigMapVolume, trustedCAConfigMapName, trustedCAKey)
	}

	exist, err = checkConfigMapExist(client, ctx, after.Namespace, serviceCAConfigMapName)
	if err != nil {
		return nil, err
	}
	if exist {
		after = updateVolume(after, serviceCAConfigMapVolume, serviceCAConfigMapName, serviceCAKey)
	}

	patch, err := duck.CreatePatch(before, after)
	if err != nil {
		return nil, err
	}

	return append(patches, patch...), nil
}

// Ensure Configmap exist or not
func checkConfigMapExist(client kubernetes.Interface, ctx context.Context, ns string, name string) (bool, error) {
	logger := logging.FromContext(ctx)
	logger.Info("finding configmap: %s/%s", ns, name)
	_, err := client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	}
	return true, nil
}

// update volume and volume mounts to mount the certs configmap
func updateVolume(pod corev1.Pod, volumeName, configmapName, key string) corev1.Pod {
	volumes := pod.Spec.Volumes

	for i, v := range volumes {
		if v.Name == volumeName {
			volumes = append(volumes[:i], volumes[i+1:]...)
			break
		}
	}

	// Let's add the trusted and service CA bundle ConfigMaps as a volume in
	// the PodSpec which will later be mounted to add certs in the pod.
	volumes = append(volumes,
		// Add trusted CA bundle
		corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configmapName},
					Items: []corev1.KeyToPath{
						{
							Key:  key,
							Path: key,
						},
					},
				},
			},
		},
	)
	pod.Spec.Volumes = volumes

	// Now that the injected certificates have been added as a volume, let's
	// mount them via volumeMounts in the containers
	for i, c := range pod.Spec.Containers {
		volumeMounts := c.VolumeMounts

		// If volume mounts for injected certificates already exist then remove them
		for i, vm := range volumeMounts {
			if vm.Name == volumeName {
				volumeMounts = append(volumeMounts[:i], volumeMounts[i+1:]...)
				break
			}
		}

		// We will mount the certs at this location so we don't override the existing certs
		sslCertDir := "/tekton-custom-certs"
		certEnvAvaiable := false

		for _, env := range c.Env {
			// If SSL_CERT_DIR env var already exists, then we don't mess with
			// it and simply carry it forward as it is
			if env.Name == "SSL_CERT_DIR" {
				sslCertDir = env.Value
				certEnvAvaiable = true
			}
		}

		if !certEnvAvaiable {
			// Here, we need to set the default value for SSL_CERT_DIR.
			// Keep in mind that if SSL_CERT_DIR is set, then it overrides the
			// system default, i.e. the system default directories will "NOT"
			// be scanned for certificates. This is risky and we don't want to
			// do this because users mount certificates at these locations or
			// build images with certificates "in" them and expect certificates
			// to get picked up, and rightfully so since this is the documented
			// way of achieving this.
			// So, let's keep the system wide default locations in place and
			// "append" our custom location to those.
			//
			// Copied from https://golang.org/src/crypto/x509/root_linux.go
			var certDirectories = []string{
				// Ordering is important here - we will be using the "first"
				// element in SSL_CERT_DIR to do the volume mounts.
				sslCertDir,                     // /tekton-custom-certs
				"/etc/ssl/certs",               // SLES10/SLES11, https://golang.org/issue/12139
				"/etc/pki/tls/certs",           // Fedora/RHEL
				"/system/etc/security/cacerts", // Android
			}

			// SSL_CERT_DIR accepts a colon separated list of directories
			sslCertDir = strings.Join(certDirectories, ":")
			c.Env = append(c.Env, corev1.EnvVar{
				Name:  "SSL_CERT_DIR",
				Value: sslCertDir,
			})
		}

		// Let's mount the certificates now.
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name: volumeName,
				// We only want the first entry in SSL_CERT_DIR for the mount
				MountPath: filepath.Join(strings.Split(sslCertDir, ":")[0], key),
				SubPath:   key,
				ReadOnly:  true,
			},
		)
		c.VolumeMounts = volumeMounts
		pod.Spec.Containers[i] = c
	}
	return pod
}

// updateAndMergeEnv will merge two slices of env
// precedence will be given to second input if exist with same name key
func updateAndMergeEnv(containerenvs []corev1.EnvVar, proxyEnv []corev1.EnvVar) []corev1.EnvVar {
	containerEnv := map[string]string{}

	for _, env := range containerenvs {
		containerEnv[env.Name] = env.Value
	}
	for _, env := range proxyEnv {
		var updated bool
		if _, ok := containerEnv[env.Name]; ok {
			// If proxy set at global level and pipelinerun/taskrun level are same
			// then priority will be given to pipelinerun/taskrun.
			updated = true
		} else {
			if env.Value != "" {
				updated = false
			} else {
				updated = true
			}
		}
		if !updated {
			containerenvs = append(containerenvs, corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			})
		}
	}
	return containerenvs
}
