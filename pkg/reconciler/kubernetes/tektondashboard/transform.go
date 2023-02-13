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

package tektondashboard

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

const (
	externalLogsArg = "--external-logs="
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		dashboard := comp.(*v1alpha1.TektonDashboard)
		trns := extension.Transformers(dashboard)
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdDashboard),
			common.AddConfiguration(dashboard.Spec.Config),
			common.AddDeploymentRestrictedPSA(),
		}
		trns = append(trns, extra...)
		if dashboard.Spec.ExternalLogs != "" {
			updatedExternalLogsArg := externalLogsArg + dashboard.Spec.ExternalLogs
			trns = append(trns, common.ReplaceDeploymentArg("tekton-dashboard", externalLogsArg, updatedExternalLogsArg))
		}
		if err := common.Transform(ctx, manifest, dashboard, trns...); err != nil {
			return nil, err
		}
		return manifest, nil
	}
}
