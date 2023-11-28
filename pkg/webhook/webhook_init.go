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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"go.uber.org/zap"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

const (
	// deprecated label, used in old versions
	// keeps this reference to remove the existing webhook installersets
	DEPRECATED_WEBHOOK_INSTALLERSET_LABEL = "validating-defaulting-webhooks.operator.tekton.dev"

	// this label is used to terminate the created webhook installerset on graceful termination
	// use unique name, to identify the resource created by this pod
	WEBHOOK_UNIQUE_LABEL = "operator.tekton.dev/webhook-unique-identifier"

	// primary label values to track webhook installersets
	labelCreatedByValue        = "operator-webhook-init"
	labelInstallerSetTypeValue = "operatorValidatingDefaultingWebhook"

	POD_NAMESPACE_ENV_KEY = "SYSTEM_NAMESPACE"
	POD_NAME_ENV_KEY      = "WEBHOOK_POD_NAME"
)

var (
	ErrNamespaceEnvNotSet = fmt.Errorf("namespace environment key %q not set", POD_NAMESPACE_ENV_KEY)

	// primary labelSelector to list available webhooks installersets
	primaryLabelSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     labelCreatedByValue,
			v1alpha1.InstallerSetType: labelInstallerSetTypeValue,
		},
	}
)

func CleanupWebhookResources(ctx context.Context) {
	logger := logging.FromContext(ctx)
	client := operatorclient.Get(ctx)

	// cannot use the ctx passed from main as it will be cancelled
	// by the time we use in kube api calls
	freshContext := context.Background()

	// delete the webhook installersets created by this pod
	err := deleteExistingInstallerSets(freshContext, client, true)
	if err != nil {
		logger.Error("error on deleting webhook installersets", err)
	}
}

func CreateWebhookResources(ctx context.Context) {
	logger := logging.FromContext(ctx)

	manifest, err := fetchManifests(ctx)
	if err != nil {
		logger.Fatalw("error creating initial manifest", zap.Error(err))
	}

	client := operatorclient.Get(ctx)
	err = deleteExistingInstallerSets(ctx, client, false)
	if err != nil {
		logger.Fatalw("error deleting webhook installerset", zap.Error(err))
	}

	if err := createInstallerSet(ctx, client, *manifest); err != nil {
		logger.Fatalw("error creating webhook installerset", zap.Error(err))
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
		return nil, ErrNamespaceEnvNotSet
	}
	tfs := []mf.Transformer{
		mf.InjectNamespace(ns),
	}
	result, err := m.Transform(tfs...)
	return &result, err
}

func deleteExistingInstallerSets(ctx context.Context, client clientset.Interface, includeUniqueIdentifier bool) error {
	// deleting the existing deprecated webhook installersets
	installerSetList, err := client.OperatorV1alpha1().TektonInstallerSets().List(
		ctx,
		metav1.ListOptions{LabelSelector: DEPRECATED_WEBHOOK_INSTALLERSET_LABEL},
	)
	if err != nil {
		return err
	}
	for _, is := range installerSetList.Items {
		err = client.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, is.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	_primaryLabelSelector := primaryLabelSelector.DeepCopy()
	// this pod name
	if includeUniqueIdentifier {
		podName, ok := os.LookupEnv(POD_NAME_ENV_KEY)
		if !ok {
			// if pod env not set return
			return fmt.Errorf("pod name environment variable[%s] details are not set", POD_NAME_ENV_KEY)
		}
		// use pod name as unique reference
		_primaryLabelSelector.MatchLabels[WEBHOOK_UNIQUE_LABEL] = podName
	}

	// delete all the existing webhook installersets
	labelSelector, err := common.LabelSelector(*_primaryLabelSelector)
	if err != nil {
		return err
	}

	installerSetList, err = client.OperatorV1alpha1().TektonInstallerSets().List(
		ctx,
		metav1.ListOptions{LabelSelector: labelSelector},
	)
	if err != nil {
		return err
	}
	for _, is := range installerSetList.Items {
		err = client.OperatorV1alpha1().TektonInstallerSets().Delete(ctx, is.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, manifest mf.Manifest) error {
	is, err := makeInstallerSet(manifest)
	if err != nil {
		return err
	}
	item, err := oc.OperatorV1alpha1().TektonInstallerSets().Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	logger := logging.FromContext(ctx)
	logger.Debugw("webhook installerset created",
		"name", item.Name,
	)
	return nil
}

func makeInstallerSet(manifest mf.Manifest) (*v1alpha1.TektonInstallerSet, error) {
	// this pod name
	podName, ok := os.LookupEnv(POD_NAME_ENV_KEY)
	if !ok {
		// if pod env not set return
		return nil, fmt.Errorf("pod name environment variable[%s] details are not set", POD_NAME_ENV_KEY)
	}
	// use pod name as unique reference
	_primaryLabelSelector := primaryLabelSelector.DeepCopy()
	_primaryLabelSelector.MatchLabels[WEBHOOK_UNIQUE_LABEL] = podName

	installerSet := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", "validating-mutating-webhook"),
			Labels:       _primaryLabelSelector.MatchLabels,
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}

	return installerSet, nil
}
