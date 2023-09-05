package upgrade

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorFake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

func TestIsUpgradeRequired(t *testing.T) {
	operatorVersion := "0.68.0"
	ctx := context.TODO()
	ug := getUpgradeStructWithFakeClients(ctx, operatorVersion)

	// there is no tektonConfig CR present
	// should return no error and false
	status, err := ug.isUpgradeRequired(ctx)
	assert.NoError(t, err)
	assert.False(t, status)

	// create tektonConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
	}
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// tektonConfig CR present, but version field is empty
	// should return no error and false
	status, err = ug.isUpgradeRequired(ctx)
	assert.NoError(t, err)
	assert.True(t, status)

	// tektonConfig CR present, but version field with different value
	// should return no error and false
	tc.Status.SetAppliedUpgradeVersion("0.67.0")
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{})
	assert.NoError(t, err)
	status, err = ug.isUpgradeRequired(ctx)
	assert.NoError(t, err)
	assert.True(t, status)

	// tektonConfig CR present, but version field as operatorVersion
	// should return no error and false
	tc.Status.SetAppliedUpgradeVersion(operatorVersion)
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Update(ctx, tc, metav1.UpdateOptions{})
	assert.NoError(t, err)
	status, err = ug.isUpgradeRequired(ctx)
	assert.NoError(t, err)
	assert.False(t, status)
}

func TestUpdateAppliedUpgradeVersion(t *testing.T) {
	operatorVersion := "0.68.0"
	ctx := context.TODO()
	ug := getUpgradeStructWithFakeClients(ctx, operatorVersion)

	// there is no tektonConfig CR present
	// should return an error
	err := ug.updateAppliedUpgradeVersion(ctx)
	assert.Error(t, err)

	// create tektonConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
	}
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// tektonConfig CR present
	// should return no error
	err = ug.updateAppliedUpgradeVersion(ctx)
	assert.NoError(t, err)

	// verify the version in tektonConfig CR
	tc, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, operatorVersion, tc.Status.GetAppliedUpgradeVersion())
}

func TestRunPreUpgrade(t *testing.T) {
	operatorVersion := "0.68.0"
	ctx := context.TODO()
	ug := getUpgradeStructWithFakeClients(ctx, operatorVersion)

	// execute pre upgrade, should return no error
	err := ug.RunPreUpgrade(ctx)
	assert.NoError(t, err)

	// create tektonConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Status: v1alpha1.TektonConfigStatus{},
	}
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// update pre upgrade functions
	preUpgradeFunctions = []upgradeFunc{
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return nil
		},
	}

	// execute pre upgrade, should return no error
	err = ug.RunPreUpgrade(ctx)
	assert.NoError(t, err)

	// update pre upgrade functions, which should return error
	preUpgradeFunctions = []upgradeFunc{
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return nil
		},
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return errors.New("error on execution")
		},
	}

	// execute pre upgrade, should return an error
	err = ug.RunPreUpgrade(ctx)
	assert.Error(t, err)

	// verify the version in tektonConfig CR
	tc, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "", tc.Status.GetAppliedUpgradeVersion())
}

func TestRunPostUpgrade(t *testing.T) {
	operatorVersion := "0.68.0"
	ctx := context.TODO()
	ug := getUpgradeStructWithFakeClients(ctx, operatorVersion)

	// execute post upgrade, should return no error
	err := ug.RunPostUpgrade(ctx)
	assert.NoError(t, err)

	// create tektonConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Status: v1alpha1.TektonConfigStatus{},
	}
	_, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// update post upgrade functions, which should return error
	postUpgradeFunctions = []upgradeFunc{
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return nil
		},
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return errors.New("error on execution")
		},
	}

	// execute post upgrade, should return an error
	err = ug.RunPostUpgrade(ctx)
	assert.Error(t, err)

	// update post upgrade functions
	postUpgradeFunctions = []upgradeFunc{
		func(ctx context.Context, logger *zap.SugaredLogger, k8sClient kubernetes.Interface, operatorClient versioned.Interface, restConfig *rest.Config) error {
			return nil
		},
	}

	// execute post upgrade, should return no error
	err = ug.RunPostUpgrade(ctx)
	assert.NoError(t, err)

	// verify the version in tektonConfig CR
	tc, err = ug.operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, operatorVersion, tc.Status.GetAppliedUpgradeVersion())
}

func getUpgradeStructWithFakeClients(ctx context.Context, operatorVersion string) *Upgrade {
	operatorClient := operatorFake.NewSimpleClientset()
	k8sClient := k8sFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	ug := &Upgrade{
		logger:          logger,
		k8sClient:       k8sClient,
		operatorClient:  operatorClient,
		operatorVersion: operatorVersion,
	}

	return ug
}
