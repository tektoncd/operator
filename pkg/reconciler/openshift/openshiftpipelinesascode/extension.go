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
	"os"
	"path/filepath"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	openshiftNS = "openshift"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	pacManifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient))
	if err != nil {
		logger.Fatalw("Error creating initial manifest", zap.Error(err))
	}

	pacLocation := filepath.Join(os.Getenv(common.KoEnvKey), "tekton-addon", "pipelines-as-code")
	if err := common.AppendManifest(&pacManifest, pacLocation); err != nil {
		logger.Fatalf("failed to fetch PAC manifest: %v", err)
	}

	prTemplates, err := fetchPipelineRunTemplates()
	if err != nil {
		logger.Fatalf("failed to fetch pipelineRun templates: %v", err)
	}

	operatorVer, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
	return openshiftExtension{
		// component version is used for metrics, passing a dummy
		// value through extension not going to affect execution
		installerSetClient:   client.NewInstallerSetClient(tisClient, operatorVer, "pipelines-as-code-ext", v1alpha1.KindOpenShiftPipelinesAsCode, nil),
		pacManifest:          &pacManifest,
		pipelineRunTemplates: prTemplates,
		kubeClientSet:        kubeclient.Get(ctx),
	}
}

type openshiftExtension struct {
	installerSetClient   *client.InstallerSetClient
	pacManifest          *mf.Manifest
	pipelineRunTemplates *mf.Manifest
	kubeClientSet        kubernetes.Interface
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		injectNamespaceOwnerForPACWebhook(oe.kubeClientSet, comp.GetSpec().GetTargetNamespace()),
	}
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

	if err := updateControllerRouteInConfigMap(oe.pacManifest, comp.GetSpec().GetTargetNamespace()); err != nil {
		logger.Error("failed to update controller route: ", err)
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

// injectNamespaceOwnerForPACWebhook adds namespace ownerReference to PAC webhook
// to ensure proper cleanup when namespace is deleted (SRVKP-8901)
func injectNamespaceOwnerForPACWebhook(kubeClient kubernetes.Interface, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := u.GetKind()
		name := u.GetName()

		// Only apply to PAC ValidatingWebhookConfiguration
		if kind != "ValidatingWebhookConfiguration" || name != "validation.pipelinesascode.tekton.dev" {
			return nil
		}

		// Get target namespace (where PAC webhook is deployed)
		ns, err := kubeClient.CoreV1().Namespaces().Get(context.TODO(), targetNamespace, metav1.GetOptions{})
		if err != nil {
			// Log but don't fail - webhook will work without ownerRef
			return nil
		}

		// Set namespace as owner
		// Note: BlockOwnerDeletion and Controller flags are omitted as they require additional RBAC
		// permissions to set finalizers on namespaces, which is a security concern.
		u.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       ns.Name,
				UID:        ns.UID,
			},
		})

		return nil
	}
}
