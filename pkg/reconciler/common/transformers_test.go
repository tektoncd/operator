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
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(map[string]string{}))
		assertNoEror(t, err)

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
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoEror(t, err)
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
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoEror(t, err)
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
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(DeploymentImages(images))
		assertNoEror(t, err)
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
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(TaskImages(images))
		assertNoEror(t, err)
		assertTaskImage(t, newManifest.Resources(), "push", image)
		assertTaskImage(t, newManifest.Resources(), "build", "$(inputs.params.BUILDER_IMAGE)")
	})

	t.Run("replace task addons param image", func(t *testing.T) {
		paramName := ParamPrefix + "builder_image"
		image := "foo.bar/image/buildah"
		images := map[string]string{
			paramName: image,
		}
		testData := path.Join("testdata", "test-replace-addon-image.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(TaskImages(images))
		assertNoEror(t, err)
		assertParamHasImage(t, newManifest.Resources(), "BUILDER_IMAGE", image)
		assertTaskImage(t, newManifest.Resources(), "push", "buildah")
	})
}

func assertNoEror(t *testing.T, err error) {
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

func TestReplaceNamespaceInDeploymentEnv(t *testing.T) {
	testData := path.Join("testdata", "test-replace-env-in-result-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	manifest, err = manifest.Transform(ReplaceNamespaceInDeploymentEnv("openshift-pipelines"))
	assertNoEror(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoEror(t, err)

	env := d.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, env[0].Value, "tcp")
	assert.Equal(t, env[1].Value, "tekton-results-mysql.openshift-pipelines.svc.cluster.local")
}

func TestReplaceNamespaceInDeploymentArgs(t *testing.T) {
	testData := path.Join("testdata", "test-replace-arg-in-result-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	manifest, err = manifest.Transform(ReplaceNamespaceInDeploymentArgs("openshift-pipelines"))
	assertNoEror(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoEror(t, err)

	args := d.Spec.Template.Spec.Containers[0].Args
	assert.Equal(t, args[0], "-api_addr")
	assert.Equal(t, args[1], "tekton-results-api-service.openshift-pipelines.svc.cluster.local:50051")
}

func TestReplaceNamespaceInClusterInterceptor(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-cluster-interceptor.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	manifest, err = manifest.Transform(injectNamespaceCRClusterInterceptorClientConfig("foobar"))
	assertNoEror(t, err)

	clusterInterceptor := manifest.Resources()[0].Object
	service, _, err := unstructured.NestedFieldNoCopy(clusterInterceptor, "spec", "clientConfig", "service")
	m := service.(map[string]interface{})
	assertNoEror(t, err)
	assert.Equal(t, "foobar", m["namespace"])
}

func TestReplaceNamespaceInClusterRole(t *testing.T) {
	testData := path.Join("testdata", "test-replace-namespace-in-cluster-role.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	manifest, err = manifest.Transform(injectNamespaceClusterRole("foobar"))
	assertNoEror(t, err)

	clusterRole := manifest.Resources()[0].Object
	rules, _, err := unstructured.NestedSlice(clusterRole, "rules")
	assertNoEror(t, err)
	//  The file has 3 rules — hard-coding this a bit
	podsecuritypolicyRule := rules[0].(map[string]interface{})
	for _, name := range podsecuritypolicyRule["resourceNames"].([]interface{}) {
		if name.(string) != "tekton-pipelines" {
			t.Errorf("Replace 'tekton-pipelines' in the wrong rule (podsecuritypolicies)")
		}
	}
	namespaceRule := rules[1].(map[string]interface{})
	for _, name := range namespaceRule["resourceNames"].([]interface{}) {
		if name.(string) != "foobar" {
			t.Errorf("Didn't replace 'tekton-pipelines' in the namespace rule")
		}
	}
	namespaceFinalizerRule := rules[2].(map[string]interface{})
	for _, name := range namespaceFinalizerRule["resourceNames"].([]interface{}) {
		if name.(string) != "foobar" {
			t.Errorf("Didn't replace 'tekton-pipelines' in the namespace rule")
		}
	}
}

func TestAddConfigMapValues_PipelineProperties(t *testing.T) {

	testData := path.Join("testdata", "test-replace-cm-values.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	prop := v1alpha1.PipelineProperties{
		EnableTektonOciBundles: ptr.Bool(true),
		EnableApiFields:        "stable",
	}

	manifest, err = manifest.Transform(AddConfigMapValues("test1", prop))
	assertNoEror(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoEror(t, err)

	assert.Equal(t, cm.Data["foo"], "bar")
	assert.Equal(t, cm.Data["enable-tekton-oci-bundles"], "true")
	assert.Equal(t, cm.Data["enable-api-fields"], "stable")
}

func TestAddConfigMapValues_OptionalPipelineProperties(t *testing.T) {

	testData := path.Join("testdata", "test-replace-cm-values.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	min := uint(120)
	prop := v1alpha1.OptionalPipelineProperties{
		DefaultTimeoutMinutes:      &min,
		DefaultManagedByLabelValue: "abc-pipeline",
		DefaultCloudEventsSink:     "abc",
	}

	manifest, err = manifest.Transform(AddConfigMapValues("test2", prop))
	assertNoEror(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[1].Object, cm)
	assertNoEror(t, err)
	// ConfigMap will have only fields which are defined in `prop` OptionalPipelineProperties above
	assert.Equal(t, cm.Data["default-cloud-events-sink"], "abc")
	assert.Equal(t, cm.Data["default-timeout-minutes"], "120")
	assert.Equal(t, cm.Data["default-managed-by-label-value"], "abc-pipeline")
	// this was not defined in struct so will be missing from configmap
	assert.Equal(t, cm.Data["default-pod-template"], "")
}

func TestInjectLabelOnNamespace(t *testing.T) {
	t.Run("TestInjectLabel", func(t *testing.T) {
		testData := path.Join("testdata", "test-namespace-inject.yaml")

		manifest, err := mf.ManifestFrom(mf.Recursive(testData))
		assertNoEror(t, err)
		newManifest, err := manifest.Transform(InjectLabelOnNamespace("operator.tekton.dev/disable-proxy=true"))
		assertNoEror(t, err)
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
	assertNoEror(t, err)

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
	assertNoEror(t, err)

	d := &v1beta1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoEror(t, err)
	assert.Equal(t, d.Spec.Template.Spec.NodeSelector["foo"], config.NodeSelector["foo"])
	assert.Equal(t, d.Spec.Template.Spec.Tolerations[0].Key, config.Tolerations[0].Key)
	assert.Equal(t, d.Spec.Template.Spec.PriorityClassName, config.PriorityClassName)
}

func TestHighAvailabilityDeploymentResourceTransform(t *testing.T) {

	testData := path.Join("testdata", "test-add-configurations.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	replicasNum := int32(3)
	containerOverride := []v1alpha1.ContainerOverride{
		{
			Name: "controller-deployment",
			Env: []corev1.EnvVar{
				{
					Name:  "KUBERNETES_MIN_VERSION",
					Value: "v1.23.0",
				},
			},
			Args: []string{
				"-kube-api-qps", "50",
			},
			Resource: corev1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("2"),
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("4"),
				},
			},
		},
	}

	config := v1alpha1.Config{
		HighAvailability: v1alpha1.HighAvailability{
			Replicas: &replicasNum,
		},
		DeploymentOverride: []v1alpha1.DeploymentOverride{
			{
				Name:       "controller",
				Containers: containerOverride,
			},
		},
	}

	manifest, err = manifest.Transform(HighAvailabilityTransform(config.HighAvailability))
	assertNoEror(t, err)
	manifest, err = manifest.Transform(DeploymentOverrideTransform(config.DeploymentOverride))
	assertNoEror(t, err)

	d := &v1beta1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoEror(t, err)

	assert.Equal(t, *d.Spec.Replicas, int32(replicasNum))
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU], resource.MustParse("1"))
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU], resource.MustParse("2"))
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory], resource.MustParse("2"))
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory], resource.MustParse("4"))

	assert.Equal(t, len(d.Spec.Template.Spec.Containers[0].Args), 5)
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Args[3], "-kube-api-qps")
	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Args[4], "50")

	assert.Equal(t, d.Spec.Template.Spec.Containers[0].Env[0], corev1.EnvVar{
		Name:  "KUBERNETES_MIN_VERSION",
		Value: "v1.23.0",
	})
}

func TestDeploymentReplicasOverrideTransform(t *testing.T) {

	testData := path.Join("testdata", "test-add-configurations.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoEror(t, err)

	HAreplicas := int32(3)
	overReplicas := int32(2)

	config := v1alpha1.Config{
		HighAvailability: v1alpha1.HighAvailability{
			Replicas: &HAreplicas,
		},
		DeploymentOverride: []v1alpha1.DeploymentOverride{
			{
				Name:     "controller",
				Replicas: &overReplicas,
			},
		},
	}

	manifest, err = manifest.Transform(HighAvailabilityTransform(config.HighAvailability))
	assertNoEror(t, err)
	manifest, err = manifest.Transform(DeploymentOverrideTransform(config.DeploymentOverride))
	assertNoEror(t, err)

	d := &v1beta1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, d)
	assertNoEror(t, err)

	assert.Equal(t, *d.Spec.Replicas, overReplicas)

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
	assertNoEror(t, err)

	expectedValues := map[string]string{
		"default-tekton-hub-catalog":        "abc-catalog",
		"default-artifact-hub-task-catalog": "some-random-catalog",
		"ignore-me-field":                   "ignore-me",
	}

	manifest, err = manifest.Transform(CopyConfigMap("hubresolver-config", expectedValues))
	assertNoEror(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assertNoEror(t, err)
	// ConfigMap will have values changed only for field which are defined
	assert.Equal(t, cm.Data["default-tekton-hub-catalog"], "abc-catalog")
	assert.Equal(t, cm.Data["default-artifact-hub-task-catalog"], "some-random-catalog")

	// fields which are not defined in expected configmap will be same as before
	assert.Equal(t, cm.Data["default-kind"], "task")
	assert.Equal(t, cm.Data["default-type"], "artifact")

	// extra fields in expected configmap will be ignore and will not be added
	assert.Equal(t, cm.Data["ignore-me-field"], "")

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
