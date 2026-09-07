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

package shipwrightbuild

import (
	"context"
	"fmt"
	"slices"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CertManagerInjectCABundleAnnotation tells cert-manager's cainjector which
// Certificate's CA to inject into a resource's caBundle field.
const CertManagerInjectCABundleAnnotation = "cert-manager.io/inject-ca-from"

// FilterAndTransform installs the Builds manifests into the openshift-builds
// namespace. The upstream operator applies the same InjectNamespace transform;
// removeRunAsUserRunAsGroup mirrors its behavior of letting the OpenShift SCC
// controller assign the runAsUser/runAsGroup so the pods pass the restricted
// SCC admission.
func FilterAndTransform(reconciler *Reconciler) client.FilterAndTransform {
	return func(ctx context.Context, manifest *manifestival.Manifest, component v1alpha1.TektonComponent) (*manifestival.Manifest, error) {
		sb := component.(*v1alpha1.ShipwrightBuild)
		// Drop the Namespace from the manifest; the target namespace is managed elsewhere.
		*manifest = manifest.Filter(manifestival.Not(manifestival.ByKind("Namespace")))
		// Tell the webhook which version was installed so it reports it accurately.
		webhookArgs := []string{WebhookVersionFlag, reconciler.componentVersion}
		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ShipwrightBuildImagePrefix))

		transformers := []manifestival.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(OperandName),
			common.DeploymentImages(images),
			common.EnvImages(images),
			common.AddConfiguration(sb.Spec.Config),
			common.AddContainerArgs(WebhookDeploymentName, WebhookContainerName, webhookArgs, false),
		}

		transformers = append(transformers, reconciler.extension.Transformers(sb)...)
		err := common.Transform(ctx, manifest, sb, transformers...)
		if err != nil {
			return nil, err
		}

		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, sb.Spec.GetTargetNamespace(), sb.Spec.Options); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}

// InjectCertManagerCABundle returns a Manifestival transformer that annotates
// resources so the cert-manager injects the CA bundle, from the provided secret
// Pass resources as nil or empty array to transform all resource of a Kind.
func InjectCertManagerCABundle(kind, secretName, namespace string, resources []string) manifestival.Transformer {
	injectFrom := fmt.Sprintf("%s/%s", namespace, secretName)
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != kind {
			return nil
		}
		if len(resources) != 0 && !slices.Contains(resources, u.GetName()) {
			return nil
		}
		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[CertManagerInjectCABundleAnnotation] = injectFrom
		u.SetAnnotations(annotations)
		return nil
	}
}

// NopFilterAndTransform applies the manifest as-is. It is used for the build
// strategies post set, which needs no namespace or image rewriting.
func NopFilterAndTransform() client.FilterAndTransform {
	return func(ctx context.Context, manifest *manifestival.Manifest, component v1alpha1.TektonComponent) (*manifestival.Manifest, error) {
		return manifest, nil
	}
}
