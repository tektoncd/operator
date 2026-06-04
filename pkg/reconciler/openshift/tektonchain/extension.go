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

package tektonchain

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	tektonChainsControllerName = "tekton-chains-controller"
	// tektonChainsMetricsService is the Service that exposes the metrics service
	// for Tekton Chains; the serving-cert Secret is named after this Service.
	tektonChainsMetricsService = "tekton-chains-metrics"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		occommon.RemoveRunAsUser(),
		occommon.RemoveRunAsGroup(),
		occommon.RemoveRunAsUserForStatefulSet(tektonChainsControllerName),
		occommon.RemoveRunAsGroupForStatefulSet(tektonChainsControllerName),
		occommon.ApplyCABundlesToDeployment,
		occommon.ApplyCABundlesForStatefulSet(tektonChainsControllerName),
		// mTLS for Prometheus scraping.
		// The metrics Service for chains is "tekton-chains-metrics" (distinct from the
		// controller Deployment name), so the Secret name is derived from the Service.
		occommon.AnnotateMetricsServingCert(tektonChainsMetricsService),
		occommon.RenameServicePort(tektonChainsMetricsService, occommon.MetricsHTTPPort, occommon.MetricsHTTPSPort),
		// Cover both Deployment and StatefulSet: chains uses RemoveRunAsUserForStatefulSet
		// and ApplyCABundlesForStatefulSet, so it can be either kind across releases.
		occommon.ApplyMetricsTLS("Deployment", tektonChainsControllerName,
			occommon.MetricsServingCertSecretName(tektonChainsMetricsService)),
		occommon.ApplyMetricsTLS("StatefulSet", tektonChainsControllerName,
			occommon.MetricsServingCertSecretName(tektonChainsMetricsService)),
	}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func (oe openshiftExtension) GetPlatformData() string {
	return ""
}
