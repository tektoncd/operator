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

package tektonkueue

import (
	"context"
	"errors"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

func (r *Reconciler) ensureInstallerSets(ctx context.Context, tp *v1alpha1.TektonKueue) error {
	logger := logging.FromContext(ctx)
	if err := r.installerSetClient.MainSet(ctx, tp, &r.manifest, filterAndTransform(r.extension)); err != nil {
		msg := fmt.Sprintf("Main Reconcilation failed: %s", err.Error())
		logger.Error(msg)
		if errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
			return err
		}
		tp.Status.MarkInstallerSetNotReady(msg)
	}

	return nil
}
