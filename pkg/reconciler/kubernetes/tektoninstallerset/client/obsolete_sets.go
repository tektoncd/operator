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
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (i *InstallerSetClient) RemoveObsoleteSets(ctx context.Context) error {
	var sets []string

	switch i.resourceKind {
	case v1alpha1.KindTektonPipeline:
		sets = []string{"pipeline", "PrePipeline", "PostPipeline"}
	case v1alpha1.KindTektonTrigger:
		sets = []string{"trigger"}
	case v1alpha1.KindTektonAddon:
		// not adding VersionedClusterTask here, as we keep versioned clustertasks on upgrade
		sets = []string{"ClusterTask", "CommunityClusterTask", "PipelinesTemplate", "TriggersResources", "ConsoleCLI", "MiscellaneousResources", "PipelinesAsCode"}
	case v1alpha1.KindTektonDashboard:
		sets = []string{"dashboard"}
	}

	labelSelector := labels.NewSelector()
	createdReq, _ := labels.NewRequirement(v1alpha1.CreatedByKey, selection.Equals, []string{i.resourceKind})
	if createdReq != nil {
		labelSelector = labelSelector.Add(*createdReq)
	}
	typeReq, _ := labels.NewRequirement(v1alpha1.InstallerSetType, selection.In, sets)
	if typeReq != nil {
		labelSelector = labelSelector.Add(*typeReq)
	}
	err := i.clientSet.DeleteCollection(ctx, metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	return nil
}
