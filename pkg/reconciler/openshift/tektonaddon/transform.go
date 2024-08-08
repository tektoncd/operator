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

package tektonaddon

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
)

func transformers(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent, addnTfs ...mf.Transformer) error {
	instance := comp.(*v1alpha1.TektonAddon)
	addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))

	addonTfs := []mf.Transformer{
		// using common.InjectOperandNameLabelPreserveExisting instead of common.InjectLabelOverwriteExisting
		// to highlight that TektonAddon is a basket of various operands(components)
		// note: using common.InjectLabelOverwriteExisting here  doesnot affect the ability to
		// use InjectOperandNameLabelPreserveExisting or InjectLabelOverwriteExisting again in the transformer chain
		// However, it is recomended to use InjectOperandNameLabelPreserveExisting here (in Addons) as we cannot be sure
		// about order of future addition of transformers in this reconciler or in sub functions which take care of various addons
		common.InjectOperandNameLabelPreserveExisting(openshift.OperandOpenShiftPipelinesAddons),
		injectLabel(labelProviderType, providerTypeRedHat, overwrite, "ClusterTask"),
		common.TaskImages(ctx, addonImages),
	}
	addonTfs = append(addonTfs, addnTfs...)
	return common.Transform(ctx, manifest, instance, addonTfs...)
}
