/*
Copyright 2024 The Tekton Authors
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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

// ListCustomSet return the lists of custom sets with the provided labelSelector
func (i *InstallerSetClient) ListCustomSet(ctx context.Context, labelSelector string) (*v1alpha1.TektonInstallerSetList, error) {
	logger := logging.FromContext(ctx)
	logger.Debugf("%v: checking installer sets with labels: %v", i.resourceKind, labelSelector)

	is, err := i.clientSet.List(ctx, v1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	if len(is.Items) == 0 {
		logger.Debugf("%v: no installer sets found with labels: %v", i.resourceKind, labelSelector)
	}
	return is, nil
}
