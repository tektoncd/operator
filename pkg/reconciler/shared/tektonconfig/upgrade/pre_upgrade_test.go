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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

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

func TestUpgradePipelineProperties(t *testing.T) {
	ctx := context.TODO()
	operatorClient := operatorFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	// there is no tektonConfig CR, returns no error
	err := upgradePipelineProperties(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// create tekconConfig CR with initial conditions
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Pipeline: v1alpha1.Pipeline{
				PipelineProperties: v1alpha1.PipelineProperties{
					// EnableStepActions: ptr.Bool(false),
				},
			},
		},
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// update enable-step-actions to true
	err = upgradePipelineProperties(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// verify the pipeline property enable-step-actions to true
	tcData, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, tcData.Spec.Pipeline.EnableStepActions, ptr.Bool(true))
}
