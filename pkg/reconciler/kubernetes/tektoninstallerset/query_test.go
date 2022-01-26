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
	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCurrentInstallerSetName(t *testing.T) {

	iSets := v1alpha1.TektonInstallerSetList{
		Items: []v1alpha1.TektonInstallerSet{
			v1alpha1.TektonInstallerSet{},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline",
					Labels: map[string]string{
						CreatedByKey:     "TektonPipeline",
						InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		CreatedByKey, "TektonPipeline",
		InstallerSetType, "pipeline",
	)
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
						CreatedByKey:     "TektonPipeline",
						InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		CreatedByKey, "TektonTriggers",
		InstallerSetType, "triggers",
	)
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
						CreatedByKey:     "TektonPipeline",
						InstallerSetType: "pipeline",
					},
				},
			},
			v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipeline-2",
					Labels: map[string]string{
						CreatedByKey:     "TektonPipeline",
						InstallerSetType: "pipeline",
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(&iSets)
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		CreatedByKey, "TektonPipeline",
		InstallerSetType, "pipeline",
	)

	name, err := CurrentInstallerSetName(context.TODO(), client, labelSelector)
	assert.Error(t, err, v1alpha1.RECONCILE_AGAIN_ERR.Error())
	assert.Equal(t, name, "")
}
