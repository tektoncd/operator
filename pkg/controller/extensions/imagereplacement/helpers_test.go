package imagereplacement

import (
	"testing"

	v1alpha1 "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type imageReplacementTest struct {
	name       string
	containers []corev1.Container
	registry   v1alpha1.Registry
	argsIndex  []int8
	expected   []string
}

var updateDeploymentImageTests = []imageReplacementTest{
	{
		name: "ImageinContainer",
		containers: []corev1.Container{{
			Name:  "tekton-pipelines-controller",
			Image: "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0"},
		},
		registry: v1alpha1.Registry{
			Override: map[string]string{
				"tekton-pipelines-controller": "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:new-tag",
			},
		},
		expected: []string{"quay.io/openshift-pipeline/tektoncd-pipeline-webhook:new-tag"},
	},

	{
		name: "ImageinArgs",
		containers: []corev1.Container{{
			Name:  "tekton-pipelines-controller",
			Image: "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			Args:  []string{"-kubeconfig-writer-image", "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:v0.4.0"},
		}},
		registry: v1alpha1.Registry{
			Override: map[string]string{
				"-kubeconfig-writer-image": "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag",
			},
		},
		argsIndex: []int8{1},
		expected: []string{"quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			"quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag"},
	},

	{
		name: "ImageinArgsMultiple",
		containers: []corev1.Container{{
			Name:  "tekton-pipelines-controller",
			Image: "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			Args: []string{"-kubeconfig-writer-image", "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:v0.4.0",
				"-creds-image", "quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:v0.4.0"},
		}},
		registry: v1alpha1.Registry{
			Override: map[string]string{
				"-kubeconfig-writer-image": "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag",
				"-creds-image":             "quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:new-tag",
			},
		},
		argsIndex: []int8{1, 3},
		expected: []string{"quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			"quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag",
			"quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:new-tag"},
	},

	{
		name: "ImageinArgsandContainerMultiple",
		containers: []corev1.Container{{
			Name:  "tekton-pipelines-controller",
			Image: "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			Args: []string{"-kubeconfig-writer-image", "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:v0.4.0",
				"-creds-image", "quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:v0.4.0"},
		}},
		registry: v1alpha1.Registry{
			Override: map[string]string{
				"tekton-pipelines-controller": "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:new-tag",
				"-kubeconfig-writer-image":    "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag",
				"-creds-image":                "quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:new-tag",
			},
		},
		argsIndex: []int8{1, 3},
		expected: []string{"quay.io/openshift-pipeline/tektoncd-pipeline-webhook:new-tag",
			"quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:new-tag",
			"quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:new-tag"},
	},
	{
		name: "replaceNothing",
		containers: []corev1.Container{{
			Name:  "tekton-pipelines-controller",
			Image: "quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			Args: []string{"-kubeconfig-writer-image", "quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:v0.4.0",
				"-creds-image", "quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:v0.4.0"},
		}},
		registry: v1alpha1.Registry{
			Override: map[string]string{},
		},
		argsIndex: []int8{1, 3},
		expected: []string{"quay.io/openshift-pipeline/tektoncd-pipeline-webhook:v0.4.0",
			"quay.io/openshift-pipeline/tektoncd-pipeline-kubeconfigwriter:v0.4.0",
			"quay.io/openshift-pipeline/tektoncd-pipeline-creds-init:v0.4.0"},
	},
}

func TestUpdateDeploymentImage(t *testing.T) {
	for _, tt := range updateDeploymentImageTests {
		t.Run(tt.name, func(t *testing.T) {
			runUpdateDeploymentImageTest(t, tt)
		})
	}
}

func runUpdateDeploymentImageTest(t *testing.T, tt imageReplacementTest) {
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: tt.name,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: tt.containers,
				},
			},
		},
	}
	log := logf.Log.WithName(tt.name)
	logf.SetLogger(logf.ZapLogger(true))

	UpdateDeployment(&deployment, &tt.registry, log)

	expecteds := tt.expected
	// Assert container[*].image
	assertEqual(t, deployment.Spec.Template.Spec.Containers[0].Image, expecteds[0])
	expecteds = expecteds[1:]
	// Assert container[*].args
	for i, argsIndex := range tt.argsIndex {
		assertEqual(t, deployment.Spec.Template.Spec.Containers[0].Args[argsIndex], expecteds[i])
	}

}

func assertEqual(t *testing.T, actual, expected string) {
	if actual == expected {
		return
	}
	t.Fatalf("expected does not equal actual. \nExpected: %v\nActual: %v", expected, actual)
}
