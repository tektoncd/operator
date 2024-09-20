/*
Copyright 2024 The Tekton Authors

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

package tektonconfig

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/openshift/api/security/v1"
	fakesecurity "github.com/openshift/client-go/security/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	//fakeoperator "github.com/tektoncd/operator/pkg/client/injection/client/fake"
	fakeoperator "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	ts "knative.dev/pkg/reconciler/testing"
)

func TestGetNamespacesToBeReconciled(t *testing.T) {
	var deletionTime = metav1.Now()
	for _, c := range []struct {
		desc           string
		wantNamespaces []corev1.Namespace
		objs           []runtime.Object
		ctx            context.Context
	}{
		{
			desc: "ignore system namespaces",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-test"}},
			},
			wantNamespaces: nil,
			ctx:            context.Background(),
		},
		{
			desc: "ignore namespaces with deletion timestamp",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-test", DeletionTimestamp: &deletionTime}},
			},
			wantNamespaces: nil,
			ctx:            context.Background(),
		},
		{
			desc: "add namespace to reconcile list if it has openshift scc operator.tekton.dev/scc annotation set ",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Annotations: map[string]string{"operator.tekton.dev/scc": "restricted"}}},
			},
			wantNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test",
						Annotations: map[string]string{"operator.tekton.dev/scc": "restricted"},
					},
				},
			},
			ctx: context.Background(),
		},
		{
			desc: "add namespace to reconcile list if it has bad label openshift-pipelines.tekton.dev/namespace-reconcile-version",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: map[string]string{"openshift-pipelines.tekton.dev/namespace-reconcile-version": ""}}},
			},
			wantNamespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test",
						Labels: map[string]string{"openshift-pipelines.tekton.dev/namespace-reconcile-version": ""},
					},
				},
			},
			ctx: context.Background(),
		},
	} {
		t.Run(c.desc, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset(c.objs...)
			r := rbac{
				kubeClientSet: kubeclient,
				version:       "devel",
			}
			namespaces, err := r.getNamespacesToBeReconciled(c.ctx)
			if err != nil {
				t.Fatalf("getNamespacesToBeReconciled: %v", err)
			}
			if d := cmp.Diff(c.wantNamespaces, namespaces); d != "" {
				t.Fatalf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestCreateResources(t *testing.T) {
	ctx, _, _ := ts.SetupFakeContextWithCancel(t)
	os.Setenv(common.KoEnvKey, "testdata")
	scc := &v1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "PipelinesSCC"}}
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:   v1alpha1.ConfigResourceName,
			Labels: map[string]string{},
		},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "foo",
			},
			Platforms: v1alpha1.Platforms{
				OpenShift: v1alpha1.OpenShift{
					SCC: &v1alpha1.SCC{
						Default: scc.Name,
					},
				},
			},
		},
		Status: v1alpha1.TektonConfigStatus{
			Status:             duckv1.Status{},
			TektonInstallerSet: map[string]string{},
		},
	}
	for _, c := range []struct {
		desc string
		objs []runtime.Object
		iSet *v1alpha1.TektonInstallerSet
		err  error
	}{
		{
			desc: "No existing installer set",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: map[string]string{"openshift-pipelines.tekton.dev/namespace-reconcile-version": ""}}},
			},
			err: v1alpha1.RECONCILE_AGAIN_ERR,
		},
		{
			desc: "existing installer set, missing cluster role edit",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: map[string]string{"openshift-pipelines.tekton.dev/namespace-reconcile-version": ""}}},
			},
			iSet: &v1alpha1.TektonInstallerSet{ObjectMeta: metav1.ObjectMeta{Name: "rhosp-rbac-001", Labels: map[string]string{v1alpha1.CreatedByKey: createdByValue, v1alpha1.InstallerSetType: componentNameRBAC}, Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey: "devel", v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace}}, Spec: v1alpha1.TektonInstallerSetSpec{}},
			err: fmt.Errorf("errors occurred in createResource during namespaces rbac reconcile"),
		},
		{
			desc: "existing installer set, all ok",
			objs: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test", Labels: map[string]string{"openshift-pipelines.tekton.dev/namespace-reconcile-version": ""}}},
				&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "edit"}},
			},
			iSet: &v1alpha1.TektonInstallerSet{ObjectMeta: metav1.ObjectMeta{Name: "rhosp-rbac-001", Labels: map[string]string{v1alpha1.CreatedByKey: createdByValue, v1alpha1.InstallerSetType: componentNameRBAC}, Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey: "devel", v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace}}, Spec: v1alpha1.TektonInstallerSetSpec{}},
		},
	} {
		t.Run(c.desc, func(t *testing.T) {
			kubeclient := fakek8s.NewSimpleClientset(c.objs...)
			fakeoperatorclient := fakeoperator.NewSimpleClientset()
			fakesecurityclient := fakesecurity.NewSimpleClientset()
			_, err := fakesecurityclient.SecurityV1().SecurityContextConstraints().Create(ctx, scc, metav1.CreateOptions{})
			if err != nil {
				t.Logf("Could not create fake scc %v", err)
			}
			if c.iSet != nil {
				_, err := fakeoperatorclient.OperatorV1alpha1().TektonInstallerSets().Create(ctx, c.iSet, metav1.CreateOptions{})
				if err != nil {
					t.Logf("Could not create fake installerSet %v", err)
				}
			}
			informers := informers.NewSharedInformerFactory(kubeclient, 0)
			nsInformer := informers.Core().V1().Namespaces()
			rbacinformer := informers.Rbac().V1().ClusterRoleBindings()

			r := rbac{
				kubeClientSet:     kubeclient,
				operatorClientSet: fakeoperatorclient,
				securityClientSet: fakesecurityclient,
				rbacInformer:      rbacinformer,
				nsInformer:        nsInformer,
				version:           "devel",
				tektonConfig:      tc,
			}
			err = r.createResources(ctx)
			if c.err != nil {
				require.Error(t, err)
				require.Equal(t, c.err.Error(), err.Error(), "Expected error '%v', got '%v'", c.err, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
