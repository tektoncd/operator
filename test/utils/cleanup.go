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

package utils

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type crDeleteVerifier wait.ConditionFunc
type deploymentDeleteVerifier wait.ConditionFunc
type crGetFunc func(ctx context.Context) error

// CleanupOnInterrupt will execute the function cleanup if an interrupt signal is caught
func CleanupOnInterrupt(cleanup func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			cleanup()
			os.Exit(1)
		}
	}()
}

// TearDownPipeline will delete created TektonPipeline CRs using clients.
func TearDownPipeline(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonPipeline().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonPipeline instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonPipeline().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonPipeline during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonPipeline().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonPipelineDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonPipeline resource, name: %s, error: %v", name, err)
	}
}

// TearDownTrigger will delete created TektonTrigger CRs using clients.
func TearDownTrigger(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonTrigger().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonTrigger instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonTrigger().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonTrigger during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonTrigger().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonTriggerDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonTrigger resource, name: %s, error: %v", name, err)
	}
}

// TearDownDashboard will delete created TektonDashboard CRs using clients.
func TearDownDashboard(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonDashboard().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonDashboard instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonDashboard().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonDashboard during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonDashboard().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonDashboardDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonDashboard resource, name: %s, error: %v", name, err)
	}
}

// TearDownAddon will delete created TektonAddon CRs using clients.
func TearDownAddon(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonAddon().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonAddon instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonAddon().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonAddon during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonAddon().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonAddonDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonAddon resource, name: %s, error: %v", name, err)
	}

}

// TearDownNamespace will delete created test Namespace
func TearDownNamespace(clients *Clients, name string) {
	ctx := context.Background()

	if clients == nil || clients.Operator == nil {
		return
	}

	_, err := clients.KubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get Namespace instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}

	err = clients.KubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete Namespace during teardown, name: %s, error: %v", name, err)
		return
	}

	err = WaitForCondition(ctx, func() (bool, error) {
		_, err := clients.KubeClient.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})

	if err != nil {
		fmt.Printf("error waiting from tearDown of Namespace resource, name: %s, error: %v", name, err)
	}
}

// TearDownConfig will delete created TektonConfig CRs using clients.
func TearDownConfig(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonConfig().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonConfig instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonConfig().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonConfig during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonConfig().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, "")
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonConfig resource, name: %s, error: %v", name, err)
	}
}

// TearDownResult will delete created TektonResult CRs using clients.
func TearDownResult(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonResult().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonResult instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonResult().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonResult during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonResult().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonResultsDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonResult resource, name: %s, error: %v", name, err)
	}
}

// TearDownChain will delete created TektonChain CRs using clients.
func TearDownChain(clients *Clients, name string) {
	ctx := context.Background()
	if clients == nil || clients.Operator == nil {
		return
	}

	tc, err := clients.TektonChains().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("error trying to get TektonChains instance during teardown, name: %s, error: %v", name, err)
		}
		return
	}
	targetNamespace := tc.Spec.TargetNamespace

	err = clients.TektonChains().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf("error trying to delete TektonChains during teardown, name: %s, error: %v", name, err)
		return
	}
	crdf := newCRDeleteVerifier(ctx, func(ctx context.Context) error {
		_, err := clients.TektonChains().Get(ctx, name, metav1.GetOptions{})
		return err
	})
	ddf := newDeploymentDeleteVerifier(ctx, clients, targetNamespace, TektonChainDeploymentLabel)
	err = waitUntilFullDeletion(ctx, crdf, ddf)
	if err != nil {
		fmt.Printf("error waiting from tearDown of TektonChains resource, name: %s, error: %v", name, err)
	}
}

func newCRDeleteVerifier(ctx context.Context, f crGetFunc) crDeleteVerifier {
	return func() (bool, error) {
		err := f(ctx)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}
}

func newDeploymentDeleteVerifier(ctx context.Context, c *Clients, namespace, labelSelector string) deploymentDeleteVerifier {
	return func() (bool, error) {
		deployemnts, err := c.KubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})

		if err != nil {
			return false, err
		}
		if len(deployemnts.Items) == 0 {
			return true, nil
		}
		return false, nil
	}
}

func waitUntilFullDeletion(ctx context.Context, cdcf crDeleteVerifier, ddcf deploymentDeleteVerifier) error {
	if err := ensureDeploymentsRemoval(ctx, ddcf); err != nil {
		return err
	}
	if err := ensureCustomResourceRemoval(ctx, cdcf); err != nil {
		return err
	}
	return nil
}

func ensureCustomResourceRemoval(ctx context.Context, verifier crDeleteVerifier) error {
	return WaitForCondition(ctx, wait.ConditionFunc(verifier))
}

func ensureDeploymentsRemoval(ctx context.Context, verifier deploymentDeleteVerifier) error {
	return WaitForCondition(ctx, wait.ConditionFunc(verifier))
}

func WaitForCondition(ctx context.Context, condition wait.ConditionFunc) error {
	return wait.PollImmediate(Interval, Timeout, func() (done bool, err error) {
		ok, err := condition()
		if err != nil {
			return false, err
		}
		return ok, nil
	})
}
