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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeOperatorV1alpha1 struct {
	*testing.Fake
}

func (c *FakeOperatorV1alpha1) ManualApprovalGates() v1alpha1.ManualApprovalGateInterface {
	return newFakeManualApprovalGates(c)
}

func (c *FakeOperatorV1alpha1) OpenShiftPipelinesAsCodes() v1alpha1.OpenShiftPipelinesAsCodeInterface {
	return newFakeOpenShiftPipelinesAsCodes(c)
}

func (c *FakeOperatorV1alpha1) TektonAddons() v1alpha1.TektonAddonInterface {
	return newFakeTektonAddons(c)
}

func (c *FakeOperatorV1alpha1) TektonChains() v1alpha1.TektonChainInterface {
	return newFakeTektonChains(c)
}

func (c *FakeOperatorV1alpha1) TektonConfigs() v1alpha1.TektonConfigInterface {
	return newFakeTektonConfigs(c)
}

func (c *FakeOperatorV1alpha1) TektonDashboards() v1alpha1.TektonDashboardInterface {
	return newFakeTektonDashboards(c)
}

func (c *FakeOperatorV1alpha1) TektonHubs() v1alpha1.TektonHubInterface {
	return newFakeTektonHubs(c)
}

func (c *FakeOperatorV1alpha1) TektonInstallerSets() v1alpha1.TektonInstallerSetInterface {
	return newFakeTektonInstallerSets(c)
}

func (c *FakeOperatorV1alpha1) TektonPipelines() v1alpha1.TektonPipelineInterface {
	return newFakeTektonPipelines(c)
}

func (c *FakeOperatorV1alpha1) TektonPruners() v1alpha1.TektonPrunerInterface {
	return newFakeTektonPruners(c)
}

func (c *FakeOperatorV1alpha1) TektonResults() v1alpha1.TektonResultInterface {
	return newFakeTektonResults(c)
}

func (c *FakeOperatorV1alpha1) TektonTriggers() v1alpha1.TektonTriggerInterface {
	return newFakeTektonTriggers(c)
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeOperatorV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
