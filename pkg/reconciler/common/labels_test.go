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

package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLabelSelector(t *testing.T) {
	for _, c := range []struct {
		name string
		ls   metav1.LabelSelector
		want string
	}{{
		name: "empty label selector",
		ls:   metav1.LabelSelector{},
		want: "",
	}, {
		name: "non empty label selector",
		ls: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"installerSetType": "pipelineResourceName",
			},
		},
		want: "installerSetType=pipelineResourceName",
	}} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := LabelSelector(c.ls)
			if got != c.want {
				t.Errorf("LabelSelector:\n got %q\nwant %q", got, c.want)
			}
		})
	}
}
