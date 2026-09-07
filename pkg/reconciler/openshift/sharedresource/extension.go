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

package sharedresource

import (
	"context"

	"github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	openshiftcommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

type openshiftExtension struct {
	kubeClientset     kubernetes.Interface
	operatorClientset versioned.Interface
}

// NewOpenShiftExtension builds the OpenShift extension for the Shared Resource component
func NewOpenShiftExtension(ctx context.Context) common.Extension {
	return &openshiftExtension{
		kubeClientset:     kubeclient.Get(ctx),
		operatorClientset: operatorclient.Get(ctx),
	}
}

func (oe *openshiftExtension) Transformers(tc v1alpha1.TektonComponent) []manifestival.Transformer {
	// Let the OpenShift restricted SCC controller assign runAsUser/runAsGroup so
	// the workloads pass restricted SCC admission.
	return []manifestival.Transformer{
		openshiftcommon.RemoveRunAsUser(),
		openshiftcommon.RemoveRunAsGroup(),
	}
}

func (oe *openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) GetPlatformData() string {
	return ""
}
