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

package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	scheduleCommon = "*/2 * * * *"
	scheduleUnique = "*/4 * * * *"
)

func TestGetPrunableNamespaces(t *testing.T) {
	keep := uint(3)
	anno1 := map[string]string{pruneSchedule: scheduleCommon}
	anno2 := map[string]string{pruneSchedule: scheduleUnique}
	defaultPrune := v1alpha1.Prune{
		Resources: []string{"something"},
		Keep:      &keep,
		Schedule:  scheduleCommon,
	}
	expected1 := map[string]struct{}{"ns-one": struct{}{}, "ns-two": struct{}{}, "ns-three": struct{}{}}
	expected2 := map[string]struct{}{"ns-four": struct{}{}}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-three", Annotations: anno1}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-four", Annotations: anno2}},
	)
	pruningNamespaces, err := prunableNamespaces(context.TODO(), client, defaultPrune)
	if err != nil {
		assert.Error(t, err, "unable to get ns list")
	}
	assert.Equal(t, len(expected1), len(pruningNamespaces.commonScheduleNs))
	for ns := range pruningNamespaces.commonScheduleNs {
		if _, ok := expected1[ns]; !ok {
			assert.Error(t, errors.New("namespace not found"), ns)
		}
	}
	assert.Equal(t, len(expected2), len(pruningNamespaces.uniqueScheduleNS))
	for ns := range pruningNamespaces.uniqueScheduleNS {
		if _, ok := expected2[ns]; !ok {
			assert.Error(t, errors.New("namespace not found"), ns)
		}
	}
}

func TestCompleteFlowPrune(t *testing.T) {

	keep := uint(3)
	anno1 := map[string]string{pruneSchedule: scheduleCommon}
	anno2 := map[string]string{pruneSchedule: scheduleUnique}
	anno4 := map[string]string{pruneSchedule: scheduleCommon}

	defaultPrune := &v1alpha1.Prune{
		Resources: []string{"pipelinerun"},
		Keep:      &keep,
		Schedule:  scheduleCommon,
	}
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    "all",
			Pruner:     *defaultPrune,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
		},
	}
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-pipelines"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two", Annotations: anno2}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-three", Annotations: anno1}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-four", Annotations: anno4}},
	)
	os.Setenv(JobsTKNImageName, "some")

	err := Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}
	cronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get cronjobs ")
	}
	// Only one ns with unique schedule than default
	if len(cronjobs.Items) != 2 {
		assert.Error(t, err, "number of cronjobs not correct")
	}
	if _, err := client.CoreV1().Namespaces().Update(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two"}}, metav1.UpdateOptions{}); err != nil {
		assert.Error(t, err, "unexpected error")
	}

	err = Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}
	cronjobs1, err := client.BatchV1().CronJobs("openshift-pipelines").List(context.TODO(), metav1.ListOptions{})
	if len(cronjobs1.Items) != 1 {
		assert.Error(t, err, "number of cronjobs not correct")
	}
}

func TestPruneCommands(t *testing.T) {
	keep := uint(2)
	keepsince := uint(300)
	expected := []string{
		"tkn pipelinerun delete --keep=2 -n=ns -f ; tkn taskrun delete --keep=2 -n=ns -f ; ",
		"tkn pipelinerun delete --keep-since=300 -n=ns -f ; tkn taskrun delete --keep-since=300 -n=ns -f ; ",
	}
	ns := "ns"
	configs := []*pruneConfigPerNS{
		{
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      &keep,
				KeepSince: nil,
				Schedule:  scheduleCommon,
			},
		},
		{
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      nil,
				KeepSince: &keepsince,
				Schedule:  scheduleCommon,
			},
		},
	}

	for i, config := range configs {
		cmd := pruneCommand(config, ns)
		assert.Equal(t, cmd, expected[i])
	}
}

func TestAnnotationCmd(t *testing.T) {
	keep := uint(3)
	annoUniqueSchedule := map[string]string{pruneSchedule: scheduleUnique}
	annoCommonSchedule := map[string]string{pruneSchedule: scheduleCommon}
	annoStrategyKeepSince := map[string]string{pruneStrategy: "keep-since", pruneKeepSince: "3200"}
	annoStrategyKeep := map[string]string{pruneStrategy: "keep", pruneKeep: "50"}
	annoKeepSinceAndKeep := map[string]string{pruneKeepSince: "3200", pruneKeep: "5"}
	annoResourceTr := map[string]string{pruneResources: "taskrun"}
	annoResourceTrPr := map[string]string{pruneResources: "taskrun, pipelinerun"}
	annoSkip := map[string]string{pruneSkip: "true"}
	defaultPrune := &v1alpha1.Prune{
		Resources: []string{"pipelinerun"},
		Keep:      &keep,
		Schedule:  scheduleCommon,
	}
	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    "all",
			Pruner:     *defaultPrune,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
		},
	}
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two", Annotations: annoUniqueSchedule}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-four", Annotations: annoCommonSchedule}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-six", Annotations: annoSkip}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-seven", Annotations: annoStrategyKeepSince}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-eight", Annotations: annoKeepSinceAndKeep}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-nine", Annotations: annoResourceTr}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-ten", Annotations: annoResourceTrPr}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-thirteen", Annotations: annoStrategyKeep}},
	)
	expected := map[string]string{
		"one-" + scheduleCommon:        "tkn pipelinerun delete --keep=3 -n=ns-one -f ; ",
		"two-" + scheduleUnique:        "tkn pipelinerun delete --keep=3 -n=ns-two -f ; ",
		"four-" + scheduleCommon:       "tkn pipelinerun delete --keep=3 -n=ns-four -f ; ",
		"seven-" + scheduleCommon:      "tkn pipelinerun delete --keep-since=3200 -n=ns-seven -f ; ",
		"eight-" + scheduleCommon:      "tkn pipelinerun delete --keep=5 -n=ns-eight -f ; ",
		"nine-" + scheduleCommon:       "tkn taskrun delete --keep=3 -n=ns-nine -f ; ",
		"ten-" + scheduleCommon:        "tkn taskrun delete --keep=3 -n=ns-ten -f ; tkn pipelinerun delete --keep=3 -n=ns-ten -f ; ",
		"ns-thirteen" + scheduleCommon: "tkn pipelinerun delete --keep=50 -n=ns-thirteen -f ; ",
	}
	os.Setenv(JobsTKNImageName, "some")

	err := Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to get ns list")
	}
	cronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get ns list")
	}
	// Only one ns with unique schedule than default
	if len(cronjobs.Items) != 2 {
		assert.Error(t, err, "unable to get ns list")
	}
	for _, cronjob := range cronjobs.Items {
		for _, container := range cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			if _, ok := expected[container.Name[14:len(container.Name)-5]+cronjob.Spec.Schedule]; ok {
				if expected[container.Name[14:len(container.Name)-5]+cronjob.Spec.Schedule] != strings.Join(container.Args, " ") {
					msg := fmt.Sprintf("expected : %s\n actual : %s \n", expected[container.Name[14:len(container.Name)-5]+cronjob.Spec.Schedule], strings.Join(container.Args, " "))
					assert.Error(t, errors.New("command created is not as expected"), msg)
				}
			}
			if container.Name == "ns-six" {
				assert.Error(t, errors.New("Should not be created as Ns have skip prune annotation"), "")
			}
		}
	}
}

// test the cronjob creation with only relevant config change
func TestConfigChange(t *testing.T) {
	keep := uint(3)
	anno1 := map[string]string{pruneKeep: "200"}

	defaultPrune := &v1alpha1.Prune{
		Resources: []string{"pipelinerun"},
		Keep:      &keep,
		Schedule:  scheduleCommon,
	}

	config := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    "all",
			Pruner:     *defaultPrune,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
		},
	}

	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-pipelines"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one", Annotations: anno1}},
	)
	os.Setenv(JobsTKNImageName, "some")

	err := Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}
	cronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get cronjobs ")
	}
	oldCronName := cronjobs.Items[0].Name
	// changes unrelated to prune config, should not give new cron
	if _, err := client.CoreV1().Namespaces().Update(context.TODO(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one",
			Annotations: map[string]string{pruneKeep: "200", "garbage": "annotation",
				pruneLastAppliedHash: "dced6f128ff38cd2686fcc2ddc3f38ff9c45f4ea3fddad0f99e722659ca57694"}}}, metav1.UpdateOptions{}); err != nil {
		assert.Error(t, err, "unexpected error")
	}

	err = Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}
	cronjobUnchanged, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if cronjobUnchanged.Items[0].Name != oldCronName {
		assert.Error(t, err, "cronjob was recreated")
	}

	// changes related to prune config, should give new cron
	if _, err := client.CoreV1().Namespaces().Update(context.TODO(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one",
			Annotations: map[string]string{pruneKeep: "200", pruneResources: "pipelinerun, taskrun",
				pruneLastAppliedHash: "dced6f128ff38cd2686fcc2ddc3f38ff9c45f4ea3fddad0f99e722659ca57694"}}}, metav1.UpdateOptions{}); err != nil {
		assert.Error(t, err, "unexpected error")
	}

	err = Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}
	cronjobChanged, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if oldCronName == cronjobChanged.Items[0].Name {
		assert.Error(t, err, "a new cronjob expected")
	}
}
