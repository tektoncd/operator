package common

import (
	"fmt"
	"slices"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// ServiceCertificateAnnotation is the annotation key the OpenShift service-ca
	// operator watches for on a Service. Its value is the name of the Secret
	// where the generated serving certificate/key pair will be stored.
	ServiceCertificateAnnotation = "service.beta.openshift.io/serving-cert-secret-name"

	// InjectCABundleAnnotation is the annotation key the OpenShift service-ca
	// operator watches for on webhook configurations (and other supported
	// resources like APIServices and CRDs). When set to "true", service-ca
	// injects the cluster's CA bundle into the appropriate caBundle field(s).
	InjectCABundleAnnotation = "service.beta.openshift.io/inject-cabundle"
)

// AddServiceCABundle returns a Manifestival transformer that annotates
// resources so the OpenShift service-ca operator injects the cluster CA bundle.
// Pass resources as nil or empty array to transform all resource of a Kind.
func AddServiceCABundle(kind string, resources []string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != kind {
			return nil
		}
		if len(resources) != 0 && !slices.Contains(resources, u.GetName()) {
			return nil
		}

		annotations := u.GetAnnotations()

		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[InjectCABundleAnnotation] = "true"
		u.SetAnnotations(annotations)

		return nil
	}
}

// AddServiceCertificate returns a Manifestival transformer that adds the
// OpenShift Service CA annotation to the Service named serviceName,
// pointing it at secretName so the service-ca operator generates and
// stores a serving certificate there.
func AddServiceCertificate(serviceName, secretName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if secretName == "" || serviceName == "" {
			return fmt.Errorf("AddServiceCertificate: secretName and serviceName must not be empty")
		}

		if u.GetKind() != common.KindService || u.GetName() != serviceName {
			return nil
		}

		annotations := u.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[ServiceCertificateAnnotation] = secretName
		u.SetAnnotations(annotations)

		return nil
	}
}
