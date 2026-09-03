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

// NOTE: SCC escalation-prevention tests for the namespace-level annotation
// (empty maxAllowed → default SCC) are covered in
// pkg/reconciler/openshift/namespacesync/reconciler_test.go.
// The test below was removed when handleSCCInNamespace was moved to
// NamespaceSyncController.

/*
func TestHandleSCCInNamespace_SecurityEscalationPrevention(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		namespace      *corev1.Namespace
		defaultSCC     string
		maxAllowedSCC  string
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "security: empty maxAllowed blocks privileged SCC escalation",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
					Annotations: map[string]string{
						openshift.NamespaceSCCAnnotation: "privileged",
					},
				},
			},
			defaultSCC:     "pipelines-scc",
			maxAllowedSCC:  "",
			wantErr:        true,
			wantErrMessage: "namespace: test-namespace has requested SCC: privileged, but it is less restrictive than the effective 'maxAllowed' SCC: pipelines-scc",
		},
		{
			name: "security: empty maxAllowed allows default SCC",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
					Annotations: map[string]string{
						openshift.NamespaceSCCAnnotation: "pipelines-scc",
					},
				},
			},
			defaultSCC:    "pipelines-scc",
			maxAllowedSCC: "",
			wantErr:       false,
		},
		{
			name: "security: empty maxAllowed with custom default",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
					Annotations: map[string]string{
						openshift.NamespaceSCCAnnotation: "custom-scc",
					},
				},
			},
			defaultSCC:    "custom-scc",
			maxAllowedSCC: "",
			wantErr:       false,
		},
		{
			name: "explicit maxAllowed still enforced",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
					Annotations: map[string]string{
						openshift.NamespaceSCCAnnotation: "anyuid",
					},
				},
			},
			defaultSCC:    "pipelines-scc",
			maxAllowedSCC: "privileged",
			wantErr:       false,
		},
		{
			name: "namespace without SCC annotation is allowed",
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace",
				},
			},
			defaultSCC:    "pipelines-scc",
			maxAllowedSCC: "",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup fake clients
			kubeClient := kubefake.NewSimpleClientset()
			securityClient := fakesecurity.NewSimpleClientset()

			// Create SCCs with priority order (lower priority = more restrictive)
			sccs := []securityv1.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "restricted"},
					Priority:   &[]int32{1}[0],
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pipelines-scc"},
					Priority:   &[]int32{5}[0],
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "custom-scc"},
					Priority:   &[]int32{5}[0],
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "anyuid"},
					Priority:   &[]int32{10}[0],
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "privileged"},
					Priority:   &[]int32{100}[0],
				},
			}

			for _, scc := range sccs {
				_, err := securityClient.SecurityV1().SecurityContextConstraints().Create(ctx, &scc, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			// Create namespace
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, tt.namespace, metav1.CreateOptions{})
			assert.NilError(t, err)

			// Create TektonConfig
			tektonConfig := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default:    tt.defaultSCC,
								MaxAllowed: tt.maxAllowedSCC,
							},
						},
					},
				},
			}

			r := &rbac{
				kubeClientSet:     kubeClient,
				securityClientSet: securityClient,
				tektonConfig:      tektonConfig,
			}

			// Execute handleSCCInNamespace
			err = r.handleSCCInNamespace(ctx, tt.namespace)

			// Verify results
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected error but got nil")
				if tt.wantErrMessage != "" {
					assert.Equal(t, err.Error(), tt.wantErrMessage)
				}
			} else {
				assert.NilError(t, err)
			}
		})
	}
}
*/
