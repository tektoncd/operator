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

package manualapprovalgate

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	manualapprovalgatereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/manualapprovalgate"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

var _ manualapprovalgatereconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a ManualApprovalGate CR.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.ManualApprovalGate) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	//Delete CRDs before deleting rest of resources so that any instance
	//of CRDs which has finalizer set will get deleted before we remove
	//the controller;s deployment for it
	if err := r.manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for ManualApprovalGate")
		return err
	}

	if err := r.installerSetClient.CleanupMainSet(ctx); err != nil {
		logger.Error("failed to cleanup main installerset: ", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}
