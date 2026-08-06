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

	"k8s.io/utils/ptr"
)

const (
	SchedulerConfigMapName      = KueueConfigMapName
	SchedulerConfigInstallerSet = "scheduler-config"
	DefaultSchedulerDisabled    = DefaultKueueDisabled
	SchedulerCreatedByValue     = "TektonScheduler"
)

// SetDefaults retains admission compatibility for the deprecated API.
func (ts *TektonScheduler) SetDefaults(_ context.Context) {
	ts.Spec.Scheduler.SetDefaults()
}

func (s *Scheduler) SetDefaults() {
	if s.Disabled == nil {
		s.Disabled = ptr.To(DefaultSchedulerDisabled)
		s.MultiClusterDisabled = DefaultMultiClusterDisabled
		s.QueueName = DefaultQueueName
	}
}

// IsDisabled returns true if the deprecated TektonScheduler is disabled.
func (s *Scheduler) IsDisabled() bool {
	if s == nil || s.Disabled == nil {
		return DefaultSchedulerDisabled
	}
	return *s.Disabled
}
