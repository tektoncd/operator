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
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagerclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cmfake "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned/fake"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "shipwright-build"

// testOwnerRef is the owner reference threaded onto the Issuer and Certificate.
func testOwnerRef() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "operator.tekton.dev/v1alpha1",
		Kind:       "ShipwrightBuild",
		Name:       "shipwright-build",
		UID:        "test-uid",
	}
}

// discoveryWithCertManager returns a fake discovery client that advertises the
// cert-manager API group when installed is true.
func discoveryWithCertManager(installed bool) *fakediscovery.FakeDiscovery {
	cs := kubefake.NewSimpleClientset()
	fd := cs.Discovery().(*fakediscovery.FakeDiscovery)
	if installed {
		fd.Resources = []*metav1.APIResourceList{{GroupVersion: CertManagerGroupVersion}}
	}
	return fd
}

func TestIsCertManagerInstalled(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		err := IsCertManagerInstalled(discoveryWithCertManager(true))
		util.AssertNoError(t, err)
	})

	t.Run("not installed", func(t *testing.T) {
		err := IsCertManagerInstalled(discoveryWithCertManager(false))
		if err == nil {
			t.Fatalf("expected an error when cert-manager API group is absent, got nil")
		}
	})
}

func TestEnsureWebhookCertificate(t *testing.T) {
	ctx := context.TODO()
	client := cmfake.NewSimpleClientset()

	err := EnsureWebhookCertificate(ctx, client, testNamespace, testOwnerRef())
	util.AssertNoError(t, err)

	// The self-signed Issuer is created.
	issuer, err := client.CertmanagerV1().Issuers(testNamespace).Get(ctx, SelfSignedIssuerName, metav1.GetOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, issuer.Name, SelfSignedIssuerName)
	if issuer.Spec.SelfSigned == nil {
		t.Errorf("expected Issuer to be self-signed, got %+v", issuer.Spec.IssuerConfig)
	}

	// The webhook serving Certificate is created.
	cert, err := client.CertmanagerV1().Certificates(testNamespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, cert.Spec.SecretName, WebhookSecretName)
}

func TestEnsureWebhookCertificateIdempotent(t *testing.T) {
	ctx := context.TODO()
	client := cmfake.NewSimpleClientset()

	// First call creates the resources.
	util.AssertNoError(t, EnsureWebhookCertificate(ctx, client, testNamespace, testOwnerRef()))
	// Second call must not error even though the resources already exist.
	util.AssertNoError(t, EnsureWebhookCertificate(ctx, client, testNamespace, testOwnerRef()))

	// Exactly one of each resource exists.
	issuers, err := client.CertmanagerV1().Issuers(testNamespace).List(ctx, metav1.ListOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, len(issuers.Items), 1)

	certs, err := client.CertmanagerV1().Certificates(testNamespace).List(ctx, metav1.ListOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, len(certs.Items), 1)
}

func TestCreateIssuer(t *testing.T) {
	ctx := context.TODO()
	client := cmfake.NewSimpleClientset()

	util.AssertNoError(t, createIssuer(ctx, client, testNamespace, testOwnerRef()))

	issuer, err := client.CertmanagerV1().Issuers(testNamespace).Get(ctx, SelfSignedIssuerName, metav1.GetOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, issuer.Name, SelfSignedIssuerName)
	util.AssertEqual(t, issuer.Namespace, testNamespace)
	if issuer.Spec.SelfSigned == nil {
		t.Errorf("expected self-signed issuer config, got %+v", issuer.Spec.IssuerConfig)
	}
	// The owner reference is propagated so the Issuer is garbage collected with the CR.
	util.AssertEqual(t, len(issuer.OwnerReferences), 1)
	util.AssertEqual(t, issuer.OwnerReferences[0].Name, testOwnerRef().Name)

	// A repeat call is a no-op rather than an error.
	util.AssertNoError(t, createIssuer(ctx, client, testNamespace, testOwnerRef()))
}

func TestCreateCertificate(t *testing.T) {
	ctx := context.TODO()
	client := cmfake.NewSimpleClientset()

	util.AssertNoError(t, createCertificate(ctx, client, testNamespace, testOwnerRef()))

	cert, err := client.CertmanagerV1().Certificates(testNamespace).Get(ctx, WebhookSecretName, metav1.GetOptions{})
	util.AssertNoError(t, err)
	util.AssertEqual(t, cert.Name, WebhookSecretName)
	util.AssertEqual(t, cert.Namespace, testNamespace)
	util.AssertEqual(t, cert.Spec.SecretName, WebhookSecretName)

	// DNS names and common name target the webhook service.
	expectedDNSNames := []string{
		fmt.Sprintf("%s.%s.svc", WebhookServiceName, testNamespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", WebhookServiceName, testNamespace),
	}
	util.AssertDeepEqual(t, cert.Spec.DNSNames, expectedDNSNames)
	util.AssertEqual(t, cert.Spec.CommonName, expectedDNSNames[0])

	// The certificate is signed by the self-signed Issuer the operator creates.
	util.AssertEqual(t, cert.Spec.IssuerRef.Name, SelfSignedIssuerName)
	util.AssertEqual(t, cert.Spec.IssuerRef.Kind, "Issuer")
	util.AssertEqual(t, cert.Spec.IssuerRef.Group, certmanagerv1.SchemeGroupVersion.Group)

	// The owner reference is propagated.
	util.AssertEqual(t, len(cert.OwnerReferences), 1)
	util.AssertEqual(t, cert.OwnerReferences[0].Name, testOwnerRef().Name)

	// A repeat call is a no-op rather than an error.
	util.AssertNoError(t, createCertificate(ctx, client, testNamespace, testOwnerRef()))
}

// compile-time assertion that the fake satisfies the interface the production
// code depends on.
var _ certmanagerclient.Interface = (*cmfake.Clientset)(nil)
