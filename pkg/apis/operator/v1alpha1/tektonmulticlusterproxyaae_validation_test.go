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

package v1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ValidateTektonMulticlusterProxyAAE(t *testing.T) {
	tests := []struct {
		name    string
		proxy   *TektonMulticlusterProxyAAE
		wantErr bool
	}{
		{
			name: "valid proxy",
			proxy: &TektonMulticlusterProxyAAE{
				ObjectMeta: metav1.ObjectMeta{
					Name: MultiClusterProxyAAEResourceName,
				},
				Spec: TektonMulticlusterProxyAAESpec{
					CommonSpec: CommonSpec{
						TargetNamespace: "tekton-pipelines",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.proxy.Validate(context.TODO())
			gotErr := errs != nil
			if gotErr != tc.wantErr {
				t.Errorf("Validate() wantErr=%v, got err=%v", tc.wantErr, errs)
			}
		})
	}
}
