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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var triggerResourceLS = metav1.LabelSelector{
	MatchLabels: map[string]string{
		v1alpha1.InstallerSetType: TriggersResourcesInstallerSet,
	},
}

func (r *Reconciler) EnsureTriggersResources(ctx context.Context, ta *v1alpha1.TektonAddon) error {

	triggerResourceLabelSelector, err := common.LabelSelector(triggerResourceLS)
	if err != nil {
		return err
	}
	exist, _, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.operatorVersion, triggerResourceLabelSelector)
	if err != nil {
		return err
	}
	if !exist {
		msg := fmt.Sprintf("%s being created/upgraded", TriggersResourcesInstallerSet)
		ta.Status.MarkInstallerSetNotReady(msg)
		return r.ensureTriggerResources(ctx, ta)
	}

	err = r.checkComponentStatus(ctx, triggerResourceLabelSelector)
	if err != nil {
		ta.Status.MarkInstallerSetNotReady(err.Error())
		return nil
	}

	return nil
}

func (r *Reconciler) ensureTriggerResources(ctx context.Context, ta *v1alpha1.TektonAddon) error {
	triggerResourcesManifest := mf.Manifest{}

	if err := applyAddons(&triggerResourcesManifest, "01-clustertriggerbindings"); err != nil {
		return err
	}
	// Run transformers
	if err := r.addonTransform(ctx, &triggerResourcesManifest, ta); err != nil {
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, ta, triggerResourcesManifest, r.operatorVersion,
		TriggersResourcesInstallerSet, "addon-triggers"); err != nil {
		return err
	}

	return nil
}
