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
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/apis"
)

func (w *ResultsWatcherProperties) Validate(path string) (errs *apis.FieldError) {
	if w == nil {
		return nil
	}

	if w.LabelSelector != "" {
		if _, err := labels.Parse(w.LabelSelector); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(w.LabelSelector, path+".label_selector", err.Error()))
		}
	}

	if w.StoreDeadline != nil && w.StoreDeadline.Duration < 0 {
		errs = errs.Also(apis.ErrInvalidValue(w.StoreDeadline.Duration, path+".store_deadline"))
	}

	if w.RequeueInterval != nil && w.RequeueInterval.Duration < 0 {
		errs = errs.Also(apis.ErrInvalidValue(w.RequeueInterval.Duration, path+".requeue_interval"))
	}

	if w.ForwardBuffer != nil && w.ForwardBuffer.Duration < 0 {
		errs = errs.Also(apis.ErrInvalidValue(w.ForwardBuffer.Duration, path+".forward_buffer"))
	}

	if w.UpdateLogTimeout != nil && w.UpdateLogTimeout.Duration < 0 {
		errs = errs.Also(apis.ErrInvalidValue(w.UpdateLogTimeout.Duration, path+".update_log_timeout"))
	}

	if w.DynamicReconcileTimeout != nil && w.DynamicReconcileTimeout.Duration < 0 {
		errs = errs.Also(apis.ErrInvalidValue(w.DynamicReconcileTimeout.Duration, path+".dynamic_reconcile_timeout"))
	}

	return errs
}
