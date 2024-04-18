package tektonconfig

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakek8s "k8s.io/client-go/kubernetes/fake"
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
