/*
Copyright 2025 The Tekton Authors

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

	kueueCommon "github.com/konflux-ci/tekton-kueue/pkg/common"
	kueueConfig "github.com/konflux-ci/tekton-kueue/pkg/config"
	"k8s.io/utils/ptr"
)

const (
	SchedulerConfigMapName      = kueueCommon.ConfigMapName
	MultiClusterRoleHub         = kueueCommon.MultiClusterRoleHub
	MultiClusterRoleSpoke       = kueueCommon.MultiClusterRoleSpoke
	SchedulerConfigInstallerSet = "scheduler-config"
	DefaultQueueName            = "pipelines-queue"
	DefaultMultiClusterEnabled  = false
	DefaultSchedulerDisabled    = false
	SchedulerCreatedByValue     = "TektonScheduler"
)

func (tp *TektonScheduler) SetDefaults(_ context.Context) {
	tp.Spec.Scheduler.SetDefaults()
}

func (p *Scheduler) SetDefaults() {
	if p.Disabled == nil {
		p.Disabled = ptr.To(DefaultSchedulerDisabled)
	}
	p.SchedulerConfig.SetDefaults()

}
func (s *SchedulerConfig) SetDefaults() {
	if s.SchedulerConfig == nil {
		s.SchedulerConfig = &kueueConfig.SchedulerConfig{
			QueueName:           DefaultQueueName,
			MultiClusterEnabled: DefaultMultiClusterEnabled,
		}
	}
}
