/*
Copyright 2020 The Tekton Authors

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

package tektonconfig

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonconfig/extension"
	pac "github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/pipelinesascode"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig/shipwrightbuild"
)

func KubernetesExtension(ctx context.Context) common.Extension {
	return kubernetesExtension{
		operatorClientSet: operatorclient.Get(ctx),
	}
}

type kubernetesExtension struct {
	operatorClientSet versioned.Interface
}

func (ke kubernetesExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (ke kubernetesExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (ke kubernetesExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)

	if configInstance.Spec.Profile == v1alpha1.ProfileAll {
		if _, err := extension.EnsureTektonDashboardExists(ctx, ke.operatorClientSet.OperatorV1alpha1().TektonDashboards(), configInstance); err != nil {
			configInstance.Status.MarkPostInstallFailed(fmt.Sprintf("TektonDashboard: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	if configInstance.Spec.Profile == v1alpha1.ProfileLite || configInstance.Spec.Profile == v1alpha1.ProfileBasic {
		return extension.EnsureTektonDashboardCRNotExists(ctx, ke.operatorClientSet.OperatorV1alpha1().TektonDashboards())
	}

	pacSpec := configInstance.Spec.PipelinesAsCodeForCurrentPlatform()
	if pacSpec != nil && pacSpec.Enable != nil && *pacSpec.Enable {
		if _, err := pac.EnsureOpenShiftPipelinesAsCodeExists(ctx, ke.operatorClientSet.OperatorV1alpha1().OpenShiftPipelinesAsCodes(), configInstance, configInstance.Status.Version, ""); err != nil {
			configInstance.Status.MarkComponentNotReady(fmt.Sprintf("OpenShiftPipelinesAsCode: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	} else {
		if err := pac.EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, ke.operatorClientSet.OperatorV1alpha1().OpenShiftPipelinesAsCodes()); err != nil {
			return err
		}
	}

	// Ensure Shipwright Build CR
	if shipwrightbuild.IsEnabled(configInstance) {
		if _, err := shipwrightbuild.CreateOrUpdate(ctx, ke.operatorClientSet.OperatorV1alpha1().ShipwrightBuilds(), configInstance, configInstance.Status.Version); err != nil {
			msg := fmt.Sprintf("ShipwrightBuild: %s", err.Error())
			configInstance.Status.MarkComponentNotReady(msg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	} else {
		if err := shipwrightbuild.DeleteIfExists(ctx, ke.operatorClientSet.OperatorV1alpha1().ShipwrightBuilds()); err != nil {
			msg := fmt.Sprintf("ShipwrightBuild: %s", err.Error())
			configInstance.Status.MarkComponentNotReady(msg)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	return nil
}
func (ke kubernetesExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == v1alpha1.ProfileAll {
		return extension.EnsureTektonDashboardCRNotExists(ctx, ke.operatorClientSet.OperatorV1alpha1().TektonDashboards())
	}

	pacSpec := configInstance.Spec.PipelinesAsCodeForCurrentPlatform()
	if pacSpec != nil && pacSpec.Enable != nil && *pacSpec.Enable {
		if err := pac.EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx, ke.operatorClientSet.OperatorV1alpha1().OpenShiftPipelinesAsCodes()); err != nil {
			return err
		}
	}

	if shipwrightbuild.IsEnabled(configInstance) {
		if err := shipwrightbuild.DeleteIfExists(ctx, ke.operatorClientSet.OperatorV1alpha1().ShipwrightBuilds()); err != nil {
			return err
		}
	}

	return nil
}

func (ke kubernetesExtension) GetPlatformData() string {
	return ""
}
