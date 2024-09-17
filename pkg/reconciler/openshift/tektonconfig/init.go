/*
Copyright 2024 The Tekton Authors

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

package tektonconfig

import (
	"context"
	"os"
	"strconv"

	"github.com/tektoncd/operator/pkg/reconciler/openshift"
	"knative.dev/pkg/logging"
)

const (
	defaultRbacMaxConcurrentCalls = 20
	minRbacMaxConcurrentCalls     = 1
	maxRbacMaxConcurrentCalls     = 50
)

var rbacMaxConcurrentCalls int

func init() {
	rbacMaxConcurrentCalls = loadRbacMaxConcurrentCalls()
}

func loadRbacMaxConcurrentCalls() int {
	logger := logging.FromContext(context.TODO())
	envValue := os.Getenv(openshift.RbacProvisioningMaxConcurrentCalls)
	if envValue == "" {
		return defaultRbacMaxConcurrentCalls
	}
	parsedValue, err := strconv.Atoi(envValue)
	if err != nil {
		logger.Infof("Failed to parse %s, setting to default: %d", openshift.RbacProvisioningMaxConcurrentCalls, defaultRbacMaxConcurrentCalls)
		return defaultRbacMaxConcurrentCalls
	}
	if parsedValue < minRbacMaxConcurrentCalls || parsedValue > maxRbacMaxConcurrentCalls {
		logger.Infof("Invalid value %d for %s. Valid range is [%d, %d]. Setting to default: %d",
			parsedValue,
			openshift.RbacProvisioningMaxConcurrentCalls,
			minRbacMaxConcurrentCalls,
			maxRbacMaxConcurrentCalls,
			defaultRbacMaxConcurrentCalls)
		return defaultRbacMaxConcurrentCalls
	}

	return parsedValue
}

func init() {
	rbacMaxConcurrentCalls = loadRbacMaxConcurrentCalls()
}

func getRBACMaxCalls() int {
	return rbacMaxConcurrentCalls
}
