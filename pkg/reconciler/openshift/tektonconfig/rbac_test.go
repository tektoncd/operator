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
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"knative.dev/pkg/logging"
)

func TestCreateResources(t *testing.T) {
	// Set KO_DATA_PATH environment variable
	os.Setenv(common.KoEnvKey, "testdata")

	// Test cases
	tests := []struct {
		name               string
		tektonConfig       *v1alpha1.TektonConfig
		existingNamespaces []corev1.Namespace
		existingSAs        []corev1.ServiceAccount
		existingRoles      []rbacv1.Role
		existingRBs        []rbacv1.RoleBinding
		existingCRBs       []rbacv1.ClusterRoleBinding
		installerSet       *v1alpha1.TektonInstallerSet
		wantErr            bool
		wantReconcileAgain bool
		wantNamespaces     int
	}{
		{
			name: "Both RBAC and CA bundles disabled",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "false"},
						{Name: "createCABundleConfigMaps", Value: "false"},
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: false,
			wantNamespaces:     0,
		},
		{
			name: "Only RBAC enabled - No InstallerSet",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "true"},
						{Name: "createCABundleConfigMaps", Value: "false"},
					},
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default: "pipelines-scc",
							},
						},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns2",
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: true, // Should reconcile again because InstallerSet needs to be created
			wantNamespaces:     2,
		},
		{
			name: "Only RBAC enabled - With InstallerSet",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "true"},
						{Name: "createCABundleConfigMaps", Value: "false"},
					},
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default: "pipelines-scc",
							},
						},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns2",
					},
				},
			},
			installerSet: &v1alpha1.TektonInstallerSet{
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
				Spec: v1alpha1.TektonInstallerSetSpec{},
			},
			wantErr:            false,
			wantReconcileAgain: false,
			wantNamespaces:     2,
		},
		{
			name: "Only CA bundles enabled",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "false"},
						{Name: "createCABundleConfigMaps", Value: "true"},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: false,
			wantNamespaces:     1,
		},
		{
			name: "Both RBAC and CA bundles enabled",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "true"},
						{Name: "createCABundleConfigMaps", Value: "true"},
					},
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default: "pipelines-scc",
							},
						},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
						Annotations: map[string]string{
							openshift.NamespaceSCCAnnotation: "privileged",
						},
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: true,
			wantNamespaces:     1,
		},
		{
			name: "Namespace with SCC annotation",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "true"},
					},
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "test-ns",
					},
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default: "pipelines-scc",
							},
						},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
						Annotations: map[string]string{
							openshift.NamespaceSCCAnnotation: "privileged",
						},
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: true,
			wantNamespaces:     1,
		},
		{
			name: "Namespace with invalid SCC annotation",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: "createRbacResource", Value: "true"},
					},
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: "test-ns",
					},
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							SCC: &v1alpha1.SCC{
								Default: "pipelines-scc",
							},
						},
					},
				},
			},
			existingNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns1",
						Annotations: map[string]string{
							openshift.NamespaceSCCAnnotation: "nonexistent-scc",
						},
					},
				},
			},
			wantErr:            false,
			wantReconcileAgain: true,
			wantNamespaces:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup fake clients
			kubeClient := kubefake.NewSimpleClientset()
			operatorClient := operatorfake.NewSimpleClientset()
			securityClient := fakesecurity.NewSimpleClientset()

			// Create default SCCs
			defaultSCCs := []securityv1.SecurityContextConstraints{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "restricted"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pipelines-scc"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "privileged"},
				},
			}
			for _, scc := range defaultSCCs {
				_, err := securityClient.SecurityV1().SecurityContextConstraints().Create(ctx, &scc, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			// Create the required "edit" ClusterRole
			editClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "edit",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"*"},
						Resources: []string{"*"},
						Verbs:     []string{"*"},
					},
				},
			}
			_, err := kubeClient.RbacV1().ClusterRoles().Create(ctx, editClusterRole, metav1.CreateOptions{})
			assert.NilError(t, err)

			// Create informers
			informers := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
			nsInformer := informers.Core().V1().Namespaces()
			rbacInformer := informers.Rbac().V1().ClusterRoleBindings()

			// Add existing resources to the fake clients
			for _, ns := range tt.existingNamespaces {
				_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			for _, sa := range tt.existingSAs {
				_, err := kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(ctx, &sa, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			for _, role := range tt.existingRoles {
				_, err := kubeClient.RbacV1().Roles(role.Namespace).Create(ctx, &role, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			for _, rb := range tt.existingRBs {
				_, err := kubeClient.RbacV1().RoleBindings(rb.Namespace).Create(ctx, &rb, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			for _, crb := range tt.existingCRBs {
				_, err := kubeClient.RbacV1().ClusterRoleBindings().Create(ctx, &crb, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			// Create installer set if specified in test case
			if tt.installerSet != nil {
				_, err := operatorClient.OperatorV1alpha1().TektonInstallerSets().Create(ctx, tt.installerSet, metav1.CreateOptions{})
				assert.NilError(t, err)
			}

			// Start informers
			stopCh := make(chan struct{})
			defer close(stopCh)
			informers.Start(stopCh)

			// Create the rbac instance
			r := &rbac{
				kubeClientSet:     kubeClient,
				operatorClientSet: operatorClient,
				securityClientSet: securityClient,
				rbacInformer:      rbacInformer,
				nsInformer:        nsInformer,
				tektonConfig:      tt.tektonConfig,
				version:           "test-version",
			}

			// Create context with logger
			ctx = logging.WithLogger(ctx, logging.FromContext(ctx))

			// Execute the function
			err = r.createResources(ctx)

			// Verify results
			if tt.wantErr {
				assert.Assert(t, err != nil)
			} else if tt.wantReconcileAgain {
				assert.Equal(t, err, v1alpha1.RECONCILE_AGAIN_ERR)
			} else {
				assert.NilError(t, err)
			}

			// Verify the number of processed namespaces
			if !tt.wantReconcileAgain {
				// For cases where we expect reconciliation to complete, verify the namespace labels
				for _, ns := range tt.existingNamespaces {
					updatedNs, err := kubeClient.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
					assert.NilError(t, err)

					// If RBAC is enabled, verify the namespace has been labeled
					createRBACResource := true
					for _, param := range tt.tektonConfig.Spec.Params {
						if param.Name == "createRbacResource" && param.Value == "false" {
							createRBACResource = false
							break
						}
					}

					if createRBACResource {
						// Verify the namespace has been labeled with the correct version
						assert.Equal(t, updatedNs.Labels[namespaceVersionLabel], r.version)
					}
				}
			}
		})
	}
}

func TestSetDefault(t *testing.T) {
	tests := []struct {
		name         string
		tektonConfig *v1alpha1.TektonConfig
		want         []v1alpha1.Param
	}{
		{
			name: "Empty params - should set all defaults",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "true"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "true"},
			},
		},
		{
			name: "Partial params - should set missing defaults",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: rbacParamName, Value: "false"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "false"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "false"},
			},
		},
		{
			name: "Invalid values - should set to true",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: rbacParamName, Value: "invalid"},
						{Name: trustedCABundleParamName, Value: "maybe"},
						{Name: legacyPipelineRbacParamName, Value: "unknown"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "true"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
			},
		},
		{
			name: "All params set correctly - should not change",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: rbacParamName, Value: "false"},
						{Name: trustedCABundleParamName, Value: "false"},
						{Name: legacyPipelineRbacParamName, Value: "false"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "false"},
				{Name: trustedCABundleParamName, Value: "false"},
				{Name: legacyPipelineRbacParamName, Value: "false"},
			},
		},
		{
			name: "Only createCABundleConfigMaps present, set to false",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: trustedCABundleParamName, Value: "false"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: trustedCABundleParamName, Value: "false"},
				{Name: rbacParamName, Value: "true"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
			},
		},
		{
			name: "Only legacyPipelineRbac present, set to true",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: legacyPipelineRbacParamName, Value: "true"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: legacyPipelineRbacParamName, Value: "true"},
				{Name: rbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "true"},
			},
		},
		{
			name: "createRbacResource present with 'true', others missing",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: rbacParamName, Value: "true"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "true"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "true"},
			},
		},
		{
			name: "Mix of valid, invalid, and missing",
			tektonConfig: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "config"},
				Spec: v1alpha1.TektonConfigSpec{
					Params: []v1alpha1.Param{
						{Name: rbacParamName, Value: "true"},
						{Name: legacyPipelineRbacParamName, Value: "invalid"},
					},
				},
			},
			want: []v1alpha1.Param{
				{Name: rbacParamName, Value: "true"},
				{Name: legacyPipelineRbacParamName, Value: "true"},
				{Name: trustedCABundleParamName, Value: "true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &rbac{
				tektonConfig: tt.tektonConfig,
			}

			r.setDefault()

			// Verify the params are set correctly
			assert.Equal(t, len(r.tektonConfig.Spec.Params), len(tt.want))

			// Create maps for easier comparison
			got := make(map[string]string)
			for _, param := range r.tektonConfig.Spec.Params {
				got[param.Name] = param.Value
			}

			want := make(map[string]string)
			for _, param := range tt.want {
				want[param.Name] = param.Value
			}

			// Compare the maps
			assert.DeepEqual(t, got, want)
		})
	}
}
