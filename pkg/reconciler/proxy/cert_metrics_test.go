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

package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

func testCertPEM(t *testing.T, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "tekton-operator-proxy-webhook"},
		NotBefore:    notAfter.Add(-24 * time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func webhookSecret(data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "proxy-webhook-certs", Namespace: "tekton-operator"},
		Data:       data,
	}
}

func TestCertificateExpiry(t *testing.T) {
	notAfter := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name    string
		secret  *corev1.Secret
		want    time.Time
		wantErr bool
	}{
		{
			name:   "valid certificate",
			secret: webhookSecret(map[string][]byte{certresources.ServerCert: testCertPEM(t, notAfter)}),
			want:   notAfter,
		},
		{
			name:    "secret not populated yet",
			secret:  webhookSecret(nil),
			wantErr: true,
		},
		{
			name:    "not PEM",
			secret:  webhookSecret(map[string][]byte{certresources.ServerCert: []byte("garbage")}),
			wantErr: true,
		},
		{
			name: "PEM without a certificate",
			secret: webhookSecret(map[string][]byte{
				certresources.ServerCert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("nope")}),
			}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := certificateExpiry(tt.secret)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("expiry = %v, want %v", got, tt.want)
			}
		})
	}
}

func collectExpiry(t *testing.T, getSecret func() (*corev1.Secret, error)) []float64 {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	if err := registerCertificateMetrics(provider.Meter(certMeterName), getSecret); err != nil {
		t.Fatalf("registerCertificateMetrics() error = %v", err)
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	var values []float64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != CertificateExpirationMetric {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[float64])
			if !ok {
				t.Fatalf("metric %s has data %T, want a float64 gauge", m.Name, m.Data)
			}
			for _, dp := range gauge.DataPoints {
				values = append(values, dp.Value)
			}
		}
	}
	return values
}

func TestRegisterCertificateMetrics(t *testing.T) {
	notAfter := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("reports the expiry", func(t *testing.T) {
		secret := webhookSecret(map[string][]byte{certresources.ServerCert: testCertPEM(t, notAfter)})
		got := collectExpiry(t, func() (*corev1.Secret, error) { return secret, nil })
		if len(got) != 1 || got[0] != float64(notAfter.Unix()) {
			t.Errorf("observed %v, want [%v]", got, float64(notAfter.Unix()))
		}
	})

	t.Run("nothing while the secret is missing", func(t *testing.T) {
		got := collectExpiry(t, func() (*corev1.Secret, error) { return nil, errors.New("not found") })
		if len(got) != 0 {
			t.Errorf("observed %v, want nothing", got)
		}
	})

	t.Run("nothing while the secret is empty", func(t *testing.T) {
		got := collectExpiry(t, func() (*corev1.Secret, error) { return webhookSecret(nil), nil })
		if len(got) != 0 {
			t.Errorf("observed %v, want nothing", got)
		}
	})
}
