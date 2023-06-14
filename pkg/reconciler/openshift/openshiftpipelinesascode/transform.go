/*
Copyright 2022 The Tekton Authors

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

package openshiftpipelinesascode

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
)

const pipelinesAsCodeCM = "pipelines-as-code"

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		pac := comp.(*v1alpha1.OpenShiftPipelinesAsCode)
		// installerSet adds it's owner as namespace's owner
		// so deleting tekton addon deletes target namespace too
		// to skip it we filter out namespace
		pacManifest := manifest.Filter(mf.Not(mf.ByKind("Namespace")))

		images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.PacImagePrefix))
		// Run transformers
		tfs := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(openshift.OperandOpenShiftPipelineAsCode),
			common.DeploymentImages(images),
			common.AddConfiguration(pac.Spec.Config),
			occommon.ApplyCABundles,
			common.CopyConfigMapWithForceUpdate(pipelinesAsCodeCM, pac.Spec.Settings, true),
			occommon.UpdateServiceMonitorTargetNamespace(pac.Spec.TargetNamespace),
		}

		allTfs := append(tfs, extension.Transformers(pac)...)
		if err := common.Transform(ctx, &pacManifest, pac, allTfs...); err != nil {
			return nil, err
		}
		return &pacManifest, nil
	}
}
