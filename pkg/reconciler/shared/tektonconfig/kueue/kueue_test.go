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

package kueue

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestMultiClusterPluginPropagation(t *testing.T) {
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "pipelines"},
			Kueue: v1alpha1.Kueue{
				Disabled: ptr.To(false), MultiClusterPlugin: v1alpha1.MultiClusterPluginManual,
				MultiClusterConfig: v1alpha1.MultiClusterConfig{MultiClusterRole: v1alpha1.MultiClusterRoleHub},
			},
		},
	}
	old := GetTektonKueueCR(config, "v1")
	clients := fake.NewSimpleClientset(old).OperatorV1alpha1().TektonKueues()
	for _, plugin := range []v1alpha1.MultiClusterPlugin{v1alpha1.MultiClusterPluginACM, v1alpha1.MultiClusterPluginManual} {
		config.Spec.Kueue.MultiClusterPlugin = plugin
		desired := GetTektonKueueCR(config, "v1")
		assert.Equal(t, desired.Spec.MultiClusterPlugin, plugin)
		_, err := UpdateKueue(t.Context(), old, desired, clients)
		assert.Equal(t, err, v1alpha1.RECONCILE_AGAIN_ERR)
		old, err = clients.Get(t.Context(), v1alpha1.TektonKueueResourceName, metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, old.Spec.MultiClusterPlugin, plugin)
	}
}
