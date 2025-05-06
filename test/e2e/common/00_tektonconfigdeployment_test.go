//go:build e2e
// +build e2e

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

package common

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	tconfig "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	namespaceDefault                 = "default"
	defaultTektonConfigProfile       = v1alpha1.ProfileAll
	pipelineControllerDeploymentName = "tekton-pipelines-controller"
)

var (
	tektonConfigProfileKubernetes = map[string]TektonProfileResource{
		v1alpha1.ProfileAll: {
			Deployments: []string{
				"tekton-dashboard",
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			ServiceAccounts: []string{
				"tekton-dashboard",
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			AddonsInstallerSets: []string{},
		},
		v1alpha1.ProfileBasic: {
			Deployments: []string{
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			ServiceAccounts: []string{
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			AddonsInstallerSets: []string{},
		},
		v1alpha1.ProfileLite: {
			Deployments: []string{
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
			},
			ServiceAccounts: []string{
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
			},
			AddonsInstallerSets: []string{},
		},
	}
	tektonConfigProfileOpenshift = map[string]TektonProfileResource{
		v1alpha1.ProfileAll: {
			Deployments: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
				"tkn-cli-serve",
			},
			ServiceAccounts: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			AddonsInstallerSets: []string{ // installerset addons prefix
				"addon-custom-clustertask",
				"addon-custom-communityclustertask",
				"addon-custom-consolecli",
				"addon-custom-openshiftconsole",
				"addon-custom-pipelinestemplate",
				"addon-custom-triggersresources",
				"addon-versioned-clustertasks",
			},
		},
		v1alpha1.ProfileBasic: {
			Deployments: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			ServiceAccounts: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
				"tekton-triggers-controller",
				"tekton-triggers-core-interceptors",
				"tekton-triggers-webhook",
			},
			AddonsInstallerSets: []string{},
		},
		v1alpha1.ProfileLite: {
			Deployments: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operator-proxy-webhook",
				pipelineControllerDeploymentName,
				"tekton-pipelines-remote-resolvers",
				"tekton-pipelines-webhook",
			},
			ServiceAccounts: []string{
				"pipelines-as-code-controller",
				"pipelines-as-code-watcher",
				"pipelines-as-code-webhook",
				"tekton-operators-proxy-webhook",
				"tekton-pipelines-controller",
				"tekton-pipelines-resolvers",
				"tekton-pipelines-webhook",
			},
			AddonsInstallerSets: []string{},
		},
	}
)

type TektonProfileResource struct {
	Deployments         []string
	ServiceAccounts     []string
	AddonsInstallerSets []string
}

type TektonConfigTestSuite struct {
	resourceNames utils.ResourceNames
	suite.Suite
	clients  *utils.Clients
	interval time.Duration
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func TestTektonConfigTestSuite(t *testing.T) {
	ts := NewTestTektonConfigTestSuite(t)
	// run the actual tests
	suite.Run(t, ts)
}

// if other suites want to execute a method on this suite,
// can create a instance by calling this function
// also used internally
func NewTestTektonConfigTestSuite(t *testing.T) *TektonConfigTestSuite {
	ts := TektonConfigTestSuite{
		resourceNames: utils.GetResourceNames(),
		interval:      5 * time.Second,
		timeout:       5 * time.Minute,
		logger:        utils.Logger(),
	}

	// setup clients
	ts.clients = client.Setup(t, ts.resourceNames.TargetNamespace)
	// if the instance created from other suite, "t" should be added,
	// other wise nil error when using "t" inside suite
	ts.SetT(t)

	return &ts
}

// before suite
func (s *TektonConfigTestSuite) SetupSuite() {
	resources.PrintClusterInformation(s.logger, s.resourceNames)
	s.recreateOperatorPod()
}

// before each tests
// reset the tekton pipelines into it's default state
func (s *TektonConfigTestSuite) SetupTest() {
	s.logger.Debug("resetting the tekton config to it's default state")
	s.resetToDefaults()
	s.logger.Debug("test environment ready. starting the actual test")
}

// after each tests
// if there is a failures, execute debug commands
func (s *TektonConfigTestSuite) TearDownTest() {
	t := s.T()
	if t.Failed() {
		s.logger.Infow("test failed, executing debug commands", "testName", t.Name())
		resources.ExecuteDebugCommands(s.logger, s.resourceNames)
	}
}

// actual tests

// delete the existing TektonConfig cr and recreate operator pod
// verify default TektonConfig cr created and resources
func (s *TektonConfigTestSuite) Test01_AutoInstall() {
	s.logger.Debug("deleting the tektonConfig cr")
	s.undeploy()

	// recreates the operator pod, on pod startup TektonConfig 'config' will be created
	s.recreateOperatorPod()

	// verify the services are up and running
	s.logger.Debug("verifying the resources")
	s.verifyProfile(defaultTektonConfigProfile, true)
}

// change the profile "spec.profile" to different values and verify resources
func (s *TektonConfigTestSuite) Test02_ChangeProfile() {
	// switch to "lite"
	newProfile := v1alpha1.ProfileLite
	s.changeProfile(newProfile)
	s.verifyProfile(newProfile, true)

	// switch to "basic"
	newProfile = v1alpha1.ProfileBasic
	s.changeProfile(newProfile)
	s.verifyProfile(newProfile, true)

	// switch to "all"
	newProfile = v1alpha1.ProfileAll
	s.changeProfile(newProfile)
	s.verifyProfile(newProfile, true)
}

// delete tektonpipeline CR and verify recreated automatically
func (s *TektonConfigTestSuite) Test03_DeletePipeline() {
	t := s.T()
	interval := s.interval
	timeout := s.timeout

	// delete pipeline cr
	s.logger.Debug("deleting pipelines cr")
	err := s.clients.TektonPipeline().Delete(context.TODO(), s.resourceNames.TektonPipeline, metav1.DeleteOptions{})
	require.NoError(t, err)

	// verify pipeline controller deployment deleted
	err = resources.WaitForDeploymentDeletion(s.clients.KubeClient, pipelineControllerDeploymentName, s.resourceNames.TargetNamespace, interval, timeout)
	require.NoError(t, err, "wait for pipeline deployment removal")
	s.logger.Debug("deleted pipelines cr")

	// verify pipeline cr, deployments, other resources are recreated
	s.logger.Debug("waiting to pipelines cr, will be recreated by operator")
	s.verifyProfile(defaultTektonConfigProfile, true)
}

// delete the tektonConfig cr and create it manually then verify services
func (s *TektonConfigTestSuite) Test04_DeleteAndCreateConfig() {
	t := s.T()

	// delete the existing tektonConfig
	s.undeploy()

	// create tektonConfig CR
	configCR := s.getDefaultConfig()
	_, err := s.clients.TektonConfig().Create(context.TODO(), configCR, metav1.CreateOptions{})
	require.NoError(t, err)

	// verify resources
	s.verifyProfile(defaultTektonConfigProfile, true)
}

// disable addons and verify resources
// applicable only to openshift platform
func (s *TektonConfigTestSuite) Test05_DisableAndEnableAddons() {
	t := s.T()
	if !utils.IsOpenShift() {
		t.Skip("skipped: This test is only supported in OpenShift")
	}

	timeout := s.timeout

	// disable addons and update
	tc := s.getCurrentConfig(timeout)
	tc.Spec.Addon.Params = []v1alpha1.Param{
		{Name: v1alpha1.CommunityResolverTasks, Value: "false"},
		{Name: v1alpha1.PipelineTemplatesParam, Value: "false"},
	}
	_, err := s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	// workaround - starts
	// fix this issue and remove the workaround
	// disabling addons operator not reacting immediately. takes approx 20 minutes
	// for now the workaround is, recreate the operator pod
	s.logger.Warnw("***WARNING*** fix the issue in product. running with workaround. restarting the operator pod",
		"issue", "https://github.com/tektoncd/operator/issues/1440",
	)
	err = resources.DeletePodByLabelSelector(s.clients.KubeClient, s.resourceNames.OperatorPodSelectorLabel, s.resourceNames.Namespace)
	require.NoError(t, err, "delete operator pod")
	// workaround - ends

	s.verifyProfile(defaultTektonConfigProfile, false)
}

// test SCC related behavior
func (s *TektonConfigTestSuite) Test06_TestSCCConfig() {
	t := s.T()
	if !utils.IsOpenShift() {
		t.Skip("skipped: This test is only supported in OpenShift")
	}

	timeout := s.timeout

	// disable addons and update
	tc := s.getCurrentConfig(timeout)

	// make sure default SCC is being set correctly
	s.logger.Debug("verifying default SCC is set to 'pipelines-scc'")
	require.Equal(t, "pipelines-scc", tc.Spec.Platforms.OpenShift.SCC.Default)

	// test: set default SCC and verify it's updated and not ignored by tektonconfig
	tc = s.getCurrentConfig(timeout)

	// set default SCC
	tc.Spec.Platforms.OpenShift.SCC.Default = "restricted-v2"

	tc, err := s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	// make sure default SCC is being set correctly
	s.logger.Debug("verifying default SCC is set to 'restricted-v2'")
	require.Equal(t, "restricted-v2", tc.Spec.Platforms.OpenShift.SCC.Default)

	// ---

	// test: set maxAllowed SCC and verify it's updated and not ignored by tektonconfig
	tc = s.getCurrentConfig(timeout)

	// set maxAllowed SCC
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "anyuid"

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	// make sure default SCC is being set correctly
	s.logger.Debug("verifying maxAllowed SCC is set to 'anyuid'")
	require.Equal(t, "anyuid", tc.Spec.Platforms.OpenShift.SCC.MaxAllowed)

	// ---

	// test: default > maxAllowed should fail
	tc = s.getCurrentConfig(timeout)

	defaultSCC := "anyuid"
	maxAllowedSCC := "nonroot"

	tc.Spec.Platforms.OpenShift.SCC.Default = defaultSCC
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = maxAllowedSCC

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.ErrorContains(t, err, "admission webhook \"validation.webhook.operator.tekton.dev\" denied the request")
	require.ErrorContains(t, err, "must be less restrictive than the default SCC")

	// test: default = maxAllowed should pass
	tc = s.getCurrentConfig(timeout)

	defaultSCC = "nonroot"
	maxAllowedSCC = "nonroot"

	tc.Spec.Platforms.OpenShift.SCC.Default = defaultSCC
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = maxAllowedSCC

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	// ---

	// test: default < maxAllowed should pass
	tc = s.getCurrentConfig(timeout)

	defaultSCC = "nonroot"
	maxAllowedSCC = "anyuid"

	tc.Spec.Platforms.OpenShift.SCC.Default = defaultSCC
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = maxAllowedSCC

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	// ---

	// test: when default SCC is removed, it should be set back to "pipelines-scc"
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.Default = ""
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "privileged"

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Equal(t, "pipelines-scc", tc.Spec.Platforms.OpenShift.SCC.Default)

	// ---

	// test: when default SCC is removed, the validation should be run again
	// and maxAllowed cannot be lower priority than default
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.Default = ""
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "restricted-v2"

	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.ErrorContains(t, err, "admission webhook \"validation.webhook.operator.tekton.dev\" denied the request: validation failed")

	// ---

	// test: any SCC can be requested in a namespace if no maxAllowed is set
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = ""
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	nsName := "scc-test-no-maxallowed"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "anyuid",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: invalid maxAllowed as per other SCCs in namespace
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.Default = "restricted-v2"
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = ""
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	nsName = "scc-test-invalid-maxallowed"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "anyuid",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "nonroot"
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.ErrorContains(t, err, "admission webhook \"validation.webhook.operator.tekton.dev\" denied the request: validation failed")

	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: valid maxAllowed as per other SCCs in namespace
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.Default = "restricted-v2"
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = ""
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	nsName = "scc-test-valid-maxallowed"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "anyuid",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "anyuid"
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: maxAllowed already set, namespace annotation valid
	tc = s.getCurrentConfig(timeout)

	tc.Spec.Platforms.OpenShift.SCC.Default = "restricted-v2"
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "anyuid"
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	nsName = "scc-test-valid-namespace-annotation"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "anyuid",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: maxAllowed already set, namespace annotation invalid

	tc = s.getCurrentConfig(timeout)
	tc.Spec.Platforms.OpenShift.SCC.Default = "restricted-v2"
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = "nonroot-v2"
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)
	// wait for tektonConfig ready status
	err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, s.interval, 1*time.Minute)

	nsName = "scc-test-invalid-namespace-annotation"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "anyuid",
			},
		},
	}, metav1.CreateOptions{})
	require.ErrorContains(t, err, "admission webhook \"namespace.operator.tekton.dev\" denied the request")

	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: non-existent SCC should not be admitted

	nonExistentSCC := "non-existent-scc"

	// default
	tc = s.getCurrentConfig(timeout)
	tc.Spec.Platforms.OpenShift.SCC.Default = nonExistentSCC
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.ErrorContains(t, err, "\"non-existent-scc\" not found")

	// maxAllowed
	tc = s.getCurrentConfig(timeout)
	tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = nonExistentSCC
	tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.ErrorContains(t, err, "\"non-existent-scc\" not found")

	// namespace SCC
	tc = s.getCurrentConfig(timeout)
	nsName = "non-existent-scc-namespace"
	_, err = s.clients.KubeClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Annotations: map[string]string{
				"operator.tekton.dev/scc": "non-existent-scc",
			},
		},
	}, metav1.CreateOptions{})
	require.ErrorContains(t, err, "\"non-existent-scc\" not found")

	err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, nsName, s.interval, 1*time.Minute)
	require.NoError(t, err)

	// ---

	// test: validate system and most used SCC priority order

	moreRestrictiveSCCList := []string{"restricted-v2", "nonroot-v2", "anyuid"}
	lessRestrictiveList := []string{"pipelines-scc", "hostnetwork-v2", "privileged"}

	for _, moreRestrictiveSCC := range moreRestrictiveSCCList {
		for _, lessRestrictiveSCC := range lessRestrictiveList {
			// passing cases
			tc = s.getCurrentConfig(timeout)

			tc.Spec.Platforms.OpenShift.SCC.Default = moreRestrictiveSCC
			tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = lessRestrictiveSCC
			tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
			require.NoError(t, err)

			// failing cases
			tc = s.getCurrentConfig(timeout)
			tc.Spec.Platforms.OpenShift.SCC.Default = lessRestrictiveSCC
			tc.Spec.Platforms.OpenShift.SCC.MaxAllowed = moreRestrictiveSCC
			tc, err = s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
			require.ErrorContains(t, err, "must be less restrictive than the default SCC")
		}
	}
}

// helper functions
func (s *TektonConfigTestSuite) changeProfile(targetProfile string) {
	t := s.T()
	timeout := 30 * time.Second

	s.logger.Debugw("changing profile", "newProfile", targetProfile)

	configCR := s.getCurrentConfig(timeout)
	require.NotEqual(t, targetProfile, configCR.Spec.Profile)

	configCR.Spec.Profile = targetProfile
	_, err := s.clients.TektonConfig().Update(context.TODO(), configCR, metav1.UpdateOptions{})
	require.NoError(t, err)

}
func (s *TektonConfigTestSuite) verifyProfile(targetProfile string, verifyProfileAddons bool) {
	t := s.T()
	interval := s.interval
	timeout := 2 * time.Minute

	s.verifyResources(targetProfile, verifyProfileAddons)

	// verify pipeline cr availability
	_, err := s.clients.TektonPipeline().Get(context.TODO(), s.resourceNames.TektonPipeline, metav1.GetOptions{})
	require.NoError(t, err)
	s.logger.Debug("pipeline cr is available")

	// verify rbac, pac, addons in openshift
	if utils.IsOpenShift() {
		s.verifyPAC()
		s.verifyAddons()
		s.verifyRbac(namespaceDefault)

		// create a namespace and verify rbac there
		testNamespace := common.SimpleNameGenerator.RestrictLengthWithRandomSuffix("e2e-tests-rbac")
		err := resources.CreateNamespace(s.clients.KubeClient, testNamespace)
		require.NoError(t, err)

		// verify rbac on created namespace
		s.verifyRbac(testNamespace)

		// delete the namespace
		err = resources.DeleteNamespaceAndWait(s.clients.KubeClient, testNamespace, interval, timeout)
		assert.NoError(t, err)
	}
}

func (s *TektonConfigTestSuite) verifyResources(expectedProfile string, verifyProfileAddons bool) {
	t := s.T()
	interval := s.interval
	timeout := s.timeout
	addonsUpdateTimeout := 3 * time.Minute

	profiles := tektonConfigProfileKubernetes
	if utils.IsOpenShift() {
		profiles = tektonConfigProfileOpenshift
	}

	// get tektonConfig cr
	configCR := s.getCurrentConfig(timeout)

	// verify profile
	require.Equal(t, expectedProfile, configCR.Spec.Profile, "verify profile match")

	// verify tektonConfig status
	// workaround - starts
	err := resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, interval, 3*time.Minute)
	if err != nil {
		// fix this issue and remove the following workaround.
		s.logger.Warnw("***WARNING*** fix the issue in product. running with workaround. restarting the operator pod",
			"issue", "https://github.com/tektoncd/operator/issues/1441",
		)
		retryCount := 3
		for {
			if retryCount == 0 {
				break
			}
			// delete operator pod
			err = resources.DeletePodByLabelSelector(s.clients.KubeClient, s.resourceNames.OperatorPodSelectorLabel, s.resourceNames.Namespace)
			require.NoError(t, err, "delete operator pod")

			// wait for tektonConfig ready status
			err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, interval, 2*time.Minute)
			if err == nil {
				break
			}
			retryCount--
		}
		// workaround - ends
		err = resources.WaitForTektonConfigReady(s.clients.TektonConfig(), s.resourceNames.TektonConfig, interval, timeout)
		require.NoError(t, err, "waiting for tektonConfig ready status")
	}
	s.logger.Debug("tektonConfig becomes ready")

	profileResources, found := profiles[configCR.Spec.Profile]
	require.True(t, found, "unknown profile received", configCR.Spec.Profile)

	// verify deployments
	for _, deploymentName := range profileResources.Deployments {
		namespace := s.resourceNames.TargetNamespace
		err = resources.WaitForDeploymentReady(s.clients.KubeClient, deploymentName, namespace, interval, timeout)
		require.NoError(t, err, "verify deployment", deploymentName, namespace)
	}

	// verify service accounts
	for _, sa := range profileResources.ServiceAccounts {
		namespace := s.resourceNames.TargetNamespace
		err = resources.WaitForServiceAccount(s.clients.KubeClient, sa, namespace, interval, timeout)
		require.NoError(t, err, "verify serviceAccount", sa, namespace)
	}

	// addons modified and differ from default profile, skipping addons test
	if !verifyProfileAddons {
		return
	}

	// verify addons
	labelSelector := fmt.Sprintf("%s=TektonAddon", v1alpha1.CreatedByKey)

	// wait for addons to up to date
	waitAddonsUpdateFunc := func() (bool, error) {
		addons, err := s.clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}
		return len(addons.Items) >= len(profileResources.AddonsInstallerSets), nil
	}
	err = wait.PollImmediate(interval, addonsUpdateTimeout, waitAddonsUpdateFunc)
	require.NoError(t, err)

	addons, err := s.clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(addons.Items), len(profileResources.AddonsInstallerSets), "addons")
	// verify individual addons
	actualAddons := make([]string, len(addons.Items))
	for index, addon := range addons.Items {
		actualAddons[index] = addon.GetName()
	}
	expectedAddons := profileResources.AddonsInstallerSets
	// addon names appended with random suffix
	// verify with prefix match
	for _, expectedAddon := range expectedAddons {
		found := false
		for _, actualAddon := range actualAddons {
			if strings.HasPrefix(actualAddon, expectedAddon) {
				found = true
				break
			}
		}
		require.True(t, found, "addon installerSet not found: %s", expectedAddon)
	}

}

// this function statically referring openshift resources
// if we plan to use this for kubernetes update this references
func (s *TektonConfigTestSuite) verifyAddons() {
	t := s.T()
	timeout := 2 * time.Minute

	configCR := s.getCurrentConfig(timeout)
	addonsEnabled := configCR.Spec.Profile == v1alpha1.ProfileAll
	addonLabelSelector := fmt.Sprintf("%s=TektonAddon", v1alpha1.CreatedByKey)

	// if addons enabled, verify expected resources
	if addonsEnabled {
		// verify addons config
		addon, err := s.clients.Operator.TektonAddons().Get(context.TODO(), v1alpha1.AddonResourceName, metav1.GetOptions{})
		require.NoError(t, err)

		// check if number of params passed in TektonConfig would be passed in TektonAddons
		tc := s.getCurrentConfig(timeout)
		itemsTektonConfig := tc.Spec.Addon.Params
		itemsTektonAddons := addon.Spec.Params
		// sort values in the slice
		sort.Slice(itemsTektonConfig, func(i, j int) bool { return itemsTektonConfig[i].Name < itemsTektonConfig[j].Name })
		sort.Slice(itemsTektonAddons, func(i, j int) bool { return itemsTektonAddons[i].Name < itemsTektonAddons[j].Name })

		diff := cmp.Diff(itemsTektonConfig, itemsTektonAddons)
		require.Empty(t, diff)

		// verify addons installer set count
		installerSets, err := s.clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{LabelSelector: addonLabelSelector})
		require.NoError(t, err)

		// get addons parameters, to guess the final count
		enabledAddonsCount := int(0)
		for _, param := range itemsTektonAddons {
			if strings.ToLower(param.Value) == "true" {
				enabledAddonsCount++
			}
		}

		addonsAll := tektonConfigProfileOpenshift[v1alpha1.ProfileAll].AddonsInstallerSets
		addonsNotByParams := []string{
			"addon-custom-consolecli",
			"addon-custom-openshiftconsole",
			"addon-custom-triggersresources",
		}

		expectedAddonsCount := 7
		expectedAddons := addonsAll

		// if addon disabled
		if enabledAddonsCount != 3 {
			expectedAddonsCount = 3
			expectedAddons = addonsNotByParams
		}
		require.GreaterOrEqual(t, len(installerSets.Items), expectedAddonsCount, "addons count")

		// addon installeset names appended with random suffix
		// verify with prefix of installerset name
		actualAddons := make([]string, len(installerSets.Items))
		for index, addon := range installerSets.Items {
			actualAddons[index] = addon.GetName()
		}
		for _, expectedAddon := range expectedAddons {
			found := false
			for _, actualAddon := range actualAddons {
				if strings.HasPrefix(actualAddon, expectedAddon) {
					found = true
					break
				}
			}
			require.True(t, found, "addon installerSet not found: %s", expectedAddon)
		}
	}

	if !addonsEnabled {
		// verify addons config not available
		_, err := s.clients.Operator.TektonAddons().Get(context.TODO(), v1alpha1.AddonResourceName, metav1.GetOptions{})
		require.True(t, apierrs.IsNotFound(err))

		// verify no addons installerset available
		installerSets, err := s.clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{LabelSelector: addonLabelSelector})
		require.NoError(t, err)
		require.Equal(t, 0, len(installerSets.Items), "addon installerSets")
	}
}

// TODO: verify pac routes, import or implement openshift route api
func (s *TektonConfigTestSuite) verifyPAC() {
	t := s.T()
	interval := s.interval
	timeout := 3 * time.Minute

	config := s.getCurrentConfig(timeout)

	// pac deployments
	pacDeployments := []string{
		"pipelines-as-code-controller",
		"pipelines-as-code-watcher",
		"pipelines-as-code-webhook",
	}

	// get pac enabled status from TektonConfig resource
	pacEnabled := false
	if config.Spec.Platforms.OpenShift.PipelinesAsCode != nil &&
		config.Spec.Platforms.OpenShift.PipelinesAsCode.Enable != nil &&
		*config.Spec.Platforms.OpenShift.PipelinesAsCode.Enable {
		pacEnabled = true
	}

	labelSelector := fmt.Sprintf("%s=OpenShiftPipelinesAsCode", v1alpha1.CreatedByKey)
	installerSets, err := s.clients.Operator.TektonInstallerSets().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	require.NoError(t, err)

	// verifications on pac enabled
	if pacEnabled {
		// verify installersets availability
		require.Equal(t, 3, len(installerSets.Items))

		// verify deployments
		for _, deploymentName := range pacDeployments {
			err = resources.WaitForDeploymentReady(s.clients.KubeClient, deploymentName, s.resourceNames.TargetNamespace, interval, timeout)
			require.NoError(t, err)
		}

	}

	// verifications on pac disabled
	if !pacEnabled {
		// verify installersets availability
		require.Equal(t, 0, len(installerSets.Items))

		// verify deployments
		for _, deploymentName := range pacDeployments {
			err = resources.WaitForDeploymentDeletion(s.clients.KubeClient, deploymentName, s.resourceNames.TargetNamespace, interval, timeout)
			require.NoError(t, err)
		}
	}

}

func (s *TektonConfigTestSuite) verifyRbac(namespace string) {
	t := s.T()
	interval := s.interval
	timeout := s.timeout

	s.logger.Debugw("running rbac verification",
		"namespace", namespace,
	)

	// get tektonConfig cr
	configCR := s.getCurrentConfig(timeout)

	// make sure default SCC is being set correctly
	s.logger.Debug("verifying default SCC is set to 'pipelines-scc'")
	require.Equal(t, "pipelines-scc", configCR.Spec.Platforms.OpenShift.SCC.Default)

	// resources
	serviceAccountPipeline := "pipeline"
	clusterRolePipelinesSCC := "pipelines-scc-clusterrole"
	configMapServiceCABundle := "config-service-cabundle"
	configMapTrustedCABundle := "config-trusted-cabundle"
	roleBindingPipelinesSCC := "pipelines-scc-rolebinding"
	roleBindingPipelinesEdit := tconfig.PipelineRoleBinding

	// verify "pipeline" sa has created
	err := resources.WaitForServiceAccount(s.clients.KubeClient, serviceAccountPipeline, namespace, interval, timeout)
	require.NoError(t, err)

	// verify cluster role
	err = resources.WaitForClusterRole(s.clients.KubeClient, clusterRolePipelinesSCC, interval, timeout)
	require.NoError(t, err)

	// verify the configMaps are available
	err = resources.WaitForConfigMap(s.clients.KubeClient, configMapServiceCABundle, namespace, interval, timeout)
	require.NoError(t, err)
	err = resources.WaitForConfigMap(s.clients.KubeClient, configMapTrustedCABundle, namespace, interval, timeout)
	require.NoError(t, err)

	// verify the roleBindings are available
	err = resources.WaitForRoleBinding(s.clients.KubeClient, roleBindingPipelinesSCC, namespace, interval, timeout)
	require.NoError(t, err)
	err = resources.WaitForRoleBinding(s.clients.KubeClient, roleBindingPipelinesEdit, namespace, interval, timeout)
	require.NoError(t, err)
}

// helper functions
func (s *TektonConfigTestSuite) undeploy() {
	t := s.T()
	interval := s.interval
	timeout := utils.Timeout
	resources.TektonConfigCRDelete(t, s.clients, s.resourceNames)
	err := resources.WaitForNamespaceDeletion(s.clients.KubeClient, s.resourceNames.TargetNamespace, interval, timeout)
	require.NoError(t, err)
	s.logger.Debugw("target namespace removed",
		"namespace", s.resourceNames.TargetNamespace,
	)
}

func (s *TektonConfigTestSuite) getCurrentConfig(timeout time.Duration) *v1alpha1.TektonConfig {
	t := s.T()
	interval := s.interval
	verifyConfig := func() (bool, error) {
		_, err := s.clients.TektonConfig().Get(context.Background(), s.resourceNames.TektonConfig, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	err := wait.PollImmediate(interval, timeout, verifyConfig)
	require.NoError(t, err)

	configCR, err := s.clients.TektonConfig().Get(context.Background(), s.resourceNames.TektonConfig, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, configCR)
	return configCR
}

func (s *TektonConfigTestSuite) getDefaultConfig() *v1alpha1.TektonConfig {
	pruneKeep := uint(100)
	configCR := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: defaultTektonConfigProfile,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: s.resourceNames.TargetNamespace,
			},
			Pruner: v1alpha1.Prune{
				Resources: []string{"pipelinerun"},
				Keep:      &pruneKeep,
				KeepSince: nil,
				Schedule:  "0 8 * * *",
			},
			// Disable the TektonPruner by default
			TektonPruner: v1alpha1.Pruner{
				Disabled: true,
			},
		},
	}
	configCR.SetDefaults(context.TODO())

	return configCR
}

// resets the tekton config to its default and make ready to run next tests
func (s *TektonConfigTestSuite) resetToDefaults() {
	t := s.T()
	timeout := s.timeout

	tc := s.getCurrentConfig(timeout)

	defaultTC := s.getDefaultConfig()
	// update to defaults
	tc.Spec = defaultTC.Spec

	_, err := s.clients.TektonConfig().Update(context.TODO(), tc, metav1.UpdateOptions{})
	require.NoError(t, err)

	s.verifyProfile(tc.Spec.Profile, true)
}

// deletes the operator pod
func (s *TektonConfigTestSuite) recreateOperatorPod() {
	t := s.T()
	interval := s.interval
	timeout := s.timeout

	// delete operator pod
	// TektonConfig 'config' will be created on operator pod startup
	s.logger.Debug("deleting the operator pod")
	err := resources.DeletePodByLabelSelector(s.clients.KubeClient, s.resourceNames.OperatorPodSelectorLabel, s.resourceNames.Namespace)
	require.NoError(t, err)

	s.logger.Debug("waiting for the operator pod get into running state")
	err = resources.WaitForPodByLabelSelector(s.clients.KubeClient, s.resourceNames.OperatorPodSelectorLabel, s.resourceNames.Namespace, interval, timeout)
	require.NoError(t, err)
}
