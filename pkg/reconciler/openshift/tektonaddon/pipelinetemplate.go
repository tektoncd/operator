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
	"fmt"
	"os"
	"path/filepath"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	tektonaddon "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon/pipelinetemplates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var pipelineTemplateLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: PipelinesTemplateInstallerSet,
	},
}

func (r *Reconciler) EnsurePipelineTemplates(ctx context.Context, enable string, ta *v1alpha1.TektonAddon) error {

	pipelineTemplateLSLabelSelector, err := common.LabelSelector(pipelineTemplateLS)
	if err != nil {
		return err
	}
	if enable == "true" {

		exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, pipelineTemplateLSLabelSelector)
		if err != nil {
			return err
		}
		if !exist {
			msg := fmt.Sprintf("%s being created/upgraded", PipelinesTemplateInstallerSet)
			ta.Status.MarkInstallerSetNotReady(msg)
			return r.ensurePipelineTemplates(ctx, ta)
		}

		if err := r.checkComponentStatus(ctx, pipelineTemplateLSLabelSelector); err != nil {
			ta.Status.MarkInstallerSetNotReady(err.Error())
			return nil
		}

	} else {
		// if disabled then delete the installer Set if exist
		if err := r.deleteInstallerSet(ctx, pipelineTemplateLSLabelSelector); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) ensurePipelineTemplates(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	pipelineTemplateManifest := mf.Manifest{}

	// Read pipeline template manifest from kodata
	if err := applyAddons(&pipelineTemplateManifest, "03-pipelines"); err != nil {
		return err
	}

	// generate pipeline templates
	if err := addPipelineTemplates(&pipelineTemplateManifest); err != nil {
		return err
	}

	// Run transformers
	if err := r.addonTransform(ctx, &pipelineTemplateManifest, ta); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, pipelineTemplateManifest, r.operatorVersion,
		PipelinesTemplateInstallerSet, "addon-pipelines"); err != nil {
		return err
	}

	return nil
}

func addPipelineTemplates(manifest *mf.Manifest) error {
	koDataDir := os.Getenv(common.KoEnvKey)
	addonLocation := filepath.Join(koDataDir, "tekton-addon", "tekton-pipeline-template")
	return tektonaddon.GeneratePipelineTemplates(addonLocation, manifest)
}
