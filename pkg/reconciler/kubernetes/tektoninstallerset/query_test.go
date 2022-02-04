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

package tektoninstallerset

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	pipelineLS = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     "TektonPipeline",
			v1alpha1.InstallerSetType: "pipeline",
		},
	}
	triggersLS = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     "TektonTriggers",
			v1alpha1.InstallerSetType: "triggers",
		},
	}
)

func TestCurrentInstallerSetName(t *testing.T) {

	iSets := v1alpha1.TektonInstallerSetList{
		Items: []v1alpha1.TektonInstallerSet{
			v1alpha1.TektonInstallerSet{},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline",
					Labels: map[string]string{
						v1alpha1.CreatedByKey:     "TektonPipeline",
						v1alpha1.InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector, err := common.LabelSelector(pipelineLS)
	if err != nil {
		t.Error(err)
	}
	name, err := CurrentInstallerSetName(context.TODO(), client, labelSelector)

	assert.NilError(t, err)
	assert.Equal(t, name, "pipeline")
}

func TestCurrentInstallerSetNameNoMatching(t *testing.T) {

	iSets := v1alpha1.TektonInstallerSetList{
		Items: []v1alpha1.TektonInstallerSet{
			v1alpha1.TektonInstallerSet{},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline",
					Labels: map[string]string{
						v1alpha1.CreatedByKey:     "TektonPipeline",
						v1alpha1.InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector, err := common.LabelSelector(triggersLS)
	if err != nil {
		t.Error(err)
	}
	name, err := CurrentInstallerSetName(context.TODO(), client, labelSelector)

	assert.NilError(t, err)
	assert.Equal(t, name, "")
}

func TestCurrentInstallerSetNameWithDuplicates(t *testing.T) {

	iSets := v1alpha1.TektonInstallerSetList{
		Items: []v1alpha1.TektonInstallerSet{
			v1alpha1.TektonInstallerSet{},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline-1",
					Labels: map[string]string{
						v1alpha1.CreatedByKey:     "TektonPipeline",
						v1alpha1.InstallerSetType: "pipeline",
					},
				},
			},
			v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline-2",
					Labels: map[string]string{
						v1alpha1.CreatedByKey:     "TektonPipeline",
						v1alpha1.InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector, err := common.LabelSelector(pipelineLS)
	if err != nil {
		t.Error(err)
	}

	name, err := CurrentInstallerSetName(context.TODO(), client, labelSelector)
	assert.Error(t, err, v1alpha1.RECONCILE_AGAIN_ERR.Error())
	assert.Equal(t, name, "")
}

func TestCleanUpObsoleteResources(t *testing.T) {

	iSets := v1alpha1.TektonInstallerSetList{
		Items: []v1alpha1.TektonInstallerSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline-1",
					Labels: map[string]string{
						v1alpha1.CreatedByKey: "Abc",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline-2",
					Labels: map[string]string{
						v1alpha1.CreatedByKey:     "Abc",
						v1alpha1.InstallerSetType: "pipeline",
					},
				},
			},
		},
	}

	// initially there are 2 installerSet
	client := fake.NewSimpleClientset(&iSets)

	err := CleanUpObsoleteResources(context.TODO(), client, "Abc")
	assert.NilError(t, err)

	// now only one installerSet should exist
	// which doesn't have InstallerSetType label
	is, err := client.OperatorV1alpha1().TektonInstallerSets().List(context.TODO(), metav1.ListOptions{})
	assert.NilError(t, err)

	// pipeline-1 is obsolete resources, so must be deleted
	assert.Equal(t, is.Items[0].Name, "pipeline-2")
}
