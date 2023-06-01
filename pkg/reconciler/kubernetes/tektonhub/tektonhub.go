/*
Copyright 2022 The Tekton Authors

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

package tektonhub

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	mf "github.com/manifestival/manifestival"
	"github.com/spf13/viper"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonhubconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonhub"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonHub resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest is empty, but with a valid client and logger. all
	// manifests are immutable, and any created during reconcile are
	// expected to be appended to this one, obviating the passing of
	// client & logger
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension       common.Extension
	operatorVersion string
}

const (
	// installerSet labels
	installerSetLabelCreatedByValue = "TektonHub"

	// installerSet names
	installerSetNameDatabase      = "DbInstallerSet"
	installerSetDatabaseMigration = "DbMigrationInstallerSet"
	installerSetNameAPI           = "ApiInstallerSet"
	installerSetNameUI            = "UiInstallerSet"

	// installerSet types
	installerSetTypeDatabase          = "tekton-hub-db"
	installerSetTypeDatabaseMigration = "tekton-hub-db-migration"
	installerSetTypeAPI               = "tekton-hub-api"
	installerSetTypeUI                = "tekton-hub-ui"

	// manifests directory names
	manifestDirDatabase          = "db"
	manifestDirDatabaseMigration = "db-migration"
	manifestDirAPI               = "api"
	manifestDirUI                = "ui"

	// resource names
	databaseSecretName = "tekton-hub-db"
	apiConfigMapName   = "tekton-hub-api"
	uiConfigMapName    = "tekton-hub-ui"

	// database secret keys
	secretKeyPostgresHost     = "POSTGRES_HOST"
	secretKeyPostgresDB       = "POSTGRES_DB"
	secretKeyPostgresUser     = "POSTGRES_USER"
	secretKeyPostgresPassword = "POSTGRES_PASSWORD"
	secretKeyPostgresPort     = "POSTGRES_PORT"

	// default postgres database values
	defaultPostgresHost     = "tekton-hub-db"
	defaultPostgresDB       = "hub"
	defaultPostgresUser     = "hub"
	defaultPostgresPassword = "hub"
	defaultPostgresPort     = "5432"
)

var (
	errKeyMissing error = fmt.Errorf("secret doesn't contains all the keys")

	// Check that our Reconciler implements controller.Reconciler
	_ tektonhubconciler.Interface = (*Reconciler)(nil)
	_ tektonhubconciler.Finalizer = (*Reconciler)(nil)

	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     installerSetLabelCreatedByValue,
			v1alpha1.InstallerSetType: v1alpha1.HubResourceName,
		},
	}

	dbKeys = []string{secretKeyPostgresHost, secretKeyPostgresDB, secretKeyPostgresUser, secretKeyPostgresPassword, secretKeyPostgresPort}
)

type Data struct {
	Catalogs   []v1alpha1.Catalog
	Categories []v1alpha1.Category
	Scopes     []v1alpha1.Scope
	Default    v1alpha1.Default
}

// FinalizeKind removes all resources after deletion of a TektonHub.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonHub) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}

	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonHub", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, th *v1alpha1.TektonHub) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	th.Status.InitializeConditions()
	th.Status.ObservedGeneration = th.Generation

	logger.Infow("Reconciling TektonHub", "status", th.Status)

	// we have to maintain only one hub across the cluster and the accepted resource name is "hub"
	if th.GetName() != v1alpha1.HubResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.HubResourceName,
			th.GetName(),
		)
		logger.Error(msg)
		th.Status.MarkNotReady(msg)
		return nil
	}

	th.SetDefaults(ctx)

	// reconcile target namespace
	if err := common.ReconcileTargetNamespace(ctx, nil, th, r.kubeClientSet); err != nil {
		logger.Errorw("error on reconciling targetNamespace",
			"targetNamespace", th.Spec.GetTargetNamespace(),
			err,
		)
		return err
	}

	// execute pre-reconcile, used in extension
	if err := r.extension.PreReconcile(ctx, th); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkPreReconcilerComplete()

	// get TektonHub version and yaml manifests directory
	version := common.TargetVersion(th)
	hubManifestDir := filepath.Join(common.ComponentDir(th), version)

	// if user has not supplied external database credentials setup a database
	if err := r.reconcileDatabaseInstallerSet(ctx, th, hubManifestDir, version); err != nil {
		return r.handleError(err, th)
	}

	if err := r.setupDatabaseMigrationInstallerSet(ctx, th, hubManifestDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkDatabaseMigrationDone()

	if err := r.reconcileApiInstallerSet(ctx, th, hubManifestDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkApiInstallerSetAvailable()

	if err := r.reconcileUiInstallerSet(ctx, th, hubManifestDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkUiInstallerSetAvailable()

	// execute post-reconcile, used in extension
	if err := r.extension.PostReconcile(ctx, th); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkPostReconcilerComplete()

	return nil
}

func (r *Reconciler) handleError(err error, th *v1alpha1.TektonHub) error {
	if err == v1alpha1.RECONCILE_AGAIN_ERR {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return err
}

func (r *Reconciler) reconcileUiInstallerSet(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, installerSetTypeUI)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkUiInstallerSetNotAvailable("UI installer set not available")
		uiLocation := filepath.Join(hubDir, manifestDirUI)

		manifest, err := r.getManifest(ctx, th, uiLocation)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, installerSetNameUI, version, installerSetTypeUI)
		if err != nil {
			return err
		}

	}

	if exist {
		// Get the installerset, check for the hash of spec
		// if not same delete the installerset.
		labels := r.getLabels(installerSetTypeUI)
		labelSelector, err := common.LabelSelector(labels)
		if err != nil {
			return err
		}

		compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
		if err != nil {
			return err
		}

		if compInstallerSet != "" {
			ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Get(ctx, compInstallerSet, metav1.GetOptions{})
			if err != nil {
				return err
			}

			lastAppliedTektonHubCRSpecHash := ctIs.Annotations[v1alpha1.LastAppliedHashKey]
			tektonHubCRSpecHash, err := hash.Compute(th.Spec)
			if err != nil {
				return err
			}

			if tektonHubCRSpecHash != lastAppliedTektonHubCRSpecHash {
				if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, ctIs.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}
				return v1alpha1.RECONCILE_AGAIN_ERR
			}
		}
	}

	err = r.checkComponentStatus(ctx, th, installerSetTypeUI)
	if err != nil {
		th.Status.MarkUiInstallerSetNotAvailable(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	return nil
}

func (r *Reconciler) reconcileApiInstallerSet(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {

	// Validate whether the secrets and configmap are created for API
	if err := r.validateApiDependencies(ctx, th, hubDir, manifestDirAPI); err != nil {
		th.Status.MarkApiDependencyMissing("api secrets not present")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	th.Status.MarkApiDependenciesInstalled()

	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, installerSetTypeAPI)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkApiInstallerSetNotAvailable("API installer set not available")
		apiLocation := filepath.Join(hubDir, manifestDirAPI)

		manifest, err := r.getManifest(ctx, th, apiLocation)
		if err != nil {
			return err
		}

		err = applyPVC(ctx, manifest, th)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, installerSetNameAPI, version, installerSetTypeAPI)
		if err != nil {
			return err
		}
	}

	if exist {
		// Get the installerset, check for the hash of db secret
		// if not same delete the installerset.
		labels := r.getLabels(installerSetTypeAPI)
		labelSelector, err := common.LabelSelector(labels)
		if err != nil {
			return err
		}

		compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
		if err != nil {
			return err
		}

		if compInstallerSet != "" {
			ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Get(ctx, compInstallerSet, metav1.GetOptions{})
			if err != nil {
				return err
			}

			lastAppliedDbSecretHash := ctIs.Annotations[v1alpha1.DbSecretHash]
			lastAppliedTektonHubCRSpecHash := ctIs.Annotations[v1alpha1.LastAppliedHashKey]

			secret, err := r.getSecret(ctx, databaseSecretName, th.Spec.GetTargetNamespace(), dbKeys)
			if err != nil {
				return err
			}

			expectedDbSecretHash, err := hash.Compute(secret.Data)
			if err != nil {
				return err
			}
			tektonHubCRSpecHash, err := hash.Compute(th.Spec)
			if err != nil {
				return err
			}

			if lastAppliedDbSecretHash != expectedDbSecretHash || tektonHubCRSpecHash != lastAppliedTektonHubCRSpecHash {

				if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, ctIs.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}
				return v1alpha1.RECONCILE_AGAIN_ERR
			}
		}
	}

	err = r.checkComponentStatus(ctx, th, installerSetTypeAPI)
	if err != nil {
		th.Status.MarkApiInstallerSetNotAvailable(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (r *Reconciler) setupDatabaseMigrationInstallerSet(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	// Check if the InstallerSet is available for DB-migration
	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, installerSetTypeDatabaseMigration)
	if err != nil {
		return err
	}

	if !exist {
		dbMigrationManifestsDir := filepath.Join(hubDir, manifestDirDatabaseMigration)
		th.Status.MarkDatabaseMigrationFailed("DB migration installerset not available")

		manifest, err := r.getManifest(ctx, th, dbMigrationManifestsDir)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, installerSetDatabaseMigration, version, installerSetTypeDatabaseMigration)
		if err != nil {
			return err
		}
	}

	if exist {
		// Get the installerset, check for the hash of db secret
		// if not same delete the installerset

		labels := r.getLabels(installerSetTypeDatabaseMigration)
		labelSelector, err := common.LabelSelector(labels)
		if err != nil {
			return err
		}

		compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
		if err != nil {
			return err
		}

		if compInstallerSet != "" {
			ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Get(ctx, compInstallerSet, metav1.GetOptions{})
			if err != nil {
				return err
			}

			lastAppliedDbSecretHash := ctIs.Annotations[v1alpha1.DbSecretHash]

			secret, err := r.getSecret(ctx, databaseSecretName, th.Spec.GetTargetNamespace(), dbKeys)
			if err != nil {
				return err
			}

			expectedDbSecretHash, err := hash.Compute(secret.Data)
			if err != nil {
				return err
			}

			if lastAppliedDbSecretHash != expectedDbSecretHash {
				if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, ctIs.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}
			}
		}
	}

	err = r.checkComponentStatus(ctx, th, installerSetTypeDatabaseMigration)
	if err != nil {
		th.Status.MarkDatabaseMigrationFailed(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (r *Reconciler) setupDatabase(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	// Check if the DB secrets are created
	if err := r.validateOrCreateDBSecrets(ctx, th); err != nil {
		th.Status.MarkDbDependencyMissing("db secrets are either invalid or not present")
		return err
	}
	th.Status.MarkDbDependenciesInstalled()

	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, installerSetTypeDatabase)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkDbInstallerSetNotAvailable("DB installer set not available")
		dbLocation := filepath.Join(hubDir, manifestDirDatabase)
		manifest, err := r.getManifest(ctx, th, dbLocation)
		if err != nil {
			return err
		}

		err = applyPVC(ctx, manifest, th)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, installerSetNameDatabase, version, installerSetTypeDatabase)
		if err != nil {
			return err
		}
	}

	err = r.checkComponentStatus(ctx, th, installerSetTypeDatabase)
	if err != nil {
		th.Status.MarkDbInstallerSetNotAvailable(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	return nil
}

// Validate DB are present on the cluster. If DB secrets are present and all the keys don't
// have values then update the remaining values with default values. If the DB secret
// is not present then create a new secret with default values.
func (r *Reconciler) validateOrCreateDBSecrets(ctx context.Context, th *v1alpha1.TektonHub) error {
	logger := logging.FromContext(ctx)
	targetNamespace := th.Spec.GetTargetNamespace()

	// th.Status.MarkDbDependencyInstalling("db secrets are being added into the namespace")

	dbSecret, err := r.getSecret(ctx, databaseSecretName, targetNamespace, dbKeys)
	if err != nil {
		newDbSecret := createDbSecret(databaseSecretName, targetNamespace, dbSecret, th)
		if apierrors.IsNotFound(err) {
			_, err = r.kubeClientSet.CoreV1().Secrets(targetNamespace).Create(ctx, newDbSecret, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", databaseSecretName))
				return err
			}
			return nil
		}
		if err == errKeyMissing {
			_, err = r.kubeClientSet.CoreV1().Secrets(targetNamespace).Update(ctx, newDbSecret, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", databaseSecretName))
				return err
			}
		} else {
			logger.Error(err)
			return err
		}
	}

	return nil
}

// TektonHub expects API secrets to be created before installing Tekton Hub API
func (r *Reconciler) validateApiDependencies(ctx context.Context, th *v1alpha1.TektonHub, hubDir, comp string) error {
	logger := logging.FromContext(ctx)
	apiSecretKeys := []string{"GH_CLIENT_ID", "GH_CLIENT_SECRET", "JWT_SIGNING_KEY", "ACCESS_JWT_EXPIRES_IN", "REFRESH_JWT_EXPIRES_IN", "GHE_URL"}

	th.Status.MarkApiDependencyInstalling("checking for api secrets in the namespace and creating the ConfigMap")

	_, err := r.getSecret(ctx, th.Spec.Api.ApiSecretName, th.Spec.GetTargetNamespace(), apiSecretKeys)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.createApiSecret(ctx, th, hubDir, comp); err != nil {
				return err
			}
		}
		if err == errKeyMissing {
			th.Status.MarkApiDependencyMissing(fmt.Sprintf("%s secret is missing the keys", th.Spec.Api.ApiSecretName))
			return err
		} else {
			logger.Error(err)
			return err
		}
	}
	return nil
}

func (r *Reconciler) getManifest(ctx context.Context, th *v1alpha1.TektonHub, manifestLocation string) (*mf.Manifest, error) {
	manifest := r.manifest.Append()

	if err := common.AppendManifest(&manifest, manifestLocation); err != nil {
		return nil, err
	}

	transformedManifest, err := r.transform(ctx, manifest, th)
	if err != nil {
		return nil, err
	}

	return transformedManifest, nil
}

func (r *Reconciler) transform(ctx context.Context, manifest mf.Manifest, th *v1alpha1.TektonHub) (*mf.Manifest, error) {
	logger := logging.FromContext(ctx)

	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.HubImagePrefix))
	trans := r.extension.Transformers(th)
	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdHub),
		mf.InjectOwner(th),
		mf.InjectNamespace(th.Spec.GetTargetNamespace()),
		common.DeploymentImages(images),
		common.JobImages(images),
		updateApiConfigMap(th, apiConfigMapName),
		addConfigMapKeyValue(uiConfigMapName, "API_URL", th.Status.ApiRouteUrl),
		addConfigMapKeyValue(uiConfigMapName, "AUTH_BASE_URL", th.Status.AuthRouteUrl),
		addConfigMapKeyValue(uiConfigMapName, "API_VERSION", "v1"),
		addConfigMapKeyValue(uiConfigMapName, "REDIRECT_URI", th.Status.UiRouteUrl),
		addConfigMapKeyValue(uiConfigMapName, "CUSTOM_LOGO_BASE64_DATA", th.Spec.CustomLogo.Base64Data),
		addConfigMapKeyValue(uiConfigMapName, "CUSTOM_LOGO_MEDIA_TYPE", th.Spec.CustomLogo.MediaType),
		common.AddDeploymentRestrictedPSA(),
		common.AddJobRestrictedPSA(),
	}

	trans = append(trans, extra...)

	manifest, err := manifest.Transform(trans...)

	if err != nil {
		logger.Error("failed to transform manifest")
		return nil, err
	}

	return &manifest, nil
}

func (r *Reconciler) getLabels(componentInstallerSetType string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     installerSetLabelCreatedByValue,
			v1alpha1.InstallerSetType: componentInstallerSetType,
		},
	}
}

func applyPVC(ctx context.Context, manifest *mf.Manifest, th *v1alpha1.TektonHub) error {
	logger := logging.FromContext(ctx)

	pvc := manifest.Filter(mf.ByKind("PersistentVolumeClaim"))
	pvcManifest, err := pvc.Transform(
		mf.InjectOwner(th),
		mf.InjectNamespace(th.Spec.GetTargetNamespace()),
	)

	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}

	if err := pvcManifest.Apply(); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) reconcileDatabaseInstallerSet(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	// Get the db secret, if not found or if any key is missing,
	// then manage the db installerset. If the value of db host
	// is different then user already has the db, hence delete
	// existing db installerset
	secret, err := r.getSecret(ctx, databaseSecretName, th.Spec.GetTargetNamespace(), dbKeys)
	if err != nil {
		// If not found create db with default db
		if apierrors.IsNotFound(err) || err == errKeyMissing {
			if err := r.setupDatabase(ctx, th, hubDir, version); err != nil {
				return r.handleError(err, th)
			}
			th.Status.MarkDbInstallerSetAvailable()
			return nil
		}
		return err
	} else if string(secret.Data[secretKeyPostgresHost]) != defaultPostgresHost {
		// Mark the database as ready state as the
		// database is already installed by the user
		th.Status.MarkDbDependenciesInstalled()
		th.Status.MarkDbInstallerSetAvailable()

		// Get and delete the default db installerset
		if err := r.getAndDeleteInstallerSet(ctx, installerSetTypeDatabase); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	} else {
		// If secret found, with no error, then make sure db is up and running
		if err := r.setupDatabase(ctx, th, hubDir, version); err != nil {
			return r.handleError(err, th)
		}
		th.Status.MarkDbInstallerSetAvailable()
	}

	return nil
}

func (r *Reconciler) setUpAndCreateInstallerSet(ctx context.Context, manifest mf.Manifest, th *v1alpha1.TektonHub, installerSetName, version, prefixName string) error {

	manifest = manifest.Filter(mf.Not(mf.Any(mf.ByKind("Secret"), mf.ByKind("PersistentVolumeClaim"), mf.ByKind("Namespace"))))

	specHash := ""
	if prefixName == installerSetTypeDatabaseMigration || prefixName == installerSetTypeAPI {
		secret, err := r.kubeClientSet.CoreV1().Secrets(th.Spec.GetTargetNamespace()).Get(ctx, databaseSecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		specHash, err = hash.Compute(secret.Data)
		if err != nil {
			return err
		}
	}
	labels := r.getLabels(prefixName).MatchLabels
	if err := createInstallerSet(ctx, r.operatorClientSet, th, manifest,
		version, installerSetName, prefixName, th.Spec.GetTargetNamespace(), labels, specHash); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) getAndDeleteInstallerSet(ctx context.Context, installerSetType string) error {
	labels := r.getLabels(installerSetType)
	labelSelector, err := common.LabelSelector(labels)
	if err != nil {
		return err
	}

	compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}

	if compInstallerSet != "" {
		_, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, compInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) checkComponentStatus(ctx context.Context, th *v1alpha1.TektonHub, installerSetType string) error {

	labels := r.getLabels(installerSetType)
	labelSelector, err := common.LabelSelector(labels)
	if err != nil {
		return err
	}

	compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}

	if compInstallerSet != "" {

		ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		ready := ctIs.Status.GetCondition(apis.ConditionReady)
		if ready == nil || ready.Status == corev1.ConditionUnknown {
			return fmt.Errorf("InstallerSet %s: waiting for installation", ctIs.Name)
		} else if ready.Status == corev1.ConditionFalse {
			return fmt.Errorf("InstallerSet %s: ", ready.Message)
		}
	}

	return nil
}

func (r *Reconciler) getSecret(ctx context.Context, name, targetNs string, verifyKeys []string) (*corev1.Secret, error) {
	secret, err := r.kubeClientSet.CoreV1().Secrets(targetNs).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, key := range verifyKeys {
		if _, ok := secret.Data[key]; !ok {
			return nil, errKeyMissing
		}
	}

	return secret, nil
}

func (r *Reconciler) createApiSecret(ctx context.Context, th *v1alpha1.TektonHub, hubDir, comp string) error {

	manifest, err := r.getHubManifest(ctx, th, hubDir, comp)
	if err != nil {
		return err
	}

	secret := manifest.Filter(mf.ByKind("Secret"))
	secretManifest, err := secret.Transform(
		mf.InjectNamespace(th.Spec.GetTargetNamespace()),
	)
	if err != nil {
		return err
	}

	if err := secretManifest.Apply(); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) getHubManifest(ctx context.Context, th *v1alpha1.TektonHub, hubDir, comp string) (*mf.Manifest, error) {
	manifestLocation := filepath.Join(hubDir, comp)

	manifest, err := r.getManifest(ctx, th, manifestLocation)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func createDbSecret(name, namespace string, existingSecret *corev1.Secret, th *v1alpha1.TektonHub) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "db",
			},
			OwnerReferences: []metav1.OwnerReference{getOwnerRef(th)},
		},
		Type: corev1.SecretTypeOpaque,
	}

	if existingSecret != nil && existingSecret.Data != nil {
		s.Data = existingSecret.Data
	}

	s.StringData = make(map[string]string)

	defaultValues := map[string]string{
		secretKeyPostgresHost:     defaultPostgresHost,
		secretKeyPostgresDB:       defaultPostgresDB,
		secretKeyPostgresUser:     defaultPostgresUser,
		secretKeyPostgresPassword: defaultPostgresPassword,
		secretKeyPostgresPort:     defaultPostgresPort,
	}

	// fill default value for absents
	for secretKey, defaultValue := range defaultValues {
		if s.Data[secretKey] == nil || len(s.Data[secretKey]) == 0 {
			s.StringData[secretKey] = defaultValue
		}
	}

	return s
}

// Get an ownerRef of TektonHub
func getOwnerRef(th *v1alpha1.TektonHub) metav1.OwnerReference {
	return *metav1.NewControllerRef(th, th.GroupVersionKind())
}

// add key value pair to the given configmap name
func addConfigMapKeyValue(configMapName, key, value string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "configmap" {
			return nil
		}
		if u.GetName() != configMapName {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		cm.Data[key] = value

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func updateApiConfigMap(th *v1alpha1.TektonHub, configMapName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {

		kind := strings.ToLower(u.GetKind())
		if kind != "configmap" {
			return nil
		}

		if u.GetName() != configMapName {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		// TODO: Remove this condition in the next release
		if th.Spec.Api.HubConfigUrl != "" {

			hubUrlConfigdata, err := getConfigDataFromHubURL(th)
			if err != nil {
				return err
			}
			cm = updateConfigMapDataFromHubConfigURL(th, cm, hubUrlConfigdata)

		} else {

			if len(th.Spec.Categories) > 0 {
				categories := ""
				for _, c := range th.Spec.Categories {
					categories += fmt.Sprintf("- %s\n", c)
				}
				cm.Data["CATEGORIES"] = categories
			}

			if len(th.Spec.Catalogs) > 0 {
				catalogs := ""
				for _, c := range th.Spec.Catalogs {
					catalogs = catalogs + getCatalogData(c, th)
				}
				cm.Data["CATALOGS"] = catalogs
			}

			if len(th.Spec.Scopes) > 0 {
				userScopes := ""
				for _, s := range th.Spec.Scopes {
					scope := ""
					scope += fmt.Sprintf("- name: %s\n", s.Name)
					scope += fmt.Sprintf("  users: [%s]\n", strings.Join(s.Users, ", "))
					userScopes = userScopes + scope
				}
				cm.Data["SCOPES"] = userScopes
			} else {
				cm.Data["SCOPES"] = ""
			}

			if len(th.Spec.Default.Scopes) > 0 {
				defaultScopes := ""
				scopes := fmt.Sprintf("%s\n", defaultScopes)
				for _, d := range th.Spec.Default.Scopes {
					scopes += fmt.Sprintf("  - %s\n", d)
				}
				defaultScopes = fmt.Sprintf(" scopes: \n%s", scopes)
				cm.Data["DEFAULT"] = defaultScopes
			}
		}

		if th.Spec.Api.CatalogRefreshInterval != "" {
			cm.Data["CATALOG_REFRESH_INTERVAL"] = th.Spec.Api.CatalogRefreshInterval
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// TODO: Remove this function in the next release
// Adds Config Data from HubConfigUrl to API Config Map
func updateConfigMapDataFromHubConfigURL(th *v1alpha1.TektonHub, cm *corev1.ConfigMap, hubUrlConfigdata *Data) *corev1.ConfigMap {
	categories := ""
	for _, c := range hubUrlConfigdata.Categories {
		categories += fmt.Sprintf("- %s\n", c.Name)
	}
	cm.Data["CATEGORIES"] = categories

	catalogs := ""
	for _, c := range hubUrlConfigdata.Catalogs {
		catalogs = catalogs + getCatalogData(c, th)
	}
	cm.Data["CATALOGS"] = catalogs

	userScopes := ""
	for _, s := range hubUrlConfigdata.Scopes {
		scope := ""
		scope += fmt.Sprintf("- name: %s\n", s.Name)
		scope += fmt.Sprintf("  users: [%s]\n", strings.Join(s.Users, ", "))
		userScopes = userScopes + scope
	}
	cm.Data["SCOPES"] = userScopes

	defaultScopes := ""
	scopes := fmt.Sprintf("%s\n", defaultScopes)
	for _, d := range hubUrlConfigdata.Default.Scopes {
		scopes += fmt.Sprintf("  - %s\n", d)
	}
	defaultScopes = fmt.Sprintf(" scopes: \n%s", scopes)
	cm.Data["DEFAULT"] = defaultScopes

	return cm
}

func getCatalogData(c v1alpha1.Catalog, th *v1alpha1.TektonHub) string {
	catalogs := ""
	v := reflect.ValueOf(c)

	for i := 0; i < v.NumField(); i++ {
		cat := ""
		key := strings.ToLower(v.Type().Field(i).Name)

		if v.Field(i).Interface() != "" {
			if key == "name" {
				key = "- " + key
				cat += fmt.Sprintf("%s: %s\n", key, v.Field(i).Interface())
			} else {
				cat += fmt.Sprintf("  %s: %s\n", key, v.Field(i).Interface())
			}
			catalogs = catalogs + cat
		}
	}
	return catalogs
}

// TODO: Remove this function in the next release
func getConfigDataFromHubURL(th *v1alpha1.TektonHub) (*Data, error) {
	var data = &Data{}
	if th.Spec.Api.HubConfigUrl != "" {
		resp, err := http.Get(th.Spec.Api.HubConfigUrl)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		viper.SetConfigType("yaml")
		if err := viper.ReadConfig(bytes.NewBuffer(body)); err != nil {
			return nil, err
		}
		if err := viper.Unmarshal(&data); err != nil {
			return nil, err
		}
	}

	return data, nil
}
