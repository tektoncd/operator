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
	"fmt"

	"knative.dev/pkg/apis"
)

func (ppp *PerformanceProperties) Validate(path string) *apis.FieldError {
	var errs *apis.FieldError

	bucketsPath := fmt.Sprintf("%s.buckets", path)
	// minimum and maximum allowed buckets value
	if ppp.Buckets != nil {
		if *ppp.Buckets < 1 || *ppp.Buckets > MaxBuckets {
			errs = errs.Also(apis.ErrOutOfBoundsValue(*ppp.Buckets, 1, 10, bucketsPath))
		}
	}

	// check for StatefulsetOrdinals and Replicas
	if ppp.StatefulsetOrdinals != nil && *ppp.StatefulsetOrdinals {
		if ppp.Replicas != nil {
			replicas := uint(*ppp.Replicas)
			if ppp.Buckets == nil {
				errs = errs.Also(apis.ErrMissingField(bucketsPath, "spec.performance.buckets must be set when statefulset ordinals is enabled"))
			} else if *ppp.Buckets != replicas {
				errs = errs.Also(apis.ErrInvalidValue(*ppp.Replicas, fmt.Sprintf("%s.replicas", path), "spec.performance.replicas must equal spec.performance.buckets for statefulset ordinals"))
			}
		}
	}

	return errs
}
