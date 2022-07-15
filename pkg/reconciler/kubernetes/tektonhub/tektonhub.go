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
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonhubconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonhub"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
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

var (
	errKeyMissing          error = fmt.Errorf("secret doesn't contains all the keys")
	namespace              string
	errconfigMapKeyMissing error  = fmt.Errorf("configMap doesn't contains all the keys")
	db                     string = fmt.Sprintf("%s-%s", hubprefix, "db")
	dbMigration            string = fmt.Sprintf("%s-%s", hubprefix, "db-migration")
	api                    string = fmt.Sprintf("%s-%s", hubprefix, "api")
	ui                     string = fmt.Sprintf("%s-%s", hubprefix, "ui")
	// Check that our Reconciler implements controller.Reconciler
	_ tektonhubconciler.Interface = (*Reconciler)(nil)
	_ tektonhubconciler.Finalizer = (*Reconciler)(nil)

	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.HubResourceName,
		},
	}

	checkRecreate = map[string]bool{
		dbMigrationKey: false,
		apiKey:         false,
	}
)

const (
	hubprefix               = "tekton-hub"
	dbInstallerSet          = "DbInstallerSet"
	dbMigrationInstallerSet = "DbMigrationInstallerSet"
	apiInstallerSet         = "ApiInstallerSet"
	uiInstallerSet          = "UiInstallerSet"
	createdByValue          = "TektonHub"
	dbSecretName            = "tekton-hub-db"
	dbMigrationKey          = "dbMigration"
	apiKey                  = "api"
)

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
	namespace = th.Spec.GetTargetNamespace()

	if err := r.targetNamespaceCheck(ctx, th); err != nil {
		return nil
	}

	version := common.TargetVersion(th)
	hubDir := filepath.Join(common.ComponentDir(th), version)

	// Create the API route based on platform
	if err := r.extension.PreReconcile(ctx, th); err != nil {
		return err
	}
	th.Status.MarkPreReconcilerComplete()

	// TODO: remove this after operator openshift-build version 1.8
	if err := r.checkDbApiPVCOwnerRef(ctx, th); err != nil {
		return err
	}

	// TODO: remove this after operator openshift-build version 1.8
	if err := r.getAndUpdateHubInstallerSetLabels(ctx); err != nil {
		return err
	}

	if err := r.checkIfUserHasDb(ctx, th, hubDir, version); err != nil {
		return err
	}

	// Manage DB migration
	if err := r.manageDbMigrationComponent(ctx, th, hubDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkDatabasebMigrationDone()

	// Manage API
	if err := r.manageApiComponent(ctx, th, hubDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkApiInstallerSetAvailable()

	// Manage UI
	if err := r.manageUiComponent(ctx, th, hubDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkUiInstallerSetAvailable()

	if err := r.extension.PostReconcile(ctx, th); err != nil {
		return err
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

func (r *Reconciler) manageUiComponent(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	if err := r.validateUiConfigMap(ctx, th); err != nil {
		th.Status.MarkUiDependencyMissing(fmt.Sprintf("UI config map not present: %v", err.Error()))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	th.Status.MarkUiDependenciesInstalled()

	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, ui)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkUiInstallerSetNotAvailable("UI installer set not available")
		uiLocation := filepath.Join(hubDir, "ui")

		manifest, err := r.getManifest(ctx, th, uiLocation)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, uiInstallerSet, version, ui)
		if err != nil {
			return err
		}

	}

	err = r.checkComponentStatus(ctx, th, uiInstallerSet)
	if err != nil {
		th.Status.MarkUiInstallerSetNotAvailable(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	return nil
}

func (r *Reconciler) manageApiComponent(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {

	if th.Spec.Db.DbSecretName != "" && !checkRecreation(apiKey) {
		if err := r.getAndDeleteInstallerSet(ctx, api); err != nil {
			return err
		}

		// This flags make sure that if the secret is provided in
		// future by user then apply the api installerset again to
		// populate the data
		setCheckAndRecreate(apiKey, true)

		return nil
	}

	// Validate whether the secrets and configmap are created for API
	if err := r.validateApiDependencies(ctx, th, hubDir, "api"); err != nil {
		th.Status.MarkApiDependencyMissing("api secrets not present")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	th.Status.MarkApiDependenciesInstalled()

	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, api)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkApiInstallerSetNotAvailable("API installer set not available")
		apiLocation := filepath.Join(hubDir, "api")

		manifest, err := r.getManifest(ctx, th, apiLocation)
		if err != nil {
			return err
		}

		err = applyPVC(ctx, manifest, th)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, apiInstallerSet, version, api)
		if err != nil {
			return err
		}
	}

	err = r.checkComponentStatus(ctx, th, apiInstallerSet)
	if err != nil {
		th.Status.MarkApiInstallerSetNotAvailable(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (r *Reconciler) manageDbMigrationComponent(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {

	if th.Spec.Db.DbSecretName != "" && !checkRecreation(dbMigrationKey) {

		if err := r.getAndDeleteInstallerSet(ctx, dbMigration); err != nil {
			return err
		}

		// This flags make sure that if the secret is provided in
		// future by user then apply the db-migration installerset
		// again to populate the data
		setCheckAndRecreate(dbMigrationKey, true)

		return nil
	}

	// Check if the InstallerSet is available for DB-migration
	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, dbMigration)
	if err != nil {
		return err
	}

	if !exist {
		dbMigrationLocation := filepath.Join(hubDir, "db-migration")
		th.Status.MarkDatabasebMigrationFailed("DB migration installerset not available")

		manifest, err := r.getManifest(ctx, th, dbMigrationLocation)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, dbMigrationInstallerSet, version, dbMigration)
		if err != nil {
			return err
		}
	}

	err = r.checkComponentStatus(ctx, th, dbMigrationInstallerSet)
	if err != nil {
		th.Status.MarkDatabasebMigrationFailed(err.Error())
		return v1alpha1.RECONCILE_AGAIN_ERR
	}
	return nil
}

func (r *Reconciler) manageDbComponent(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	// Check if the DB secrets are created
	if err := r.validateOrCreateDBSecrets(ctx, th); err != nil {
		th.Status.MarkDbDependencyMissing("db secrets are either invalid or not present")
		return err
	}
	th.Status.MarkDbDependenciesInstalled()

	exist, err := r.checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, db)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkDbInstallerSetNotAvailable("DB installer set not available")
		dbLocation := filepath.Join(hubDir, "db")
		manifest, err := r.getManifest(ctx, th, dbLocation)
		if err != nil {
			return err
		}

		err = applyPVC(ctx, manifest, th)
		if err != nil {
			return err
		}

		err = r.setUpAndCreateInstallerSet(ctx, *manifest, th, dbInstallerSet, version, db)
		if err != nil {
			return err
		}
	}

	err = r.checkComponentStatus(ctx, th, dbInstallerSet)
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
	dbKeys := []string{"POSTGRES_HOST", "POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_PORT"}

	// th.Status.MarkDbDependencyInstalling("db secrets are being added into the namespace")

	dbSecret, err := r.getSecret(ctx, dbSecretName, namespace, dbKeys)
	if err != nil {
		newDbSecret := createDbSecret(dbSecretName, namespace, dbSecret, th)
		if apierrors.IsNotFound(err) {
			_, err = r.kubeClientSet.CoreV1().Secrets(namespace).Create(ctx, newDbSecret, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", dbSecretName))
				return err
			}
			return nil
		}
		if err == errKeyMissing {
			_, err = r.kubeClientSet.CoreV1().Secrets(namespace).Update(ctx, newDbSecret, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", dbSecretName))
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
	apiConfigMapKeys := []string{"CONFIG_FILE_URL", "CATALOG_REFRESH_INTERVAL"}

	th.Status.MarkApiDependencyInstalling("checking for api secrets in the namespace and creating the ConfigMap")

	_, err := r.getSecret(ctx, th.Spec.Api.ApiSecretName, namespace, apiSecretKeys)
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

	_, err = r.getConfigMap(ctx, api, namespace, apiConfigMapKeys)
	if err != nil {
		if apierrors.IsNotFound(err) {
			configMap := createApiConfigMap(api, namespace, th)
			_, err = r.kubeClientSet.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkApiDependencyMissing(fmt.Sprintf("%s configMap is missing", api))
				return err
			}
			return nil
		}
		if err == errKeyMissing {
			th.Status.MarkApiDependencyMissing(fmt.Sprintf("%s configMap is missing the keys", api))
			return err
		} else {
			logger.Error(err)
			return err
		}
	}

	return nil
}

func (r *Reconciler) validateUiConfigMap(ctx context.Context, th *v1alpha1.TektonHub) error {
	logger := logging.FromContext(ctx)

	uiConfigMapKeys := []string{"API_URL", "AUTH_BASE_URL", "API_VERSION", "REDIRECT_URI"}
	_, err := r.getConfigMap(ctx, ui, namespace, uiConfigMapKeys)
	if err != nil {
		if apierrors.IsNotFound(err) {
			configMap := createUiConfigMap(ui, namespace, th)
			_, err = r.kubeClientSet.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkUiDependencyMissing(fmt.Sprintf("%s configMap is missing", ui))
				return err
			}
			return nil
		}
		if err == errconfigMapKeyMissing {
			th.Status.MarkUiDependencyMissing(fmt.Sprintf("%s configMap is missing the keys", ui))
			return err
		} else {
			logger.Error(err)
			return err
		}
	}

	return nil
}

func createUiConfigMap(name, namespace string, th *v1alpha1.TektonHub) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"ui": "tektonhub-ui",
			},
			OwnerReferences: []metav1.OwnerReference{getOwnerRef(th)},
		},
		Data: map[string]string{
			"API_URL":       th.Status.ApiRouteUrl,
			"AUTH_BASE_URL": th.Status.AuthRouteUrl,
			"API_VERSION":   "v1",
			"REDIRECT_URI":  th.Status.UiRouteUrl,
		},
	}
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
		mf.InjectNamespace(namespace),
		common.DeploymentImages(images),
		common.JobImages(images),
	}
	trans = append(trans, extra...)

	manifest, err := manifest.Transform(trans...)

	if err != nil {
		logger.Error("failed to transform manifest")
		return nil, err
	}

	return &manifest, nil
}

// TODO: remove this after operator openshift-build version 1.8
func (r *Reconciler) getAndUpdateHubInstallerSetLabels(ctx context.Context) error {
	// Get and Update db labels
	dbIs, err := r.getHubInstallerSet(ctx, db)
	if err != nil {
		return err
	}

	if dbIs != nil {
		dbIs.Labels = r.getLabels(db).MatchLabels
		if err := r.updateHubInstallerSet(ctx, dbIs); err != nil {
			return err
		}
	}

	// Get and update db-migration labels
	dbMigrationIs, err := r.getHubInstallerSet(ctx, dbMigration)
	if err != nil {
		return err
	}

	if dbMigrationIs != nil {
		dbMigrationIs.Labels = r.getLabels(dbMigration).MatchLabels
		if err := r.updateHubInstallerSet(ctx, dbMigrationIs); err != nil {
			return err
		}
	}

	// Get and update api labels
	apiIs, err := r.getHubInstallerSet(ctx, api)
	if err != nil {
		return err
	}

	if apiIs != nil {
		apiIs.Labels = r.getLabels(api).MatchLabels
		if err := r.updateHubInstallerSet(ctx, apiIs); err != nil {
			return err
		}
	}

	// Get and update ui labels
	uiIs, err := r.getHubInstallerSet(ctx, ui)
	if err != nil {
		return err
	}

	if uiIs != nil {
		uiIs.Labels = r.getLabels(ui).MatchLabels
		if err := r.updateHubInstallerSet(ctx, uiIs); err != nil {
			return err
		}
	}

	return nil
}

// Check if user has it's own db, if not create the default database
func (r *Reconciler) checkIfUserHasDb(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {

	dbKeys := []string{"POSTGRES_HOST", "POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_PORT"}

	if th.Spec.Db.DbSecretName == "" {
		th.Status.MarkDbDependencyInstalling("db secrets are being added into the namespace")
		// Manage DB
		if err := r.manageDbComponent(ctx, th, hubDir, version); err != nil {
			return r.handleError(err, th)
		}
		th.Status.MarkDbInstallerSetAvailable()

		// This flags make sure that if the secret is
		// provided in future by user then apply the
		// api and db-migration installerset again
		setCheckAndRecreate(dbMigrationKey, false)
		setCheckAndRecreate(apiKey, false)

	} else {
		_, err := r.getSecret(ctx, th.Spec.Db.DbSecretName, th.Spec.GetTargetNamespace(), dbKeys)
		if err != nil {
			return err
		}

		// Mark the databse as ready state as the
		// database is already installed by the user
		th.Status.MarkDbDependenciesInstalled()
		th.Status.MarkDbInstallerSetAvailable()

		// Get and delete db installerset
		if err := r.getAndDeleteDbInstallerSet(ctx); err != nil {
			return err
		}

	}

	return nil
}

func (r *Reconciler) getHubInstallerSet(ctx context.Context, prefixName string) (*v1alpha1.TektonInstallerSet, error) {
	labels := getOldLabels(prefixName)

	labelSelector, err := common.LabelSelector(labels)
	if err != nil {
		return nil, err
	}

	ctIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	if len(ctIs.Items) == 0 {
		return nil, nil
	}

	if len(ctIs.Items) == 1 {
		return &ctIs.Items[0], nil
	}

	// len(iSets.Items) > 1
	// delete all installerSets as it cannot be decided which one is the desired one
	err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().DeleteCollection(ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return nil, err
	}
	return nil, v1alpha1.RECONCILE_AGAIN_ERR
}

func (r *Reconciler) updateHubInstallerSet(ctx context.Context, installerSet *v1alpha1.TektonInstallerSet) error {
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().Update(ctx, installerSet, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) getAndDeleteDbInstallerSet(ctx context.Context) error {
	labels := r.getLabels(db)
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

func getOldLabels(installerSetPrefix string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.HubResourceName,
			v1alpha1.Component:        installerSetPrefix,
		},
	}
}

func (r *Reconciler) getLabels(componentInstallerSetType string) metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:      createdByValue,
			v1alpha1.InstallerSetType:  componentInstallerSetType,
			v1alpha1.ReleaseVersionKey: r.operatorVersion,
		},
	}
}

// TODO: remove this after operator openshift-build version 1.8
func (r *Reconciler) checkDbApiPVCOwnerRef(ctx context.Context, th *v1alpha1.TektonHub) error {
	// Check and update pvc for db component
	dbPvc, err := r.checkPVC(ctx, th, "tekton-hub-db")
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if dbPvc != nil {
		if err := r.checkAndUpdatePVCOwnerRef(ctx, dbPvc, th); err != nil {
			return err
		}
	}

	// Check and update pvc for api component
	apiPvc, err := r.checkPVC(ctx, th, "tekton-hub-api")
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if apiPvc != nil {
		if err := r.checkAndUpdatePVCOwnerRef(ctx, apiPvc, th); err != nil {
			return err
		}
	}

	return nil
}

func checkRecreation(key string) bool {
	return checkRecreate[key]
}

func setCheckAndRecreate(key string, value bool) {
	checkRecreate[key] = value
}

// TODO: remove this after operator openshift-build version 1.8
// This patch checks if the ownerRef is set to `TektonHub`,
// if not it sets and updates the ownerRef of pvc to `TektonHub`
func (r *Reconciler) checkAndUpdatePVCOwnerRef(ctx context.Context, pvc *corev1.PersistentVolumeClaim, th *v1alpha1.TektonHub) error {
	if !r.checkPVCOwnerRef(pvc, th) {
		ownerRef := *metav1.NewControllerRef(th, th.GroupVersionKind())
		pvc.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

		_, err := r.kubeClientSet.CoreV1().PersistentVolumeClaims(th.Spec.GetTargetNamespace()).Update(ctx, pvc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) checkPVC(ctx context.Context, th *v1alpha1.TektonHub, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc, err := r.kubeClientSet.CoreV1().PersistentVolumeClaims(th.Spec.GetTargetNamespace()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

// TODO: remove this after operator openshift-build version 1.8
func (r *Reconciler) checkPVCOwnerRef(pvc *corev1.PersistentVolumeClaim, th *v1alpha1.TektonHub) bool {
	if len(pvc.GetOwnerReferences()) == 1 {
		if pvc.GetOwnerReferences()[0].Kind == th.Kind {
			return true
		}
	}
	return false
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

func (r *Reconciler) setUpAndCreateInstallerSet(ctx context.Context, manifest mf.Manifest, th *v1alpha1.TektonHub, installerSetName, version, prefixName string) error {
	manifest = manifest.Filter(mf.Not(mf.Any(mf.ByKind("Secret"), mf.ByKind("PersistentVolumeClaim"), mf.ByKind("Namespace"), mf.ByKind("ConfigMap"))))

	labels := r.getLabels(prefixName).MatchLabels
	if err := createInstallerSet(ctx, r.operatorClientSet, th, manifest,
		version, installerSetName, prefixName, namespace, labels); err != nil {
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

func (r *Reconciler) checkComponentStatus(ctx context.Context, th *v1alpha1.TektonHub, component string) error {

	// Check if installer set is already created
	compInstallerSet, ok := th.Status.HubInstallerSet[component]
	if !ok {
		return nil
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

func (r *Reconciler) getSecret(ctx context.Context, name, targetNs string, keys []string) (*corev1.Secret, error) {
	secret, err := r.kubeClientSet.CoreV1().Secrets(targetNs).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return nil, errKeyMissing
		}
	}

	return secret, nil
}

func (r *Reconciler) getConfigMap(ctx context.Context, name, targetNs string, keys []string) (*corev1.ConfigMap, error) {
	configMap, err := r.kubeClientSet.CoreV1().ConfigMaps(targetNs).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		if _, ok := configMap.Data[key]; !ok {
			return nil, errKeyMissing
		}
	}

	return configMap, nil
}

func createApiConfigMap(name, namespace string, th *v1alpha1.TektonHub) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "api",
			},
			OwnerReferences: []metav1.OwnerReference{getOwnerRef(th)},
		},
		Data: map[string]string{
			"CONFIG_FILE_URL":          th.Spec.Api.HubConfigUrl,
			"CATALOG_REFRESH_INTERVAL": th.Spec.Api.CatalogRefreshInterval,
		},
	}
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

	if s.Data["POSTGRES_DB"] == nil || len(s.Data["POSTGRES_DB"]) == 0 {
		s.StringData["POSTGRES_DB"] = "hub"
	}

	if s.Data["POSTGRES_USER"] == nil || len(s.Data["POSTGRES_USER"]) == 0 {
		s.StringData["POSTGRES_USER"] = "hub"
	}

	if s.Data["POSTGRES_PASSWORD"] == nil || len(s.Data["POSTGRES_PASSWORD"]) == 0 {
		s.StringData["POSTGRES_PASSWORD"] = "hub"
	}

	if s.Data["POSTGRES_PORT"] == nil || len(s.Data["POSTGRES_PORT"]) == 0 {
		s.StringData["POSTGRES_PORT"] = "5432"
	}

	if s.Data["POSTGRES_HOST"] == nil || len(s.Data["POSTGRES_HOST"]) == 0 {
		s.StringData["POSTGRES_HOST"] = "tekton-hub-db"
	}

	return s
}

// Get an ownerRef of TektonHub
func getOwnerRef(th *v1alpha1.TektonHub) metav1.OwnerReference {
	return *metav1.NewControllerRef(th, th.GroupVersionKind())
}

func (r *Reconciler) targetNamespaceCheck(ctx context.Context, th *v1alpha1.TektonHub) error {
	_, err := r.kubeClientSet.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := common.CreateTargetNamespace(ctx, map[string]string{}, th, r.kubeClientSet); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}
