package tektonconfig

import (
	"context"
	"os"
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	fakesecurity "github.com/openshift/client-go/security/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// ---------------------------------------------------------------------------
// createResources
// ---------------------------------------------------------------------------

// TestCreateResources_EnsuresPrerequisites verifies that createResources calls
// ensurePreRequisites and returns RECONCILE_AGAIN_ERR when the InstallerSet
// does not yet exist.
func TestCreateResources_EnsuresPrerequisites(t *testing.T) {
	os.Setenv(common.KoEnvKey, "testdata")

	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
		Spec: v1alpha1.TektonConfigSpec{
			Platforms: v1alpha1.Platforms{
				OpenShift: v1alpha1.OpenShift{
					SCC:           &v1alpha1.SCC{Default: "pipelines-scc"},
					NamespaceSync: &v1alpha1.NamespaceSyncConfig{},
				},
			},
		},
	}

	kubeClient := kubefake.NewSimpleClientset()
	operatorClient := operatorfake.NewSimpleClientset()
	secClient := fakesecurity.NewSimpleClientset(&securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines-scc"},
	})

	r := &rbac{
		kubeClientSet:     kubeClient,
		operatorClientSet: operatorClient,
		securityClientSet: secClient,
		version:           "test-version",
		tektonConfig:      tc,
	}

	err := r.createResources(context.Background())
	// InstallerSet does not exist yet → RECONCILE_AGAIN_ERR
	assert.Equal(t, v1alpha1.RECONCILE_AGAIN_ERR, err)
}

// TestCreateResources_WithInstallerSet verifies that createResources succeeds
// (no error) when the InstallerSet is already present.
func TestCreateResources_WithInstallerSet(t *testing.T) {
	os.Setenv(common.KoEnvKey, "testdata")

	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
		Spec: v1alpha1.TektonConfigSpec{
			Platforms: v1alpha1.Platforms{
				OpenShift: v1alpha1.OpenShift{
					SCC:           &v1alpha1.SCC{Default: "pipelines-scc"},
					NamespaceSync: &v1alpha1.NamespaceSyncConfig{},
				},
			},
		},
	}

	existingISet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rhosp-rbac-001",
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     createdByValue,
				v1alpha1.InstallerSetType: componentNameRBAC,
			},
			Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey: "test-version",
			},
		},
	}

	scc := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{Name: "pipelines-scc"},
	}
	kubeClient := kubefake.NewSimpleClientset()
	operatorClient := operatorfake.NewSimpleClientset(existingISet)
	secClient := fakesecurity.NewSimpleClientset()
	secClient.PrependReactor("get", "securitycontextconstraints", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, scc, nil
	})
	secClient.PrependReactor("list", "securitycontextconstraints", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &securityv1.SecurityContextConstraintsList{Items: []securityv1.SecurityContextConstraints{*scc}}, nil
	})

	r := &rbac{
		kubeClientSet:     kubeClient,
		operatorClientSet: operatorClient,
		securityClientSet: secClient,
		version:           "test-version",
		tektonConfig:      tc,
	}

	err := r.createResources(context.Background())
	assert.NilError(t, err)
}
