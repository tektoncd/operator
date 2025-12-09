/*
Copyright 2020 The Tekton Authors

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

package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/tektoncd/pruner/pkg/config"
	"gopkg.in/yaml.v3"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
)

func TestCommonTransformers(t *testing.T) {
	targetNamespace := "test-ns"
	component := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
		},
	}
	in := []unstructured.Unstructured{namespacedResource("test/v1", "TestCR", "another-ns", "test-resource")}
	manifest, err := mf.ManifestFrom(mf.Slice(in))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}
	if err := Transform(context.Background(), &manifest, component); err != nil {
		t.Fatalf("Failed to transform manifest: %v", err)
	}
	t.Log(manifest.Resources())
	resource := &manifest.Resources()[0]

	// Verify namespace is carried over.
	if got, want := resource.GetNamespace(), targetNamespace; got != want {
		t.Fatalf("GetNamespace() = %s, want %s", got, want)
	}

	// Transform with a platform extension
	ext := TestExtension("fubar")
	if err := Transform(context.Background(), &manifest, component, ext.Transformers(component)...); err != nil {
		t.Fatalf("Failed to transform manifest: %v", err)
	}
	resource = &manifest.Resources()[0]

	// Verify namespace is transformed
	if got, want := resource.GetNamespace(), string(ext); got != want {
		t.Fatalf("GetNamespace() = %s, want %s", got, want)
	}

	// Verify OwnerReference is set.
	if len(resource.GetOwnerReferences()) == 0 {
		t.Fatalf("len(GetOwnerReferences()) = 0, expected at least 1")
	}
	ownerRef := resource.GetOwnerReferences()[0]

	apiVersion, kind := component.GroupVersionKind().ToAPIVersionAndKind()
	wantOwnerRef := metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               component.GetName(),
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(true),
	}

	if !cmp.Equal(ownerRef, wantOwnerRef) {
		t.Fatalf("Unexpected ownerRef: %s", cmp.Diff(ownerRef, wantOwnerRef))
	}
}

func TestImagesFromEnv(t *testing.T) {
	t.Setenv("IMAGE_PIPELINES_CONTROLLER", "docker.io/pipeline")
	data := ImagesFromEnv(PipelinesImagePrefix)
	if !cmp.Equal(data, map[string]string{"CONTROLLER": "docker.io/pipeline"}) {
		t.Fatalf("Unexpected ImageFromEnv: %s", cmp.Diff(data, map[string]string{"CONTROLLER": "docker.io/pipeline"}))
	}
	convertToLower := ToLowerCaseKeys(data)
	if !cmp.Equal(convertToLower, map[string]string{"controller": "docker.io/pipeline"}) {
		t.Fatalf("Unexpected ToLowerCaseKeys: %s", cmp.Diff(convertToLower, map[string]string{"controller": "docker.io/pipeline"}))
	}
}

func TestReplaceImages(t *testing.T) {
	t.Run("ignore non deployment", func(t *testing.T) {
		testData := path.Join("testdata", "test-replace-kind.yaml")
		expected, _ := mf.ManifestFrom(mf.Recursive(testData))

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(map[string]string{}))
		assertNoError(t, err)

		if d := cmp.Diff(expected.Resources(), newManifest.Resources()); d != "" {
			t.Errorf("failed to update deployment %s", diff.PrintWantGot(d))
		}
	})

	t.Run("replace containers by name", func(t *testing.T) {
		image := "foo.bar/image/controller"
		images := map[string]string{
			"controller_deployment": image,
		}
		testData := path.Join("testdata", "test-replace-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoError(t, err)
		assertDeployContainersHasImage(t, newManifest.Resources(), "controller-deployment", image)
		assertDeployContainersHasImage(t, newManifest.Resources(), "sidecar", "busybox")
	})

	t.Run("replace containers args by space", func(t *testing.T) {
		arg := ArgPrefix + "__bash_image"
		image := "foo.bar/image/bash"
		images := map[string]string{
			arg: image,
		}
		testData := path.Join("testdata", "test-replace-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoError(t, err)
		assertDeployContainerArgsHasImage(t, newManifest.Resources(), "-bash", image)
		assertDeployContainerArgsHasImage(t, newManifest.Resources(), "-git", "git")
	})

	t.Run("of_container_args_has_equal", func(t *testing.T) {
		arg := ArgPrefix + "__nop"
		image := "foo.bar/image/nop"
		images := map[string]string{
			arg: image,
		}
		testData := path.Join("testdata", "test-replace-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoError(t, err)
		assertDeployContainerArgsHasImage(t, newManifest.Resources(), "-nop", image)
		assertDeployContainerArgsHasImage(t, newManifest.Resources(), "-git", "git")
	})

	t.Run("replace task addons step image", func(t *testing.T) {
		stepName := "push_image"
		image := "foo.bar/image/buildah"
		images := map[string]string{
			stepName: image,
		}
		testData := path.Join("testdata", "test-replace-addon-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(TaskImages(context.TODO(), images))
		assertNoError(t, err)
		assertTaskImage(t, newManifest.Resources(), "push", image)
		assertTaskImage(t, newManifest.Resources(), "build", "$(inputs.params.BUILDER_IMAGE)")
	})

	t.Run("replace stepaction image", func(t *testing.T) {
		stepActionName := "git_clone"
		image := "foo.bar/image/git-clone"
		images := map[string]string{
			stepActionName: image,
		}
		testData := path.Join("testdata", "test-replace-stepaction-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(StepActionImages(context.TODO(), images))
		assertNoError(t, err)
		assertStepActionImage(t, newManifest.Resources(), "git-clone", image)
	})

	t.Run("replace containers by name", func(t *testing.T) {
		image := "foo.bar/image/controller"
		images := map[string]string{
			"controller_deployment": image,
		}
		testData := path.Join("testdata", "test-replace-statefulset-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(StatefulSetImages(images))
		assertNoError(t, err)
		assertStatefulSetContainersHasImage(t, newManifest.Resources(), "controller-deployment", image)
		assertStatefulSetContainersHasImage(t, newManifest.Resources(), "sidecar", "busybox")
	})

	t.Run("replace task addons param image", func(t *testing.T) {
		paramName := ParamPrefix + "builder_image"
		image := "foo.bar/image/buildah"
		images := map[string]string{
			paramName: image,
		}
		testData := path.Join("testdata", "test-replace-addon-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(TaskImages(context.TODO(), images))
		assertNoError(t, err)
		assertParamHasImage(t, newManifest.Resources(), "BUILDER_IMAGE", image)
		assertTaskImage(t, newManifest.Resources(), "push", "buildah")
	})
}

func TestImageRegistryDomainOverride(t *testing.T) {
	t.Setenv("TEKTON_REGISTRY_OVERRIDE", "custom-registry.io/custom-path")
	// Array of images to be replaced
	imageNameList := map[string]string{
		"IMAGE_A": "docker.io/tekton/controller:latest",
		"IMAGE_B": "gcr.io/tekton-releases/dogfooding/tekton-controller:latest",
		"IMAGE_C": "quay.io/tekton/controller:latest",
	}

	expectedResult := map[string]string{
		"IMAGE_A": "custom-registry.io/custom-path/tekton/controller:latest",
		"IMAGE_B": "custom-registry.io/custom-path/tekton-releases/dogfooding/tekton-controller:latest",
		"IMAGE_C": "custom-registry.io/custom-path/tekton/controller:latest",
	}

	data := ImageRegistryDomainOverride(imageNameList)
	if !cmp.Equal(data, expectedResult) {
		t.Fatalf("Unexpected ImageRegistryDomainOverride: %s", cmp.Diff(data, expectedResult))
	}
}

func TestImageRegistryDomainWithoutOverride(t *testing.T) {
	t.Setenv("TEKTON_REGISTRY_OVERRIDE", "")
	// Array of images to be replaced
	imageNameList := map[string]string{
		"IMAGE_A": "docker.io/tekton/controller:latest",
		"IMAGE_B": "gcr.io/tekton-releases/dogfooding/tekton-controller:latest",
		"IMAGE_C": "quay.io/tekton/controller:latest",
	}

	data := ImageRegistryDomainOverride(imageNameList)
	if !cmp.Equal(data, imageNameList) {
		t.Fatalf("Unexpected ImageRegistryDomainOverride: %s", cmp.Diff(data, imageNameList))
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("assertion failed; expected no error %v", err)
	}
}

func assertDeployContainersHasImage(t *testing.T, resources []unstructured.Unstructured, name string, image string) {
	t.Helper()

	for _, resource := range resources {
		deployment := deploymentFor(t, resource)
		containers := deployment.Spec.Template.Spec.Containers

		for _, container := range containers {
			if container.Name != name {
				continue
			}

			if container.Image != image {
				t.Errorf("assertion failed; unexpected image: expected %s and got %s", image, container.Image)
			}
		}
	}
}

func assertStatefulSetContainersHasImage(t *testing.T, resources []unstructured.Unstructured, name string, image string) {
	t.Helper()

	for _, resource := range resources {
		set := statefulSetFor(t, resource)
		containers := set.Spec.Template.Spec.Containers

		for _, container := range containers {
			if container.Name != name {
				continue
			}

			if container.Image != image {
				t.Errorf("assertion failed; unexpected image: expected %s and got %s", image, container.Image)
			}
		}
	}
}

func assertDeployContainerArgsHasImage(t *testing.T, resources []unstructured.Unstructured, arg string, image string) {
	t.Helper()

	for _, resource := range resources {
		deployment := deploymentFor(t, resource)
		containers := deployment.Spec.Template.Spec.Containers

		for _, container := range containers {
			if len(container.Args) == 0 {
				continue
			}

			for a, argument := range container.Args {
				if argument == arg && container.Args[a+1] != image {
					t.Errorf("not equal: expected %v, got %v", image, container.Args[a+1])
				}
			}
		}
	}
}

func assertParamHasImage(t *testing.T, resources []unstructured.Unstructured, name string, image string) {
	t.Helper()

	for _, r := range resources {
		params, found, err := unstructured.NestedSlice(r.Object, "spec", "params")
		if err != nil {
			t.Errorf("assertion failed; %v", err)
		}
		if !found {
			continue
		}

		for _, p := range params {
			param := p.(map[string]interface{})
			n, ok := param["name"].(string)
			if !ok {
				t.Errorf("assertion failed; step name not found")
				continue
			}
			if n != name {
				continue
			}

			i, ok := param["default"].(string)
			if !ok {
				t.Errorf("assertion failed; default image not found")
				continue
			}
			if i != image {
				t.Errorf("assertion failed; unexpected image: expected %s, got %s", image, i)
			}
		}
	}
}

func assertStepActionImage(t *testing.T, resources []unstructured.Unstructured, name string, image string) {
	t.Helper()

	for _, r := range resources {
		stepActionData, found, err := unstructured.NestedFieldCopy(r.Object, "metadata", "name")
		if err != nil {
			t.Error(err)
		}
		if !found {
			continue
		}
		stepActionName, ok := stepActionData.(string)
		if !ok {
			t.Errorf("assertion failed; name not found")
			continue
		}
		if stepActionName != name {
			t.Errorf("assertion failed; unexpected name: expected %s, got %s", name, stepActionData.(string))
		}
		stepActionImage, found, err := unstructured.NestedMap(r.Object, "spec")
		if err != nil {
			t.Errorf("assertion failed; %v", err)
		}
		if !found {
			continue
		}
		i, ok := stepActionImage["image"]
		if !ok {
			t.Errorf("assertion failed; image not found")
			continue
		}
		if i != image {
			t.Errorf("assertion failed; unexpected image: expected %s, got %s", image, i)
		}
	}
}

func assertTaskImage(t *testing.T, resources []unstructured.Unstructured, name string, image string) {
	t.Helper()

	for _, r := range resources {
		steps, found, err := unstructured.NestedSlice(r.Object, "spec", "steps")
		if err != nil {
			t.Errorf("assertion failed; %v", err)
		}
		if !found {
			continue
		}

		for _, s := range steps {
			step := s.(map[string]interface{})
			n, ok := step["name"].(string)
			if !ok {
				t.Errorf("assertion failed; step name not found")
				continue
			}
			if n != name {
				continue
			}

			i, ok := step["image"].(string)
			if !ok {
				t.Errorf("assertion failed; image not found")
				continue
			}
			if i != image {
				t.Errorf("assertion failed; unexpected image: expected %s, got %s", image, i)
			}
		}
	}
}

func deploymentFor(t *testing.T, unstr unstructured.Unstructured) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.Object, deployment)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}
	return deployment
}

func statefulSetFor(t *testing.T, unstr unstructured.Unstructured) *appsv1.StatefulSet {
	set := &appsv1.StatefulSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.Object, set)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}
	return set
}

func TestReplaceNamespaceInDeploymentEnv(t *testing.T) {
	testData := path.Join("testdata", "test-replace-env-in-result-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	manifest, err = manifest.Transform(ReplaceNamespaceInDeploymentEnv([]string{"tekton-results-watcher", "tekton-results-api"}, "openshift-pipelines"))
	assertNoError(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoError(t, err)

	env := d.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, env[0].Value, "tcp")
	assert.Equal(t, env[1].Value, "tekton-results-mysql.openshift-pipelines.svc.cluster.local")

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[1].Object, d)
	assertNoError(t, err)

	env = d.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, env[0].Value, "tcp")
	assert.Equal(t, env[1].Value, "tekton-results-api-service.openshift-pipelines.svc.cluster.local")
}

func TestReplaceNamespaceInDeploymentArgs(t *testing.T) {
	testData := path.Join("testdata", "test-replace-arg-in-result-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	manifest, err = manifest.Transform(ReplaceNamespaceInDeploymentArgs([]string{"tekton-results-watcher"}, "openshift-pipelines"))
	assertNoError(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoError(t, err)

	args := d.Spec.Template.Spec.Containers[0].Args
	assert.Equal(t, args[0], "-api_addr")
	assert.Equal(t, args[1], "tekton-results-api-service.openshift-pipelines.svc.cluster.local:50051")
}

func TestReplaceNamespaceInClusterInterceptor(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-cluster-interceptor.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	manifest, err = manifest.Transform(injectNamespaceCRClusterInterceptorClientConfig("foobar"))
	assertNoError(t, err)

	clusterInterceptor := manifest.Resources()[0].Object
	service, _, err := unstructured.NestedFieldNoCopy(clusterInterceptor, "spec", "clientConfig", "service")
	m := service.(map[string]interface{})
	assertNoError(t, err)
	assert.Equal(t, "foobar", m["namespace"])
}

func TestReplaceNamespaceInClusterRole(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-cluster-role.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	manifest, err = manifest.Transform(injectNamespaceClusterRole("foobar"))
	assertNoError(t, err)

	clusterRole := manifest.Resources()[0].Object
	rules, _, err := unstructured.NestedSlice(clusterRole, "rules")
	assertNoError(t, err)
	namespaceRule := rules[0].(map[string]interface{})
	for _, name := range namespaceRule["resourceNames"].([]interface{}) {
		if name.(string) != "foobar" {
			t.Errorf("Didn't replace 'tekton-pipelines' in the namespace rule")
		}
	}
	namespaceFinalizerRule := rules[1].(map[string]interface{})
	for _, name := range namespaceFinalizerRule["resourceNames"].([]interface{}) {
		if name.(string) != "foobar" {
			t.Errorf("Didn't replace 'tekton-pipelines' in the namespace rule")
		}
	}
}

func TestAddConfigMapValues_PipelineProperties(t *testing.T) {
	testData := path.Join("testdata", "test-replace-cm-values.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	prop := v1alpha1.PipelineProperties{
		EnableApiFields: "stable",
	}

	manifest, err = manifest.Transform(AddConfigMapValues("test1", prop))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)

	assert.Equal(t, cm.Data["foo"], "bar")
	assert.Equal(t, cm.Data["enable-api-fields"], "stable")
}

func TestAddConfigMapValues_OptionalPipelineProperties(t *testing.T) {
	testData := path.Join("testdata", "test-replace-cm-values.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	min := uint(120)
	prop := v1alpha1.OptionalPipelineProperties{
		DefaultTimeoutMinutes:      &min,
		DefaultManagedByLabelValue: "abc-pipeline",
		DefaultCloudEventsSink:     "abc",
	}

	manifest, err = manifest.Transform(AddConfigMapValues("test2", prop))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[1].Object, cm)
	assertNoError(t, err)
	// ConfigMap will have only fields which are defined in `prop` OptionalPipelineProperties above
	assert.Equal(t, cm.Data["default-cloud-events-sink"], "abc")
	assert.Equal(t, cm.Data["default-timeout-minutes"], "120")
	assert.Equal(t, cm.Data["default-managed-by-label-value"], "abc-pipeline")
	// this was not defined in struct so will be missing from configmap
	assert.Equal(t, cm.Data["default-pod-template"], "")
}

// TestAddConfigMapValues_StructValues tests that the ConfigMap is created with the struct values marshalled into YAML format.
func TestAddConfigMapValues_StructValues(t *testing.T) {
	testData := path.Join("testdata", "test-struct-cm-values.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)
	prop := v1alpha1.TektonPrunerConfig{
		GlobalConfig: &config.GlobalConfig{
			PrunerConfig: config.PrunerConfig{
				SuccessfulHistoryLimit: ptr.Int32(123),
				HistoryLimit:           ptr.Int32(456),
			},
		},
	}
	manifest, err = manifest.Transform(AddConfigMapValues("tekton-pruner-default-spec", prop))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)
	// ConfigMap will have only fields which are defined in `prop` OptionalPipelineProperties above
	expectedValue, _ := yaml.Marshal(prop.GlobalConfig)
	assert.Equal(t, cm.Data["global-config"], string(expectedValue))
}

func TestAddConfigMapValues(t *testing.T) {
	configMapName := "test-add-config"
	configMapNameWithData := "test-add-config-with-data"
	testData := path.Join("testdata", "test-add-configmap-values.yaml")
	uintPtr := uint(2048)

	tests := []struct {
		name                string
		targetConfigMapName string
		props               interface{}
		expectedData        map[string]string
		keysShouldNotBeIn   []string
		doesConfigMapExists bool
	}{
		{
			name:                "verify-values",
			targetConfigMapName: configMapName,
			props: struct {
				StringValue  string   `json:"stringValue"`
				StringPtr    *string  `json:"stringPtr"`
				Int          int      `json:"int"`
				Int8         int8     `json:"int8"`
				Int16        int16    `json:"int16"`
				Int32        int32    `json:"int32"`
				Int64        int64    `json:"int64"`
				Int64Ptr     *int64   `json:"int64Ptr"`
				Uint         uint     `json:"uint"`
				Uint8        uint8    `json:"uint8"`
				Uint16       uint16   `json:"uint16"`
				Uint32       uint32   `json:"uint32"`
				Uint64       uint64   `json:"uint64"`
				UintPtr      *uint    `json:"uintPtr"`
				Float32      float32  `json:"float32"`
				Float64      float64  `json:"float64"`
				Float64Ptr   *float64 `json:"float64Ptr"`
				Bool         bool     `json:"bool"`
				BoolPtr      *bool    `json:"boolPtr"`
				NestedStruct struct {
					Foo string `json:"foo"`
				}
			}{
				StringValue: "foo",
				StringPtr:   ptr.String("fooPtr"),
				Int:         -256,
				Int8:        -128,
				Int16:       512,
				Int32:       2048,
				Int64:       4096,
				Uint:        256,
				Uint8:       254,
				Uint16:      512,
				Uint32:      1024,
				Uint64:      2048,
				Int64Ptr:    ptr.Int64(-2049),
				UintPtr:     &uintPtr,
				Float32:     512.512,
				Float64:     1024.1024567,
				Float64Ptr:  ptr.Float64(-1024.1023),
				Bool:        true,
				BoolPtr:     ptr.Bool(true),
				NestedStruct: struct {
					Foo string `json:"foo"`
				}{
					Foo: "level-1",
				},
			},
			doesConfigMapExists: true,
		},
		{
			name:                "verify-with-nil-values",
			targetConfigMapName: configMapName,
			props: struct {
				StringPtr    *string     `json:"stringPtr"`
				StringPtr2   *string     `json:"stringPtr2"`
				Int32Ptr     *int32      `json:"int32Ptr"`
				Int32Ptr2    *int32      `json:"int32Ptr2"`
				Int64Ptr     *int64      `json:"int64Ptr"`
				Int64Ptr2    *int64      `json:"int64Ptr2"`
				UintPtr      *uint       `json:"uint"`
				Uint8Ptr     *uint8      `json:"uint8"`
				Uint16Ptr    *uint16     `json:"uint16"`
				Uint32Ptr    *uint32     `json:"uint32"`
				Uint64Ptr    *uint64     `json:"uint64"`
				Float32Ptr   *float32    `json:"float32"`
				Float64Ptr   *float64    `json:"float64Ptr"`
				BoolPtr      *bool       `json:"boolPtr"`
				NestedStruct interface{} `json:"interfaceNil"`
			}{
				StringPtr2: ptr.String("hi"),
				Int32Ptr2:  ptr.Int32(21),
				Int64Ptr2:  ptr.Int64(22),
			},
			expectedData: map[string]string{
				"stringPtr2": "hi",
				"int32Ptr2":  "21",
				"int64Ptr2":  "22",
			},
			keysShouldNotBeIn: []string{
				"stringPtr",
				"int32Ptr",
				"int64Ptr",
				"uint",
				"uint8",
				"uint16",
				"uint32",
				"uint64",
				"float32",
				"float64Ptr",
				"boolPtr",
				"interfaceNil",
			},
			doesConfigMapExists: true,
		},
		{
			name:                "verify-values-with-existing-data",
			targetConfigMapName: configMapNameWithData,
			props: struct {
				Hello string `json:"hello"`
			}{
				Hello: "hi",
			},
			expectedData: map[string]string{
				"foo":   "bar", // existing data in the map
				"hello": "hi",
			},
			keysShouldNotBeIn:   []string{},
			doesConfigMapExists: true,
		},
		{
			name:                "verify-configmap-not-found",
			targetConfigMapName: "not-found-cm", // config map not exists
			props: struct {
				Name string `json:"name"`
			}{
				Name: "test",
			},
			doesConfigMapExists: false,
		},
		{
			name:                "verify-props-as-map",
			targetConfigMapName: configMapNameWithData,
			props:               map[string]string{"name": "hi"},
			expectedData:        map[string]string{"foo": "bar"},
			keysShouldNotBeIn:   []string{"name"},
			doesConfigMapExists: true,
		},
		{
			name:                "verify-nil-props",
			targetConfigMapName: configMapNameWithData,
			props:               nil,
			expectedData:        map[string]string{"foo": "bar"},
			doesConfigMapExists: true,
		},
		{
			name:                "verify-with-empty-string-pointer",
			targetConfigMapName: configMapName,
			props: struct {
				StringPtr      *string `json:"stringPtr"`
				NilStringPtr   *string `json:"nilStringPtr"`
				EmptyStringPtr *string `json:"emptyStringPtr"`
				String         string  `json:"string"`
				EmptyString    string  `json:"emptyString"`
			}{
				StringPtr:      ptr.String("hi"),
				NilStringPtr:   nil,
				EmptyStringPtr: ptr.String(""),
				String:         "hello",
				EmptyString:    "",
			},
			expectedData: map[string]string{
				"stringPtr":      "hi",
				"emptyStringPtr": "",
				"string":         "hello",
			},
			keysShouldNotBeIn: []string{
				"nilStringPtr",
				"emptyString",
			},
			doesConfigMapExists: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// get a manifest
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)

			manifest, err = manifest.Transform(AddConfigMapValues(test.targetConfigMapName, test.props))
			assert.NilError(t, err)

			var cm *corev1.ConfigMap
			for _, resource := range manifest.Resources() {
				if resource.GetKind() == "ConfigMap" && resource.GetName() == test.targetConfigMapName {
					cm = &corev1.ConfigMap{}
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, cm)
					assert.NilError(t, err)
				}
			}

			if test.doesConfigMapExists {
				assert.Equal(t, false, cm == nil, fmt.Sprintf("configMap[%s] not found and not loaded", test.targetConfigMapName))
			} else {
				assert.Equal(t, true, cm == nil, fmt.Sprintf("configMap[%s] found and loaded", test.targetConfigMapName))
				// return from here. no assertion needed
				return
			}

			for key, value := range test.expectedData {
				keyFound := false
				for targetMapKey, targetMapValue := range cm.Data {
					if targetMapKey == key {
						assert.Equal(t, value, targetMapValue, fmt.Sprintf("value not matching. key:[%s], value:[%s], expected:[%s], configMap:[%s]", targetMapKey, targetMapValue, value, test.targetConfigMapName))
						keyFound = true
						break
					}
				}
				assert.Equal(t, true, keyFound, fmt.Sprintf("key[%s] not found in configMap[%s]", key, test.targetConfigMapName))
			}

			// keys should not be in
			for targetMapKey, targetMapValue := range cm.Data {
				for _, keyShouldNotBeIn := range test.keysShouldNotBeIn {
					if keyShouldNotBeIn == targetMapKey {
						assert.Equal(t, false, keyShouldNotBeIn == targetMapKey, fmt.Sprintf("key should not be in, but found. key:[%s], value:[%s], configMap:[%s]", targetMapKey, targetMapValue, test.targetConfigMapName))
					}
				}
			}
		})
	}
}

func TestInjectLabelOnNamespace(t *testing.T) {
	t.Run("TestInjectLabel", func(t *testing.T) {
		testData := path.Join("testdata", "test-namespace-inject.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoError(t, err)
		newManifest, err := manifest.Transform(InjectLabelOnNamespace("operator.tekton.dev/disable-proxy=true"))
		assertNoError(t, err)
		for _, resource := range newManifest.Resources() {
			labels := resource.GetLabels()
			value, ok := labels["operator.tekton.dev/disable-proxy"]
			if ok {
				assert.DeepEqual(t, value, "true")
			}
			if !ok {
				t.Errorf("namespace did not have label")
			}
		}
	})
}

func TestAddConfiguration(t *testing.T) {
	testData := path.Join("testdata", "test-add-configurations.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	config := v1alpha1.Config{
		NodeSelector: map[string]string{
			"foo": "bar",
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "foo",
				Operator: "equals",
				Value:    "bar",
				Effect:   "noSchedule",
			},
		},
		PriorityClassName: string("system-cluster-critical"),
	}

	manifest, err = manifest.Transform(AddConfiguration(config))
	assertNoError(t, err)

	d := &v1beta1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoError(t, err)
	assert.Equal(t, d.Spec.Template.Spec.NodeSelector["foo"], config.NodeSelector["foo"])
	assert.Equal(t, d.Spec.Template.Spec.Tolerations[0].Key, config.Tolerations[0].Key)
	assert.Equal(t, d.Spec.Template.Spec.PriorityClassName, config.PriorityClassName)
}

func TestAddPSA(t *testing.T) {
	testData := path.Join("testdata", "test-add-psa.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	newManifest, err := manifest.Transform(AddDeploymentRestrictedPSA())
	assert.NilError(t, err)

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	testData = path.Join("testdata", "test-add-psa-expected.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	expected := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("failed to update deployment %s", diff.PrintWantGot(d))
	}
}

func TestAddStatefulSetPSA(t *testing.T) {
	testData := path.Join("testdata", "test-add-psa-statefulset.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	newManifest, err := manifest.Transform(AddStatefulSetRestrictedPSA())
	assert.NilError(t, err)

	got := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	testData = path.Join("testdata", "test-add-psa-expected-statefulset.yaml")
	expectedManifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	expected := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(expectedManifest.Resources()[0].Object, expected)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("failed to update deployment %s", diff.PrintWantGot(d))
	}
}

func TestCopyConfigMapValues(t *testing.T) {
	testData := path.Join("testdata", "test-resolver-config.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	expectedValues := map[string]string{
		"default-tekton-hub-catalog":        "abc-catalog",
		"default-artifact-hub-task-catalog": "some-random-catalog",
		"ignore-me-field":                   "ignore-me",
	}

	manifest, err = manifest.Transform(CopyConfigMap("hubresolver-config", expectedValues))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)
	// ConfigMap will have values changed only for field which are defined
	assert.Equal(t, cm.Data["default-tekton-hub-catalog"], "abc-catalog")
	assert.Equal(t, cm.Data["default-artifact-hub-task-catalog"], "some-random-catalog")

	// fields which are not defined in expected configmap will be same as before
	assert.Equal(t, cm.Data["default-kind"], "task")
	assert.Equal(t, cm.Data["default-type"], "artifact")

	// extra fields in expected configmap will be added
	assert.Equal(t, cm.Data["ignore-me-field"], "ignore-me")
}

func TestCopyEmptyConfigMapValues(t *testing.T) {
	testData := path.Join("testdata", "test-empty-config.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	expectedValues := map[string]string{
		"default-tekton-hub-catalog":        "abc-catalog",
		"default-artifact-hub-task-catalog": "some-random-catalog",
		"ignore-me-field":                   "ignore-me",
	}

	manifest, err = manifest.Transform(CopyConfigMap("empty-config", expectedValues))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)
	// ConfigMap will have all expected values
	assert.Equal(t, cm.Data["default-tekton-hub-catalog"], "abc-catalog")
	assert.Equal(t, cm.Data["default-artifact-hub-task-catalog"], "some-random-catalog")
	assert.Equal(t, cm.Data["ignore-me-field"], "ignore-me")
}

func TestCopyConfigMapWithEmptyExpectedValues(t *testing.T) {
	testData := path.Join("testdata", "test-empty-config.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	expectedValues := map[string]string{}

	manifest, err = manifest.Transform(CopyConfigMap("empty-config", expectedValues))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)
}

func TestCopyConfigMapWithWrongKind(t *testing.T) {
	testData := path.Join("testdata", "test-namespace-inject.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	expectedValues := map[string]string{}

	manifest, err = manifest.Transform(CopyConfigMap("tekton-pipelines", expectedValues))
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoError(t, err)
}

func TestReplaceDeploymentArg(t *testing.T) {
	testData := path.Join("testdata", "test-dashboard-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	existingArg := "--external-logs="
	newArg := "--external-logs=abc"

	newManifest, err := manifest.Transform(ReplaceDeploymentArg("tekton-dashboard", existingArg, newArg))
	assert.NilError(t, err)

	got := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, got)
	if err != nil {
		t.Errorf("failed to load deployment yaml")
	}

	found := false
	for _, a := range got.Spec.Template.Spec.Containers[0].Args {
		if a == newArg {
			found = true
		}
	}

	if !found {
		t.Fatalf("failed to find new arg in deployment")
	}
}

func TestReplaceNamespaceInServiceAccount(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-service-account.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	targetNamespace := "foobar"
	manifest, err = manifest.Transform(ReplaceNamespaceInServiceAccount(targetNamespace))
	assertNoError(t, err)

	for _, resource := range manifest.Resources() {
		if resource.GetNamespace() != targetNamespace {
			t.Errorf("namespace not updated")
		}
	}
}

func TestReplaceNamespaceInClusterRoleBinding(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-cluster-role-binding.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	targetNamespace := "foobar"
	manifest, err = manifest.Transform(ReplaceNamespaceInClusterRoleBinding(targetNamespace))
	assertNoError(t, err)

	for _, resource := range manifest.Resources() {
		crb := &rbacv1.ClusterRoleBinding{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, crb)
		assertNoError(t, err)

		for _, subject := range crb.Subjects {
			if subject.Namespace != targetNamespace {
				t.Errorf("namespace not updated")
			}
		}
	}
}

func TestCopyConfigMap(t *testing.T) {
	tests := []struct {
		name string
		data map[string]string
	}{
		{
			name: "force-update-disabled",
			data: map[string]string{
				"default-tekton-hub-catalog": "foo",
				"default-kind":               "pipeline",
			},
		},
		{
			name: "force-update-enabled",
			data: map[string]string{
				"default-tekton-hub-catalog": "foo",
				"default-kind":               "pipeline",
				"my_custom_field":            "secret_data",
			},
		},
	}

	configMapName := "hubresolver-config"

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// get a manifest
			testData := path.Join("testdata", "test-resolver-config.yaml")
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)

			manifest, err = manifest.Transform(CopyConfigMap(configMapName, test.data))
			assert.NilError(t, err)

			cm := &corev1.ConfigMap{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
			assert.NilError(t, err)

			for key, value := range test.data {
				keyFound := false
				for targetMapKey, targetMapValue := range cm.Data {
					if targetMapKey == key {
						assert.Equal(t, value, targetMapValue)
						keyFound = true
					}
				}
				assert.Equal(t, true, keyFound, fmt.Sprintf("key[%s] not found in config map", key))
			}
		})
	}
}

func TestReplaceNamespace(t *testing.T) {
	tests := []struct {
		name            string
		targetNamespace string
	}{
		{
			name:            "target-ns-foo",
			targetNamespace: "foo",
		},
		{
			name:            "target-ns-bar",
			targetNamespace: "bar",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// get a manifest
			testData := path.Join("testdata", "test-replace-namespace.yaml")
			manifest, err := mf.ManifestFrom(mf.Recursive(testData))
			assert.NilError(t, err)

			manifest, err = manifest.Transform(ReplaceNamespace(test.targetNamespace))
			assert.NilError(t, err)

			// verify the changes
			for _, resource := range manifest.Resources() {
				// assert namespace
				assert.Equal(t, test.targetNamespace, resource.GetNamespace())

				switch resource.GetKind() {
				case "ClusterRoleBinding":
					crb := &rbacv1.ClusterRoleBinding{}
					err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, crb)
					assert.NilError(t, err)
					// verify namespace
					for index := range crb.Subjects {
						assert.Equal(t, test.targetNamespace, crb.Subjects[index].Namespace)
					}
				}

			}
		})
	}
}

func TestAddSecretData(t *testing.T) {
	tests := []struct {
		name        string
		inputObj    *unstructured.Unstructured
		inputData   map[string][]byte
		inputAnnot  map[string]string
		expectedObj *unstructured.Unstructured
		expectError bool
	}{
		{
			name: "Add data to empty Secret",
			inputObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]interface{}{},
				},
			},
			inputData:  map[string][]byte{"key": []byte("value")},
			inputAnnot: map[string]string{"anno": "value"},
			expectedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"anno": "value",
						},
					},
					"data": map[string]interface{}{
						"key": base64.StdEncoding.EncodeToString([]byte("value")),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Do not overwrite existing data",
			inputObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]interface{}{},
					"data": map[string]interface{}{
						"existingKey": []byte("existingValue"),
					},
				},
			},
			inputData:  map[string][]byte{"newKey": []byte("newValue")},
			inputAnnot: map[string]string{"anno": "value"},
			expectedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"anno": "value",
						},
					},
					"data": map[string]interface{}{
						"existingKey": base64.StdEncoding.EncodeToString([]byte("existingValue")),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Non-Secret resource",
			inputObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata":   map[string]interface{}{},
				},
			},
			inputData:  map[string][]byte{"key": []byte("value")},
			inputAnnot: map[string]string{"anno": "value"},
			expectedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata":   map[string]interface{}{},
				},
			},
			expectError: false,
		},
		{
			name: "Empty input data",
			inputObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]interface{}{},
				},
			},
			inputData:  map[string][]byte{},
			inputAnnot: map[string]string{"anno": "value"},
			expectedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]interface{}{},
				},
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := AddSecretData(test.inputData, test.inputAnnot)(test.inputObj)

			if test.expectError {
				t.Errorf("Error %v", err)
			} else {
				assertNoError(t, err)
				// Remove creationTimestamp from both expected and actual objects
				if metadata, ok := test.expectedObj.Object["metadata"].(map[string]interface{}); ok {
					delete(metadata, "creationTimestamp")
				}
				if metadata, ok := test.inputObj.Object["metadata"].(map[string]interface{}); ok {
					delete(metadata, "creationTimestamp")
				}

				if diff := cmp.Diff(test.expectedObj.Object, test.inputObj.Object); diff != "" {
					t.Errorf("Objects do not match (-expected +actual):\n%s", diff)
				}
			}
		})
	}
}

func TestUpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(t *testing.T) {
	leaderElectionPipelineConfig := "config-leader-election-controller"
	pipelinesControllerDeployment := "tekton-pipelines-controller"
	pipelinesControllerContainer := "tekton-pipelines-controller"
	pipelineCR := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline",
			Namespace: "xyz",
		},
	}
	buckets := uint(2)
	workers := int(3)
	burst := int(33)
	pipelineCR.Spec.Performance.Buckets = &buckets
	pipelineCR.Spec.Performance.DisableHA = true
	pipelineCR.Spec.Performance.KubeApiQPS = ptr.Float32(40.03)
	pipelineCR.Spec.Performance.KubeApiBurst = &burst
	pipelineCR.Spec.Performance.ThreadsPerController = &workers

	depInput := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelinesControllerDeployment,
			Namespace: "xyz",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "hello"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "hello"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "hello",
							Image: "xyz",
						},
						{
							Name:  pipelinesControllerContainer,
							Image: "xyz",
							Args:  []string{"-flag1", "v1", "-flag2", "v2", "-disable-ha"},
						},
					},
				},
			},
		},
	}

	depExpected := depInput.DeepCopy()
	depExpected.Spec.Template.Labels = map[string]string{
		"app": "hello",
		"config-leader-election-controller.data.buckets": "2",
		"deployment.spec.replicas":                       "1",
	}

	depExpected.Spec.Template.Spec.Containers[1].Args = []string{
		"-flag1", "v1",
		"-flag2", "v2",
		"-disable-ha=true",
		"-kube-api-burst=33",
		"-kube-api-qps=40.03",
		"-threads-per-controller=3",
	}

	jsonBytes, err := json.Marshal(&depInput)
	assert.NilError(t, err)
	ud := &unstructured.Unstructured{}
	err = json.Unmarshal(jsonBytes, ud)
	assert.NilError(t, err)

	transformer := UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(&pipelineCR.Spec.Performance, leaderElectionPipelineConfig, pipelinesControllerDeployment, pipelinesControllerContainer)
	err = transformer(ud)
	assert.NilError(t, err)

	outDep := &appsv1.Deployment{}
	err = apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(ud.Object, outDep)
	assert.NilError(t, err)

	assert.Equal(t, true, reflect.DeepEqual(outDep, depExpected), fmt.Sprintf("transformed output:[%+v], expected:[%+v]", outDep, depExpected))
}

func TestGetSortedKeys(t *testing.T) {
	in := map[string]interface{}{
		"a1":  1,
		"z1":  false,
		"a2":  2,
		"a3":  3,
		"a10": 10,
		"a11": 11,
	}
	expectedOut := []string{"a1", "a10", "a11", "a2", "a3", "z1"}

	out := getSortedKeys(in)
	assert.Equal(t, true, reflect.DeepEqual(out, expectedOut))
}
