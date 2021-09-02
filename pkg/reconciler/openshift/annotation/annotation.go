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

package annotation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/markbates/inflect"
	"github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	admissionlisters "k8s.io/client-go/listers/admissionregistration/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmp"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
	certresources "knative.dev/pkg/webhook/certificates/resources"
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

	pluralEL := strings.ToLower(inflect.Pluralize("EventListener"))
	rules := []admissionregistrationv1.RuleWithOperations{
		{
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
			},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"triggers.tekton.dev"},
				APIVersions: []string{"v1alpha1", "v1beta1"},
				Resources:   []string{pluralEL, pluralEL + "/status"},
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
				Key:      "operator.tekton.dev/enable-annotation",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"enabled"},
			}, {
				// "control-plane" is added to support Azure's AKS, otherwise the controllers fight.
				// See knative/pkg#1590 for details.
				Key:      "control-plane",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}},
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
	if gvk.Group != "triggers.tekton.dev" || !(gvk.Version == "v1alpha1" || gvk.Version == "v1beta1") || gvk.Kind != "EventListener" {
		logger.Error("Unhandled kind: ", gvk)
		return nil, fmt.Errorf("unhandled kind: %v", gvk)
	}

	// nil values denote absence of `old` (create) or `new` (delete) objects.
	var oldObj, newObj v1beta1.EventListener

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
	if patches, err = setDefaults(ctx, patches, newObj); err != nil {
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
func setDefaults(ctx context.Context, patches duck.JSONPatch, el v1beta1.EventListener) (duck.JSONPatch, error) {
	before, after := el.DeepCopyObject(), el

	secretName := "el-" + el.Name
	after.Annotations = map[string]string{
		"service.beta.openshift.io/serving-cert-secret-name": secretName,
	}
	if after.Spec.Resources.KubernetesResource == nil {
		after.Spec.Resources.KubernetesResource = &v1beta1.KubernetesResource{}
		after.Spec.Resources.KubernetesResource = &v1beta1.KubernetesResource{
			WithPodSpec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: getContainers(secretName),
					},
				},
			},
		}
	}
	envNames := map[string]string{}
	if after.Spec.Resources.KubernetesResource != nil {
		if len(after.Spec.Resources.KubernetesResource.Template.Spec.Containers) == 0 {
			after.Spec.Resources.KubernetesResource.Template.Spec.Containers = getContainers(secretName)
		} else {
			for i := range after.Spec.Resources.KubernetesResource.Template.Spec.Containers {
				if len(after.Spec.Resources.KubernetesResource.Template.Spec.Containers[i].Env) == 0 {
					after.Spec.Resources.KubernetesResource.Template.Spec.Containers[i].Env = getEnv(secretName)
				} else {
					for _, v := range after.Spec.Resources.KubernetesResource.Template.Spec.Containers[i].Env {
						envNames[v.Name] = v.Name
					}
					if envNames["TLS_CERT"] != "TLS_CERT" && envNames["TLS_KEY"] != "TLS_KEY" {
						after.Spec.Resources.KubernetesResource.Template.Spec.Containers[i].Env = getEnv(secretName)
					}
				}
			}
		}
	}

	patch, err := duck.CreatePatch(before, after)
	if err != nil {
		return nil, err
	}

	return append(patches, patch...), nil
}

func getContainers(secretName string) []corev1.Container {
	return []corev1.Container{{
		Env: getEnv(secretName),
	}}
}

func getEnv(secretName string) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name: "TLS_CERT",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: "tls.crt",
			},
		},
	}, {
		Name: "TLS_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: "tls.key",
			},
		},
	}}
}
