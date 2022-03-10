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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonhubconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonhub"
	"github.com/tektoncd/operator/pkg/reconciler/common"
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
	extension common.Extension
	// enqueueAfter enqueues a obj after a duration
	enqueueAfter func(obj interface{}, after time.Duration)
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
)

const (
	hubprefix               = "tekton-hub"
	dbInstallerSet          = "DbInstallerSet"
	dbMigrationInstallerSet = "DbMigrationInstallerSet"
	apiInstallerSet         = "ApiInstallerSet"
	uiInstallerSet          = "UiInstallerSet"
	createdByValue          = "TektonHub"
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

	// Manage DB
	if err := r.manageDbComponent(ctx, th, hubDir, version); err != nil {
		return r.handleError(err, th)
	}
	th.Status.MarkDbInstallerSetAvailable()

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
		r.enqueueAfter(th, 10*time.Second)
		return nil
	}
	return err
}

func (r *Reconciler) manageUiComponent(ctx context.Context, th *v1alpha1.TektonHub, hubDir, version string) error {
	if err := r.validateUiConfigMap(ctx, th); err != nil {
		th.Status.MarkUiDependencyMissing(fmt.Sprintf("UI config map not present: %v", err.Error()))
		r.enqueueAfter(th, 10*time.Second)
		return nil
	}

	th.Status.MarkUiDependenciesInstalled()

	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, uiInstallerSet)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkUiInstallerSetNotAvailable("UI installer set not available")
		uiLocation := filepath.Join(hubDir, "ui")
		err := r.setupAndCreateInstallerSet(ctx, uiLocation, th, uiInstallerSet, version, ui)
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
	// Validate whether the secrets and configmap are created for API
	if err := r.validateApiDependencies(ctx, th); err != nil {
		th.Status.MarkApiDependencyMissing("api secrets not present")
		r.enqueueAfter(th, 10*time.Second)
		return err
	}

	th.Status.MarkApiDependenciesInstalled()

	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, apiInstallerSet)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkApiInstallerSetNotAvailable("API installer set not available")
		apiLocation := filepath.Join(hubDir, "api")
		err := r.setupAndCreateInstallerSet(ctx, apiLocation, th, apiInstallerSet, version, api)
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
	// Check if the InstallerSet is available for DB-migration
	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, dbMigrationInstallerSet)
	if err != nil {
		return err
	}

	if !exist {
		dbMigrationLocation := filepath.Join(hubDir, "db-migration")
		th.Status.MarkDatabasebMigrationFailed("DB migration installerset not available")
		err = r.setupAndCreateInstallerSet(ctx, dbMigrationLocation, th, dbMigrationInstallerSet, version, dbMigration)
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

	exist, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, version, th, dbInstallerSet)
	if err != nil {
		return err
	}

	if !exist {
		th.Status.MarkDbInstallerSetNotAvailable("DB installer set not available")
		dbLocation := filepath.Join(hubDir, "db")
		err := r.setupAndCreateInstallerSet(ctx, dbLocation, th, dbInstallerSet, version, db)
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
	dbKeys := []string{"POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_PORT"}

	// th.Status.MarkDbDependencyInstalling("db secrets are being added into the namespace")

	dbSecret, err := r.getSecret(ctx, th.Spec.Db.DbSecretName, namespace, dbKeys)
	if err != nil {
		newDbSecret := createDbSecret(th.Spec.Db.DbSecretName, namespace, dbSecret)
		if apierrors.IsNotFound(err) {
			_, err = r.kubeClientSet.CoreV1().Secrets(namespace).Create(ctx, newDbSecret, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", th.Spec.Db.DbSecretName))
				return err
			}
			return nil
		}
		if err == errKeyMissing {
			_, err = r.kubeClientSet.CoreV1().Secrets(namespace).Update(ctx, newDbSecret, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err)
				th.Status.MarkDbDependencyMissing(fmt.Sprintf("%s secret is missing", th.Spec.Db.DbSecretName))
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
func (r *Reconciler) validateApiDependencies(ctx context.Context, th *v1alpha1.TektonHub) error {
	logger := logging.FromContext(ctx)
	apiSecretKeys := []string{"GH_CLIENT_ID", "GH_CLIENT_SECRET", "JWT_SIGNING_KEY", "ACCESS_JWT_EXPIRES_IN", "REFRESH_JWT_EXPIRES_IN", "GHE_URL"}
	apiConfigMapKeys := []string{"CONFIG_FILE_URL"}

	th.Status.MarkApiDependencyInstalling("checking for api secrets in the namespace and creating the ConfigMap")

	_, err := r.getSecret(ctx, th.Spec.Api.ApiSecretName, namespace, apiSecretKeys)
	if err != nil {
		if apierrors.IsNotFound(err) {
			th.Status.MarkApiDependencyMissing(fmt.Sprintf("%s secret is missing", th.Spec.Api.ApiSecretName))
			return err
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
		},
		Data: map[string]string{
			"API_URL":       th.Status.ApiRouteUrl,
			"AUTH_BASE_URL": th.Status.AuthRouteUrl,
			"API_VERSION":   "v1",
			"REDIRECT_URI":  th.Status.UiRouteUrl,
		},
	}
}

func (r *Reconciler) setupAndCreateInstallerSet(ctx context.Context, manifestLocation string, th *v1alpha1.TektonHub, installerSetName, version, prefixName string) error {
	manifest := r.manifest.Append()
	logger := logging.FromContext(ctx)

	if err := common.AppendManifest(&manifest, manifestLocation); err != nil {
		return err
	}

	manifest = manifest.Filter(mf.Not(mf.Any(mf.ByKind("Secret"), mf.ByKind("Namespace"), mf.ByKind("ConfigMap"))))

	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.HubImagePrefix))
	trans := r.extension.Transformers(th)
	extra := []mf.Transformer{
		mf.InjectOwner(th),
		mf.InjectNamespace(namespace),
		common.DeploymentImages(images),
	}
	trans = append(trans, extra...)

	manifest, err := manifest.Transform(trans...)

	if err != nil {
		logger.Error("failed to transform manifest")
		return err
	}

	if err := createInstallerSet(ctx, r.operatorClientSet, th, manifest,
		version, installerSetName, prefixName, namespace); err != nil {
		return err
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
		},
		Data: map[string]string{
			"CONFIG_FILE_URL": th.Spec.Api.HubConfigUrl,
		},
	}
}

func createDbSecret(name, namespace string, existingSecret *corev1.Secret) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "db",
			},
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

	return s
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
