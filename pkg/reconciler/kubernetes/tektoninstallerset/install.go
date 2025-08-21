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

package tektoninstallerset

import (
	"context"
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	annotationsPath = "metadata.annotations"
	labelsPath      = "metadata.labels"
)

type installer struct {
	manifest        *mf.Manifest
	mfClient        mf.Client
	kubeClientSet   kubernetes.Interface
	logger          *zap.SugaredLogger
	crds            []unstructured.Unstructured
	clusterScoped   []unstructured.Unstructured
	namespaceScoped []unstructured.Unstructured
	deployment      []unstructured.Unstructured
	statefulset     []unstructured.Unstructured
	job             []unstructured.Unstructured
}

func NewInstaller(manifest *mf.Manifest, mfClient mf.Client, kubeClientSet kubernetes.Interface, logger *zap.SugaredLogger) *installer {
	installer := &installer{
		manifest:        manifest,
		mfClient:        mfClient,
		kubeClientSet:   kubeClientSet,
		logger:          logger,
		crds:            []unstructured.Unstructured{},
		clusterScoped:   []unstructured.Unstructured{},
		namespaceScoped: []unstructured.Unstructured{},
		deployment:      []unstructured.Unstructured{},
		statefulset:     []unstructured.Unstructured{},
		job:             []unstructured.Unstructured{},
	}

	// we filter out resource as some resources are dependent on others
	// for eg. namespace should be created before configmap
	// non k8s core resources like openshift resources will be classified as
	// namespace scoped
	for _, res := range manifest.Resources() {
		if strings.ToLower(res.GetKind()) == "customresourcedefinition" {
			installer.crds = append(installer.crds, res)
			continue
		} else if res.GetKind() == "Deployment" {
			installer.deployment = append(installer.deployment, res)
			continue
		} else if res.GetKind() == "StatefulSet" {
			installer.statefulset = append(installer.statefulset, res)
			continue
		} else if res.GetKind() == "Job" {
			installer.job = append(installer.job, res)
			continue
		}
		if isClusterScoped(res.GetKind()) && strings.ToLower(res.GetKind()) != "clusterrolebinding" {
			installer.clusterScoped = append(installer.clusterScoped, res)
			continue
		}
		installer.namespaceScoped = append(installer.namespaceScoped, res)
	}
	return installer
}

// https://github.com/manifestival/manifestival/blob/af1baacf01ec54390c3cbd46ee561d52b2b4ab14/transform.go#L107
func isClusterScoped(kind string) bool {
	switch strings.ToLower(kind) {
	case "componentstatus",
		"namespace",
		"node",
		"persistentvolume",
		"mutatingwebhookconfiguration",
		"validatingwebhookconfiguration",
		"customresourcedefinition",
		"apiservice",
		"meshpolicy",
		"tokenreview",
		"selfsubjectaccessreview",
		"selfsubjectrulesreview",
		"subjectaccessreview",
		"certificatesigningrequest",
		"clusterrolebinding",
		"clusterrole",
		"priorityclass",
		"storageclass",
		"volumeattachment":
		return true
	}
	return false
}

func (i *installer) ensureResources(resources []unstructured.Unstructured) error {
	for _, r := range resources {
		ressourceLogger := i.logger.With(
			"kind", r.GetKind(),
			"namespace", r.GetNamespace(),
			"name", r.GetName(),
		)
		expectedHash, err := hash.Compute(r.Object)
		if err != nil {
			ressourceLogger.Error("failed to compute resource hash", "error", err)
			return err
		}
		ressourceLogger.Debug("fetching resource")

		res, err := i.mfClient.Get(&r)
		if err != nil {
			if apierrs.IsNotFound(err) {
				ressourceLogger.Debug("creating new resource")
				// add hash on the resource of expected manifest and create
				anno := r.GetAnnotations()
				if anno == nil {
					anno = map[string]string{}
				}
				anno[v1alpha1.LastAppliedHashKey] = expectedHash
				r.SetAnnotations(anno)
				err = i.mfClient.Create(&r)
				if err != nil {
					ressourceLogger.Error("failed to create resource", "error", err)
					return err
				}
				ressourceLogger.Debug("resource created successfully")
				continue
			}
			ressourceLogger.Error("failed to get resource", "error", err)
			return err
		}

		if res.GetDeletionTimestamp() != nil {
			ressourceLogger.Debug("resource is being deleted, will reconcile again")
			return v1alpha1.RECONCILE_AGAIN_ERR
		}

		ressourceLogger.Debug("resource exists, checking for updates")

		// if resource exist then check if expected hash is different from the one
		// on the resource
		hashOnResource := res.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if expectedHash == hashOnResource {
			ressourceLogger.Debug("resource is up-to-date, no changes needed")
			continue
		}

		ressourceLogger.Debug("resource needs update",
			"currentHash", hashOnResource,
			"expectedHash", expectedHash)

		anno := r.GetAnnotations()
		if anno == nil {
			anno = map[string]string{}
		}
		anno[v1alpha1.LastAppliedHashKey] = expectedHash
		r.SetAnnotations(anno)

		installManifests, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{r}), mf.UseClient(i.mfClient))
		if err != nil {
			ressourceLogger.Error("failed to create manifest", "error", err)
			return err
		}
		if err := installManifests.Apply(); err != nil {
			ressourceLogger.Error("failed to apply manifest", "error", err)
			return err
		}
		ressourceLogger.Debug("resource updated successfully")
	}
	return nil
}

func (i *installer) EnsureCRDs() error {
	return i.ensureResources(i.crds)
}

func (i *installer) EnsureClusterScopedResources() error {
	return i.ensureResources(i.clusterScoped)
}

func (i *installer) EnsureNamespaceScopedResources() error {
	return i.ensureResources(i.namespaceScoped)
}

func (i *installer) EnsureStatefulSetResources(ctx context.Context) error {
	for _, s := range i.statefulset {
		if err := i.ensureResource(ctx, &s); err != nil {
			return err
		}
		if err := i.isStatefulSetAvailable(&s); err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) EnsureDeploymentResources(ctx context.Context) error {
	for _, d := range i.deployment {
		if err := i.ensureResource(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) EnsureJobResources() error {
	return i.ensureResources(i.job)
}

// list of fields should be reconciled
func (i *installer) resourceReconcileFields(u *unstructured.Unstructured) []string {
	switch u.GetKind() {
	case "Deployment", "StatefulSet":
		return []string{
			annotationsPath,
			labelsPath,
			"spec",
		}

	default:
		return []string{}
	}
}

// this method is written as generic to all the resources
// currently tested with deployments and StatefulSet
// TODO: (jkandasa) needs to be tested with other resources too
func (i *installer) ensureResource(ctx context.Context, expected *unstructured.Unstructured) error {
	loggerWithContext := i.logger.With(
		"name", expected.GetName(),
		"namespace", expected.GetNamespace(),
		"kind", expected.GetKind(),
	)
	loggerWithContext.Debug("verifying a resource")

	// update specific things to deployments and statefulSets
	if expected.GetKind() == "Deployment" || expected.GetKind() == "StatefulSet" {

		// update proxy settings
		err := common.ApplyProxySettings(expected)
		if err != nil {
			loggerWithContext.Errorw("failed to apply proxy settings", "error", err)
			return err
		}

		// if a deployment or statefulSets managed by HPA, ignore replicas from user input(TektonConfig CR)
		// and take replicas from HPA status(DesiredReplicas)

		// lists the available HPAs
		hpaList, err := i.kubeClientSet.AutoscalingV2().HorizontalPodAutoscalers(expected.GetNamespace()).List(ctx, metav1.ListOptions{})
		if err != nil {
			loggerWithContext.Errorw("failed to list HPAs", "error", err)
			return err
		}

		// check the expected resource configured with HPA
		var hpa *autoscalingv2.HorizontalPodAutoscaler
		for _, _hpa := range hpaList.Items {
			target := _hpa.Spec.ScaleTargetRef
			if target.Kind == expected.GetKind() && target.Name == expected.GetName() {
				hpa = _hpa.DeepCopy()
				break
			}
		}

		// if a hpa found to this resource, update replicas value from the hpa
		if hpa != nil {
			hpaLogger := loggerWithContext.With(
				"hpaName", hpa.GetName(),
				"hpaNamespace", hpa.GetNamespace(),
			)
			hpaLogger.Debug("HPA found for resource, verifying replicas")

			hpaScalingDisabled := true
			// verify HPA status from ScalingActive condition
			for _, condition := range hpa.Status.Conditions {
				if condition.Type == autoscalingv2.ScalingActive && condition.Status != corev1.ConditionFalse {
					hpaScalingDisabled = false
					break
				}
			}

			// working description:
			//---------------------
			// variables description:
			// - desiredReplicas - taken from hpa status.desiredReplicas
			// - minReplicas - taken from hpa spec.minReplicas. this can be nil or zero. we set it as 1, if the value is nil or zero.
			// - maxReplicas - taken from hpa spec.maxReplicas
			// - manifestReplicas - taken from expected resource(manifest), can be a deployment or statefulSet the value is from spec.replicas
			// The desiredReplicas calculated as follows,
			// - if scaling is enabled compares minReplicas and desiredReplicas from hpa, take the higher one
			// - if scaling is disabled, take manifestReplicas and compare with scaling range from hpa
			// -- if the manifestReplicas value is lesser than the minReplicas, takes minReplicas as desiredReplicas
			// -- if the manifestReplicas value is higher than the maxReplicas, takes the maxReplicas as desiredReplicas
			// -- if the manifestReplicas value is in range. that is "minReplicas >= manifestReplicas <= maxReplicas", takes manifestReplicas as desiredReplicas

			desiredReplicas := hpa.Status.DesiredReplicas
			maxReplicas := hpa.Spec.MaxReplicas
			minReplicas := hpa.Spec.MinReplicas
			// minReplicas can be nil or zero. in that case, we keep it as 1
			if minReplicas == nil || *minReplicas == 0 {
				minReplicas = ptr.Int32(1)
			}

			if hpaScalingDisabled {
				hpaLogger.Info("HPA scaling disabled, adjusting replicas to scaling range")

				manifestReplicas, manifestReplicasFound, err := unstructured.NestedInt64(expected.Object, "spec", "replicas")
				if err != nil {
					hpaLogger.Errorw("failed to get manifest replicas", "error", err)
				} else if !manifestReplicasFound {
					hpaLogger.Debug("manifest replicas not found, defaulting to 1")
					// set default value as 1
					manifestReplicas = 1
				}

				// adjust the manifest replicas value to hpa's scaling range
				if manifestReplicas < int64(*minReplicas) {
					originalReplicas := manifestReplicas
					manifestReplicas = int64(*minReplicas)
					hpaLogger.Infow("adjusted replicas to minReplicas",
						"originalReplicas", originalReplicas,
						"newReplicas", manifestReplicas,
						"minReplicas", *minReplicas,
					)
				} else if manifestReplicas > int64(maxReplicas) {
					originalReplicas := manifestReplicas
					manifestReplicas = int64(maxReplicas)
					hpaLogger.Infow("adjusted replicas to maxReplicas",
						"originalReplicas", originalReplicas,
						"newReplicas", manifestReplicas,
						"maxReplicas", maxReplicas,
					)
				}

				// updates the desiredReplicas
				desiredReplicas = int32(manifestReplicas)

			} else { // hpa scaling is enabled
				hpaLogger.Debugw("HPA scaling enabled",
					"desiredReplicas", desiredReplicas,
					"minReplicas", *minReplicas,
					"maxReplicas", maxReplicas,
					"scaleTargetKind", hpa.Spec.ScaleTargetRef.Kind,
					"scaleTargetName", hpa.Spec.ScaleTargetRef.Name,
				)

				// if there is no metrics data available in the cluster the HPA desiredReplicas will be zero
				// compare minReplicas and desiredReplicas and take the higher one
				if desiredReplicas < *minReplicas {
					hpaLogger.Debugw("HPA desiredReplicas less than minReplicas, adjusting desiredReplicas to minReplicas",
						"OriginalDesiredReplicas", desiredReplicas,
						"minReplicas", *minReplicas,
						"scaleTargetKind", hpa.Spec.ScaleTargetRef.Kind,
						"scaleTargetName", hpa.Spec.ScaleTargetRef.Name,
					)
					desiredReplicas = *minReplicas
				}
			}

			hpaLogger.Infow("calculated final desired replicas", "desiredReplicas", desiredReplicas, "hpaScalingEnabled", !hpaScalingDisabled)

			// update the replicas value from HPA in expected object
			// note: converting the replicas value to int64, "DeepCopyJSONValue" not accepts int32, it is available inside "SetNestedField"
			err = unstructured.SetNestedField(expected.Object, int64(desiredReplicas), "spec", "replicas")
			if err != nil {
				hpaLogger.Errorw("failed to set replicas value", "error", err)
				return err
			}

		}
	}

	// check if the resource already exists
	existing, err := i.mfClient.Get(expected)
	if err != nil {
		// If the resource doesn't exist, then create new
		if apierrs.IsNotFound(err) {
			loggerWithContext.Debug("resource not found, creating")
			err = i.mfClient.Create(expected)
			if err != nil {
				loggerWithContext.Errorw("failed to create resource", "error", err)
				return err
			}
			loggerWithContext.Debug("resource created successfully")
		}
		loggerWithContext.Errorw("failed to get resource", "error", err)
		return err
	}

	loggerWithContext.Debug("resource found in cluster, checking for changes")

	if existing.GetDeletionTimestamp() != nil {
		loggerWithContext.Debug("resource is being deleted, waiting for completion")
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	// get list of reconcile fields
	reconcileFields := i.resourceReconcileFields(expected)

	// compute hash value for the expected deployment or statefulset
	expectedHashValue, err := i.computeResourceHash(expected, reconcileFields...)
	if err != nil {
		loggerWithContext.Errorw("failed to compute hash for expected resource", "error", err)
		return fmt.Errorf("failed to compute hash value for expected resource, name:%s, error: %v", expected.GetName(), err)
	}

	// compute hash value for the existing resource
	// remove extra annotations and labels to keep the consistence hash
	existingCloned := existing.DeepCopy()
	existingCloned.SetAnnotations(i.removeExtraKeyInMap(existingCloned.GetAnnotations(), expected.GetAnnotations()))
	existingCloned.SetLabels(i.removeExtraKeyInMap(existingCloned.GetLabels(), expected.GetLabels()))
	// compute hash
	existingHashValue, err := i.computeResourceHash(existingCloned, reconcileFields...)
	if err != nil {
		loggerWithContext.Errorw("failed to compute hash for existing resource", "error", err)
		return fmt.Errorf("failed to compute hash value for existing resource, name:%s, namespace:%s, kind:%s error: %v",
			existingCloned.GetName(), existingCloned.GetNamespace(), existingCloned.GetKind(), err,
		)
	}

	// if change detected in hash value, update the resource with changes
	if existingHashValue != expectedHashValue {
		loggerWithContext.Debugw("change detected, updating resource",
			"existingHash", existingHashValue,
			"expectedHash", expectedHashValue,
		)

		err = i.copyResourceFields(expected, existing, reconcileFields...)
		if err != nil {
			loggerWithContext.Errorw("failed to copy resource fields", "error", err)
			return err
		}

		err = i.mfClient.Update(existing)
		if err != nil {
			loggerWithContext.Errorw("failed to update resource", "error", err)
			return v1alpha1.RECONCILE_AGAIN_ERR
		}

		loggerWithContext.Debug("resource updated successfully")
		return nil
	}
	loggerWithContext.Debug("no changes detected, resource is up-to-date")
	return nil
}

func (i *installer) removeExtraKeyInMap(src, dst map[string]string) map[string]string {
	newMap := map[string]string{}
	if len(src) == 0 {
		return newMap
	}
	for dstKey, dstValue := range dst {
		for srcKey := range src {
			if dstKey == srcKey {
				newMap[dstKey] = dstValue
				break
			}
		}
	}
	return newMap
}

func (i *installer) computeResourceHash(u *unstructured.Unstructured, reconcileFieldKeys ...string) (string, error) {
	// always keep the empty annotations and labels as empty, NOT nil
	if u.GetAnnotations() == nil {
		u.SetAnnotations(map[string]string{})
	}
	if u.GetLabels() == nil {
		u.SetLabels(map[string]string{})
	}

	// if there is no reconcile key specified, compute the hash to the entire object
	if len(reconcileFieldKeys) == 0 {
		return hash.Compute(u.Object)
	}

	// holds the required fieldsMap
	fieldsMap := map[string]interface{}{}

	// collect all the required fields to compute hash value
	for _, fieldKey := range reconcileFieldKeys {
		// split the fields with comma
		nestedKeys := strings.Split(fieldKey, ".")
		fieldValue, _, err := unstructured.NestedFieldCopy(u.Object, nestedKeys...)
		if err != nil {
			return "", err
		}
		fieldsMap[fieldKey] = fieldValue
	}

	// compute hash to the collected fieldMaps
	return hash.Compute(fieldsMap)
}

func (i *installer) mergeMaps(src, dst map[string]string) map[string]string {
	if len(dst) == 0 {
		return src
	}
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func (i *installer) copyResourceFields(src, dst *unstructured.Unstructured, reconcileFieldKeys ...string) error {
	// if there is no reconcile key specified, compute the hash to the entire object
	if len(reconcileFieldKeys) == 0 {
		srcCloned := src.DeepCopy()
		// merge annotations
		srcCloned.SetAnnotations(i.mergeMaps(srcCloned.GetAnnotations(), dst.GetAnnotations()))
		// merge labels
		srcCloned.SetLabels(i.mergeMaps(srcCloned.GetLabels(), dst.GetLabels()))

		dst.Object = srcCloned.Object
		return nil
	}

	for _, fieldKey := range reconcileFieldKeys {
		switch fieldKey {
		case annotationsPath: // merge annotations
			dst.SetAnnotations(i.mergeMaps(src.GetAnnotations(), dst.GetAnnotations()))

		case labelsPath: // merge labels
			dst.SetLabels(i.mergeMaps(src.GetLabels(), dst.GetLabels()))

		default:
			// split the fields with comma
			nestedKeys := strings.Split(fieldKey, ".")
			fieldValue, found, err := unstructured.NestedFieldCopy(src.Object, nestedKeys...)
			if err != nil {
				return err
			}
			if found {
				err = unstructured.SetNestedField(dst.Object, fieldValue, nestedKeys...)
				if err != nil {
					return err
				}
			} else {
				unstructured.RemoveNestedField(dst.Object, nestedKeys...)
			}
		}
	}

	return nil
}

func (i *installer) IsWebhookReady() error {
	for _, u := range i.deployment {
		if !strings.Contains(u.GetName(), "webhook") {
			continue
		}
		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) IsControllerReady() error {
	for _, u := range i.deployment {
		if !strings.Contains(u.GetName(), "controller") {
			continue
		}
		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) AllDeploymentsReady() error {
	for _, u := range i.deployment {
		if strings.Contains(u.GetName(), "controller") ||
			strings.Contains(u.GetName(), "webhook") {
			continue
		}
		err := i.isDeploymentReady(&u)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) IsJobCompleted(ctx context.Context, labels map[string]string, installSetName string) error {
	for _, u := range i.manifest.Filter(mf.ByKind("Job")).Resources() {
		resource, err := i.mfClient.Get(&u)
		if err != nil {
			return err
		}
		job := &batchv1.Job{}
		if err := scheme.Scheme.Convert(resource, job, nil); err != nil {
			return err
		}

		logger := logging.FromContext(ctx)
		if !isJobCompleted(job) {
			logger.Info("job not ready in installerset, name: %s, created-by: %s, in namespace: %s", installSetName, labels[v1alpha1.CreatedByKey], job.GetNamespace())
			return fmt.Errorf("Job not successful")
		}
	}

	return nil
}

func (i *installer) isStatefulSetAvailable(sfs *unstructured.Unstructured) error {
	resource, err := i.mfClient.Get(sfs)
	if err != nil {
		return err
	}

	statefulSet := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, statefulSet)
	if err != nil {
		return err
	}

	if !isStatefulSetReady(statefulSet) {
		i.logger.Infof("statefulset %v not ready, returning will retry!", statefulSet.GetName())
		return fmt.Errorf("%s statefulset is not ready", statefulSet.GetName())
	}
	return nil
}

func (i *installer) isDeploymentReady(d *unstructured.Unstructured) error {
	resource, err := i.mfClient.Get(d)
	if err != nil {
		return err
	}

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, deployment)
	if err != nil {
		return err
	}

	if msg := isFailedToCreateState(deployment); msg != "" {
		i.logger.Infof("deployment %v is in failed state, deleting! reason: ", msg)
		err := i.mfClient.Delete(resource)
		if err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if !isDeploymentAvailable(deployment) {
		i.logger.Infof("deployment %v not ready, returning will retry!", deployment.GetName())
		return fmt.Errorf("%s deployment not ready", deployment.GetName())
	}

	return nil
}

func isFailedToCreateState(d *appsv1.Deployment) string {
	for _, c := range d.Status.Conditions {
		if string(c.Type) == string(appsv1.ReplicaSetReplicaFailure) && c.Status == corev1.ConditionTrue && c.Reason == "FailedCreate" {
			return c.Message
		}
	}
	return ""
}

func isDeploymentAvailable(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isStatefulSetReady(sfs *appsv1.StatefulSet) bool {
	if sfs.Spec.Replicas != nil {
		if sfs.Status.ReadyReplicas == *sfs.Spec.Replicas {
			return true
		}
	}
	return false
}

func isJobCompleted(d *batchv1.Job) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// DeleteResources Deletes all resources except CRDs, PVCs and Namespace as they
// are own by owner of TektonInstallerSet.
// They will be deleted when the component CR is deleted
func (i *installer) DeleteResources() error {
	// delete clusterScope resources first
	if err := i.delete(i.clusterScoped); err != nil {
		return err
	}
	if err := i.delete(i.namespaceScoped); err != nil {
		return err
	}
	if err := i.deleteWithPolicy(i.job, metav1.DeletePropagationForeground); err != nil {
		return err
	}
	if err := i.delete(i.deployment); err != nil {
		return err
	}
	return nil
}

func (i *installer) delete(resources []unstructured.Unstructured) error {
	for _, r := range resources {
		if skipDeletion(r.GetKind()) {
			continue
		}
		resource, err := i.mfClient.Get(&r)
		if err != nil {
			// if error occurs log and move on, as we have owner reference set for resources, those
			// will be removed eventually and manifestival breaks the pod during uninstallation,
			// when CRD is deleted, CRs are removed but when we delete installer set, manifestival
			// breaks during deleting those CRs
			i.logger.Errorf("failed to get resource, skipping deletion: %v/%v: %v ", r.GetKind(), r.GetName(), err)
			continue
		}
		err = i.mfClient.Delete(resource)
		if err != nil {
			return err
		}

	}
	return nil
}

func (i *installer) deleteWithPolicy(resources []unstructured.Unstructured, policy metav1.DeletionPropagation) error {
	for _, r := range resources {
		if skipDeletion(r.GetKind()) {
			continue
		}
		resource, err := i.mfClient.Get(&r)
		if err != nil {
			// if error occurs log and move on, as we have owner reference set for resources, those
			// will be removed eventually and manifestival breaks the pod during uninstallation,
			// when CRD is deleted, CRs are removed but when we delete installer set, manifestival
			// breaks during deleting those CRs
			i.logger.Errorf("failed to get resource, skipping deletion: %v/%v: %v ", r.GetKind(), r.GetName(), err)
			continue
		}

		err = i.mfClient.Delete(resource, mf.DeleteOption(mf.PropagationPolicy(policy)))
		if err != nil {
			return err
		}

	}
	return nil
}

func skipDeletion(kind string) bool {
	if kind == "Namespace" ||
		kind == "PersistentVolumeClaim" ||
		kind == "CustomResourceDefinition" {
		return true
	}
	return false
}
