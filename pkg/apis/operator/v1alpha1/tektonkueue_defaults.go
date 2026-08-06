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

package v1alpha1

import (
	"context"

	"github.com/konflux-ci/tekton-kueue/pkg/common"
	"k8s.io/utils/ptr"
)

const (
	KueueConfigMapName                           = common.ConfigMapName
	KueueConfigInstallerSet                      = "kueue-config"
	DefaultQueueName                             = "pipelines-queue"
	DefaultMultiClusterDisabled                  = true
	DefaultKueueDisabled                         = true
	KueueCreatedByValue                          = "TektonKueue"
	MultiClusterRoleSpoke       MultiClusterRole = "Spoke"
	MultiClusterRoleHub         MultiClusterRole = "Hub"
)

func (tp *TektonKueue) SetDefaults(_ context.Context) {
	tp.Spec.Kueue.SetDefaults()
}

func (s *Kueue) SetDefaults() {
	if s.Disabled == nil {
		s.Disabled = ptr.To(DefaultKueueDisabled)
		s.MultiClusterDisabled = DefaultMultiClusterDisabled
		s.QueueName = DefaultQueueName
	}
}
