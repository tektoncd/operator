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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

// reactor required for the GenerateName field to work when using the fake client
func generateNameReactor(action k8sTesting.Action) (bool, runtime.Object, error) {
	resource := action.(k8sTesting.CreateAction).GetObject()
	meta, ok := resource.(metav1.Object)
	if !ok {
		return false, resource, nil
	}

	if meta.GetName() == "" && meta.GetGenerateName() != "" {
		meta.SetName(SimpleNameGenerator.RestrictLengthWithRandomSuffix(meta.GetGenerateName()))
	}
	return false, resource, nil
}

func getTestKubeClient() *fake.Clientset {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two"}},
	)
	// add reactor to update generateName
	fakeClient.PrependReactor("create", "*", generateNameReactor)

	return fakeClient
}

func getTestTektonConfig() *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-operator-ns",
			},
			Pruner: v1alpha1.Prune{
				Resources:        []string{"pipelinerun", "taskrun"},
				PrunePerResource: false,
				Keep:             new(uint),
				KeepSince:        nil,
				Schedule:         "* * * * *",
			},
			Config: v1alpha1.Config{
				NodeSelector:      map[string]string{},
				Tolerations:       []corev1.Toleration{},
				PriorityClassName: "",
			},
		},
	}
}
func TestPrunerContainerImageEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		expectError bool
	}{
		{name: "TestWithImage", image: "tkn-image:tag-123", expectError: false},
		{name: "TestWithoutImage", image: "", expectError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(prunerContainerImageEnvKey, test.image)
			pruner, err := getPruner(context.Background(), getTestKubeClient(), getTestTektonConfig())
			assert.NoError(t, err)

			err = pruner.reconcile()
			if test.expectError {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.image, pruner.tknImage)
			}
		})
	}
}

func TestPrunerOwnerReference(t *testing.T) {
	t.Setenv(prunerContainerImageEnvKey, "tkn_image:tag-123")
	tc := getTestTektonConfig()
	tc.Name = "config"
	pruner, err := getPruner(context.Background(), getTestKubeClient(), tc)
	assert.NoError(t, err)
	ownerReference := pruner.ownerRef
	assert.Equal(t, "TektonConfig", ownerReference.Kind)
	assert.Equal(t, "config", ownerReference.Name)
}

func TestGeneratePrunerCommandArgs(t *testing.T) {
	getUintWithReference := func(value uint) *uint {
		return &value
	}
	tests := []struct {
		name                string
		pruneConfigs        []pruneConfig
		expectedCommandArgs string
	}{
		// single namespace
		{
			name: "TestSingleNamespace",
			pruneConfigs: []pruneConfig{
				{Namespace: "ns-1", Keep: getUintWithReference(21), KeepSince: getUintWithReference(100), Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: false},
			},
			expectedCommandArgs: "ns-1;--keep=21,--keep-since=100;pipelinerun,taskrun;false",
		},
		// multiple namespaces
		{
			name: "TestMultipleNamespaces",
			pruneConfigs: []pruneConfig{
				{Namespace: "ns-a", Keep: getUintWithReference(21), KeepSince: getUintWithReference(100), Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: false},
				{Namespace: "ns-b", Keep: getUintWithReference(12), KeepSince: getUintWithReference(99), Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: false},
				{Namespace: "ns-c", Keep: getUintWithReference(13), KeepSince: getUintWithReference(101), Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: true},
				{Namespace: "ns-d", Keep: getUintWithReference(14), KeepSince: getUintWithReference(102), Resources: []string{"pipelinerun"}, PrunePerResource: true},
				{Namespace: "ns-e", Keep: getUintWithReference(15), KeepSince: getUintWithReference(103), Resources: []string{"taskrun"}, PrunePerResource: true},
			},
			expectedCommandArgs: "ns-a;--keep=21,--keep-since=100;pipelinerun,taskrun;false ns-b;--keep=12,--keep-since=99;pipelinerun,taskrun;false ns-c;--keep=13,--keep-since=101;pipelinerun,taskrun;true ns-d;--keep=14,--keep-since=102;pipelinerun;true ns-e;--keep=15,--keep-since=103;taskrun;true",
		},
		// multiple namespaces with nil values
		{
			name: "TestMultipleNamespacesWithNilValues",
			pruneConfigs: []pruneConfig{
				{Namespace: "ns-a", Keep: nil, KeepSince: getUintWithReference(100), Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: false},
				{Namespace: "ns-b", Keep: getUintWithReference(12), KeepSince: nil, Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: false},
				{Namespace: "ns-c", Keep: getUintWithReference(13), KeepSince: nil, Resources: []string{"pipelinerun", "taskrun"}, PrunePerResource: true},
				{Namespace: "ns-d", Keep: getUintWithReference(14), KeepSince: nil, Resources: []string{"pipelinerun"}, PrunePerResource: true},
				{Namespace: "ns-e", Keep: nil, KeepSince: getUintWithReference(103), Resources: []string{"taskrun"}, PrunePerResource: true},
			},
			expectedCommandArgs: "ns-a;--keep-since=100;pipelinerun,taskrun;false ns-b;--keep=12;pipelinerun,taskrun;false ns-c;--keep=13;pipelinerun,taskrun;true ns-d;--keep=14;pipelinerun;true ns-e;--keep-since=103;taskrun;true",
		},
	}

	t.Setenv(prunerContainerImageEnvKey, "tkn_image:tag-123")
	pruner, err := getPruner(context.Background(), getTestKubeClient(), getTestTektonConfig())
	assert.NoError(t, err)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualCommandArgs := pruner.generatePrunerCommandArgs(test.pruneConfigs)
			assert.Equal(t, test.expectedCommandArgs, actualCommandArgs)
		})
	}
}

func TestPrunerReconcile(t *testing.T) {
	ctx := context.Background()
	getUintWithReference := func(value uint) *uint {
		return &value
	}
	type reconciles struct {
		name            string
		applyChanges    func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func()
		scheduleAndArgs map[string]string // schedule and command args
	}
	tests := []struct {
		name              string
		tknContainerImage string
		targetNamespace   string
		tektonConfig      *v1alpha1.TektonConfig
		client            *fake.Clientset
		reconciles        []reconciles
	}{
		{
			name:              "TestCronReconcile",
			tknContainerImage: "my-custom-tkn-image:tag-v1.0.0",
			targetNamespace:   "test",
			client:            getTestKubeClient(),
			tektonConfig:      getTestTektonConfig(),
			reconciles: []reconciles{
				{ // startup - reconcile #1
					name: "TestGlobalConfig",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner = v1alpha1.Prune{
							Keep:             getUintWithReference(10),
							KeepSince:        nil,
							Resources:        []string{"pipelinerun", "taskrun"},
							Schedule:         "* * * * *",
							PrunePerResource: false,
						}
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *": "ns-one;--keep=10;pipelinerun,taskrun;false ns-two;--keep=10;pipelinerun,taskrun;false",
					},
				},
				{ // reconcile #2
					name: "TestAddNamespaceWithSchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						// add a prune config to a namespace
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "t2-reconcile-2",
							Annotations: map[string]string{
								pruneAnnotationKeep:      "1",
								pruneAnnotationResources: "pipelinerun",
								pruneAnnotationStrategy:  "keep",
								pruneAnnotationSchedule:  "*/2 * * * *",
							},
						}}
						_, err := client.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *":   "ns-one;--keep=10;pipelinerun,taskrun;false ns-two;--keep=10;pipelinerun,taskrun;false",
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false",
					},
				},
				{ // reconcile #3
					name: "TestUpdateTektonConfigKeepAndAddNamespaceWithSameSchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						// change keep in tekton config
						tektonConfig.Spec.Pruner.Keep = getUintWithReference(20)

						// add a prune config to a namespace
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "t2-reconcile-3",
							Annotations: map[string]string{
								pruneAnnotationKeepSince:        "100",
								pruneAnnotationResources:        "pipelinerun",
								pruneAnnotationStrategy:         "keep-since",
								pruneAnnotationSchedule:         "*/2 * * * *",
								pruneAnnotationPrunePerResource: "true",
							},
						}}
						_, err := client.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *":   "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false",
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #4
					name: "TestAddNamespaceWithDifferentSchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						// add a prune config to a namespace
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "t2-reconcile-4",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationKeepSince:        "99",
								pruneAnnotationResources:        "pipelinerun,taskrun",
								pruneAnnotationSchedule:         "*/4 * * * *",
								pruneAnnotationPrunePerResource: "true",
							},
						}}
						_, err := client.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *":   "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false",
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
						"*/4 * * * *": "t2-reconcile-4;--keep=20,--keep-since=99;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #5
					name: "TestAddNamespaceWithoutAnnotations",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						// add a namespace without annotations
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "t2-reconcile-5"}}
						_, err := client.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *":   "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false t2-reconcile-5;--keep=20;pipelinerun,taskrun;false",
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
						"*/4 * * * *": "t2-reconcile-4;--keep=20,--keep-since=99;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #6
					name: "TestUpdateGlobalConfigSchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Schedule = "*/10 * * * *"
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/10 * * * *": "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false t2-reconcile-5;--keep=20;pipelinerun,taskrun;false",
						"*/2 * * * *":  "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
						"*/4 * * * *":  "t2-reconcile-4;--keep=20,--keep-since=99;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #7
					name: "TestDeleteNamespaces",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						// add a namespace without annotations
						err := client.CoreV1().Namespaces().Delete(ctx, "ns-two", metav1.DeleteOptions{})
						assert.NoError(t, err)
						err = client.CoreV1().Namespaces().Delete(ctx, "t2-reconcile-4", metav1.DeleteOptions{})
						assert.NoError(t, err)
						err = client.CoreV1().Namespaces().Delete(ctx, "t2-reconcile-5", metav1.DeleteOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/10 * * * *": "ns-one;--keep=20;pipelinerun,taskrun;false",
						"*/2 * * * *":  "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #8
					name: "TestNamespaceAnnotations",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "true",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *":  "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
						"*/12 * * * *": "ns-one;--keep=20;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #9
					name: "TestNamespaceWithInvalidResource",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "false",
								pruneAnnotationSkip:             "false",
								pruneAnnotationResources:        "hello-resource", // invalid resource
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #10
					name: "TestNamespaceKeepWithZero",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "false",
								pruneAnnotationSkip:             "false",
								pruneAnnotationResources:        "pipelinerun, taskrun",
								pruneAnnotationKeep:             "0",
								pruneAnnotationKeepSince:        "20",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #10
					name: "TestNamespaceKeepSinceWithZero",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "false",
								pruneAnnotationSkip:             "false",
								pruneAnnotationResources:        "pipelinerun, taskrun",
								pruneAnnotationKeep:             "20",
								pruneAnnotationKeepSince:        "0",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #11
					name: "TestNamespaceInvalidKeepValue",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "false",
								pruneAnnotationSkip:             "false",
								pruneAnnotationResources:        "pipelinerun, taskrun",
								pruneAnnotationKeep:             "hello",
								pruneAnnotationKeepSince:        "100",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #12
					name: "TestNamespaceInvalidKeepSinceValue",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "false",
								pruneAnnotationSkip:             "false",
								pruneAnnotationResources:        "pipelinerun, taskrun",
								pruneAnnotationKeep:             "10",
								pruneAnnotationKeepSince:        "hi",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #13
					name: "TestNamespaceSkipDisabledAnnotations",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "true",
								pruneAnnotationSkip:             "false",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *":  "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
						"*/12 * * * *": "ns-one;--keep=20;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #14
					name: "TestNamespaceSkipEnabledAnnotations",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-one",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "true",
								pruneAnnotationSkip:             "true",
							},
						}}
						_, err := client.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #15
					name: "TestOutdatedCronDeletion",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						cronName := "outdated-cron-job"
						cronJob := batchv1.CronJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      cronName,
								Namespace: tektonConfig.Spec.TargetNamespace,
								Labels:    map[string]string{pruneCronLabel: "true"}, // label is the key to identify the right job
							},
							Spec: batchv1.CronJobSpec{
								Schedule: "* * * * *",
								JobTemplate: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name:  "hello",
														Image: "my-custom-image:tag-123",
													},
												},
											},
										},
									},
								},
							},
						}
						_, err := client.BatchV1().CronJobs(tektonConfig.Spec.TargetNamespace).Create(ctx, &cronJob, metav1.CreateOptions{})
						assert.NoError(t, err)
						// verify that job deleted successfully
						verifyCronExistence := func() {
							receivedCron, err := client.BatchV1().CronJobs(tektonConfig.Spec.TargetNamespace).Get(ctx, cronName, metav1.GetOptions{})
							assert.True(t, apierrors.IsNotFound(err))
							assert.Nil(t, receivedCron)
						}
						return verifyCronExistence
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
				{ // reconcile #16
					name: "TestIrrelevantCronAddition",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						cronName := "my-cluster-cron-job"
						cronJob := batchv1.CronJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      cronName,
								Namespace: tektonConfig.Spec.TargetNamespace,
							},
							Spec: batchv1.CronJobSpec{
								Schedule: "* * * * *",
								JobTemplate: batchv1.JobTemplateSpec{
									Spec: batchv1.JobSpec{
										Template: corev1.PodTemplateSpec{
											Spec: corev1.PodSpec{
												Containers: []corev1.Container{
													{
														Name:  "hello",
														Image: "my-custom-image:tag-123",
													},
												},
											},
										},
									},
								},
							},
						}
						_, err := client.BatchV1().CronJobs(tektonConfig.Spec.TargetNamespace).Create(ctx, &cronJob, metav1.CreateOptions{})
						assert.NoError(t, err)
						// verify that the job is retained
						verifyCronExistence := func() {
							receivedCron, err := client.BatchV1().CronJobs(tektonConfig.Spec.TargetNamespace).Get(ctx, cronName, metav1.GetOptions{})
							assert.NoError(t, err)
							assert.NotNil(t, receivedCron)
							assert.Equal(t, cronName, receivedCron.GetName())
						}
						return verifyCronExistence
					},
					scheduleAndArgs: map[string]string{
						"*/2 * * * *": "t2-reconcile-2;--keep=1;pipelinerun;false t2-reconcile-3;--keep-since=100;pipelinerun;true",
					},
				},
			},
		},
		{
			name:              "TestPrunerConfig",
			tknContainerImage: "my-custom-tkn-image:tag-v1.0.0",
			targetNamespace:   "tekton-operator",
			client:            getTestKubeClient(),
			tektonConfig:      getTestTektonConfig(),
			reconciles: []reconciles{
				{ // startup - reconcile #1
					name: "TestPrunerEnabled",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner = v1alpha1.Prune{
							Disabled:         false,
							Keep:             getUintWithReference(20),
							KeepSince:        nil,
							Resources:        []string{"pipelinerun", "taskrun"},
							Schedule:         "* * * * *",
							PrunePerResource: false,
						}
						// add an annotation to a namespace
						ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
							Name: "ns-seven",
							Annotations: map[string]string{
								// strategy is empty, keep should be taken from tektonConfig
								pruneAnnotationSchedule:         "*/12 * * * *",
								pruneAnnotationPrunePerResource: "true",
								pruneAnnotationSkip:             "false",
								pruneAnnotationKeepSince:        "42",
							},
						}}
						_, err := client.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
						assert.NoError(t, err)
						return nil
					},
					scheduleAndArgs: map[string]string{
						"* * * * *":    "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #2
					// only global schedule will be disable, if a namespace has prune schedule annotation, there will be a job for that
					name: "TestPrunerGlobalScheduleDisabledWithEmptySchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Schedule = ""
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #3
					// re-enables global schedule
					name: "TestPrunerReEnabledSchedule",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Schedule = "*/5 * * * *"
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #4
					// removes global resources
					name: "TestPrunerWithGlobalEmptyResources",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Resources = []string{}
						return nil
					},
					// default prune resources will be taken
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=20;pipelinerun;false ns-two;--keep=20;pipelinerun;false",
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun;true",
					},
				},
				{ // reconcile #5
					// adds global resources
					name: "TestPrunerWithGlobalResources",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Resources = []string{"pipelinerun"}
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=20;pipelinerun;false ns-two;--keep=20;pipelinerun;false",
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun;true",
					},
				},
				{ // reconcile #6
					// modifies global resources
					name: "TestPrunerUpdateGlobalResources",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Resources = []string{"pipelinerun", "taskrun"}
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=20;pipelinerun,taskrun;false ns-two;--keep=20;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=20,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #7
					// removes global keep and keepSince
					name: "TestPrunerWithNilArguments",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Keep = nil
						tektonConfig.Spec.Pruner.KeepSince = nil
						return nil
					},
					// uses default keep value
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=100;pipelinerun,taskrun;false ns-two;--keep=100;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=100,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #9
					// removes global resources
					name: "TestPrunerWithWithEmptyResources",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Resources = nil
						return nil
					},
					// takes default resources
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=100;pipelinerun;false ns-two;--keep=100;pipelinerun;false",
						"*/12 * * * *": "ns-seven;--keep=100,--keep-since=42;pipelinerun;true",
					},
				},
				{ // reconcile #10
					// adds global resources
					name: "TestPrunerWithWithResources",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Resources = []string{"pipelinerun", "taskrun"}
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=100;pipelinerun,taskrun;false ns-two;--keep=100;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=100,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
				{ // reconcile #11
					name: "TestPrunerDisableAll",
					// disables pruner feature
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Disabled = true
						return nil
					},
					scheduleAndArgs: map[string]string{},
				},
				{ // reconcile #12
					// re enables pruner feature
					name: "TestPrunerReEnable",
					applyChanges: func(tektonConfig *v1alpha1.TektonConfig, client *fake.Clientset, t *testing.T) func() {
						tektonConfig.Spec.Pruner.Disabled = false
						return nil
					},
					scheduleAndArgs: map[string]string{
						"*/5 * * * *":  "ns-one;--keep=100;pipelinerun,taskrun;false ns-two;--keep=100;pipelinerun,taskrun;false",
						"*/12 * * * *": "ns-seven;--keep=100,--keep-since=42;pipelinerun,taskrun;true",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { // run a individual test
			t.Setenv(prunerContainerImageEnvKey, test.tknContainerImage)
			// update namespace
			test.tektonConfig.Spec.CommonSpec.TargetNamespace = test.targetNamespace

			pruner, err := getPruner(ctx, test.client, test.tektonConfig)
			assert.NoError(t, err)
			assert.Equal(t, test.targetNamespace, pruner.targetNamespace)

			// perform test with reconcile
			for _, reconcile := range test.reconciles {
				t.Run(reconcile.name, func(t *testing.T) { // run as individual test
					// apply changes
					var callbackFunc func()
					if reconcile.applyChanges != nil {
						callbackFunc = reconcile.applyChanges(test.tektonConfig, test.client, t)
					}

					err = pruner.reconcile()
					assert.NoError(t, err)

					// custom call back function, to assert custom things
					if callbackFunc != nil {
						callbackFunc()
					}

					// verify image taken from environment variable
					assert.Equal(t, test.tknContainerImage, pruner.tknImage)

					labelSelector := fmt.Sprintf("%s=true", pruneCronLabel)
					cronJobsList, err := test.client.BatchV1().CronJobs(pruner.targetNamespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
					assert.NoError(t, err)
					assert.Equal(t, len(reconcile.scheduleAndArgs), len(cronJobsList.Items))
					for _, cronJob := range cronJobsList.Items {
						podSpec := cronJob.Spec.JobTemplate.Spec.Template.Spec

						// verify command, for now we have only one podSpec in the pod
						container := podSpec.Containers[0]
						assert.Equal(t, []string{"/bin/sh", "-c", prunerCommand}, container.Command)

						// verify container image
						assert.Equal(t, pruner.tknImage, container.Image)

						// verify schedule
						actualSchedule := cronJob.Spec.Schedule
						expectedArgs, found := reconcile.scheduleAndArgs[actualSchedule]
						assert.True(t, found, "schedule not found", actualSchedule)

						// verify command args, args index 0 holds "-s", so taking index 1
						actualArgs := container.Args[1]
						assert.Equal(t, expectedArgs, actualArgs)

						// remove verified schedule from map
						delete(reconcile.scheduleAndArgs, actualSchedule)

						// verify service account name
						assert.Equal(t, prunerServiceAccountName, podSpec.ServiceAccountName)

						// verify toleration, nodeSelector, priorityClassName
						assert.Equal(t, test.tektonConfig.Spec.Config.Tolerations, podSpec.Tolerations)
						assert.Equal(t, test.tektonConfig.Spec.Config.NodeSelector, podSpec.NodeSelector)
						assert.Equal(t, test.tektonConfig.Spec.Config.PriorityClassName, podSpec.PriorityClassName)
					}

					// confirm all the schedules verified
					assert.Empty(t, reconcile.scheduleAndArgs)
				})
			}
		})
	}
}
