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
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/client/informers/externalversions"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type cleanupExtension struct {
	common.Extension
	err error
}

func (e cleanupExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return e.err
}

func TestPlatformCleanupBeforeDependencies(t *testing.T) {
	for _, tt := range []struct {
		err, want error
		// reason is what the status reports, if anything.
		reason string
	}{
		{err: v1alpha1.REQUEUE_EVENT_AFTER, want: v1alpha1.REQUEUE_EVENT_AFTER},
		// A wrapped requeue is retried as well, and its reason is reported.
		{err: errors.Wrap(v1alpha1.REQUEUE_EVENT_AFTER, "plugin is not ready"), want: v1alpha1.REQUEUE_EVENT_AFTER, reason: "plugin is not ready"},
		// Other failures are recorded on the status and wait for the next event.
		{err: fmt.Errorf("cleanup failed"), reason: "cleanup failed"},
	} {
		// Missing clients deliberately ensure dependency checks cannot run before cleanup.
		r := Reconciler{extension: cleanupExtension{err: tt.err}}
		kueue := &v1alpha1.TektonKueue{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.TektonKueueResourceName}}
		assert.Equal(t, r.ReconcileKind(t.Context(), kueue), tt.want)
		assert.Assert(t, !kueue.Status.IsReady())
		condition := kueue.Status.GetCondition(v1alpha1.PreReconciler)
		assert.Equal(t, condition.IsFalse(), tt.reason != "")
		assert.Assert(t, strings.Contains(condition.Message, tt.reason), condition.Message)
	}
}

// Tekton Kueue's webhook needs cert-manager certificates, so nothing is
// installed until both APIs are served.
func TestDependencies(t *testing.T) {
	pipeline := &v1alpha1.TektonPipeline{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.PipelineResourceName}}
	pipeline.Status.MarkPreReconcilerComplete()
	pipeline.Status.MarkInstallerSetAvailable()
	pipeline.Status.MarkInstallerSetReady()
	pipeline.Status.MarkPostReconcilerComplete()
	pipelines := externalversions.NewSharedInformerFactory(operatorfake.NewSimpleClientset(), 0).Operator().V1alpha1().TektonPipelines()
	assert.NilError(t, pipelines.Informer().GetIndexer().Add(pipeline))

	for _, tt := range []struct {
		name    string
		apis    []string
		missing string
	}{
		{name: "installed", apis: []string{"cert-manager.io/v1", "kueue.x-k8s.io/v1beta1"}},
		{name: "cert-manager missing", apis: []string{"kueue.x-k8s.io/v1beta1"}, missing: "cert-manager.io/v1"},
		{name: "kueue missing", apis: []string{"cert-manager.io/v1"}, missing: "kueue.x-k8s.io/v1beta1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			kube := k8sfake.NewSimpleClientset()
			discovery := kube.Discovery().(*discoveryfake.FakeDiscovery)
			for _, gv := range tt.apis {
				discovery.Resources = append(discovery.Resources, &metav1.APIResourceList{GroupVersion: gv})
			}
			kueue := &v1alpha1.TektonKueue{}
			err := (&Reconciler{pipelineInformer: pipelines, kubeClientSet: kube}).ensureDependenciesInstalled(kueue)
			assert.Equal(t, err == nil, tt.missing == "")
			if tt.missing != "" {
				want := fmt.Sprintf("Dependency missing: API %s does not exist. Please install %s", tt.missing, tt.missing)
				assert.Equal(t, kueue.Status.GetCondition(v1alpha1.DependenciesInstalled).Message, want)
			}
		})
	}
}
