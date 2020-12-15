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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonconfig/extension"
)

func KubernetesExtension(ctx context.Context) common.Extension {
	return kubernetesExtension{
		operatorClientSet: operatorclient.Get(ctx),
	}
}

type kubernetesExtension struct {
	operatorClientSet versioned.Interface
}

func (oe kubernetesExtension) Append(ctx context.Context, m *mf.Manifest) error {
	return nil
}
func (oe kubernetesExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (oe kubernetesExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe kubernetesExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		return extension.CreateDashboardCR(comp, oe.operatorClientSet.OperatorV1alpha1())
	}
	return nil
}
func (oe kubernetesExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == common.ProfileAll {
		return extension.TektonDashboardCRDelete(oe.operatorClientSet.OperatorV1alpha1().TektonDashboards(), common.DashboardResourceName)
	}
	return nil
}
