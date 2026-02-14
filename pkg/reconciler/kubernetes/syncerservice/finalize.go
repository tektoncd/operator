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

package syncerservice

import (
	"context"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	syncerservicereconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/syncerservice"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

var _ syncerservicereconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a SyncerService.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.SyncerService) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by SyncerService", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}
