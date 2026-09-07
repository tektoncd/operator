package shipwrightbuild

import (
	"context"
	"time"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Well-known names used across the Shipwright Build install: the deployments and
// container the manifests ship, the operand label value that tags every applied
// resource, and the configmap the operator reads the component version from.
const (
	ControllerDeploymentName = "shipwright-build-controller"
	KindClusterBuildStrategy = "ClusterBuildStrategy"
	OperandName              = "shipwright-build"
	VersionConfigMap         = "shipwright-build"
	WebhookContainerName     = "shipwright-build-webhook"
	WebhookDeploymentName    = "shipwright-build-webhook"
	// WebhookSecretName / WebhookServiceName / WebhookSecretDuration describe the
	// TLS serving certificate the webhook needs; the operator provisions it since
	// the upstream manifests expect the secret to already exist.
	WebhookSecretName     = "shipwright-build-webhook-cert"
	WebhookServiceName    = "shp-build-webhook"
	WebhookSecretDuration = 365 * 24 * time.Hour
	// WebhookVersionFlag is passed to the webhook so it advertises the version the
	// operator installed.
	WebhookVersionFlag = "--version"
)

// ShipwrightBuildCRDNames lists the CRDs the component owns; they are deleted
// explicitly on finalize so lingering instances do not block teardown.
//var (
//	ShipwrightBuildCRDNames = []string{
//		"builds.shipwright.io",
//		"buildruns.shipwright.io",
//		"buildstrategies.shipwright.io",
//		"clusterbuildstrategies.shipwright.io",
//	}
//)

// CreateWebhookSecret provisions the webhook's TLS serving certificate when it is
// missing. The secret is owned by the ShipwrightBuild CR so it is garbage collected
// with the component, and an existing secret is left untouched to avoid rotating a
// certificate the webhook is already serving.
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
		s.SetOwnerReferences([]metav1.OwnerReference{ownerReference})
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, s, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
