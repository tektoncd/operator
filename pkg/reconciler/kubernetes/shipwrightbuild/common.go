package shipwrightbuild

import (
	"context"
	"time"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	ControllerDeploymentName = "shipwright-build-controller"
	KindClusterBuildStrategy = "ClusterBuildStrategy"
	OperandName              = "shipwright-build"
	VersionConfigMap     = "shipwright-build"
	WebhookContainerName = "shipwright-build-webhook"
	WebhookDeploymentName = "shipwright-build-webhook"
	WebhookSecretName     = "shipwright-build-webhook-cert"
	WebhookServiceName    = "shp-build-webhook"
	WebhookSecretDuration = 365 * 24 * time.Hour
	WebhookVersionFlag = "-version"
)

func CreateWebhookSecret(ctx context.Context, client kubernetes.Interface, namespace string, ownerReference metav1.OwnerReference) error {
	s, err := client.CoreV1().Secrets(namespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		s, err = common.NewCertificateSecret(ctx, WebhookServiceName, WebhookSecretName, namespace, WebhookSecretDuration)
		if err != nil {
			return err
		}
		s.SetOwnerReferences([]metav1.OwnerReference{ ownerReference })
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, s, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
