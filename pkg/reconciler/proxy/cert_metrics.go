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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	kwebhook "knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	certresources "knative.dev/pkg/webhook/certificates/resources"
)

const (
	certMeterName = "github.com/tektoncd/operator/pkg/reconciler/proxy"

	// CertificateExpirationMetric is the Unix time at which the proxy webhook
	// serving certificate expires. Alert on it getting close to time().
	CertificateExpirationMetric = "tekton_operator_proxy_webhook_certificate_expiration_timestamp_seconds"
)

// NewCertificateController returns the knative webhook certificates controller
// and also exports the expiry of the serving certificate it manages, so an
// expiring or expired certificate can be alerted on before the webhook starts
// failing admission requests.
func NewCertificateController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	lister := secretinformer.Get(ctx).Lister()
	namespace := system.Namespace()
	secretName := kwebhook.GetOptions(ctx).SecretName

	getSecret := func() (*corev1.Secret, error) {
		return lister.Secrets(namespace).Get(secretName)
	}
	if err := registerCertificateMetrics(otel.Meter(certMeterName), getSecret); err != nil {
		logging.FromContext(ctx).Warnw("Failed to register proxy webhook certificate metrics", zap.Error(err))
	}

	return certificates.NewController(ctx, cmw)
}

// registerCertificateMetrics registers an observable gauge with the expiry of
// the serving certificate. Nothing is reported while the secret is missing or
// doesn't hold a certificate yet.
func registerCertificateMetrics(meter metric.Meter, getSecret func() (*corev1.Secret, error)) error {
	gauge, err := meter.Float64ObservableGauge(CertificateExpirationMetric,
		metric.WithDescription("Unix time when the proxy webhook serving certificate expires"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		secret, err := getSecret()
		if err != nil {
			return nil
		}
		expiry, err := certificateExpiry(secret)
		if err != nil {
			return nil
		}
		o.ObserveFloat64(gauge, float64(expiry.Unix()))
		return nil
	}, gauge)
	return err
}

// certificateExpiry returns the NotAfter time of the serving certificate stored
// in the webhook secret.
func certificateExpiry(secret *corev1.Secret) (time.Time, error) {
	data, ok := secret.Data[certresources.ServerCert]
	if !ok || len(data) == 0 {
		return time.Time{}, fmt.Errorf("secret %s/%s has no %s", secret.Namespace, secret.Name, certresources.ServerCert)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, errors.New("serving certificate is not PEM encoded")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse serving certificate: %w", err)
	}
	return cert.NotAfter, nil
}
