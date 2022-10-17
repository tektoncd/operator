package client

/*
Copyright 2022 The Tekton Authors

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

import (
	"strings"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	fake2 "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	testing2 "knative.dev/pkg/reconciler/testing"
)

var (
	serviceAccount = namespacedResource("v1", "ServiceAccount", "test", "test-service-account")
	deployment     = namespacedResource("v1", "Deployment", "test", "test-deployment")
)

// namespacedResource is an unstructured resource with the given apiVersion, kind, ns and name.
func namespacedResource(apiVersion, kind, ns, name string) unstructured.Unstructured {
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion(apiVersion)
	resource.SetKind(kind)
	resource.SetNamespace(ns)
	resource.SetName(name)
	return resource
}

func TestInstallerSetClient_Create(t *testing.T) {
	releaseVersion := "devel"
	comp := &v1alpha1.TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name: "trigger",
		},
		Spec: v1alpha1.TektonTriggerSpec{
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "test"},
		},
	}

	tests := []struct {
		name      string
		resources []unstructured.Unstructured
		setType   string
		wantIS    int
		wantErr   error
	}{
		{
			name:      "create pre set",
			setType:   InstallerTypePre,
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantIS:    1,
			wantErr:   nil,
		},
		{
			name:      "create post set",
			setType:   InstallerTypePost,
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantIS:    1,
			wantErr:   nil,
		},
		{
			name:      "create main set",
			setType:   InstallerTypeMain,
			resources: []unstructured.Unstructured{serviceAccount, deployment},
			wantIS:    2,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := testing2.SetupFakeContext(t)

			fakeclient := fake.NewSimpleClientset()
			tisClient := fakeclient.OperatorV1alpha1().TektonInstallerSets()

			manifest, err := mf.ManifestFrom(mf.Slice(tt.resources))
			if err != nil {
				t.Fatalf("Failed to generate manifest: %v", err)
			}

			client := NewInstallerSetClient(tisClient, releaseVersion, "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})

			// fake.NewSimpleClientset() doesn't consider generate name when creating a resources
			// so we write a fake client to test
			// if we create one installerSet, it saves the name as "", then for the second installeSet
			// it tries save as "", and return already exist error

			if tt.setType == InstallerTypeMain {
				fakeClient := fake2.NewFakeISClient()
				client = NewInstallerSetClient(fakeClient, releaseVersion, "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})
			}

			iSs, gotErr := client.create(ctx, comp, &manifest, filterAndTransform(common.NoExtension(ctx)), tt.setType)

			if tt.wantErr != nil {
				assert.Equal(t, gotErr, tt.wantErr)
				return
			}
			assert.NilError(t, gotErr)
			assert.Equal(t, len(iSs), tt.wantIS)

			if !strings.Contains(iSs[0].GenerateName, tt.setType) {
				t.Fatalf("expected installer set type in generate name")
			}
			if tt.setType == InstallerTypeMain {
				assert.Assert(t, len(iSs[0].Spec.Manifests) != 0, "resource list must not be empty here")
				assert.Assert(t, len(iSs[1].Spec.Manifests) != 0, "resource list must not be empty here")
			}
		})
	}
}
