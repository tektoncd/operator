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

package client

import (
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (i *InstallerSetClient) getSetLabels(setType string) string {
	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	typeReq, _ := labels.NewRequirement(v1alpha1.InstallerSetType, selection.Equals, []string{setType})
	if typeReq != nil {
		labelSelector = labelSelector.Add(*typeReq)
	}
	return labelSelector.String()
}
