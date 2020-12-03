package resources

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"

	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"github.com/tektoncd/operator/test/utils"

	"k8s.io/apimachinery/pkg/util/wait"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTestNamespaceExists creates a Test Namespace
func EnsureTestNamespaceExists(clientSet *kubernetes.Clientset, name string) (*corev1.Namespace, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ns, err := clientSet.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		return clientSet.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	}
	return ns, err
}

func AssertServiceAccount(t *testing.T, clients *utils.Clients, ns, targetSA string) {
	t.Helper()

	err := wait.Poll(Interval, Timeout, func() (bool, error) {
		saList, err := clients.KubeClient.Kube.CoreV1().ServiceAccounts(ns).List(context.TODO(), metav1.ListOptions{})
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

	err := wait.Poll(Interval, Timeout, func() (bool, error) {
		rbList, err := clients.KubeClient.Kube.RbacV1().RoleBindings(ns).List(context.TODO(), metav1.ListOptions{})
		for _, item := range rbList.Items {
			if item.Name == roleBindingName {
				return true, nil
			}
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("could not find serviceaccount %s/%s: %q", ns, roleBindingName, err)
	}
}
