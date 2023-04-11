//go:build e2e
// +build e2e

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

package common

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DeploymentNameTektonChain = "tekton-chains-controller"
)

type TektonChainTutorialTestSuite struct {
	resourceNames utils.ResourceNames
	suite.Suite
	clients  *utils.Clients
	interval time.Duration
	timeout  time.Duration
	logger   *zap.SugaredLogger
}

func TestTektonChainTutorialTestSuite(t *testing.T) {
	platform := utils.GetOSAndArchitecture()
	if platform == utils.LinuxPPC64LE || platform == utils.LinuxS390X {
		t.Skipf("Tekton chain is not available for %q", platform)
	}

	ts := NewTektonChainTutorialTestSuite(t)
	// run the actual tests
	suite.Run(t, ts)
}

// if other suites want to execute a method on this suite,
// can create a instance by calling this function
// also used internally
func NewTektonChainTutorialTestSuite(t *testing.T) *TektonChainTutorialTestSuite {
	ts := TektonChainTutorialTestSuite{
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
func (s *TektonChainTutorialTestSuite) SetupSuite() {
	resources.PrintClusterInformation(s.logger, s.resourceNames)

	// reset the pipelines into default state
	s.logger.Debug("resetting TektonConfig to it's default state")
	tcSuite := NewTestTektonConfigTestSuite(s.T())
	tcSuite.recreateOperatorPod()
	tcSuite.resetToDefaults()
}

// after suite
func (s *TektonChainTutorialTestSuite) TearDownSuite() {
	// noop
}

// before each tests
func (s *TektonChainTutorialTestSuite) SetupTest() {
	// noop
}

// after each tests
// if there is a failures, execute debug commands
func (s *TektonChainTutorialTestSuite) TearDownTest() {
	t := s.T()
	if t.Failed() {
		s.logger.Infow("test failed, executing debug commands", "testName", t.Name())
		resources.ExecuteDebugCommands(s.logger, s.resourceNames)
	}
}

// actual tests

// TestTektonChainDeployment verifies the TektonChain creation, deployment recreation, and TektonChain deletion.
func (s *TektonChainTutorialTestSuite) Test01() {
	t := s.T()
	//interval := s.interval
	//timeout := s.timeout

	// Create a TektonChain
	if _, err := resources.EnsureTektonChainExists(s.clients.TektonChains(), s.resourceNames); err != nil {
		t.Fatalf("TektonChain %q failed to create: %v", s.resourceNames.TektonChain, err)
	}

	// Test if TektonChain can reach the READY status
	t.Run("create-chain", func(t *testing.T) {
		resources.AssertTektonChainCRReadyStatus(t, s.clients, s.resourceNames)
	})

	cosignSecret := "signing-secrets"
	chainsConfigMapName := "chains-config"

	// cosign generate-key-pair k8s://tekton-chains/signing-secrets
	t.Run("create cosign key pair", func(t *testing.T) {
		err := resources.CosignGenerateKeyPair(s.resourceNames.TargetNamespace, cosignSecret)
		if err != nil {
			t.Fatal(err)
		}
	})

	// kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.oci.storage": "", "artifacts.taskrun.format":"in-toto", "artifacts.taskrun.storage": "tekton"}}'
	chainsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainsConfigMapName,
			Namespace: s.resourceNames.TargetNamespace,
		},
		Data: map[string]string{
			"artifacts.oci.storage":     "",
			"artifacts.taskrun.format":  "in-toto",
			"artifacts.taskrun.storage": "tekton",
		},
	}

	t.Run("replace Chains ConfigMap", func(t *testing.T) {
		_, err := resources.ReplaceConfigMap(s.clients.KubeClient, chainsConfigMap)
		if err != nil {
			t.Fatal(err)
		}
	})

	// kubectl delete po -n tekton-chains -l app=tekton-chains-controller
	t.Run("restart chains pod", func(t *testing.T) {
		err := resources.DeleteChainsPod(s.clients.KubeClient, s.resourceNames.TargetNamespace)
		if err != nil {
			t.Fatal(err)
		}

		// make sure the new pod is up and running again
		resources.AssertTektonChainCRReadyStatus(t, s.clients, s.resourceNames)
	})

	// kubectl create -f https://raw.githubusercontent.com/tektoncd/chains/main/examples/taskruns/task-output-image.yaml
	taskRunPath, err := filepath.Abs("./testdata/chain_tutorial_task_output_image.yaml")
	if err != nil {
		t.Fatal(err)
	}
	taskRunYAML, err := ioutil.ReadFile(taskRunPath)
	if err != nil {
		t.Fatal(err)
	}

	// Run chains test in a new namespace
	// as pipelines enforce the pod security fields this test would fail if ran
	// in tekton-pipelines namespace, as pipelines controller doesn't creates pods
	// with pod security fields
	testNamespace := common.SimpleNameGenerator.RestrictLengthWithRandomSuffix("chains-test")
	s.logger.Debugw("chain test namespace generated",
		"namespace", testNamespace,
	)
	// delete this namespace on exit
	defer func() {
		err := resources.DeleteNamespace(s.clients.KubeClient, testNamespace)
		require.NoError(t, err)
	}()

	if _, err := resources.EnsureTestNamespaceExists(s.clients, testNamespace); err != nil {
		t.Fatalf("failed to create test namespace: %s, %q", testNamespace, err)
	}

	taskRunName := "build-push-run-output-image-test"
	taskOutputImageTaskRun := parse.MustParseV1beta1TaskRun(t, string(taskRunYAML))
	taskOutputImageTaskRun.Namespace = testNamespace

	t.Run("create TaskRun", func(t *testing.T) {
		_, err := resources.EnsureTaskRunExists(s.clients.TektonClient, taskOutputImageTaskRun)
		if err != nil {
			t.Fatal(err)
		}
	})

	// wait for TR to finish
	taskRunSucceededFunc := func(taskRun *v1beta1.TaskRun) (bool, error) {
		allConditionsHappy := true
		if len(taskRun.Status.Conditions) == 0 {
			return false, nil
		}
		for _, condition := range taskRun.Status.Conditions {
			if condition.Status != corev1.ConditionTrue || condition.Reason != "Succeeded" {
				allConditionsHappy = false
				break
			}
		}
		if !allConditionsHappy {
			// TaskRun has still not Succeeded
			return false, nil
		}
		return true, nil
	}

	t.Run("wait for TaskRun to succeed", func(t *testing.T) {
		err := resources.WaitForTaskRunHappy(s.clients.TektonClient, testNamespace, taskRunName, taskRunSucceededFunc)
		if err != nil {
			t.Fatal(err)
		}
	})

	taskRunSignedFunc := func(taskRun *v1beta1.TaskRun) (bool, error) {
		if taskRun.Annotations["chains.tekton.dev/signed"] == "true" {
			return true, nil
		}
		return false, nil
	}

	t.Run("wait for TaskRun to get signed", func(t *testing.T) {
		err := resources.WaitForTaskRunHappy(s.clients.TektonClient, testNamespace, taskRunName, taskRunSignedFunc)
		if err != nil {
			t.Fatal(err)
		}
	})

	taskRun, err := s.clients.TektonClient.TaskRuns(testNamespace).Get(context.TODO(), taskRunName, metav1.GetOptions{})

	// export TASKRUN_UID=$(tkn tr describe --last -o  jsonpath='{.metadata.uid}')
	taskRunUID := taskRun.ObjectMeta.UID

	// tkn tr describe --last -o jsonpath="{.metadata.annotations.chains\.tekton\.dev/signature-taskrun-$TASKRUN_UID}" | base64 -d > signature
	encodedSignature, ok := taskRun.Annotations["chains.tekton.dev/signature-taskrun-"+string(taskRunUID)]
	if !ok {
		t.Fatal(fmt.Errorf("no signature found on TaskRun %v", taskRunName))
	}
	signature, err := base64.StdEncoding.DecodeString(encodedSignature)
	if err != nil {
		t.Fatal(err)
	}

	// tkn tr describe --last -o jsonpath="{.metadata.annotations.chains\.tekton\.dev/payload-taskrun-$TASKRUN_UID}" | base64 -d > payload
	encodedPayload, ok := taskRun.Annotations["chains.tekton.dev/payload-taskrun-"+string(taskRunUID)]
	if !ok {
		t.Fatal(fmt.Errorf("no payload found on TaskRun %v", taskRunName))
	}
	payload, err := base64.StdEncoding.DecodeString(encodedPayload)
	if err != nil {
		t.Fatal(err)
	}

	// cosign verify-blob-attestation --insecure-ignore-tlog --key k8s://tekton-chains/signing-secrets --signature signature --type slsaprovenance --check-claims=false /dev/null
	t.Run("cosign very-blob-attestation", func(t *testing.T) {
		err := resources.CosignVerifyBlobAttestation(fmt.Sprintf("k8s://%v/%v", s.resourceNames.TargetNamespace, cosignSecret), string(signature), string(payload))
		if err != nil {
			t.Fatal(err)
		}
	})

}

func (s *TektonChainTutorialTestSuite) deleteChainCR() {
	t := s.T()
	interval := s.interval
	timeout := s.timeout

	err := s.clients.TektonChains().Delete(context.TODO(), s.resourceNames.TektonChain, metav1.DeleteOptions{})
	if err != nil && apierrs.IsNotFound(err) {
		return
	}
	require.NoError(t, err, "delete tektonChain cr")

	err = resources.WaitForDeploymentDeletion(s.clients.KubeClient, DeploymentNameTektonChain, s.resourceNames.TargetNamespace, interval, timeout)
	require.NoError(t, err, "wait for tektonChain deployment deletion")
}
