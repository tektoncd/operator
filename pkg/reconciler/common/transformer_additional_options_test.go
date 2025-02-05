/*
Copyright 2023 The Tekton Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestExecuteAdditionalOptionsTransformer(t *testing.T) {
	// context
	ctx := context.TODO()
	targetNamespace := "tekton-pipelines"
	ignorePolicy := admissionregistrationv1.Ignore
	failPolicy := admissionregistrationv1.Fail
	sideEffectUnknown := admissionregistrationv1.SideEffectClassUnknown

	// verify the changes applied on the manifest

	tcs := []struct {
		name                   string
		additionalOptions      v1alpha1.AdditionalOptions
		inputFilename          string
		expectedResultFilename string
	}{
		{
			name: "verify-disabled",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
			},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-base.yaml",
		},
		{
			name: "verify-disabled-with-nil",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: nil,
			},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-base.yaml",
		},
		{
			name: "test-configmap",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				ConfigMaps: map[string]corev1.ConfigMap{
					"config-defaults": {
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{"new-label": "foo"},
							Annotations: map[string]string{"custom-annotation": "hello"},
						},
						Data: map[string]string{
							"user-data1": "top-secret",
						},
					},
					"config-events": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"my-custom-label": "bar"},
						},
					},
					"config-tracing": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"component": "operator",
							},
							Annotations: map[string]string{
								"tracing-enabled": "true",
							},
						},
						Data: map[string]string{
							"hostname": "localhost",
							"port":     "14560",
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-configmap.yaml",
		},
		{
			name:                   "test-empty-options",
			additionalOptions:      v1alpha1.AdditionalOptions{},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-base.yaml",
		},
		{
			name: "test-deployment",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				Deployments: map[string]appsv1.Deployment{
					"tekton-pipelines-controller": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"controlled-by-options": "true",
							},
							Annotations: map[string]string{
								"hpa-enabled": "false",
							},
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: ptr.Int32(4),
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"label-foo": "label-bar",
									},
									Annotations: map[string]string{
										"annotation-foo": "annotation-bar",
									},
								},
								Spec: corev1.PodSpec{
									NodeSelector: map[string]string{
										"zone": "east",
									},
									Tolerations: []corev1.Toleration{
										{
											Key:      "zone",
											Operator: corev1.TolerationOpEqual,
											Value:    "west",
											Effect:   corev1.TaintEffectNoSchedule,
										},
									},
									TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
										{
											MaxSkew:           1,
											TopologyKey:       "kubernetes.io/hostname",
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"app": "foo",
												},
											},
											MatchLabelKeys: []string{"pod-template-hash"},
										},
									},
									Affinity: &corev1.Affinity{
										NodeAffinity: &corev1.NodeAffinity{
											RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
												NodeSelectorTerms: []corev1.NodeSelectorTerm{
													{
														MatchExpressions: []corev1.NodeSelectorRequirement{
															{
																Key:      "disktype",
																Operator: "In",
																Values:   []string{"ssd", "nvme", "ramdisk"},
															},
														},
													},
												},
											},
										},
									},
									PriorityClassName: "test",
									Volumes: []corev1.Volume{
										{
											Name: "my-custom-logs",
											VolumeSource: corev1.VolumeSource{
												HostPath: &corev1.HostPathVolumeSource{
													Path: "/var/custom/logs",
												},
											},
										},
										{
											Name: "config-logging",
											VolumeSource: corev1.VolumeSource{
												HostPath: &corev1.HostPathVolumeSource{
													Path: "/etc/config-logging",
												},
											},
										},
									},
									Containers: []corev1.Container{
										{
											Name: "tekton-pipelines-controller",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("4Gi"),
												},
											},
											Env: []corev1.EnvVar{
												{
													Name:  "ENV_FOO",
													Value: "bar",
												},
												{
													Name: "ENV_FROM_CONFIG_MAP",
													ValueFrom: &corev1.EnvVarSource{
														ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
															LocalObjectReference: corev1.LocalObjectReference{
																Name: "config-map-foo",
															},
															Key:      "foo",
															Optional: ptr.Bool(true),
														},
													},
												},
												{
													Name:  "CONFIG_LOGGING_NAME",
													Value: "pipeline-config-logging",
												},
											},
											Args: []string{
												"--disable-ha=false",
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "custom-mount",
													MountPath: "/etc/custom-mount",
												},
												{
													Name:      "config-logging",
													MountPath: "/etc/config-logging-tmp",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-deployment.yaml",
		},
		{
			name: "empty-labels-and-annotations",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				ConfigMaps: map[string]corev1.ConfigMap{
					"config-defaults": {
						ObjectMeta: metav1.ObjectMeta{
							Labels:      map[string]string{},
							Annotations: map[string]string{},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base.yaml",
			expectedResultFilename: "./testdata/test-additional-options-base.yaml",
		},
		{
			name: "test-statefulsets",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				StatefulSets: map[string]appsv1.StatefulSet{
					"web": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"controlled-by-options": "true",
							},
						},
						Spec: appsv1.StatefulSetSpec{
							Replicas:            ptr.Int32(4),
							ServiceName:         "www-n",
							PodManagementPolicy: appsv1.ParallelPodManagement,
							VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "www",
									},
									Spec: corev1.PersistentVolumeClaimSpec{
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteMany,
										},
										Resources: corev1.VolumeResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("2Gi"),
											},
										},
									},
								},
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "www-2",
									},
									Spec: corev1.PersistentVolumeClaimSpec{
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteMany,
										},
										Resources: corev1.VolumeResourceRequirements{
											Requests: corev1.ResourceList{
												"storage": resource.MustParse("4Gi"),
											},
										},
									},
								},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										"label-foo": "label-bar",
									},
									Annotations: map[string]string{
										"annotation-foo": "annotation-bar",
									},
								},
								Spec: corev1.PodSpec{
									NodeSelector: map[string]string{
										"zone": "east",
									},
									Tolerations: []corev1.Toleration{
										{
											Key:      "zone",
											Operator: corev1.TolerationOpEqual,
											Value:    "west",
											Effect:   corev1.TaintEffectNoSchedule,
										},
									},
									TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
										{
											MaxSkew:           1,
											TopologyKey:       "kubernetes.io/hostname",
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"app": "foo",
												},
											},
											MatchLabelKeys: []string{"pod-template-hash"},
										},
									},
									Affinity: &corev1.Affinity{
										NodeAffinity: &corev1.NodeAffinity{
											RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
												NodeSelectorTerms: []corev1.NodeSelectorTerm{
													{
														MatchExpressions: []corev1.NodeSelectorRequirement{
															{
																Key:      "disktype",
																Operator: "In",
																Values:   []string{"ssd", "nvme", "ramdisk"},
															},
														},
													},
												},
											},
										},
									},
									PriorityClassName: "test",
									Volumes: []corev1.Volume{
										{
											Name: "my-custom-logs",
											VolumeSource: corev1.VolumeSource{
												HostPath: &corev1.HostPathVolumeSource{
													Path: "/var/custom/logs",
												},
											},
										},
										{
											Name: "config-logging",
											VolumeSource: corev1.VolumeSource{
												HostPath: &corev1.HostPathVolumeSource{
													Path: "/etc/config-logging",
												},
											},
										},
									},
									Containers: []corev1.Container{
										{
											Name: "nginx",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("2"),
													corev1.ResourceMemory: resource.MustParse("4Gi"),
												},
											},
											Env: []corev1.EnvVar{
												{
													Name:  "ENV_FOO",
													Value: "bar",
												},
												{
													Name: "ENV_FROM_CONFIG_MAP",
													ValueFrom: &corev1.EnvVarSource{
														ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
															LocalObjectReference: corev1.LocalObjectReference{
																Name: "config-map-foo",
															},
															Key:      "foo",
															Optional: ptr.Bool(true),
														},
													},
												},
												{
													Name:  "CONFIG_LOGGING_NAME",
													Value: "pipeline-config-logging",
												},
											},
											Args: []string{
												"--mode=production",
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "custom-mount",
													MountPath: "/etc/custom-mount",
												},
												{
													Name:      "config-logging",
													MountPath: "/etc/config-logging-tmp",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-statefulsets.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-statefulsets.yaml",
		},
		{
			name: "test-hpa",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				HorizontalPodAutoscalers: map[string]autoscalingv2.HorizontalPodAutoscaler{
					"new-hpa": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"foo": "bar",
							},
							Annotations: map[string]string{
								"foo": "bar",
							},
						},
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "foo",
							},
							MinReplicas: ptr.Int32(2),
							MaxReplicas: 5,
							Metrics: []autoscalingv2.MetricSpec{
								{
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: "cpu",
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: ptr.Int32(100),
										},
									},
									Type: autoscalingv2.ResourceMetricSourceType,
								},
							},
						},
					},
					"existing-hpa": {
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"foo":    "bar",
								"label1": "value1",
							},
							Annotations: map[string]string{
								"foo":         "bar",
								"annotation1": "value1",
							},
						},
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "bar",
							},
							MinReplicas: ptr.Int32(3),
							MaxReplicas: 10,
							Metrics: []autoscalingv2.MetricSpec{
								{
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: "cpu",
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: ptr.Int32(80),
										},
									},
									Type: autoscalingv2.ResourceMetricSourceType,
								},
							},
							Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
								ScaleUp: &autoscalingv2.HPAScalingRules{
									StabilizationWindowSeconds: ptr.Int32(10),
								},
								ScaleDown: &autoscalingv2.HPAScalingRules{
									StabilizationWindowSeconds: ptr.Int32(20),
								},
							},
						},
					},
					"test-metrics": {
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "bar",
							},
							MinReplicas: nil,
							MaxReplicas: 0,
							Metrics: []autoscalingv2.MetricSpec{
								{
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: "cpu",
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: ptr.Int32(80),
										},
									},
									Type: autoscalingv2.ResourceMetricSourceType,
								},
							},
						},
					},
					"test-max-replicas": {
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							MinReplicas: nil,
							MaxReplicas: 9,
						},
					},
					"test-min-replicas": {
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							MinReplicas: ptr.Int32(2),
							MaxReplicas: 0,
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-hpa.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-hpa.yaml",
		},
		{
			name: "test-webhook",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				WebhookConfigurationOptions: map[string]v1alpha1.WebhookConfigurationOptions{
					"validation.webhook.pipeline.tekton.dev": {
						FailurePolicy:  &ignorePolicy,
						TimeoutSeconds: ptr.Int32(10),
						SideEffects:    &sideEffectUnknown,
					},
					"webhook.pipeline.tekton.dev": {
						FailurePolicy:  &failPolicy,
						TimeoutSeconds: ptr.Int32(10),
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-webhook.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-webhook.yaml",
		},
		{
			name: "test-runtimeclassname-for-deployments",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				Deployments: map[string]appsv1.Deployment{
					"tekton-pipelines-controller": {
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-deployment.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-deployment.yaml",
		},
		{
			name: "test-runtimeclassname-for-deployments-add-container",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				Deployments: map[string]appsv1.Deployment{
					"tekton-pipelines-controller": {
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
									Containers: []corev1.Container{
										{
											Name:  "kube-rbac-proxy",
											Image: "registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.12",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("500m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
											},
											Args: []string{
												"--secure-listen-address=0.0.0.0:9443",
												"--upstream=http://127.0.0.1:9090/",
												"--logtostderr=true",
												"--v=6",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-deployment.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-deployment-add-container.yaml",
		},
		{
			name: "test-runtimeclassname-for-deployments-add-init-container",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				Deployments: map[string]appsv1.Deployment{
					"tekton-pipelines-controller": {
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
									InitContainers: []corev1.Container{
										{
											Name:  "test-init-container",
											Image: "busybox:latest",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("100m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
											},
											Args: []string{
												"-c",
												"echo foo",
											},
											Command: []string{
												"/bin/bash",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-deployment.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-deployment-add-initcontainer.yaml",
		},
		{
			name: "test-runtimeclassname-for-statefulsets",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				StatefulSets: map[string]appsv1.StatefulSet{
					"web": {
						Spec: appsv1.StatefulSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
								},
							},
						},
					},
				},
				Deployments: map[string]appsv1.Deployment{
					"tekton-pipelines-controller": {
						Spec: appsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-statefulset.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-statefulset.yaml",
		},
		{
			name: "test-runtimeclassname-for-statefulsets-add-container",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				StatefulSets: map[string]appsv1.StatefulSet{
					"web": {
						Spec: appsv1.StatefulSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
									Containers: []corev1.Container{
										{
											Name:  "kube-rbac-proxy",
											Image: "registry.redhat.io/openshift4/ose-kube-rbac-proxy:v4.12",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("500m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
											},
											Args: []string{
												"--secure-listen-address=0.0.0.0:9443",
												"--upstream=http://127.0.0.1:9090/",
												"--logtostderr=true",
												"--v=6",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-statefulset.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-statefulset-add-container.yaml",
		},
		{
			name: "test-runtimeclassname-for-statefulsets-add-initContainers",
			additionalOptions: v1alpha1.AdditionalOptions{
				Disabled: ptr.Bool(false),
				StatefulSets: map[string]appsv1.StatefulSet{
					"web": {
						Spec: appsv1.StatefulSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									RuntimeClassName: ptr.String("foo"),
									InitContainers: []corev1.Container{
										{
											Name:  "test-init-container",
											Image: "busybox:latest",
											Resources: corev1.ResourceRequirements{
												Limits: corev1.ResourceList{
													corev1.ResourceCPU:    resource.MustParse("100m"),
													corev1.ResourceMemory: resource.MustParse("128Mi"),
												},
											},
											Args: []string{
												"-c",
												"echo foo",
											},
											Command: []string{
												"/bin/bash",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			inputFilename:          "./testdata/test-additional-options-base-runtimeclassname-statefulset.yaml",
			expectedResultFilename: "./testdata/test-additional-options-test-runtimeclassname-statefulset-add-initcontainer.yaml",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// fetch base manifests
			targetManifest, err := Fetch(tc.inputFilename)
			require.NoError(t, err)

			// fetch expected manifests
			expectedManifest, err := Fetch(tc.expectedResultFilename)
			require.NoError(t, err)

			// execute with additional options transformer
			err = ExecuteAdditionalOptionsTransformer(ctx, &targetManifest, targetNamespace, tc.additionalOptions)
			require.NoError(t, err)
			if d := cmp.Diff(expectedManifest.Resources(), targetManifest.Resources()); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
