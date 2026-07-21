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
	"testing"
	"time"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTektonResultWatcherPropertiesValidate(t *testing.T) {
	tr := &TektonResult{
		ObjectMeta: metav1.ObjectMeta{
			Name: "result",
		},
		Spec: TektonResultSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}

	gracePeriod := metav1.Duration{Duration: 24 * time.Hour}
	checkOwner := true
	tr.Spec.Watcher = ResultsWatcherProperties{
		CompletedRunGracePeriod: &gracePeriod,
		CheckOwner:              &checkOwner,
		LabelSelector:           "app=foo",
	}
	errs := tr.Validate(context.TODO())
	assert.Equal(t, "", errs.Error())

	tr.Spec.Watcher.LabelSelector = "not a valid selector=="
	errs = tr.Validate(context.TODO())
	assert.Assert(t, errs != nil)
	assert.Assert(t, errs.Error() != "")
}

func TestResultsWatcherPropertiesValidate_NegativeDurations(t *testing.T) {
	negative := metav1.Duration{Duration: -time.Second}

	tests := []struct {
		name    string
		watcher ResultsWatcherProperties
		field   string
	}{
		{
			name:    "store_deadline",
			watcher: ResultsWatcherProperties{StoreDeadline: &negative},
			field:   "store_deadline",
		},
		{
			name:    "requeue_interval",
			watcher: ResultsWatcherProperties{RequeueInterval: &negative},
			field:   "requeue_interval",
		},
		{
			name:    "forward_buffer",
			watcher: ResultsWatcherProperties{ForwardBuffer: &negative},
			field:   "forward_buffer",
		},
		{
			name:    "update_log_timeout",
			watcher: ResultsWatcherProperties{UpdateLogTimeout: &negative},
			field:   "update_log_timeout",
		},
		{
			name:    "dynamic_reconcile_timeout",
			watcher: ResultsWatcherProperties{DynamicReconcileTimeout: &negative},
			field:   "dynamic_reconcile_timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.watcher.Validate("spec.watcher")
			assert.Assert(t, errs != nil)
			assert.ErrorContains(t, errs, tt.field)
		})
	}
}

func TestResultsWatcherPropertiesValidate_NilReceiver(t *testing.T) {
	var w *ResultsWatcherProperties
	assert.Assert(t, w.Validate("spec.watcher") == nil)
}
