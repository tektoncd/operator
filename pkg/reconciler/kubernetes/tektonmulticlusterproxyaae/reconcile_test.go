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

package tektonmulticlusterproxyaae

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/client/informers/externalversions"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// The proxy is deployed only after Kueue serves MultiKueueClusters.
func TestDependenciesWaitForMultiKueueClusters(t *testing.T) {
	pipeline := &v1alpha1.TektonPipeline{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.PipelineResourceName}}
	pipeline.Status.MarkPreReconcilerComplete()
	pipeline.Status.MarkInstallerSetAvailable()
	pipeline.Status.MarkInstallerSetReady()
	pipeline.Status.MarkPostReconcilerComplete()
	pipelines := externalversions.NewSharedInformerFactory(operatorfake.NewSimpleClientset(), 0).Operator().V1alpha1().TektonPipelines()
	assert.NilError(t, pipelines.Informer().GetIndexer().Add(pipeline))

	for _, tt := range []struct {
		name string
		// resources are served in kueue.x-k8s.io/v1beta1; nil means the group version is missing.
		resources []metav1.APIResource
		served    bool
	}{
		{name: "Kueue is not installed"},
		// Kueue's CRDs are created one by one.
		{name: "MultiKueueClusters are not served yet", resources: []metav1.APIResource{{Name: "clusterqueues"}}},
		{name: "MultiKueueClusters are served", resources: []metav1.APIResource{{Name: "clusterqueues"}, {Name: "multikueueclusters"}}, served: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			kube := k8sfake.NewSimpleClientset()
			if tt.resources != nil {
				kube.Discovery().(*discoveryfake.FakeDiscovery).Resources = []*metav1.APIResourceList{
					{GroupVersion: "kueue.x-k8s.io/v1beta1", APIResources: tt.resources},
				}
			}
			proxy := &v1alpha1.TektonMulticlusterProxyAAE{}
			err := (&Reconciler{pipelineInformer: pipelines, kubeClientSet: kube}).ensureDependenciesInstalled(proxy)
			if tt.served {
				assert.NilError(t, err)
				return
			}
			assert.Equal(t, err, v1alpha1.REQUEUE_EVENT_AFTER)
			assert.Equal(t, proxy.Status.GetCondition(v1alpha1.DependenciesInstalled).Message,
				"Dependency missing: Kueue API kueue.x-k8s.io/v1beta1 multikueueclusters does not exist")
		})
	}
}
