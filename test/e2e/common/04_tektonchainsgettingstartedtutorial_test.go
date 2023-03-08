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
	"os"
	"path/filepath"
	"testing"

	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestTektonChainDeployment verifies the TektonChain creation, deployment recreation, and TektonChain deletion.
func TestTektonChainsGettingStartedTutorial(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonPipeline:  "pipeline",
		TektonChain:     "chain",
		TargetNamespace: "tekton-pipelines",
	}

	if os.Getenv("TARGET") == "openshift" {
		crNames.TargetNamespace = "openshift-pipelines"
	}
	platform := os.Getenv("PLATFORM")
	if platform == "linux/ppc64le" || platform == "linux/s390x" {
		t.Skipf("Tekton chain is not available for %q", platform)
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	utils.CleanupOnInterrupt(func() { utils.TearDownPipeline(clients, crNames.TektonPipeline) })
	utils.CleanupOnInterrupt(func() { utils.TearDownChain(clients, crNames.TektonChain) })
	utils.CleanupOnInterrupt(func() { utils.TearDownNamespace(clients, crNames.TargetNamespace) })
	defer utils.TearDownNamespace(clients, crNames.TargetNamespace)
	defer utils.TearDownPipeline(clients, crNames.TektonPipeline)
	defer utils.TearDownChain(clients, crNames.TektonChain)

	resources.EnsureNoTektonConfigInstance(t, clients, crNames)

	// Create a TektonPipeline
	if _, err := resources.EnsureTektonPipelineExists(clients.TektonPipeline(), crNames); err != nil {
		t.Fatalf("TektonPipeline %q failed to create: %v", crNames.TektonPipeline, err)
	}

	// Test if TektonPipeline can reach the READY status
	t.Run("create-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
	})

	// Create a TektonChain
	if _, err := resources.EnsureTektonChainExists(clients.TektonChains(), crNames); err != nil {
		t.Fatalf("TektonChain %q failed to create: %v", crNames.TektonChain, err)
	}

	// Test if TektonChain can reach the READY status
	t.Run("create-chain", func(t *testing.T) {
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
	})

	cosignSecret := "signing-secrets"
	chainsConfigMapName := "chains-config"

	// cosign generate-key-pair k8s://tekton-chains/signing-secrets
	t.Run("create cosign key pair", func(t *testing.T) {
		err := resources.CosignGenerateKeyPair(crNames.TargetNamespace, cosignSecret)
		if err != nil {
			t.Fatal(err)
		}
	})

	// kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.oci.storage": "", "artifacts.taskrun.format":"in-toto", "artifacts.taskrun.storage": "tekton"}}'
	chainsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      chainsConfigMapName,
			Namespace: crNames.TargetNamespace,
		},
		Data: map[string]string{
			"artifacts.oci.storage":     "",
			"artifacts.taskrun.format":  "in-toto",
			"artifacts.taskrun.storage": "tekton",
		},
	}

	t.Run("replace Chains ConfigMap", func(t *testing.T) {
		_, err := resources.ReplaceConfigMap(clients.KubeClient, chainsConfigMap)
		if err != nil {
			t.Fatal(err)
		}
	})

	// kubectl delete po -n tekton-chains -l app=tekton-chains-controller
	t.Run("restart chains pod", func(t *testing.T) {
		err := resources.DeleteChainsPod(clients.KubeClient, crNames.TargetNamespace)
		if err != nil {
			t.Fatal(err)
		}

		// make sure the new pod is up and running again
		resources.AssertTektonChainCRReadyStatus(t, clients, crNames)
	})

	// kubectl create -f https://raw.githubusercontent.com/tektoncd/chains/main/examples/taskruns/task-output-image.yaml
	taskRunPath, err := filepath.Abs("../../testdata/task-output-image.yaml")
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
	testNamespace := "chains-test"

	if _, err := resources.EnsureTestNamespaceExists(clients, testNamespace); err != nil {
		t.Fatalf("failed to create test namespace: %s, %q", testNamespace, err)
	}

	taskRunName := "build-push-run-output-image-test"
	taskOutputImageTaskRun := parse.MustParseV1beta1TaskRun(t, string(taskRunYAML))
	taskOutputImageTaskRun.Namespace = testNamespace

	t.Run("create TaskRun", func(t *testing.T) {
		_, err := resources.EnsureTaskRunExists(clients.TektonClient, taskOutputImageTaskRun)
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
		err := resources.WaitForTaskRunHappy(clients.TektonClient, testNamespace, taskRunName, taskRunSucceededFunc)
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
		err := resources.WaitForTaskRunHappy(clients.TektonClient, testNamespace, taskRunName, taskRunSignedFunc)
		if err != nil {
			t.Fatal(err)
		}
	})

	taskRun, err := clients.TektonClient.TaskRuns(testNamespace).Get(context.TODO(), taskRunName, metav1.GetOptions{})

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
		err := resources.CosignVerifyBlobAttestation(fmt.Sprintf("k8s://%v/%v", crNames.TargetNamespace, cosignSecret), string(signature), string(payload))
		if err != nil {
			t.Fatal(err)
		}
	})

	// Delete the TektonPipeline CR instance to see if all resources will be removed
	t.Run("delete-pipeline", func(t *testing.T) {
		resources.AssertTektonPipelineCRReadyStatus(t, clients, crNames)
		resources.TektonPipelineCRDelete(t, clients, crNames)
	})
}
