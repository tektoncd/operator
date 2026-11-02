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

package tektonkueue

import (
	"context"
	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cleanupExtension struct {
	common.Extension
	err error
}

func (e cleanupExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return e.err
}

func TestPlatformCleanupBeforeDependencies(t *testing.T) {
	for _, tt := range []struct{ err, want error }{
		{err: v1alpha1.REQUEUE_EVENT_AFTER, want: v1alpha1.REQUEUE_EVENT_AFTER},
		// Other failures are recorded on the status and wait for the next event.
		{err: fmt.Errorf("cleanup failed")},
	} {
		// Missing clients deliberately ensure dependency checks cannot run before cleanup.
		r := Reconciler{extension: cleanupExtension{err: tt.err}}
		kueue := &v1alpha1.TektonKueue{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.TektonKueueResourceName}}
		assert.Equal(t, r.ReconcileKind(t.Context(), kueue), tt.want)
		assert.Assert(t, !kueue.Status.IsReady())
		assert.Equal(t, kueue.Status.GetCondition(v1alpha1.PreReconciler).IsFalse(), tt.want == nil)
	}
}
