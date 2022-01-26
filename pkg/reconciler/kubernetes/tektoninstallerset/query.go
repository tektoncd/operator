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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CurrentInstallerSetName(ctx context.Context, client clientset.Interface, labelSelector string) (string, error) {
	iSets, err := client.OperatorV1alpha1().TektonInstallerSets().List(ctx, v1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}
	if len(iSets.Items) == 0 {
		return "", nil
	}
	if len(iSets.Items) == 1 {
		iSetName := iSets.Items[0].GetName()
		return iSetName, nil
	}

	// len(iSets.Items) > 1
	// delete all installerSets as it cannot be decided which one is the desired one
	err = client.OperatorV1alpha1().TektonInstallerSets().DeleteCollection(ctx,
		v1.DeleteOptions{},
		v1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return "", err
	}
	return "", v1alpha1.RECONCILE_AGAIN_ERR
}
