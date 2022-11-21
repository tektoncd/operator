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
	"testing"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTektonHubDeploymentWithExternalDatabase(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonHub:       "hub",
		TargetNamespace: "tekton-pipelines",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	// Install db in different namespace
	c, _ := mfc.NewUnsafeDynamicClient(clients.Dynamic)
	manifest, err := mf.NewManifest("testdata/db.yaml", mf.UseClient(c))
	if err != nil {
		t.Error(err)
	}

	if err := manifest.Apply(); err != nil {
		t.Error(err)
	}

	// Create a TektonHub
	if _, err := resources.EnsureTektonHubExists(clients.TektonHub(), crNames); err != nil {
		t.Fatalf("TektonHub %q failed to create: %v", crNames.TektonHub, err)
	}

	// Test if TektonHub can reach the READY status
	t.Run("create-hub", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
	})

	// Replace and update `tekton-hub-db` secret
	sec, err := clients.KubeClient.CoreV1().Secrets("tekton-pipelines").Get(context.Background(), "tekton-hub-db", metav1.GetOptions{})
	if err != nil {
		if apierror.IsNotFound(err) {
			secManifest, err := mf.NewManifest("testdata/secret.yaml", mf.UseClient(c))
			if err != nil {
				t.Error(err)
			}

			// Save the manifest resources
			if err := secManifest.Apply(); err != nil {
				t.Error(err)
			}
		}
		t.Error(err)
	} else {
		if err := clients.KubeClient.CoreV1().Secrets("tekton-pipelines").Delete(context.Background(), sec.Name, metav1.DeleteOptions{}); err != nil {
			t.Error(err)
		}

		secManifest, err := mf.NewManifest("testdata/secret.yaml", mf.UseClient(c))
		if err != nil {
			t.Error(err)
		}

		// Save the manifest resources
		if err := secManifest.Apply(); err != nil {
			t.Error(err)
		}
	}

	// Update tekton hub spec
	ks, err := clients.TektonHub().Get(context.TODO(), crNames.TektonHub, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	ks.Spec.Db.DbSecretName = "tekton-hub-db"

	_, err = clients.TektonHub().Update(context.Background(), ks, metav1.UpdateOptions{})
	if err != nil {
		t.Error(err)
	}

	// Test if TektonHub can reach the READY status
	t.Run("validate-hub", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonHub CR instance to see if all resources will be removed
	t.Run("delete-hub", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
		resources.TektonHubCRDelete(t, clients, crNames)
	})
}

func TestTektonHubDeployment(t *testing.T) {
	crNames := utils.ResourceNames{
		TektonConfig:    "config",
		TektonHub:       "hub",
		TargetNamespace: "tekton-hub",
	}

	clients := client.Setup(t, crNames.TargetNamespace)

	// Create a TektonHub
	if _, err := resources.EnsureTektonHubExists(clients.TektonHub(), crNames); err != nil {
		t.Fatalf("TektonHub %q failed to create: %v", crNames.TektonHub, err)
	}

	// Test if TektonHub can reach the READY status
	t.Run("create-hub", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore-hub-deployments", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
		resources.DeleteAndVerifyDeployments(t, clients, crNames.TargetNamespace, utils.TektonHubDeploymentLabel)
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
	})

	// Delete the TektonHub CR instance to see if all resources will be removed
	t.Run("delete-hub", func(t *testing.T) {
		resources.AssertTektonHubCRReadyStatus(t, clients, crNames)
		resources.TektonHubCRDelete(t, clients, crNames)
	})
}
