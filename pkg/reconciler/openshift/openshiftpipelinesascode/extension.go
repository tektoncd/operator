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
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"knative.dev/pkg/logging"
)

const (
	openshiftNS = "openshift"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	prTemplates, err := fetchPipelineRunTemplates()
	if err != nil {
		logger.Fatalf("failed to fetch pipelineRun templates: %v", err)
	}

	operatorVer, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	pacVersion := ctx.Value("pipelines-as-code-version").(string)

	tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
	return openshiftExtension{
		installerSetClient:   client.NewInstallerSetClient(tisClient, operatorVer, pacVersion, v1alpha1.KindOpenShiftPipelinesAsCode, nil),
		pipelineRunTemplates: prTemplates,
	}
}

type openshiftExtension struct {
	installerSetClient   *client.InstallerSetClient
	pipelineRunTemplates *mf.Manifest
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return nil
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)

	if err := oe.installerSetClient.PostSet(ctx, comp, oe.pipelineRunTemplates, extFilterAndTransform()); err != nil {
		logger.Error("failed post set creation: ", err)
		return err
	}
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func extFilterAndTransform() client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		prTemplates, err := manifest.Transform(mf.InjectNamespace(openshiftNS))
		if err != nil {
			return nil, err
		}
		return &prTemplates, nil
	}
}
