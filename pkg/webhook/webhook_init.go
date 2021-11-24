package webhook

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const WEBHOOK_INSTALLERSET_LABEL = "validating-defaulting-webhooks.operator.tekton.dev"

func CreateWebhookResources(ctx context.Context) {
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

	// Read manifests
	koDataDir := os.Getenv(common.KoEnvKey)
	validating_defaulting_webhooks := filepath.Join(koDataDir, "validating-defaulting-webhook")
	if err := common.AppendManifest(&manifest, validating_defaulting_webhooks); err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	client := operatorclient.Get(ctx)
	err = checkAndDeleteInstallerSet(ctx, client)
	if err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}

	if err := createInstallerSet(ctx, client, manifest); err != nil {
		logger.Fatalw("error creating client from injected config", zap.Error(err))
	}
}

func checkAndDeleteInstallerSet(ctx context.Context, oc clientset.Interface) error {
	ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: WEBHOOK_INSTALLERSET_LABEL,
		})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if len(ctIs.Items) >= 0 {
		for _, item := range ctIs.Items {
			err = oc.OperatorV1alpha1().TektonInstallerSets().
				Delete(ctx, item.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, manifest mf.Manifest) error {
	is := makeInstallerSet(manifest)
	_, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func makeInstallerSet(manifest mf.Manifest) *v1alpha1.TektonInstallerSet {
	//TODO: find ownerReference of the operator controller deployment and use that as the
	// ownerReference for this TektonInstallerSet
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", "validating-mutating-webhoook"),
			Labels: map[string]string{
				WEBHOOK_INSTALLERSET_LABEL: "",
			},
			Annotations: map[string]string{
				"releaseVersionKey": "v1.6.0",
			},
			//OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
