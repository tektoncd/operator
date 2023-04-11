/*
Copyright 2023 The Tekton Authors

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

package resources

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/tektoncd/operator/test/utils"
	"go.uber.org/zap"
)

const (
	debugNamespacedResources = "deployment,pod,configmap,serviceaccount,role,rolebinding"
)

var (
	printClusterInformation sync.Once
)

func getDebugCommands(resourceNames utils.ResourceNames) []string {
	operatorNamespace := resourceNames.Namespace
	pipelinesNamespace := resourceNames.TargetNamespace

	commands := []string{
		"kubectl get tektonconfig",
		"kubectl get tektoninstallerset",
		"kubectl get tektonpipeline",
		"kubectl get tektonchain",
		"kubectl get tektontrigger",
		"kubectl get tektonhub",
		"kubectl get tektondashboard",
		fmt.Sprintf("kubectl get %s --output=wide --namespace=%s", debugNamespacedResources, pipelinesNamespace),
		fmt.Sprintf("kubectl get %s --output=wide --namespace=%s", debugNamespacedResources, operatorNamespace),
		fmt.Sprintf("kubectl get %s --output=wide --namespace=hub-external-db", debugNamespacedResources),
	}
	if utils.IsOpenShift() {
		openshiftCommands := []string{
			fmt.Sprintf("kubectl get ClusterServiceVersion --namespace=%s", operatorNamespace),
		}
		commands = append(commands, openshiftCommands...)
	}

	return commands
}

func ExecuteDebugCommands(logger *zap.SugaredLogger, resourceNames utils.ResourceNames) {
	commands := getDebugCommands(resourceNames)
	RunCommand(logger, "------------------- debug information -------------------", commands)
}

func PrintClusterInformation(logger *zap.SugaredLogger, resourceNames utils.ResourceNames) {
	printClusterInformation.Do(func() {
		commands := []string{
			"kubectl version --output=yaml",
			"kubectl get nodes --output=wide",
		}
		platform := "kubernetes"
		if utils.IsOpenShift() {
			platform = "OpenShift"
			openshiftCommands := []string{
				fmt.Sprintf("kubectl get ClusterServiceVersion --namespace=%s", resourceNames.Namespace),
			}
			commands = append(commands, openshiftCommands...)
		}
		title := fmt.Sprintf("------------- cluster version information (%s) ---------------", platform)
		RunCommand(logger, title, commands)
	})
}

func RunCommand(logger *zap.SugaredLogger, title string, commands []string) {
	stdout := os.Stdout
	stderr := os.Stderr
	fmt.Fprintf(stdout, "\n%s\n", title)
	for index, command := range commands {
		commandWithArgs := strings.Split(command, " ")
		cmd := exec.Command(commandWithArgs[0], commandWithArgs[1:]...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		fmt.Fprintf(stdout, "$ %s\n", command)
		err := cmd.Run()
		if err != nil {
			logger.Errorw("error on executing a command",
				"command", command,
				err,
			)
		}
		fmt.Fprintln(stdout)
		// do not print extra newline for last command
		if index != (len(commands) - 1) {
			fmt.Fprintln(stdout)
		}
	}
	fmt.Fprintf(stdout, "--------------------------- end -------------------------\n\n")
}
