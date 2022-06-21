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
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const WEBHOOK_INSTALLERSET_LABEL = "validating-defaulting-webhooks.operator.tekton.dev"
const POD_NAMESPACE_ENV_KEY = "SYSTEM_NAMESPACE"

var (
	ERR_NAMESPACE_ENV_NOT_SET = fmt.Errorf("Pod namespace env %q not set", POD_NAMESPACE_ENV_KEY)
)

func CreateWebhookResources(ctx context.Context) {
	logger := logging.FromContext(ctx)

	manifest, err := fetchManifests(ctx)
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	client := operatorclient.Get(ctx)
	err = checkAndDeleteInstallerSet(ctx, client)
	if err != nil {
		logger.Fatalw("error deleting webhook installerset", zap.Error(err))
	}

	if err := createInstallerSet(ctx, client, *manifest); err != nil {
		logger.Fatalw("error creating webhook installerset", zap.Error(err))
	}

}

func CleanupWebhookResources(ctx context.Context) {
	logger := logging.FromContext(ctx)
	client := operatorclient.Get(ctx)

	// cannot use the ctx passed from main as it will be cancelled
	// by the time we use in kube api calls
	freshContext := context.Background()

	err := checkAndDeleteInstallerSet(freshContext, client)
	if err != nil {
		logger.Fatalw("error deleting webhook installerset", zap.Error(err))
	}
}

func fetchManifests(ctx context.Context) (*mf.Manifest, error) {
	logger := logging.FromContext(ctx)
	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		return nil, err
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, err := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))
	if err != nil {
		return nil, err
	}

	// Read manifests
	koDataDir := os.Getenv(common.KoEnvKey)
	validating_defaulting_webhooks := filepath.Join(koDataDir, "validating-defaulting-webhook")
	if err := common.AppendManifest(&manifest, validating_defaulting_webhooks); err != nil {
		return nil, err
	}
	return manifestTransform(&manifest)
}

func manifestTransform(m *mf.Manifest) (*mf.Manifest, error) {
	ns, ok := os.LookupEnv(POD_NAMESPACE_ENV_KEY)
	if !ok || ns == "" {
		return nil, ERR_NAMESPACE_ENV_NOT_SET
	}
	tfs := []mf.Transformer{
		mf.InjectNamespace(ns),
	}
	result, err := m.Transform(tfs...)
	return &result, err
}

func checkAndDeleteInstallerSet(ctx context.Context, oc clientset.Interface) error {
	ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: WEBHOOK_INSTALLERSET_LABEL,
		})
	if err != nil {
		if apierror.IsNotFound(err) {
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
