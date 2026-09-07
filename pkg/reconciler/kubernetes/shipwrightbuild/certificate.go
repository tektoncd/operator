/*
Copyright 2026 The Tekton Authors

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

package shipwrightbuild

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

// CertManagerGroupVersion is the Group and Resource identifier for Cert Manager APIs.
const CertManagerGroupVersion = "cert-manager.io/v1"

// SelfSignedIssuerName is the self-signed cert-manager Issuer the operator creates
// to sign the Shipwright Build webhook serving certificate.
const SelfSignedIssuerName = "shipwright-build-selfsigned-issuer"

// IsCertManagerInstalled reports whether the cert-manager API (cert-manager.io/v1)
// is served by the cluster. A missing API group is treated as "not installed"
// rather than an error so the caller can fall back to a self-signed secret.
func IsCertManagerInstalled(discoveryClient discovery.DiscoveryInterface) error {
	_, err := discoveryClient.ServerResourcesForGroupVersion(CertManagerGroupVersion)
	if err != nil {
		return err
	}
	return nil
}

// EnsureWebhookCertificate provisions the webhook serving certificate through
// cert-manager when cert-manager is installed. It creates a self-signed Issuer and
// a Certificate; cert-manager then materializes them into the WebhookSecretName
// secret the webhook deployment mounts.
//
// It returns (true, nil) when cert-manager handled provisioning, and (false, nil)
// when cert-manager is not installed so the caller can fall back to the operator's
// own self-signed secret (see createOrUpdateWebhookSecret). Both the Issuer and the
// Certificate are owned by the ShipwrightBuild CR so they are garbage collected
// with the component.
func EnsureWebhookCertificate(ctx context.Context, client certmanagerclient.Interface, namespace string, ownerReference metav1.OwnerReference) error {
	if err := createIssuer(ctx, client, namespace, ownerReference); err != nil {
		return err
	}
	if err := createCertificate(ctx, client, namespace, ownerReference); err != nil {
		return err
	}
	return nil
}

// createIssuer creates the self-signed Issuer if it does not already exist.
func createIssuer(ctx context.Context, client certmanagerclient.Interface, namespace string, ownerReference metav1.OwnerReference) error {
	issuer := &certmanagerv1.Issuer{
		Name:            SelfSignedIssuerName,
		Namespace:       namespace,
		OwnerReferences: []metav1.OwnerReference{ownerReference},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: certmanagerv1.IssuerConfig{
				SelfSigned: &certmanagerv1.SelfSignedIssuer{},
			},
		},
	}

	if _, err := client.CertmanagerV1().Issuers(namespace).Create(ctx, issuer, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create Issuer %q: %w", SelfSignedIssuerName, err)
	}
	return nil
}

// createCertificate creates the webhook serving Certificate if it does not already
// exist. cert-manager stores the issued key pair in WebhookSecretName.
func createCertificate(ctx context.Context, client certmanagerclient.Interface, namespace string, ownerReference metav1.OwnerReference) error {
	dnsNames := []string{
		fmt.Sprintf("%s.%s.svc", WebhookServiceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", WebhookServiceName, namespace),
	}

	certificate := &certmanagerv1.Certificate{
		Name:            WebhookSecretName,
		Namespace:       namespace,
		OwnerReferences: []metav1.OwnerReference{ownerReference},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: WebhookSecretName,
			CommonName: dnsNames[0],
			DNSNames:   dnsNames,
			IssuerRef: certmanagermetav1.IssuerReference{
				Name:  SelfSignedIssuerName,
				Kind:  "Issuer",
				Group: certmanagerv1.SchemeGroupVersion.Group,
			},
		},
	}

	if _, err := client.CertmanagerV1().Certificates(namespace).Create(ctx, certificate, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("failed to create Certificate %q: %w", WebhookSecretName, err)
	}
	return nil
}
