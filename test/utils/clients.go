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

// This file contains an object which encapsulates k8s clients which are useful for e2e tests.

package utils

import (
	operatorVersioned "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	pipelineVersioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
)

// Clients holds instances of interfaces for making requests to Tekton Pipelines.
type Clients struct {
	KubeClient    kubernetes.Interface
	Dynamic       dynamic.Interface
	Operator      operatorv1alpha1.OperatorV1alpha1Interface
	TektonClient  v1beta1.TektonV1beta1Interface
	Config        *rest.Config
	KubeClientSet *kubernetes.Clientset
}

// NewClients instantiates and returns several clientsets required for making request to the
// TektonPipeline cluster specified by the combination of clusterName and configPath.
func NewClients(configPath string, clusterName string, namespace string) (*Clients, error) {
	clients := &Clients{}
	cfg, err := buildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	// We poll, so set our limits high.
	cfg.QPS = 100
	cfg.Burst = 200

	clients.KubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.Operator, err = newTektonOperatorAlphaClients(cfg)
	if err != nil {
		return nil, err
	}

	clients.KubeClientSet, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	clients.TektonClient, err = newTektonBetaClients(cfg)
	if err != nil {
		return nil, err
	}

	clients.Config = cfg
	return clients, nil
}

func buildClientConfig(kubeConfigPath string, clusterName string) (*rest.Config, error) {
	overrides := clientcmd.ConfigOverrides{}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&overrides).ClientConfig()
}

func newTektonOperatorAlphaClients(cfg *rest.Config) (operatorv1alpha1.OperatorV1alpha1Interface, error) {
	cs, err := operatorVersioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cs.OperatorV1alpha1(), nil
}

func newTektonBetaClients(cfg *rest.Config) (v1beta1.TektonV1beta1Interface, error) {
	cs, err := pipelineVersioned.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return cs.TektonV1beta1(), nil
}

func (c *Clients) TektonPipeline() operatorv1alpha1.TektonPipelineInterface {
	return c.Operator.TektonPipelines()
}

func (c *Clients) TektonPipelineAll() operatorv1alpha1.TektonPipelineInterface {
	return c.Operator.TektonPipelines()
}

func (c *Clients) TektonTrigger() operatorv1alpha1.TektonTriggerInterface {
	return c.Operator.TektonTriggers()
}

func (c *Clients) TektonTriggerAll() operatorv1alpha1.TektonTriggerInterface {
	return c.Operator.TektonTriggers()
}

func (c *Clients) TektonDashboard() operatorv1alpha1.TektonDashboardInterface {
	return c.Operator.TektonDashboards()
}

func (c *Clients) TektonDashboardAll() operatorv1alpha1.TektonDashboardInterface {
	return c.Operator.TektonDashboards()
}
func (c *Clients) TektonAddon() operatorv1alpha1.TektonAddonInterface {
	return c.Operator.TektonAddons()
}

func (c *Clients) TektonAddonAll() operatorv1alpha1.TektonAddonInterface {
	return c.Operator.TektonAddons()
}

func (c *Clients) TektonConfig() operatorv1alpha1.TektonConfigInterface {
	return c.Operator.TektonConfigs()
}

func (c *Clients) TektonConfigAll() operatorv1alpha1.TektonConfigInterface {
	return c.Operator.TektonConfigs()
}

func (c *Clients) TektonResult() operatorv1alpha1.TektonResultInterface {
	return c.Operator.TektonResults()
}

func (c *Clients) TektonResultAll() operatorv1alpha1.TektonResultInterface {
	return c.Operator.TektonResults()
}

func (c *Clients) TektonChains() operatorv1alpha1.TektonChainInterface {
	return c.Operator.TektonChains()
}

func (c *Clients) TektonChainsAll() operatorv1alpha1.TektonChainInterface {
	return c.Operator.TektonChains()
}

func (c *Clients) TektonHub() operatorv1alpha1.TektonHubInterface {
	return c.Operator.TektonHubs()
}

func (c *Clients) TektonHubAll() operatorv1alpha1.TektonHubInterface {
	return c.Operator.TektonHubs()
}

func (c *Clients) TektonInstallerSet() operatorv1alpha1.TektonInstallerSetInterface {
	return c.Operator.TektonInstallerSets()
}

func (c *Clients) TektonInstallerSetAll() operatorv1alpha1.TektonInstallerSetInterface {
	return c.Operator.TektonInstallerSets()
}
