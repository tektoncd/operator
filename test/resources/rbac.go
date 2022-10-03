package resources

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/test/utils"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnsureTestNamespaceExists creates a Test Namespace
func EnsureTestNamespaceExists(clients *utils.Clients, name string) (*corev1.Namespace, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ns, err := clients.KubeClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		return clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	}
	return ns, err
}

func AssertServiceAccount(t *testing.T, clients *utils.Clients, ns, targetSA string) {
	t.Helper()

	err := wait.Poll(utils.Interval, utils.Timeout, func() (bool, error) {
		saList, err := clients.KubeClient.CoreV1().ServiceAccounts(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, item := range saList.Items {
			if item.Name == targetSA {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("could not find serviceaccount %s/%s: %q", ns, targetSA, err)
	}
}

func AssertRoleBinding(t *testing.T, clients *utils.Clients, ns, roleBindingName string) {
	t.Helper()

	err := wait.Poll(utils.Interval, utils.Timeout, func() (bool, error) {
		rbList, err := clients.KubeClient.RbacV1().RoleBindings(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, item := range rbList.Items {
			if item.Name == roleBindingName {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("could not find rolebinding %s/%s: %q", ns, roleBindingName, err)
	}
}

func AssertConfigMap(t *testing.T, clients *utils.Clients, ns, configMapName string) {
	t.Helper()

	err := wait.Poll(utils.Interval, utils.Timeout, func() (bool, error) {
		rbList, err := clients.KubeClient.CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, item := range rbList.Items {
			if item.Name == configMapName {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("could not find ConfigMap %s/%s: %q", ns, configMapName, err)
	}
}

func AssertClusterRole(t *testing.T, clients *utils.Clients, clusterRoleName string) {
	t.Helper()

	err := wait.Poll(utils.Interval, utils.Timeout, func() (bool, error) {
		rbList, err := clients.KubeClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, item := range rbList.Items {
			if item.Name == clusterRoleName {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("could not find ClusterRole %s: %q", clusterRoleName, err)
	}
}
