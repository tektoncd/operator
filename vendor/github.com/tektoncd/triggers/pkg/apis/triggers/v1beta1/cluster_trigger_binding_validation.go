/*
Copyright 2021 The Tekton Authors

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

package v1beta1

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/validate"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ resourcesemantics.VerbLimited = (*ClusterTriggerBinding)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (ctb *ClusterTriggerBinding) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

func (ctb *ClusterTriggerBinding) Validate(ctx context.Context) *apis.FieldError {
	if err := validate.ObjectMetadata(ctb.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return ctb.Spec.Validate(ctx)
}
