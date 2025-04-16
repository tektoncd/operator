/*
Copyright 2023 The Tekton Authors

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
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
)

const (
	leaderElectionChainConfig                       = "tekton-chains-config-leader-election"
	chainControllerDeployment                       = "tekton-chains-controller"
	chainControllerContainer                        = "tekton-chains-controller"
	tektonChainsControllerName                      = "tekton-chains-controller"
	tektonChainsServiceName                         = "tekton-chains-controller"
	tektonChainsControllerStatefulServiceName       = "STATEFUL_SERVICE_NAME"
	tektonChainsControllerStatefulControllerOrdinal = "STATEFUL_CONTROLLER_ORDINAL"
)

func filterAndTransform(extension common.Extension) client.FilterAndTransform {
	return func(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) (*mf.Manifest, error) {
		chainCR := comp.(*v1alpha1.TektonChain)
		chainImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ChainsImagePrefix))
		extra := []mf.Transformer{
			common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdChains),
			common.DeploymentImages(chainImages),
			common.DeploymentEnvVarKubernetesMinVersion(),
			common.AddConfiguration(chainCR.Spec.Config),
			common.AddConfigMapValues(ChainsConfig, chainCR.Spec.Chain.ChainProperties),
			common.AddDeploymentRestrictedPSA(),
			AddControllerEnv(chainCR.Spec.Chain.ControllerEnvs),
			common.UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(&chainCR.Spec.Performance, leaderElectionChainConfig, chainControllerDeployment, chainControllerContainer),
		}
		if chainCR.Spec.GenerateSigningSecret {
			extra = append(extra, common.AddSecretData(generateSigningSecrets(ctx), map[string]string{
				secretTISSigningAnnotation: "true",
			}))
		}

		if chainCR.Spec.Performance.StatefulsetOrdinals != nil && *chainCR.Spec.Performance.StatefulsetOrdinals {
			extra = append(extra,
				common.ConvertDeploymentToStatefulSet(tektonChainsControllerName, tektonChainsServiceName),
				common.AddStatefulEnvVars(
					tektonChainsControllerName, tektonChainsServiceName, tektonChainsControllerStatefulServiceName, tektonChainsControllerStatefulControllerOrdinal))
		}

		extra = append(extra, extension.Transformers(chainCR)...)
		err := common.Transform(ctx, manifest, chainCR, extra...)
		if err != nil {
			return &mf.Manifest{}, err
		}
		// additional options transformer
		// always execute as last transformer, so that the values in options will be final update values on the manifests
		if err := common.ExecuteAdditionalOptionsTransformer(ctx, manifest, chainCR.Spec.GetTargetNamespace(), chainCR.Spec.Options); err != nil {
			return &mf.Manifest{}, err
		}

		return manifest, nil
	}
}
