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

package upgrade

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorFake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sFake "k8s.io/client-go/kubernetes/fake"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

func TestUpgradeChainProperties(t *testing.T) {
	ctx := context.TODO()
	operatorClient := operatorFake.NewSimpleClientset()
	k8sClient := k8sFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	// there is no tektonConfig CR available, returns error
	err := upgradeChainProperties(ctx, logger, k8sClient, operatorClient, nil)
	assert.Error(t, err)

	// create tekconConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "foo",
			},
			Chain: v1alpha1.Chain{
				ChainProperties: v1alpha1.ChainProperties{
					BuilderID: "bar",
				},
			},
		},
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// there is no chains-config configMap, return no error
	err = upgradeChainProperties(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)

	// verify chains existing field, should not be empty
	tc, err = operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "", tc.Spec.Chain.ChainProperties.BuilderID)

	// create a config map with values
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "chains-config",
			Namespace: tc.Spec.GetTargetNamespace(),
		},
		Data: map[string]string{
			"builder.id":           "123",
			"transparency.enabled": "false",
			"unknown_field":        "hello",
		},
	}
	_, err = k8sClient.CoreV1().ConfigMaps(tc.Spec.GetTargetNamespace()).Create(ctx, config, metav1.CreateOptions{})
	assert.NoError(t, err)

	// execute chains upgrade
	err = upgradeChainProperties(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)

	// verify chains with new configMap, map values should be updated
	tc, err = operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "123", tc.Spec.Chain.ChainProperties.BuilderID)
	assert.Equal(t, v1alpha1.BoolValue("false"), tc.Spec.Chain.ChainProperties.TransparencyConfigEnabled)

}
func TestResetTektonConfigConditions(t *testing.T) {
	ctx := context.TODO()
	operatorClient := operatorFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	// there is no tektonConfig CR, returns no error
	err := resetTektonConfigConditions(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// create tekconConfig CR with initial conditions
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Status: v1alpha1.TektonConfigStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   v1alpha1.AllDeploymentsReady,
					Status: "notReady",
				}},
			},
		},
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// resets the conditions
	err = resetTektonConfigConditions(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// verify the conditions field is empty
	_tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Empty(t, _tc.Status.Conditions)
}
