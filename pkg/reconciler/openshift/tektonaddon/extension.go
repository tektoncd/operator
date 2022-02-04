/*
Copyright 2020 The Tekton Authors

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

package tektonaddon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/route/clientset/versioned/scheme"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	logger := logging.FromContext(ctx)

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	version := os.Getenv(versionKey)
	if version == "" {
		logger.Fatal("Failed to find version from env")
	}

	ext := openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		manifest:          manifest,
		version:           version,
	}
	return ext
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	manifest          mf.Manifest
	version           string
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	addonImages := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
	return []mf.Transformer{
		common.TaskImages(addonImages),
	}
}
func (oe openshiftExtension) PreReconcile(context.Context, v1alpha1.TektonComponent) error {
	return nil
}
func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	logger := logging.FromContext(ctx)
	addon := comp.(*v1alpha1.TektonAddon)

	miscellaneousLS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: MiscellaneousResourcesInstallerSet,
		},
	}
	miscellaneousLabelSelector, err := common.LabelSelector(miscellaneousLS)
	if err != nil {
		return err
	}
	exist, err := checkIfInstallerSetExist(ctx, oe.operatorClientSet, oe.version, miscellaneousLabelSelector)
	if err != nil {
		return err
	}
	if !exist {
		manifest, err := getMiscellaneousManifest(ctx, addon, oe.manifest, comp)
		if err != nil {
			return err
		}

		if err := createInstallerSet(ctx, oe.operatorClientSet, addon, manifest, oe.version,
			MiscellaneousResourcesInstallerSet, "addon-openshift"); err != nil {
			return err
		}
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	// Check if installer set is already created
	installedTIS, err := oe.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: miscellaneousLabelSelector,
		})
	if err != nil {
		if apierrors.IsNotFound(err) {
			manifest, err := getMiscellaneousManifest(ctx, addon, oe.manifest, comp)
			if err != nil {
				return err
			}

			if err := createInstallerSet(ctx, oe.operatorClientSet, addon, manifest, oe.version,
				MiscellaneousResourcesInstallerSet, "addon-openshift"); err != nil {
				return err
			}
			return v1alpha1.RECONCILE_AGAIN_ERR
		}
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	expectedSpecHash, err := hash.Compute(addon.Spec)
	if err != nil {
		return err
	}

	// spec hash stored on installerSet
	lastAppliedHash := installedTIS.Items[0].GetAnnotations()[v1alpha1.LastAppliedHashKey]

	if lastAppliedHash != expectedSpecHash {

		manifest, err := getMiscellaneousManifest(ctx, addon, oe.manifest, comp)
		if err != nil {
			return err
		}

		// Update the spec hash
		current := installedTIS.Items[0].GetAnnotations()
		current[v1alpha1.LastAppliedHashKey] = expectedSpecHash
		installedTIS.Items[0].SetAnnotations(current)

		// Update the manifests
		installedTIS.Items[0].Spec.Manifests = manifest.Resources()

		if _, err = oe.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
			Update(ctx, &installedTIS.Items[0], metav1.UpdateOptions{}); err != nil {
			return err
		}

		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	installedAddonIS, err := oe.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: miscellaneousLabelSelector,
		})
	if err != nil {
		logger.Error("failed to get InstallerSet: %s", err)
		return err
	}

	ready := installedAddonIS.Items[0].Status.GetCondition(apis.ConditionReady)
	if ready == nil {
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	if ready.Status != corev1.ConditionTrue {
		return v1alpha1.RECONCILE_AGAIN_ERR
	}

	consolecliManifest := oe.manifest

	consoleCLILS := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: ConsoleCLIInstallerSet,
		},
	}
	consoleCLILabelSelector, err := common.LabelSelector(consoleCLILS)
	if err != nil {
		return err
	}
	exist, err = checkIfInstallerSetExist(ctx, oe.operatorClientSet, oe.version, consoleCLILabelSelector)
	if err != nil {
		return err
	}
	if !exist {
		tknservecliManifest := oe.manifest
		if err := applyAddons(&tknservecliManifest, "05-tkncliserve"); err != nil {
			return err
		}
		routeHost, err := getRouteHost(&tknservecliManifest)
		if err != nil {
			return err
		}

		if err := applyAddons(&consolecliManifest, "04-consolecli"); err != nil {
			return err
		}

		if err := consoleCLITransform(ctx, &consolecliManifest, routeHost); err != nil {
			return err
		}

		if err := createInstallerSet(ctx, oe.operatorClientSet, addon, consolecliManifest, oe.version,
			ConsoleCLIInstallerSet, "addon-consolecli"); err != nil {
			return err
		}
	}

	return nil
}
func (oe openshiftExtension) Finalize(context.Context, v1alpha1.TektonComponent) error {
	return nil
}

func getOptionalAddons(manifest *mf.Manifest, comp v1alpha1.TektonComponent) error {
	koDataDir := os.Getenv(common.KoEnvKey)

	optionalLocation := filepath.Join(koDataDir, "tekton-addon", "optional", "samples")
	if err := common.AppendManifest(manifest, optionalLocation); err != nil {
		return err
	}

	optionalLocation = filepath.Join(koDataDir, "tekton-addon", "optional", "quickstarts")
	return common.AppendManifest(manifest, optionalLocation)
}

func addonTransform(ctx context.Context, manifest *mf.Manifest, comp v1alpha1.TektonComponent, extra ...mf.Transformer) error {
	return common.Transform(ctx, manifest, comp, extra...)
}

func consoleCLITransform(ctx context.Context, manifest *mf.Manifest, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("route url should not be empty")
	}
	logger := logging.FromContext(ctx)
	logger.Debug("Transforming manifest")

	transformers := []mf.Transformer{
		replaceURLCCD(baseURL),
	}

	transformManifest, err := manifest.Transform(transformers...)
	if err != nil {
		return err
	}

	*manifest = transformManifest
	return nil
}

func getRouteHost(manifest *mf.Manifest) (string, error) {
	var hostUrl string
	for _, r := range manifest.Filter(mf.ByKind("Route")).Resources() {
		u, err := manifest.Client.Get(&r)
		if err != nil {
			return "", err
		}
		if u.GetName() == "tkn-cli-serve" {
			route := &routev1.Route{}
			if err := scheme.Scheme.Convert(u, route, nil); err != nil {
				return "", err
			}
			hostUrl = route.Spec.Host
		}
	}
	return hostUrl, nil
}

func getMiscellaneousManifest(ctx context.Context, addon *v1alpha1.TektonAddon, miscellaneousManifest mf.Manifest, comp v1alpha1.TektonComponent) (mf.Manifest, error) {
	if err := applyAddons(&miscellaneousManifest, "05-tkncliserve"); err != nil {
		return mf.Manifest{}, err
	}

	if err := getOptionalAddons(&miscellaneousManifest, comp); err != nil {
		return mf.Manifest{}, err
	}

	images := common.ToLowerCaseKeys(common.ImagesFromEnv(common.AddonsImagePrefix))
	extraTranformers := []mf.Transformer{
		common.DeploymentImages(images),
		common.AddConfiguration(addon.Spec.Config),
	}
	if err := addonTransform(ctx, &miscellaneousManifest, addon, extraTranformers...); err != nil {
		return mf.Manifest{}, err
	}
	return miscellaneousManifest, nil
}
