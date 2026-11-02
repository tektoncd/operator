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

package tektonkueue

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/logging"
)

const multiKueuePluginSet = "multikueue-plugin"

func (oe openshiftExtension) reconcilePlugin(ctx context.Context, kueue *v1alpha1.TektonKueue) error {
	// These APIs are only required for ACM-managed worker registration.
	for _, gv := range []string{
		"cluster.open-cluster-management.io/v1",
		"authentication.open-cluster-management.io/v1beta1",
		"work.open-cluster-management.io/v1",
		"operators.coreos.com/v1",
		"operators.coreos.com/v1alpha1",
		"kueue.openshift.io/v1",
		"kueue.x-k8s.io/v1beta2",
	} {
		if _, err := oe.discovery.ServerResourcesForGroupVersion(gv); err != nil {
			kueue.Status.MarkPostReconcilerFailed(fmt.Sprintf("ACM plugin requires API %s: %v", gv, err))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}
	if err := oe.installerSetClient.CustomSet(ctx, kueue, multiKueuePluginSet, &oe.manifest, transformPlugin, nil); err != nil {
		kueue.Status.MarkPostReconcilerFailed(fmt.Sprintf("ACM plugin installation is not ready: %v", err))
		return err
	}
	return nil
}

// cleanupPlugin removes the plugin installer set. It requeues until the set is
// gone, and also when the removal fails, so that it is retried.
func (oe openshiftExtension) cleanupPlugin(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	if err := oe.installerSetClient.CleanupCustomSet(ctx, multiKueuePluginSet); err != nil {
		logger.Errorw("failed to remove ACM plugin installer set", "error", err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	selector := labels.Set{
		v1alpha1.CreatedByKey:     v1alpha1.KindTektonKueue,
		v1alpha1.InstallerSetType: "custom-" + multiKueuePluginSet,
	}.String()
	sets, err := oe.installerSetClient.ListCustomSet(ctx, selector)
	if err != nil {
		logger.Errorw("failed to list ACM plugin installer sets", "error", err)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	if len(sets.Items) != 0 {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return nil
}

func transformPlugin(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// This binding targets the external Kueue controller, not our operand namespace.
	updated, err := manifest.Filter(mf.Not(mf.ByKind("Namespace"))).Transform(func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ClusterRoleBinding" && u.GetName() == "pipelines-multikueue-plugin-kueue-rolebinding" {
			annotations := u.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[common.AnnotationPreserveNS] = "true"
			u.SetAnnotations(annotations)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	images := common.ImageRegistryDomainOverride(common.ToLowerCaseKeys(common.ImagesFromEnv(common.MultiKueuePluginImagePrefix)))
	if err := common.Transform(ctx, &updated, comp,
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.TektonKueueResourceName),
		common.DeploymentImages(images),
		common.DeploymentEnvVars([]corev1.EnvVar{{Name: "KUEUE_NAMESPACE", Value: openshift.KueueNameSpace}}),
		occommon.RemoveRunAsUser(), occommon.RemoveRunAsGroup(),
	); err != nil {
		return nil, err
	}
	kueue := comp.(*v1alpha1.TektonKueue)
	// ConfigMaps and HPAs from the shared options are owned by the main set.
	options := v1alpha1.AdditionalOptions{Disabled: kueue.Spec.Options.Disabled, Deployments: kueue.Spec.Options.Deployments}
	if err := common.ExecuteAdditionalOptionsTransformer(ctx, &updated, kueue.Spec.TargetNamespace, options); err != nil {
		return nil, err
	}
	return &updated, nil
}
