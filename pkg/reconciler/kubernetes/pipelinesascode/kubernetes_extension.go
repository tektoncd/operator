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

package pipelinesascode

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	pacctrl "github.com/tektoncd/operator/pkg/reconciler/openshift/openshiftpipelinesascode"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"
)

// kubernetesExtension implements common.Extension for the Kubernetes operator: PipelineRun
// template PostSet  and webhook owner ref. It does not load the full PAC
// manifest or sync OpenShift Routes (see openshift/openshiftpipelinesascode/extension.go).
type kubernetesExtension struct {
	installerSetClient   *client.InstallerSetClient
	pipelineRunTemplates *mf.Manifest
	kubeClientSet        kubernetes.Interface
}

func (e kubernetesExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		pacctrl.InjectNamespaceOwnerForPACWebhook(e.kubeClientSet, comp.GetSpec().GetTargetNamespace()),
	}
}

func (e kubernetesExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (e kubernetesExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	if err := e.installerSetClient.PostSet(ctx, comp, e.pipelineRunTemplates, kubernetesTemplateFilter()); err != nil {
		logger.Error("failed post set creation: ", err)
		return err
	}
	return nil
}

func (e kubernetesExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (e kubernetesExtension) GetPlatformData() string {
	return ""
}

// kubernetesTemplateFilter applies templates into the component target namespace (unlike the
// OpenShift extension, which uses the openshift namespace for templates).
func kubernetesTemplateFilter() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		ns := comp.GetSpec().GetTargetNamespace()
		prTemplates, err := manifest.Transform(mf.InjectNamespace(ns))
		if err != nil {
			return nil, err
		}
		return &prTemplates, nil
	}
}

// NewKubernetesExtension builds the extension passed to pacctrl.NewExtendedController
// (shared controller wiring for the OpenShiftPipelinesAsCode CRD).
func NewKubernetesExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	operatorVer, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	prTemplates, err := pacctrl.FetchPipelineRunTemplates()
	if err != nil {
		logger.Fatalf("failed to fetch pipelineRun templates: %v", err)
	}

	tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
	return kubernetesExtension{
		installerSetClient:   client.NewInstallerSetClient(tisClient, operatorVer, "pipelines-as-code-ext", v1alpha1.KindOpenShiftPipelinesAsCode, nil),
		pipelineRunTemplates: prTemplates,
		kubeClientSet:        kubeclient.Get(ctx),
	}
}
