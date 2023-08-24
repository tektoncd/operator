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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/logging"
)

const (
	annotationsPath = "metadata.annotations"
	labelsPath      = "metadata.labels"
)

type installer struct {
	manifest        *mf.Manifest
	mfClient        mf.Client
	logger          *zap.SugaredLogger
	crds            []unstructured.Unstructured
	clusterScoped   []unstructured.Unstructured
	namespaceScoped []unstructured.Unstructured
	deployment      []unstructured.Unstructured
}

func NewInstaller(manifest *mf.Manifest, mfClient mf.Client, logger *zap.SugaredLogger) *installer {
	installer := &installer{
		manifest:        manifest,
		mfClient:        mfClient,
		logger:          logger,
		crds:            []unstructured.Unstructured{},
		clusterScoped:   []unstructured.Unstructured{},
		namespaceScoped: []unstructured.Unstructured{},
		deployment:      []unstructured.Unstructured{},
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
		expectedHash, err := hash.Compute(r.Object)
		if err != nil {
			return err
		}

		i.logger.Infof("fetching resource %s: %s/%s", r.GetKind(), r.GetNamespace(), r.GetName())

		res, err := i.mfClient.Get(&r)
		if err != nil {
			if apierrs.IsNotFound(err) {
				i.logger.Infof("resource not found, creating %s: %s/%s", r.GetKind(), r.GetNamespace(), r.GetName())
				// add hash on the resource of expected manifest and create
				anno := r.GetAnnotations()
				if anno == nil {
					anno = map[string]string{}
				}
				anno[v1alpha1.LastAppliedHashKey] = expectedHash
				r.SetAnnotations(anno)
				err = i.mfClient.Create(&r)
				if err != nil {
					return err
				}
				continue
			}
			return err
		}

		i.logger.Infof("found resource %s: %s/%s, checking for update!", r.GetKind(), r.GetNamespace(), r.GetName())

		// if resource exist then check if expected hash is different from the one
		// on the resource
		hashOnResource := res.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if expectedHash == hashOnResource {
			continue
		}

		i.logger.Infof("updating resource %s: %s/%s", r.GetKind(), r.GetNamespace(), r.GetName())

		anno := r.GetAnnotations()
		if anno == nil {
			anno = map[string]string{}
		}
		anno[v1alpha1.LastAppliedHashKey] = expectedHash
		r.SetAnnotations(anno)

		installManifests, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{r}), mf.UseClient(i.mfClient))
		if err != nil {
			return err
		}
		if err := installManifests.Apply(); err != nil {
			return err
		}
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

func (i *installer) EnsureDeploymentResources() error {
	for _, d := range i.deployment {
		if err := i.ensureResource(&d); err != nil {
			return err
		}
	}
	return nil
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
// currently tested only with deployments
// TODO: (jkandasa) needs to be tested with other resources too
func (i *installer) ensureResource(expected *unstructured.Unstructured) error {
	i.logger.Debugw("verifying a resource",
		"name", expected.GetName(),
		"namespace", expected.GetNamespace(),
		"kind", expected.GetKind(),
	)

	// update proxy settings
	if expected.GetKind() == "Deployment" {
		err := common.ApplyProxySettings(expected)
		if err != nil {
			return err
		}
	}

	// check if the resource already exists
	existing, err := i.mfClient.Get(expected)
	if err != nil {
		// If the resource doesn't exist, then create new
		if apierrs.IsNotFound(err) {
			i.logger.Debugw("resource not found, creating",
				"name", expected.GetName(),
				"namespace", expected.GetNamespace(),
				"kind", expected.GetKind(),
			)
			err = i.mfClient.Create(expected)
			if err != nil {
				i.logger.Debugw("error on creating a resource",
					"name", expected.GetName(),
					"namespace", expected.GetNamespace(),
					"kind", expected.GetKind(),
					err,
				)
				return err
			}
		}
		return err
	}

	i.logger.Debugw("resource found in cluster, checking for changes",
		"name", existing.GetName(),
		"namespace", existing.GetNamespace(),
		"kind", existing.GetKind(),
	)

	// get list of reconcile fields
	reconcileFields := i.resourceReconcileFields(expected)

	// compute hash value for the expected deployment
	expectedHashValue, err := i.computeResourceHash(expected, reconcileFields...)
	if err != nil {
		i.logger.Errorw("error on compute a hash value to a expected resource",
			"name", expected.GetName(),
			"namespace", expected.GetNamespace(),
			"kind", expected.GetKind(),
		)
		return fmt.Errorf("failed to compute hash value to expected resource, name:%s, error: %v", expected.GetName(), err)
	}

	// compute hash value for the existing resource
	// remove extra annotations and labels to keep the consistence hash
	existingCloned := existing.DeepCopy()
	existingCloned.SetAnnotations(i.removeExtraKeyInMap(existingCloned.GetAnnotations(), expected.GetAnnotations()))
	existingCloned.SetLabels(i.removeExtraKeyInMap(existingCloned.GetLabels(), expected.GetLabels()))
	// compute hash
	existingHashValue, err := i.computeResourceHash(existingCloned, reconcileFields...)
	if err != nil {
		i.logger.Errorw("error on computing hash value to an existing resource",
			"name", existingCloned.GetName(),
			"namespace", existingCloned.GetNamespace(),
			"kind", existingCloned.GetKind(),
		)
		return fmt.Errorf("failed to compute hash value of a existing resource, name:%s, namespace:%s, kind:%s error: %v",
			existingCloned.GetName(), existingCloned.GetNamespace(), existingCloned.GetKind(), err,
		)
	}

	// if change detected in hash value, update the resource with changes
	if existingHashValue != expectedHashValue {
		i.logger.Debugw("change detected in the resource, reconciling",
			"name", existing.GetName(),
			"namespace", existing.GetNamespace(),
			"kind", existing.GetKind(),
			"existingHashValue", existingHashValue,
			"expectedHashValue", expectedHashValue,
		)
		err = i.copyResourceFields(expected, existing, reconcileFields...)
		if err != nil {
			return err
		}

		err = i.mfClient.Update(existing)
		if err != nil {
			i.logger.Errorw("error on updating a resource",
				"resourceName", existing.GetName(),
				"namespace", existing.GetNamespace(),
				"kind", existing.GetKind(),
				err,
			)
			return v1alpha1.RECONCILE_AGAIN_ERR
		}

		i.logger.Debugw("reconciliation successful",
			"name", existing.GetName(),
			"namespace", existing.GetNamespace(),
			"kind", existing.GetKind(),
		)
		return nil
	}

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

func skipDeletion(kind string) bool {
	if kind == "Namespace" ||
		kind == "PersistentVolumeClaim" ||
		kind == "CustomResourceDefinition" {
		return true
	}
	return false
}
