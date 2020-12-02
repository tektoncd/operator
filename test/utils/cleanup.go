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
	"os"
	"os/signal"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	if clients != nil && clients.Operator != nil {
		_ = clients.TektonPipeline().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// TearDownTrigger will delete created TektonTrigger CRs using clients.
func TearDownTrigger(clients *Clients, name string) {
	if clients != nil && clients.Operator != nil {
		_ = clients.TektonTrigger().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// TearDownDashboard will delete created TektonDashboard CRs using clients.
func TearDownDashboard(clients *Clients, name string) {
	if clients != nil && clients.Operator != nil {
		_ = clients.TektonDashboard().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// TearDownAddon will delete created TektonAddon CRs using clients.
func TearDownAddon(clients *Clients, name string) {
	if clients != nil && clients.Operator != nil {
		_ = clients.TektonAddon().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// TearDownNamespace will delete created test Namespace
func TearDownNamespace(clients *Clients, name string) {
	if clients != nil && clients.KubeClient != nil {
		_ = clients.KubeClient.Kube.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

// TearDownConfig will delete created TektonConfig CRs using clients.
func TearDownConfig(clients *Clients, name string) {
	if clients != nil && clients.Operator != nil {
		_ = clients.TektonConfig().Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}
