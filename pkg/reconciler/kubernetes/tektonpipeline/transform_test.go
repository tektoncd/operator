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

package tektonpipeline

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestUpdatePerformanceFlagsInDeployment(t *testing.T) {
	pipelineCR := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline",
			Namespace: "xyz",
		},
	}
	buckets := uint(2)
	workers := int(3)
	burst := int(33)
	pipelineCR.Spec.Performance.Buckets = &buckets
	pipelineCR.Spec.Performance.DisableHA = true
	pipelineCR.Spec.Performance.KubeApiQPS = ptr.Float32(40.03)
	pipelineCR.Spec.Performance.KubeApiBurst = &burst
	pipelineCR.Spec.Performance.ThreadsPerController = &workers

	depInput := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelinesControllerDeployment,
			Namespace: "xyz",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "hello"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "hello"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "hello",
							Image: "xyz",
						},
						{
							Name:  pipelinesControllerContainer,
							Image: "xyz",
							Args:  []string{"-flag1", "v1", "-flag2", "v2", "-disable-ha"},
						},
					},
				},
			},
		},
	}

	// update expected output
	depExpected := depInput.DeepCopy()
	depExpected.Spec.Template.Labels = map[string]string{"app": "hello", "config-leader-election.data.buckets": "2"}
	// flags order is important
	depExpected.Spec.Template.Spec.Containers[1].Args = []string{
		"-flag1", "v1",
		"-flag2", "v2",
		"-disable-ha", "true",
		"-kube-api-burst", "33",
		"-kube-api-qps", "40.03",
		"-threads-per-controller", "3",
	}

	// convert to unstructured object
	jsonBytes, err := json.Marshal(&depInput)
	assert.NilError(t, err)
	ud := &unstructured.Unstructured{}
	err = json.Unmarshal(jsonBytes, ud)
	assert.NilError(t, err)

	// apply transformer
	transformer := updatePerformanceFlagsInDeployment(pipelineCR)
	err = transformer(ud)
	assert.NilError(t, err)

	// get transformed deployment
	outDep := &appsv1.Deployment{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, outDep)
	assert.NilError(t, err)

	assert.Equal(t, true, reflect.DeepEqual(outDep, depExpected), fmt.Sprintf("transformed output:[%+v], expected:[%+v]", outDep, depExpected))
}

func TestGetSortedKeys(t *testing.T) {
	in := map[string]interface{}{
		"a1":  1,
		"z1":  false,
		"a2":  2,
		"a3":  3,
		"a10": 10,
		"a11": 11,
	}
	expectedOut := []string{"a1", "a10", "a11", "a2", "a3", "z1"}

	out := getSortedKeys(in)
	assert.Equal(t, true, reflect.DeepEqual(out, expectedOut))
}
