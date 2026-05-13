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
	tektonConfiginformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	openshiftNS                = "openshift"
	pacControllerDeployment    = "pipelines-as-code-controller"
	pacControllerContainerName = "pac-controller"
	pacWatcherDeployment       = "pipelines-as-code-watcher"
	pacWatcherContainerName    = "pac-watcher"
	pacWebhookDeployment       = "pipelines-as-code-webhook"
	pacWebhookContainerName    = "pac-webhook"
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

	pacLocation := filepath.Join(os.Getenv(common.KoEnvKey), common.PipelinesAsCodeManifestDir)
	if err := common.AppendManifest(&pacManifest, pacLocation); err != nil {
		logger.Fatalf("failed to fetch PAC manifest: %v", err)
	}

	prTemplates, err := FetchPipelineRunTemplates()
	if err != nil {
		logger.Fatalf("failed to fetch pipelineRun templates: %v", err)
	}

	operatorVer, err := common.OperatorVersion(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	tisClient := operatorclient.Get(ctx).OperatorV1alpha1().TektonInstallerSets()
	return &openshiftExtension{
		// component version is used for metrics, passing a dummy
		// value through extension not going to affect execution
		installerSetClient:   client.NewInstallerSetClient(tisClient, operatorVer, "pipelines-as-code-ext", v1alpha1.KindOpenShiftPipelinesAsCode, nil),
		pacManifest:          &pacManifest,
		pipelineRunTemplates: prTemplates,
		kubeClientSet:        kubeclient.Get(ctx),
		tektonConfigLister:   tektonConfiginformer.Get(ctx).Lister(),
	}
}

type openshiftExtension struct {
	installerSetClient   *client.InstallerSetClient
	pacManifest          *mf.Manifest
	pipelineRunTemplates *mf.Manifest
	kubeClientSet        kubernetes.Interface
	tektonConfigLister   occommon.TektonConfigLister
	resolvedTLSConfig    *occommon.TLSEnvVars
}

func (oe *openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	trns := []mf.Transformer{
		InjectNamespaceOwnerForPACWebhook(oe.kubeClientSet, comp.GetSpec().GetTargetNamespace()),
	}

	// Inject APIServer TLS profile env vars into all three PAC deployments so that
	// they apply the cluster-wide TLS version and cipher suite policy (PQC readiness).
	if oe.resolvedTLSConfig != nil {
		trns = append(trns,
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", pacControllerDeployment, []string{pacControllerContainerName}),
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", pacWatcherDeployment, []string{pacWatcherContainerName}),
			occommon.InjectTLSEnvVars(oe.resolvedTLSConfig, "Deployment", pacWebhookDeployment, []string{pacWebhookContainerName}),
		)
	}

	return trns
}

func (oe *openshiftExtension) PreReconcile(ctx context.Context, _ v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)

	resolvedTLS, err := occommon.ResolveCentralTLSToEnvVars(ctx, oe.tektonConfigLister)
	if err != nil {
		return err
	}
	oe.resolvedTLSConfig = resolvedTLS
	if oe.resolvedTLSConfig != nil {
		logger.Infof("Injecting central TLS config into PAC deployments: MinVersion=%s", oe.resolvedTLSConfig.MinVersion)
	}

	return nil
}

func (oe *openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
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

func (oe *openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe *openshiftExtension) GetPlatformData() string {
	return ""
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

// InjectNamespaceOwnerForPACWebhook adds namespace ownerReference to PAC webhook
// to ensure proper cleanup when namespace is deleted (SRVKP-8901)
func InjectNamespaceOwnerForPACWebhook(kubeClient kubernetes.Interface, targetNamespace string) mf.Transformer {
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
