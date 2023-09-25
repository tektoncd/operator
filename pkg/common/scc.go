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

package common

import (
	"context"
	"fmt"
	"sort"

	securityv1 "github.com/openshift/api/security/v1"
	sccSort "github.com/openshift/apiserver-library-go/pkg/securitycontextconstraints/util/sort"
	security "github.com/openshift/client-go/security/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

func GetSecurityClient(ctx context.Context) *security.Clientset {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		logging.FromContext(ctx).Panic(err)
	}
	securityClient, err := security.NewForConfig(restConfig)
	if err != nil {
		logging.FromContext(ctx).Panic(err)
	}
	return securityClient
}

func VerifySCCExists(ctx context.Context, sccName string, securityClient *security.Clientset) error {
	_, err := securityClient.SecurityV1().SecurityContextConstraints().Get(ctx, sccName, metav1.GetOptions{})
	return err
}

func GetSCCRestrictiveList(ctx context.Context, securityClient *security.Clientset) ([]*securityv1.SecurityContextConstraints, error) {
	logger := logging.FromContext(ctx)
	sccList, err := securityClient.SecurityV1().SecurityContextConstraints().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("Error listing SCCs")
		return nil, err
	}
	var sccPointerList []*securityv1.SecurityContextConstraints
	for i := range sccList.Items {
		sccPointerList = append(sccPointerList, &sccList.Items[i])
	}

	// This will sort the sccPointerList from most restrictive to least restrictive.
	// ByRestrictions implements the sort interface so sort.Sort() can be run on it.
	sort.Sort(sccSort.ByRestrictions(sccPointerList))

	sccLog := "SCCs sorted from most restrictive to least restrictive:"
	for _, sortedSCC := range sccPointerList {
		sccLog = fmt.Sprintf("%s %s", sccLog, sortedSCC.Name)
	}
	logger.Info(sccLog)
	return sccPointerList, nil
}

func SCCAMoreRestrictiveThanB(prioritizedSCCList []*securityv1.SecurityContextConstraints, sccA string, sccB string) (bool, error) {
	var sccAIndex, sccBIndex int
	var sccAFound, sccBFound bool
	for i, scc := range prioritizedSCCList {
		if scc.Name == sccA {
			sccAFound = true
			sccAIndex = i
		}
		if scc.Name == sccB {
			sccBFound = true
			sccBIndex = i
		}
		if sccAFound && sccBFound {
			break
		}
	}

	if !sccAFound || !sccBFound {
		return false, fmt.Errorf("SCCs not found while looking up priorities, found SCC %s: %t, found SCC %s: %t", sccA, sccAFound, sccB, sccBFound)
	}

	return sccAIndex <= sccBIndex, nil
}
