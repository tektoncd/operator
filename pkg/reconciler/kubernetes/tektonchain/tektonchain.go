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

package tektonchain

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	tektonchainreconciler "github.com/tektoncd/operator/pkg/client/injection/reconciler/operator/v1alpha1/tektonchain"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Chains ConfigMap
	ChainsConfig = "chains-config"
	// Chains Container Name
	chainsContainerName = "tekton-chains-controller"
	// Deployment Name
	chainsDeploymentName = "tekton-chains-controller"
)

// Reconciler implements controller.Reconciler for TektonChain resources.
type Reconciler struct {
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest has the source manifest of Tekton Triggers for a
	// particular version
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
	// chainVersion describes the current chain version
	chainVersion    string
	operatorVersion string
	// pipelineInformer provides access to a shared informer and lister for
	// TektonPipelines
	pipelineInformer pipelineinformer.TektonPipelineInformer
}

// Check that our Reconciler implements controller.Reconciler
var _ tektonchainreconciler.Interface = (*Reconciler)(nil)
var _ tektonchainreconciler.Finalizer = (*Reconciler)(nil)

const (
	createdByValue          = "TektonChain"
	secretChainInstallerset = "chain-secret"
)

var (
	ls = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: v1alpha1.ChainResourceName,
		},
	}
	secretLs = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: secretChainInstallerset,
		},
	}
)

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, tc *v1alpha1.TektonChain) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	tc.Status.InitializeConditions()
	tc.Status.ObservedGeneration = tc.Generation

	logger.Infow("Reconciling TektonChain", "status", tc.Status)

	if tc.GetName() != v1alpha1.ChainResourceName {
		msg := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s",
			v1alpha1.ChainResourceName,
			tc.GetName(),
		)
		logger.Error(msg)
		tc.Status.MarkNotReady(msg)
		return nil
	}

	// find a valid TektonPipeline installation
	if _, err := common.PipelineReady(r.pipelineInformer); err != nil {
		if err.Error() == common.PipelineNotReady || err == v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR {
			tc.Status.MarkDependencyInstalling("TektonPipeline is still installing")
			// wait for TektonPipeline status to change
			return fmt.Errorf(common.PipelineNotReady)
		}
		// (tektonpipeline.operator.tekton.dev instance not available yet)
		tc.Status.MarkDependencyMissing("TektonPipeline does not exist")
		return err
	}
	tc.Status.MarkDependenciesInstalled()

	// Pass the object through defaulting
	tc.SetDefaults(ctx)

	// Mark TektonChain Instance as Not Ready if an upgrade is needed
	if err := r.markUpgrade(ctx, tc); err != nil {
		return err
	}

	if err := r.extension.PreReconcile(ctx, tc); err != nil {
		tc.Status.MarkPreReconcilerFailed(fmt.Sprintf("PreReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PreReconcile Complete
	tc.Status.MarkPreReconcilerComplete()

	// Check if a Tekton Chain InstallerSet already exists, if not then create one
	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return err
	}
	if existingInstallerSet == "" {
		tc.Status.MarkInstallerSetNotAvailable("Chain InstallerSet not available")

		createdIs, err := r.createInstallerSet(ctx, tc)
		if err != nil {
			return err
		}

		return r.updateTektonChainStatus(tc, createdIs)
	}

	// If exists, then fetch the Tekton Chain InstallerSet
	installedTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			createdIs, err := r.createInstallerSet(ctx, tc)
			if err != nil {
				return err
			}
			return r.updateTektonChainStatus(tc, createdIs)
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	installerSetTargetNamespace := installedTIS.Annotations[v1alpha1.TargetNamespaceKey]
	installerSetReleaseVersion := installedTIS.Labels[v1alpha1.ReleaseVersionKey]

	// Check if TargetNamespace of existing Tekton Chain InstallerSet is same as expected
	// Check if Release Version in Tekton Chain InstallerSet is same as expected
	// If any of the above things is not same then delete the existing Tekton Chain InstallerSet
	// and create a new with expected properties

	if installerSetTargetNamespace != tc.Spec.TargetNamespace || installerSetReleaseVersion != r.operatorVersion {
		// Delete the existing Tekton Chain InstallerSet
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, existingInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			logger.Error("failed to delete InstallerSet: %s", err)
			return err
		}

		// Make sure the Tekton Chain InstallerSet is deleted
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, existingInstallerSet, metav1.GetOptions{})
		if err == nil {
			tc.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}
		return nil

	} else {
		// If target namespace and version are not changed then check if Chain
		// spec is changed by checking hash stored as annotation on
		// Tekton Chain InstallerSet with computing new hash of TektonChain Spec

		// Hash of TektonChain Spec
		expectedSpecHash, err := hash.Compute(tc.Spec)
		if err != nil {
			return err
		}

		// spec hash stored on installerSet
		lastAppliedHash := installedTIS.GetAnnotations()[v1alpha1.LastAppliedHashKey]

		if lastAppliedHash != expectedSpecHash {

			manifest := r.manifest
			// installerSet adds it's owner as namespace's owner
			// so deleting tekton chain deletes target namespace too
			// to skip it we filter out namespace if pipeline have same namespace
			pipelineNamespace, err := common.PipelineTargetNamspace(r.pipelineInformer)
			if err != nil {
				logger.Error("unable to fetch pipeline namespace:  ", err)
				return err
			}
			if tc.Spec.GetTargetNamespace() == pipelineNamespace {
				manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))
			}
			// remove secret from this installerset as this installerset will be deleted on upgrade
			manifest = manifest.Filter(mf.Not(mf.ByKind("Secret")))
			if err := r.transform(ctx, &manifest, tc); err != nil {
				logger.Error("manifest transformation failed:  ", err)
				return err
			}

			// Update the spec hash
			current := installedTIS.GetAnnotations()
			current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
			installedTIS.SetAnnotations(current)

			// Update the manifests
			installedTIS.Spec.Manifests = manifest.Resources()

			if _, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
				Update(ctx, installedTIS, metav1.UpdateOptions{}); err != nil {
				return err
			}

			// after updating installer set enqueue after a duration
			// to allow changes to get deployed
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	// Check if a Tekton Chain Secret InstallerSet already exists, if not then create one
	secretLabelSelector, err := common.LabelSelector(secretLs)
	if err != nil {
		return err
	}
	existingSecretInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, secretLabelSelector)
	if err != nil {
		return err
	}
	if existingSecretInstallerSet == "" {
		tc.Status.MarkInstallerSetNotAvailable("Chain Secret InstallerSet not available")
		_, err := r.createSecretInstallerSet(ctx, tc)
		if err != nil {
			return err
		}
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	// If exists, then fetch the Tekton Chain Secret InstallerSet
	installedSecretTIS, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingSecretInstallerSet, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err := r.createSecretInstallerSet(ctx, tc)
			if err != nil {
				return err
			}
			return v1alpha1.RECONCILE_AGAIN_ERR
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	secretInstallerSetTargetNamespace := installedSecretTIS.Annotations[v1alpha1.TargetNamespaceKey]
	// if the namespace has been changed for chainsCR, then deleted the Tekton Chain Secret Installerset
	if secretInstallerSetTargetNamespace != tc.Spec.TargetNamespace {
		// Delete the existing Tekton Chain Secret InstallerSet
		err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, existingSecretInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			logger.Error("failed to delete TektonChainSecret InstallerSet: %s", err)
			return err
		}

		// Make sure the Tekton Chain Secret InstallerSet is deleted
		_, err = r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, existingSecretInstallerSet, metav1.GetOptions{})
		if err == nil {
			tc.Status.MarkNotReady("Waiting for previous installer set to get deleted")
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
		if !apierrors.IsNotFound(err) {
			logger.Error("failed to get InstallerSet: %s", err)
			return err
		}
		return nil
	}

	// Mark InstallerSetAvailable
	tc.Status.MarkInstallerSetAvailable()

	ready := installedTIS.Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		tc.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	if ready.Status == corev1.ConditionUnknown {
		tc.Status.MarkInstallerSetNotReady("Waiting for installation")
		return v1alpha1.REQUEUE_EVENT_AFTER
	} else if ready.Status == corev1.ConditionFalse {
		tc.Status.MarkInstallerSetNotReady(ready.Message)
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// Mark InstallerSet Ready
	tc.Status.MarkInstallerSetReady()

	if err := r.extension.PostReconcile(ctx, tc); err != nil {
		tc.Status.MarkPostReconcilerFailed(fmt.Sprintf("PostReconciliation failed: %s", err.Error()))
		return err
	}

	// Mark PostReconcile Complete
	tc.Status.MarkPostReconcilerComplete()

	// Update the object for any spec changes
	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonChains().Update(ctx, tc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// FinalizeKind removes all resources after deletion of a TektonChain.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonChain) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Delete CRDs before deleting rest of resources so that any instance
	// of CRDs which has finalizer set will get deleted before we remove
	// the controller's deployment for it
	if err := r.manifest.Filter(mf.CRDs).Delete(); err != nil {
		logger.Error("Failed to deleted CRDs for TektonChain")
		return err
	}

	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		return err
	}
	if err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		logger.Error("Failed to delete installer set created by TektonChain", err)
		return err
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}

	return nil
}

func (r *Reconciler) updateTektonChainStatus(tc *v1alpha1.TektonChain, createdIs *v1alpha1.TektonInstallerSet) error {
	// update the tc with TektonInstallerSet and releaseVersion
	tc.Status.SetTektonInstallerSet(createdIs.Name)
	tc.Status.SetVersion(r.chainVersion)

	return v1alpha1.RECONCILE_AGAIN_ERR
}

// transform mutates the passed manifest to one with common, component
// and platform transformations applied
func (r *Reconciler) transform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	instance := comp.(*v1alpha1.TektonChain)
	chainImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.ChainsImagePrefix))
	extra := []mf.Transformer{
		common.InjectOperandNameLabelOverwriteExisting(v1alpha1.OperandTektoncdChains),
		common.DeploymentImages(chainImages),
		common.AddConfiguration(instance.Spec.Config),
		common.AddConfigMapValues(ChainsConfig, instance.Spec.Chain.ChainProperties),
		common.AddDeploymentRestrictedPSA(),
		AddControllerEnv(instance.Spec.Chain.ControllerEnvs),
	}
	extra = append(extra, r.extension.Transformers(instance)...)
	return common.Transform(ctx, manifest, instance, extra...)
}

func (r *Reconciler) createInstallerSet(ctx context.Context, tc *v1alpha1.TektonChain) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	// installerSet adds it's owner as namespace's owner
	// so deleting tekton chain deletes target namespace too
	// to skip it we filter out namespace if pipeline have same namespace
	pipelineNamespace, err := common.PipelineTargetNamspace(r.pipelineInformer)
	if err != nil {
		return nil, err
	}
	if tc.Spec.GetTargetNamespace() == pipelineNamespace {
		manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))
	}

	// remove secret from this installerset as this installerset will be deleted on upgrade
	manifest = manifest.Filter(mf.Not(mf.ByKind("Secret")))
	if err := r.transform(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// generate installer set
	tis := makeInstallerSet(tc, manifest, v1alpha1.ChainResourceName, r.operatorVersion)

	// compute the hash of tektonchain spec and store as an annotation
	// in further reconciliation we compute hash of tc spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tc.Spec)
	if err != nil {
		return nil, err
	}
	tis.Annotations[v1alpha1.LastAppliedHashKey] = specHash

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func (r *Reconciler) createSecretInstallerSet(ctx context.Context, tc *v1alpha1.TektonChain) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	// filter only secret for this installerset as this needs
	// to be restored over upgrade
	manifest = manifest.Filter(mf.ByKind("Secret"))
	if err := r.transform(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// generate installer set
	tis := makeInstallerSet(tc, manifest, secretChainInstallerset, r.operatorVersion)

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func makeInstallerSet(tc *v1alpha1.TektonChain, manifest mf.Manifest, installerSetType, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", installerSetType),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.ReleaseVersionKey: releaseVersion,
				v1alpha1.InstallerSetType:  installerSetType,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func (r *Reconciler) markUpgrade(ctx context.Context, tc *v1alpha1.TektonChain) error {
	labels := tc.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if ok && ver == r.operatorVersion {
		return nil
	}
	if ok && ver != r.operatorVersion {
		tc.Status.MarkInstallerSetNotReady(v1alpha1.UpgradePending)
		tc.Status.MarkPreReconcilerFailed(v1alpha1.UpgradePending)
		tc.Status.MarkPostReconcilerFailed(v1alpha1.UpgradePending)
		tc.Status.MarkNotReady(v1alpha1.UpgradePending)
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels[v1alpha1.ReleaseVersionKey] = r.operatorVersion
	tc.SetLabels(labels)

	if _, err := r.operatorClientSet.OperatorV1alpha1().TektonChains().Update(ctx,
		tc, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func AddControllerEnv(controllerEnvs []corev1.EnvVar) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || len(controllerEnvs) == 0 || u.GetName() != chainsDeploymentName {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i, c := range d.Spec.Template.Spec.Containers {
			if c.Name != chainsContainerName {
				continue
			}

			existingEnv := c.Env
			for _, v := range controllerEnvs {
				newEnv := corev1.EnvVar{
					Name:      v.Name,
					Value:     v.Value,
					ValueFrom: v.ValueFrom,
				}
				appendNewEnv := true
				for existingEnvIndex, env := range existingEnv {
					// Check for the key, if found replace it
					if env.Name == newEnv.Name {
						existingEnv[existingEnvIndex] = newEnv
						appendNewEnv = false
						break
					}
				}
				// If not found append the new env
				if appendNewEnv {
					existingEnv = append(existingEnv, newEnv)
				}
			}

			// update the changes into the actual container
			d.Spec.Template.Spec.Containers[i].Env = existingEnv
			break
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}
