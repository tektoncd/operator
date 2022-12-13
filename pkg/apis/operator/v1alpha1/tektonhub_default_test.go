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

package v1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetDefault(t *testing.T) {

	t.Setenv("DEFAULT_TARGET_NAMESPACE", "tekton-pipelines")
	th := &TektonHub{
		ObjectMeta: metav1.ObjectMeta{
			Name: "hub",
		},
		Spec: TektonHubSpec{
			Api: ApiSpec{
				ApiSecretName: "tetkon-hub-api",
			},
		},
	}
	th.SetDefaults(context.TODO())
	if th.Spec.TargetNamespace != "tekton-pipelines" {
		t.Error("Setting default failed for TektonHub (spec.targetNamespace)")
	}
}
