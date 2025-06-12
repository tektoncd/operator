/*
Copyright 2021 The Tekton Authors

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

package tektonresult

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineInformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonresultconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonresult"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	DefaultDbSecretName          = "tekton-results-postgres"
	TlsSecretName                = "tekton-results-tls"
	CertificateBlockType         = "CERTIFICATE"
	PostgresUser                 = "result"
	ECPrivateKeyBlockType        = "EC PRIVATE KEY"
	tektonResultStatefulSetLabel = "statefulset"
	tektonResultDeploymentLabel  = "deployment"
)

// Reconciler implements controller.Reconciler for TektonResult resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// installer Set client to do CRUD operations for components
	installerSetClient *client.InstallerSetClient
	// manifest is empty, but with a valid client and logger. all
	// manifests are immutable, and any created during reconcile are
	// expected to be appended to this one, obviating the passing of
	// client & logger
	manifest *mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension

	pipelineInformer pipelineInformer.TektonPipelineInformer

	operatorVersion string
	resultsVersion  string
	recorder        *Recorder
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonresultconciler.Interface = (*Reconciler)(nil)
var _ tektonresultconciler.Finalizer = (*Reconciler)(nil)

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.ResultResourceName,
		},
	}
)

const createdByValue = "TektonResult"

// FinalizeKind removes all resources after deletion of a TektonResult.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonResult", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tr *v1alpha1.TektonResult) pkgreconciler.Event {
	logger := logging.FromContext(ctx).With("tektonresult", tr.Name)
	defer r.recorder.LogMetrics(r.resultsVersion, tr.Spec, logger)

	tr.Status.InitializeConditions()
	tr.Status.ObservedGeneration = tr.Generation

	logger.Infow("Starting TektonResults reconciliation",
		"version", r.resultsVersion,
		"generation", tr.Generation,
		"status", tr.Status.GetCondition(apis.ConditionReady))

	manifest := *r.manifest

	if tr.GetName() != v1alpha1.ResultResourceName {
		logger.Errorw("Invalid resource name",
			"expectedName", v1alpha1.ResultResourceName,
			"actualName", tr.GetName())
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ResultResourceName, tr.GetName())
		tr.Status.MarkNotReady(msg)
		return nil
	}

	// find the valid tekton-pipeline installation
	tp, err := common.PipelineReady(r.pipelineInformer)
	if err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			logger.Infow("Waiting for tekton-pipelines installation to complete")
			tr.Status.MarkDependencyInstalling("tekton-pipelines is still installing")
			// wait for pipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// tektonpipeline.operator.tekton.dev instance not available yet
		logger.Errorw("Pipeline dependency not found", "error", err)
		tr.Status.MarkDependencyMissing("tekton-pipelines does not exist")
		return err
	}

	if tp.GetSpec().GetTargetNamespace() != tr.GetSpec().GetTargetNamespace() {
		errMsg := fmt.Sprintf("tekton-pipelines is missing in %s namespace", tr.GetSpec().GetTargetNamespace())
		logger.Errorw("Namespace mismatch for pipeline dependency",
			"resultNamespace", tr.GetSpec().GetTargetNamespace(),
			"pipelineNamespace", tp.GetSpec().GetTargetNamespace())
		tr.Status.MarkDependencyMissing(errMsg)
		return errors.New(errMsg)
	}

	// If the external database is disabled, create a default database and a TLS secret.
	// Otherwise, verify if the default database secret is already created, and ensure the TLS secret is also created.
	if !tr.Spec.IsExternalDB && tr.Spec.DBSecretName == "" {
		logger.Debugw("Creating database secret for internal database")
		if err := r.createDBSecret(ctx, tr); err != nil {
			logger.Errorw("Failed to create database secret", "error", err)
			return err
		}
		logger.Debugw("Creating TLS secret for internal database")
		if err := r.createTLSSecret(ctx, tr); err != nil {
			logger.Errorw("Failed to create TLS secret", "error", err)
			return err
		}
		logger.Infow("Successfully created database and TLS secrets")
	} else {
		customDbSecretName := DefaultDbSecretName
		if tr.Spec.DBSecretName != "" {
			customDbSecretName = tr.Spec.DBSecretName
		}
		logger.Debugw("Validating external database secrets")
		if err := r.validateSecretsAreCreated(ctx, tr, customDbSecretName); err != nil {
			logger.Errorw("Failed to validate database secrets", "error", err)
			return err
		}
		logger.Debugw("Creating TLS secret for external database")
		if err := r.createTLSSecret(ctx, tr); err != nil {
			logger.Errorw("Failed to create TLS secret", "error", err)
			return err
		}
		logger.Info("Successfully validated database secrets and created TLS secret")
	}

	tr.Status.MarkDependenciesInstalled()
	logger.Info("All dependencies installed successfully")

	//Result watcher is deployed as statefulset, ensure deployment installerset is deleted
	if tr.Spec.Performance.StatefulsetOrdinals != nil && *tr.Spec.Performance.StatefulsetOrdinals {
		if err := r.installerSetClient.CleanupWithLabelInstallTypeDeployment(ctx, v1alpha1.ResultResourceName); err != nil {
			logger.Error("failed to delete main deployment installer set: %v", err)
			return err
		}
	} else {
		// Result watcher is deployed as deployment, ensure statefulset installerset is deleted
		if err := r.installerSetClient.CleanupWithLabelInstallTypeStatefulset(ctx, v1alpha1.ResultResourceName); err != nil {
			logger.Error("failed to delete main statefulset installer set: %v", err)
			return err
		}
	}

	if err := r.extension.PreReconcile(ctx, tr); err != nil {
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Info("PreReconciliation requested requeue")
			return err
		}
		msg := fmt.Sprintf("PreReconciliation failed: %s", err.Error())
		logger.Errorw("PreReconciliation failed", "error", err)
		tr.Status.MarkPreReconcilerFailed(msg)
		return nil
	}

	tr.Status.MarkPreReconcilerComplete()
	logger.Info("PreReconciliation completed successfully")

	// Check if an tektoninstallerset already exists, if not then create
	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		logger.Errorw("Failed to create label selector", "error", err)
		return err
	}

	logger.Debugw("Checking for existing installer set")
	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		logger.Errorw("Failed to get current installer set name", "error", err)
		return err
	}

	if existingInstallerSet == "" {
		logger.Info("No existing installer set found, creating new one")
		createdIs, err := r.createInstallerSet(ctx, tr, manifest)

		if err != nil {
			logger.Errorw("Failed to create installer set", "error", err)
			return err
		}
		logger.Infow("Successfully created installer set", "name", createdIs.Name)
		r.updateTektonResultsStatus(ctx, tr, createdIs)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	// If exists, then fetch the TektonInstallerSet
	logger.Debugw("Fetching existing installer set", "name", existingInstallerSet)
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Infow("Existing installer set not found, creating new one", "missingSet", existingInstallerSet)
			createdIs, err := r.createInstallerSet(ctx, tr, manifest)
			if err != nil {
				logger.Errorw("Failed to create installer set", "error", err)
				return err
			}
			logger.Infow("Successfully created installer set", "name", createdIs.Name)
			r.updateTektonResultsStatus(ctx, tr, createdIs)
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		logger.Errorw("Failed to get existing installer set", "name", existingInstallerSet, "error", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[v1alpha1.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Labels[v1alpha1.ReleaseVersionKey]

	// Check if TargetNamespace of existing TektonInstallerSet is same as expected
	// Check if Release Version in TektonInstallerSet is same as expected
	// If any of the thing above is not same then delete the existing TektonInstallerSet
	// and create a new with expected properties
	if installerSetTargetNamespace != tr.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {
		logger.Infow("Configuration changed, deleting existing installer set",
			"existingNamespace", installerSetTargetNamespace,
			"newNamespace", tr.Spec.TargetNamespace,
			"existingVersion", installerSetReleaseVersion,
			"newVersion", r.operatorVersion)
		// Delete the existing TektonInstallerSet
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, existingInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			logger.Errorw("Failed to delete installer set", "name", existingInstallerSet, "error", err)
			return err
		}
		logger.Infow("Successfully deleted installer set", "name", existingInstallerSet)

		// Make sure the TektonInstallerSet is deleted
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, existingInstallerSet, metav1.GetOptions{})
		if err == nil {
			logger.Infow("Waiting for previous installer set to be deleted", "name", existingInstallerSet)
			tr.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if !apierrors.IsNotFound(err) {
			logger.Errorw("Failed to verify installer set deletion", "name", existingInstallerSet, "error", err)
			return err
		}
		logger.Infow("Confirmed installer set deletion", "name", existingInstallerSet)
		return nil

	} else {
		// If target namespace and version are not changed then check if spec
		// of TektonResult is changed by checking hash stored as annotation on
		// TektonInstallerSet with computing new hash of TektonResult Spec
		logger.Debug("Checking for spec changes in TektonResult")
		// Hash of TektonResult Spec
		expectedSpecHash, err := hash.Compute(tr.Spec)
		if err != nil {
			logger.Errorw("Failed to compute spec hash", "error", err)
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {
			logger.Infow("TektonResult spec changed, updating installer set",
				"previousHash", lastAppliedHash,
				"newHash", expectedSpecHash)

			if err := r.transform(ctx, &manifest, tr); err != nil {
				logger.Errorw("Manifest transformation failed", "error", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = manifest.Resources()
			updatedIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(ctx, installedTIS, metav1.UpdateOptions{})
			if err != nil {
				logger.Errorw("Failed to update installer set", "name", installedTIS.Name, "error", err)
				return err
			}
			logger.Infow("Successfully updated installer set with new spec",
				"name", updatedIS.Name,
				"newHash", expectedSpecHash)
			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			return v1alpha1.REQUEUE_EVENT_AFTER
		} else {
			logger.Debugw("No changes detected in TektonResult spec", "hash", expectedSpecHash)
		}
	}

	r.updateTektonResultsStatus(ctx, tr, installedTIS)

	// Mark InstallerSet Available
	tr.Status.MarkInstallerSetAvailable()
	logger.Infow("Marked installer set as available", "name", installedTIS.Name)

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		logger.Infow("Installer set not yet reporting status", "name", installedTIS.Name)
		tr.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if ready.Status == corev1.ConditionUnknown {
		logger.Infow("Installer set status is unknown, waiting", "name", installedTIS.Name)
		tr.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else if ready.Status == corev1.ConditionFalse {
		logger.Warnw("Installer set not ready", "name", installedTIS.Name, "message", ready.Message)
		tr.Status.MarkInstallerSetNotReady(ready.Message)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// MarkInstallerSetReady
	tr.Status.MarkInstallerSetReady()
	logger.Infow("Installer set is ready", "name", installedTIS.Name)

	if err := r.extension.PostReconcile(ctx, tr); err != nil {
		if err == v1alpha1.REQUEUE_EVENT_AFTER {
			logger.Infow("PostReconciliation requested requeue")
			return err
		}
		msg := fmt.Sprintf("PostReconciliation failed: %s", err.Error())
		logger.Errorw("PostReconciliation failed", "error", err)
		tr.Status.MarkPostReconcilerFailed(msg)
		return nil
	}

	// Mark PostReconcile Complete
	logger.Infow("PostReconciliation completed successfully")
	tr.Status.MarkPostReconcilerComplete()
	r.updateTektonResultsStatus(ctx, tr, installedTIS)

	logger.Infow("TektonResults reconciliation completed successfully",
		"ready", tr.Status.GetCondition(apis.ConditionReady).IsTrue(),
		"generation", tr.Status.ObservedGeneration)

	return nil
}

func (r *Reconciler) updateTektonResultsStatus(ctx context.Context, tr *v1alpha1.TektonResult, createdIs *v1alpha1.TektonInstallerSet) {
	// update the tr with TektonInstallerSet
	tr.Status.SetTektonInstallerSet(createdIs.Name)
	tr.Status.SetVersion(r.resultsVersion)
}

// TektonResults expects secrets to be created before installing
func (r *Reconciler) validateSecretsAreCreated(ctx context.Context, tr *v1alpha1.TektonResult, secretName string) error {
	logger := logging.FromContext(ctx)
	_, err := r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err)
			tr.Status.MarkDependencyMissing(fmt.Sprintf("%s secret is missing", secretName))
			return err
		}
		logger.Error(err)
		return err
	}
	return nil
}

// Generate the DB secret
func (r *Reconciler) generateDBSecret(name string, namespace string, tr *v1alpha1.TektonResult) (*corev1.Secret, error) {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{getOwnerRef(tr)},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{},
	}
	password, err := generateRandomBaseString(20)
	if err != nil {
		return nil, err
	}
	s.StringData["POSTGRES_PASSWORD"] = password
	s.StringData["POSTGRES_USER"] = PostgresUser
	return s, nil
}

// Create Result default database secret
func (r *Reconciler) createDBSecret(ctx context.Context, tr *v1alpha1.TektonResult) error {
	logger := logging.FromContext(ctx)

	// Get the DB secret, if not found then create the DB secret
	_, err := r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Get(ctx, DefaultDbSecretName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logger.Errorf("failed to find default TektonResult database secret %s in namespace %s: %v", DefaultDbSecretName, tr.Spec.TargetNamespace, err)
		return err
	}
	newDBSecret, err := r.generateDBSecret(DefaultDbSecretName, tr.Spec.TargetNamespace, tr)
	if err != nil {
		logger.Errorf("failed to generate default TektonResult database secret %s: %s", DefaultDbSecretName, err)
		return err
	}
	_, err = r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Create(ctx, newDBSecret, metav1.CreateOptions{})
	if err != nil {
		logger.Errorf("failed to create default TektonResult database secret %s in namespace %s: %v", DefaultDbSecretName, tr.Spec.TargetNamespace, err)
		tr.Status.MarkDependencyMissing(fmt.Sprintf("Default db %s creation is failing", DefaultDbSecretName))
		return err
	}
	return nil
}

// Create default TLS certificates for the database
func (r *Reconciler) createTLSSecret(ctx context.Context, tr *v1alpha1.TektonResult) error {
	logger := logging.FromContext(ctx)

	if v1alpha1.IsOpenShiftPlatform() {
		logger.Info("Skipping default TLS secret creation: running on OpenShift platform")
		return nil
	}

	_, err := r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Get(ctx, TlsSecretName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logger.Errorf("failed to find default TektonResult TLS secret %s in namespace %s: %v", TlsSecretName, tr.Spec.TargetNamespace, err)
		return err
	}
	certPEM, keyPEM, err := generateTLSCertificate(tr.Spec.TargetNamespace)
	if err != nil {
		logger.Errorf("failed to generate default TektonResult TLS certificate: %v", err)
		return err
	}
	// Create Kubernetes TLS secret
	err = r.createKubernetesTLSSecret(ctx, tr.Spec.TargetNamespace, TlsSecretName, certPEM, keyPEM, tr)
	if err != nil {
		logger.Errorf("failed to create TLS secret %s in namespace %s: %v", TlsSecretName, tr.Spec.TargetNamespace, err)

	}
	return nil
}

// Get an owner reference of Tekton Result
func getOwnerRef(tr *v1alpha1.TektonResult) metav1.OwnerReference {
	return *metav1.NewControllerRef(tr, tr.GroupVersionKind())
}

func generateRandomBaseString(size int) (string, error) {
	bytes := make([]byte, size)

	// Generate random bytes
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	// Encode the random bytes into a Base64 string
	base64String := base64.StdEncoding.EncodeToString(bytes)

	return base64String, nil
}

// generateTLSCertificate generates a self-signed TLS certificate and private key.
func generateTLSCertificate(targetNS string) (certPEM, keyPEM []byte, err error) {

	// Define subject and DNS names
	dnsName := fmt.Sprintf("tekton-results-api-service.%s.svc.cluster.local", targetNS)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Issuer:       pkix.Name{},
		Subject: pkix.Name{
			CommonName: dnsName,
		},
		DNSNames:              []string{dnsName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: CertificateBlockType, Bytes: certDER})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: ECPrivateKeyBlockType, Bytes: privBytes})

	return certPEM, keyPEM, nil
}

// createKubernetesSecret creates a Kubernetes TLS secret with the given cert and key.
func (r *Reconciler) createKubernetesTLSSecret(ctx context.Context, namespace, secretName string, certPEM, keyPEM []byte, tr *v1alpha1.TektonResult) error {

	// Define the secret
	logger := logging.FromContext(ctx)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
		},
	}

	_, err := r.kubeClientSet.CoreV1().Secrets(tr.Spec.TargetNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		logger.Errorf("failed to create TLS secret %s in namespace %s: %v", secretName, namespace, err)
		tr.Status.MarkDependencyMissing(fmt.Sprintf("Default TLS Secret %s creation is failing", secretName))
		return err
	}

	logger.Infof("Secret '%s' created successfully in namespace '%s'\n", secretName, namespace)
	return nil
}
