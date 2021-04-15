package extension

import (
	"context"
	"regexp"

	"k8s.io/apimachinery/pkg/api/errors"

	v1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/tektoncd/operator/pkg/reconciler/openshift/rbac"

	"knative.dev/pkg/logging"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

const (
	cleanupClusterRoleName       = "pipeline-anyuid"
	cleanupRoleBindingName       = "pipeline-anyuid"
	cleanupClusterRoleTP12       = "privileged-scc-role"
	clenupClusterRoleBindingTP12 = "openshift-pipelines-privileged"
)

func RbacCleanup(ctx context.Context, client kubernetes.Interface) error {
	logger := logging.FromContext(ctx)

	nsClient := client.CoreV1().Namespaces()
	rbacClient := client.RbacV1()
	namespaces, err := nsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	rexp, err := regexp.Compile(rbac.NamespaceIgnorePattern)
	if err != nil {
		return err
	}
	for _, namespace := range namespaces.Items {
		nsName := namespace.GetName()
		// skip namespaces which match the ignore pattern
		// mar 26, 2021 the logic is to ignre any namespace that has a
		// openshift- or kube- prefix
		if ignore := rexp.Match([]byte(nsName)); ignore {
			continue
		}
		logger.Infow("cleaning up namespace: ", "name", nsName)

		err = cleanUpRoleBinding(ctx, rbacClient, nsName)
		if err != nil {
			return err
		}
	}
	err = cleanUpClusterRole(ctx, rbacClient)
	if err != nil {
		return err
	}

	err = cleanUpTP12Rbac(ctx, rbacClient)
	if err != nil {
		return err
	}

	return nil
}

func cleanUpRoleBinding(ctx context.Context, rbacClient v1.RbacV1Interface, namespace string) error {
	err := rbacClient.RoleBindings(namespace).Delete(ctx, cleanupRoleBindingName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func cleanUpClusterRole(ctx context.Context, rbacClient v1.RbacV1Interface) error {
	err := rbacClient.ClusterRoles().Delete(ctx, cleanupClusterRoleName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func cleanUpTP12Rbac(ctx context.Context, rbacClient v1.RbacV1Interface) error {
	err := rbacClient.ClusterRoles().Delete(ctx, cleanupClusterRoleTP12, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	err = rbacClient.ClusterRoleBindings().Delete(ctx, clenupClusterRoleBindingTP12, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
