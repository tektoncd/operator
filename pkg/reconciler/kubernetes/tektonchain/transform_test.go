/*
Copyright 2025 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestUpdateStatefulSetOrdinalsForChains(t *testing.T) {
	desiredReplicas := int32(3)
	cr := &v1alpha1.TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chains",
			Namespace: "tekton-pipelines",
		},
		Spec: v1alpha1.TektonChainSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-chains",
			},
			Chain: v1alpha1.Chain{
				ChainProperties: v1alpha1.ChainProperties{
					Performance: v1alpha1.PerformanceProperties{
						PerformanceStatefulsetOrdinalsConfig: v1alpha1.PerformanceStatefulsetOrdinalsConfig{
							StatefulsetOrdinals: ptr.Bool(true),
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	manifest, err := common.Fetch("../../common/testdata/test-convert-chain-deployment-to-statefulset.yaml")
	if err != nil {
		t.Fatalf("Failed to fetch test data: %v", err)
	}

	extension := common.NoExtension(ctx)
	transformers := filterAndTransform(extension)
	result, err := transformers(ctx, &manifest, cr)
	if err != nil {
		t.Fatalf("Error applying transformers: %v", err)
	}

	foundStatefulSet := false
	for _, resource := range result.Resources() {
		if resource.GetKind() == "StatefulSet" && resource.GetName() == tektonChainsControllerName {
			foundStatefulSet = true

			sts := &appsv1.StatefulSet{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, sts); err != nil {
				t.Fatalf("Failed to convert resource to StatefulSet: %v", err)
			}

			if sts.Spec.Replicas == nil || *sts.Spec.Replicas != desiredReplicas {
				t.Errorf("Expected StatefulSet replicas to be %d, got %v", desiredReplicas, sts.Spec.Replicas)
			}

			if sts.Spec.ServiceName != tektonChainsServiceName {
				t.Errorf("Expected StatefulSet serviceName to be %s, got %s", tektonChainsServiceName, sts.Spec.ServiceName)
			}

			foundOrdinalEnv := false
			foundServiceEnv := false
			if len(sts.Spec.Template.Spec.Containers) > 0 {
				for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
					if env.Name == tektonChainsControllerStatefulControllerOrdinal {
						foundOrdinalEnv = true
					}
					if env.Name == tektonChainsControllerStatefulServiceName {
						foundServiceEnv = true
					}
				}
			}

			if !foundOrdinalEnv {
				t.Errorf("Expected to find environment variable %s", tektonChainsControllerStatefulControllerOrdinal)
			}

			if !foundServiceEnv {
				t.Errorf("Expected to find environment variable %s", tektonChainsControllerStatefulServiceName)
			}

			break
		}
	}

	if !foundStatefulSet {
		t.Error("Expected to find a StatefulSet in the transformed manifest, but none was found")
	}

	for _, resource := range result.Resources() {
		if resource.GetKind() == "Deployment" && resource.GetName() == tektonChainsControllerName {
			t.Error("Expected Deployment to be removed from manifest")
			break
		}
	}
}
