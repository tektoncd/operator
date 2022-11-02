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

package client

import (
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	fake2 "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client/fake"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	testing2 "knative.dev/pkg/reconciler/testing"
)

func TestInstallerSetClient_PreSet_NewInstallation(t *testing.T) {
	ctx, _ := testing2.SetupFakeContext(t)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{serviceAccount, deployment}))
	assert.NilError(t, err)

	// fake.NewSimpleClientset() doesn't consider generate name when creating a resources
	// so we write a fake client to test
	// if we create one installerSet, it saves the name as "", then for the second installeSet
	// it tries save as "", and return already exist error
	fakeClient := fake2.NewFakeISClient()
	client := NewInstallerSetClient(fakeClient, "releaseVersion", "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.Equal(t, err, v1alpha1.REQUEUE_EVENT_AFTER)

	// set installer sets as false
	createdSets, err := fakeClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	for _, s := range createdSets.Items {
		is := s
		is.Status.InitializeConditions()
		is.Status.MarkNotReady("some error occurs")
		_, err := fakeClient.Update(ctx, &is, metav1.UpdateOptions{})
		assert.NilError(t, err)
	}

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.Assert(t, err != nil)

	// set installer sets as false
	createdSets, err = fakeClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)
	for _, s := range createdSets.Items {
		is := s
		markStatusReady(&is)
		_, err := fakeClient.Update(ctx, &is, metav1.UpdateOptions{})
		assert.NilError(t, err)
	}

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.NilError(t, err)
}

func TestInstallerSetClient_PreSet_Upgrade_Update(t *testing.T) {
	ctx, _ := testing2.SetupFakeContext(t)
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{deployment}))
	assert.NilError(t, err)

	oldSetName := "trigger-old-pre-test"
	existingIS := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: oldSetName,
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      v1alpha1.KindTektonTrigger,
				v1alpha1.ReleaseVersionKey: "prev",
				v1alpha1.InstallerSetType:  InstallerTypePre,
			},
		},
	}

	fakeClient := fake2.NewFakeISClient(existingIS)
	client := NewInstallerSetClient(fakeClient, "releaseVersion", "test-version", v1alpha1.KindTektonTrigger, &testMetrics{})

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.Equal(t, err, v1alpha1.REQUEUE_EVENT_AFTER)

	// in further reconciliation new installer set should get created
	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.Equal(t, err, v1alpha1.REQUEUE_EVENT_AFTER)

	list, err := fakeClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	assert.Assert(t, len(list.Items) != 0)
	assert.Assert(t, list.Items[0].GetName() != oldSetName)

	// set installer sets as false
	for _, s := range list.Items {
		is := s
		markStatusReady(&is)
		_, err := fakeClient.Update(ctx, &is, metav1.UpdateOptions{})
		assert.NilError(t, err)
	}

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.NilError(t, err)

	// to check update, updating manifest too
	// previously only deployment was there in installer set
	// now there will 2 resources
	comp.Spec.DefaultServiceAccount = "abc"
	updatedM, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{serviceAccount}))
	assert.NilError(t, err)
	manifest = manifest.Append(updatedM)

	err = client.PreSet(ctx, comp, &manifest, filterAndTransform(nil))
	assert.NilError(t, err)

	list, err = fakeClient.List(ctx, metav1.ListOptions{})
	assert.NilError(t, err)

	assert.Assert(t, len(list.Items) != 0)
	assert.Equal(t, len(list.Items[0].Spec.Manifests), 2)
}
