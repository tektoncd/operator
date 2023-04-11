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
	"fmt"
	"strings"
	"testing"
	"time"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/test/client"
	"github.com/tektoncd/operator/test/resources"
	"github.com/tektoncd/operator/test/utils"
	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	hubExternalDatabaseYaml       = "testdata/hub_externaldb.yaml"
	hubExternalDatabaseSecretYaml = "testdata/hub_externaldb_secret.yaml"
	hubDatabaseExternalSecretName = "tekton-hub-db"
	hubExternalDatabaseNamespace  = "hub-external-db"
	hubDatabaseDeploymentName     = "tekton-hub-db"
)

type TektonHubTestSuite struct {
	suite.Suite
	resourceNames      utils.ResourceNames
	clients            *utils.Clients
	deployments        []string
	dbMigrationJobName string
	interval           time.Duration
	timeout            time.Duration
	logger             *zap.SugaredLogger
}

func TestTektonHubTestSuite(t *testing.T) {
	hs := NewTektonHubTestSuite(t)

	// actual tests starts here
	suite.Run(t, hs)
}

// if other suites want to execute a method on this suite,
// can create a instance by calling this function
// also used internally
func NewTektonHubTestSuite(t *testing.T) *TektonHubTestSuite {
	hs := TektonHubTestSuite{
		resourceNames: utils.GetResourceNames(),
		deployments: []string{
			hubDatabaseDeploymentName,
			"tekton-hub-api",
			"tekton-hub-ui",
		},
		dbMigrationJobName: "tekton-hub-db-migration",
		interval:           5 * time.Second,
		timeout:            5 * time.Minute,
		logger:             utils.Logger(),
	}

	// setup clients
	hs.clients = client.Setup(t, hs.resourceNames.TargetNamespace)

	return &hs
}

// before suite
func (s *TektonHubTestSuite) SetupSuite() {
	resources.PrintClusterInformation(s.logger, s.resourceNames)
}

// after suite
func (s *TektonHubTestSuite) TearDownSuite() {
	// removes tekton hub CR and external database
	s.undeploy("")
	s.undeployExternalDatabase()
}

// before each tests
// clean up existing hub cr and external database
func (s *TektonHubTestSuite) SetupTest() {
	t := s.T()
	s.logger.Debug("removing the tekton hub cr if any")
	s.undeploy("")
	s.undeployExternalDatabase()
	err := resources.CreateNamespace(s.clients.KubeClient, s.resourceNames.TargetNamespace)
	require.NoError(t, err, "create namespace: %s", s.resourceNames.TargetNamespace)
	s.logger.Debug("test environment ready. starting the actual test")
}

// after each tests
// if there is a failures, execute debug commands
func (s *TektonHubTestSuite) TearDownTest() {
	t := s.T()
	if t.Failed() {
		s.logger.Infow("test failed, executing debug commands", "testName", t.Name())
		resources.ExecuteDebugCommands(s.logger, s.resourceNames)

		// remove tektonHub and external database, if any
		s.undeployExternalDatabase()
		s.undeploy("")
	}
}

// actual tests
// TODO: add tests to verify data from UI and API endpoint

// deploys default TektonHub CR and verify resources
func (s *TektonHubTestSuite) Test01_DeployDefault() {
	s.deploy("", s.resourceNames.TektonHub)
	s.verifyResources("")
}

// deploys TektonHub CR external database
func (s *TektonHubTestSuite) Test02_DeployWithExternalDatabase() {
	// deploy external database
	s.deployExternalDatabase()

	// deploy tektonHub
	s.deploy(hubDatabaseExternalSecretName, s.resourceNames.TektonHub)

	// verify resources
	s.verifyResources(hubExternalDatabaseNamespace)
}

// deploys default TektonHub CR and updates the CR to external database
func (s *TektonHubTestSuite) Test03_DeployDefaultThenUpdateToExternalDatabase() {
	t := s.T()
	pollInterval := s.interval
	timeout := s.timeout

	// deploy tektonHub
	s.deploy("", s.resourceNames.TektonHub)
	s.logger.Debug("deployed hub with self provisioned database")

	// verify resources
	s.verifyResources("")

	s.logger.Debug("deploying hub with external database")
	// deploy external database
	s.deployExternalDatabase()

	// update hub cr
	hubCR, err := s.clients.TektonHub().Get(context.TODO(), s.resourceNames.TektonHub, metav1.GetOptions{})
	require.NoError(t, err)
	hubCR.Spec.Db.DbSecretName = hubDatabaseExternalSecretName

	// update hub cr
	_, err = s.clients.TektonHub().Update(context.Background(), hubCR, metav1.UpdateOptions{})
	require.NoError(t, err)

	// verify internal database removed
	err = resources.WaitForDeploymentDeletion(s.clients.KubeClient, hubDatabaseDeploymentName, s.resourceNames.TargetNamespace, pollInterval, timeout)
	require.NoError(t, err)

	// verify hub is updated and running
	s.verifyResources(hubExternalDatabaseNamespace)
}

// deploys the hub with invalid name
// operator accepts only TektonHub name with "hub"
func (s *TektonHubTestSuite) Test04_DeployWithInvalidHubName() {
	t := s.T()

	// random hub name
	hubName := common.SimpleNameGenerator.RestrictLengthWithRandomSuffix("custom-hub")

	// deploy tektonHub
	s.deploy("", hubName)

	// remove this hub cr on exit
	defer func() {
		err := s.clients.TektonHub().Delete(context.TODO(), hubName, metav1.DeleteOptions{})
		if err != nil {
			s.logger.Errorw("error on cleaning the hub cr", "name", hubName)
		}
	}()

	expectedMessage := fmt.Sprintf("Resource ignored, Expected Name: %s, Got Name: %s", s.resourceNames.TektonHub, hubName)
	// wait for tektonHub status for 30 seconds
	verifyStatus := func() (bool, error) {
		th, err := s.clients.TektonHub().Get(context.TODO(), hubName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if th.Status.IsReady() {
			return false, fmt.Errorf("TektonHub is in ready status. name:%s", hubName)
		}
		for _, condition := range th.Status.Conditions {
			if condition.Type == "Ready" {
				s.logger.Debugw("ready status", "condition", condition)
				return strings.Contains(condition.Message, expectedMessage), nil
			}
		}
		return false, nil
	}

	err := wait.PollImmediate(s.interval, 30*time.Second, verifyStatus)
	require.NoError(t, err)
}

// helper functions

func (s *TektonHubTestSuite) deploy(databaseSecretName, hubName string) {
	t := s.T()

	if hubName == "" {
		hubName = s.resourceNames.TektonHub
	}

	hubSpec := &v1alpha1.TektonHub{
		ObjectMeta: metav1.ObjectMeta{
			Name: hubName,
		},
		Spec: v1alpha1.TektonHubSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: s.resourceNames.TargetNamespace,
			},
		},
	}

	if databaseSecretName != "" {
		hubSpec.Spec.Db.DbSecretName = databaseSecretName
	}

	// create a TektonHub CR
	_, err := resources.EnsureTektonHubExists(s.clients.TektonHub(), hubSpec)
	require.NoError(t, err, "TektonHub %q failed to create: %v", hubSpec.GetName(), err)
}

func (s *TektonHubTestSuite) verifyResources(databaseNamespace string) {
	t := s.T()
	interval := s.interval
	timeout := s.timeout

	// assert tekton hub ready status
	resources.AssertTektonHubCRReadyStatus(t, s.clients, s.resourceNames)

	// verify deployments are ready
	for _, deploymentName := range s.deployments {
		namespace := s.resourceNames.TargetNamespace
		if deploymentName == hubDatabaseDeploymentName && databaseNamespace != "" {
			namespace = databaseNamespace
		}
		err := resources.WaitForDeploymentReady(s.clients.KubeClient, deploymentName, namespace, interval, timeout)
		require.NoError(t, err, "wait for a deployment ready status, deployment:%s, namespace:%s", deploymentName, namespace)
	}

	// verify migration job has completed successfully
	err := resources.WaitForJobCompletion(s.clients.KubeClient, s.dbMigrationJobName, s.resourceNames.TargetNamespace, interval, timeout)
	require.NoError(t, err, "wait for database migration job completion")
}

func (s *TektonHubTestSuite) undeploy(databaseNamespace string) {
	t := s.T()
	pollInterval := s.interval
	timeout := s.timeout

	// delete the hub cr
	resources.TektonHubCRDelete(t, s.clients, s.resourceNames)

	// verify deployments are removed
	for _, deploymentName := range s.deployments {
		namespace := s.resourceNames.TargetNamespace
		if deploymentName == hubDatabaseDeploymentName && databaseNamespace != "" {
			// no need to verify external database removal
			continue
		}
		err := resources.WaitForDeploymentDeletion(s.clients.KubeClient, deploymentName, namespace, pollInterval, timeout)
		require.NoError(t, err)
	}
	// verify migration job is removed
	err := resources.WaitForJobDeletion(s.clients.KubeClient, s.dbMigrationJobName, s.resourceNames.TargetNamespace, pollInterval, timeout)
	require.NoError(t, err)
}

func (s *TektonHubTestSuite) deployExternalDatabase() {
	t := s.T()
	pollInterval := s.interval
	timeout := s.timeout

	// install db in different namespace and wait for deployment ready
	mfClient, err := mfc.NewUnsafeDynamicClient(s.clients.Dynamic)
	require.NoError(t, err)

	manifest, err := mf.NewManifest(hubExternalDatabaseYaml, mf.UseClient(mfClient))
	require.NoError(t, err)

	// apply transformers
	updatedManifest, err := manifest.Transform(s.externalDatabaseTransformer)
	require.NoError(t, err)
	manifest = updatedManifest

	// apply manifests
	err = manifest.Apply()
	require.NoError(t, err)

	s.logger.Debugw("wait for external database deployment becomes ready",
		"namespace", hubExternalDatabaseNamespace,
		"deploymentName", hubDatabaseDeploymentName,
	)
	err = resources.WaitForDeploymentReady(s.clients.KubeClient, hubDatabaseDeploymentName, hubExternalDatabaseNamespace, pollInterval, timeout)
	require.NoError(t, err, "external database deployment is not ready")
	s.logger.Debug("external database deployment is ready")

	// create secret to namespace where hub api will be deployed
	// remove the existing secret, if any
	err = s.clients.KubeClient.CoreV1().Secrets(s.resourceNames.TargetNamespace).Delete(context.TODO(), hubDatabaseExternalSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		require.NoError(t, err)
	}
	// create secret
	secManifest, err := mf.NewManifest(hubExternalDatabaseSecretYaml, mf.UseClient(mfClient))
	require.NoError(t, err)

	// apply transformers
	updatedManifest, err = secManifest.Transform(s.externalDatabaseTransformer)
	require.NoError(t, err)
	secManifest = updatedManifest

	// apply manifests
	err = secManifest.Apply()
	require.NoError(t, err)
}

func (s *TektonHubTestSuite) undeployExternalDatabase() {
	t := s.T()
	pollInterval := s.interval
	timeout := 3 * time.Minute

	err := resources.DeleteNamespaceAndWait(s.clients.KubeClient, hubExternalDatabaseNamespace, pollInterval, timeout)
	require.NoError(t, err)

	// remove secret from pipelines namespace
	err = s.clients.KubeClient.CoreV1().Secrets(s.resourceNames.TargetNamespace).Delete(context.TODO(), hubDatabaseExternalSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		require.NoError(t, err)
	}
}

// in openshift can not run with fsGroup: 65532 and runAsUser: 65532
// remove those fields from yaml configuration and continue
func (s *TektonHubTestSuite) externalDatabaseTransformer(u *unstructured.Unstructured) error {
	if utils.IsOpenShift() {
		if u.GetKind() == "Deployment" && u.GetName() == "tekton-hub-db" {
			// removes, spec.template.spec.securityContext.fsGroup: 65532
			unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "securityContext", "fsGroup")
			containers, _, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
			if err != nil {
				return err
			}
			for _, c := range containers {
				// removes, spec.template.spec.containers[*].securityContext.runAsUser: 65532
				unstructured.RemoveNestedField(c.(map[string]interface{}), "securityContext", "runAsUser")
			}
			// update containers
			if err := unstructured.SetNestedField(u.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				return err
			}
		} else if utils.IsOpenShift() && u.GetKind() == "Secret" && u.GetNamespace() == "tekton-pipelines" {
			u.SetNamespace(s.resourceNames.TargetNamespace)
		}
	}
	return nil
}
