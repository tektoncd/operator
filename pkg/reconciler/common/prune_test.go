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
	"strings"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	scheduleCommon  = "*/2 * * * *"
	scheduleUnique  = "*/4 * * * *"
	scheduleUnique2 = "*/6 * * * *"
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
	expected1 := map[string]struct{}{"ns-one": {}, "ns-two": {}, "ns-three": {}}
	expected2 := map[string]struct{}{"ns-four": {}}

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
	t.Setenv(JobsTKNImageName, "some")

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
		"ns;--keep=2;pipelinerun,taskrun",
		"ns;--keep-since=300;pipelinerun,taskrun",
		"ns;--keep=2 --keep-since=300;pipelinerun,taskrun",
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
		{
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      &keep,
				KeepSince: &keepsince,
				Schedule:  scheduleCommon,
			},
		},
	}

	for i, config := range configs {
		cmd := generatePruneConfigPerNamespace(config, ns)
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
	t.Setenv(JobsTKNImageName, "some")

	err := Prune(context.TODO(), client, config)
	if err != nil {
		assert.Error(t, err, "pruning failed ")
	}
	cronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get ns list")
	}
	// Only one ns with unique schedule than default
	if len(cronjobs.Items) != 2 {
		assert.Error(t, err, "number of cronjobs created is not right")
	}
	expected := map[string]struct{}{
		"ns-two;--keep=3;pipelinerun":                     {},
		"ns-eight;--keep=5;pipelinerun":                   {},
		"ns-four;--keep=3;pipelinerun":                    {},
		"ns-nine;--keep=3;taskrun":                        {},
		"ns-one;--keep=3;pipelinerun":                     {},
		"ns-seven;--keep-since=3200;pipelinerun":          {},
		"ns-eight;--keep=5 --keep-since=3200;pipelinerun": {},
		"ns-ten;--keep=3;taskrun,pipelinerun":             {},
		"ns-thirteen;--keep=50;pipelinerun":               {},
	}
	for _, cronjob := range cronjobs.Items {
		assert.Equal(t, len(cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers), 1)
		for _, container := range cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			args := strings.Split(container.Args[1], " ")
			if len(args) == 1 {
				if _, ok := expected[args[0]]; !ok {
					assert.Error(t, err, "expected args not found")
				}
			}
			for _, arg := range args[1:] {
				if _, ok := expected[arg]; !ok {
					assert.Error(t, errors.New("not found"), "expected command not found")
				}
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
	t.Setenv(JobsTKNImageName, "some")

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

// test the cronjob creation when NodeSelector and Toleration changes
func TestNodeSelectorOrTolerationsChange(t *testing.T) {
	keep := uint(3)
	annoUniqueSchedule := map[string]string{pruneSchedule: scheduleUnique}
	annoUniqueSchedule2 := map[string]string{pruneSchedule: scheduleUnique2}
	defaultPrune := &v1alpha1.Prune{
		Resources: []string{"pipelinerun"},
		Keep:      &keep,
		Schedule:  scheduleCommon,
	}
	tektonConfigSpecConfig := v1alpha1.Config{
		NodeSelector: map[string]string{
			"foo": "bar",
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "foo",
				Operator: "equals",
				Value:    "bar",
				Effect:   "noSchedule",
			},
		},
	}
	tektonConfigSpecConfigEmpty := v1alpha1.Config{}

	config1 := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    "all",
			Pruner:     *defaultPrune,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
			Config:     tektonConfigSpecConfig,
		},
	}

	config2 := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile:    "all",
			Pruner:     *defaultPrune,
			CommonSpec: v1alpha1.CommonSpec{TargetNamespace: "openshift-pipelines"},
			Config:     tektonConfigSpecConfigEmpty,
		},
	}
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-pipelines"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two", Annotations: annoUniqueSchedule}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-three", Annotations: annoUniqueSchedule2}},
	)
	t.Setenv(JobsTKNImageName, "some")

	// Pruning with the nodes and Tolerations
	err := Prune(context.TODO(), client, config1)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}

	oldCronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get cronjobs ")
	}
	oldCronList := []string{}
	for _, cronjob := range oldCronjobs.Items {
		oldCronList = append(oldCronList, cronjob.Name)
	}

	// Pruning after removing nodes and Tolerations
	err = Prune(context.TODO(), client, config2)
	if err != nil {
		assert.Error(t, err, "unable to initiate prune")
	}

	newCronjobs, err := client.BatchV1().CronJobs("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		assert.Error(t, err, "unable to get cronjobs ")
	}
	newCronList := []string{}
	for _, cronjob := range newCronjobs.Items {
		newCronList = append(newCronList, cronjob.Name)
	}
	assert.Assert(t, len(oldCronList) == len(newCronList), "Number of cronJobs should be same after nodeselector and Toleration config change")
	// checking if config change recreated all the cron Jobs
	cronjobChanged := true
	for _, oldCronName := range oldCronList {
		for _, newCronName := range newCronList {
			if oldCronName == newCronName {
				cronjobChanged = false
				break
			}
		}
		assert.Assert(t, cronjobChanged, "nodeselector and Toleration Config change should recreate all the cron Jobs")
	}

}

func TestGetJobContainer(t *testing.T) {
	keep := uint(2)
	keepsince := uint(300)
	configs := map[string]*pruneConfigPerNS{
		"ns-one": {
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      &keep,
				KeepSince: nil,
				Schedule:  scheduleCommon,
			},
		},
		"ns-two": {
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      nil,
				KeepSince: &keepsince,
				Schedule:  scheduleCommon,
			},
		},
		"ns-three": {
			config: v1alpha1.Prune{
				Resources: []string{"pipelinerun", "taskrun"},
				Keep:      &keep,
				KeepSince: &keepsince,
				Schedule:  scheduleCommon,
			},
		},
	}

	expected := " ns-one;--keep=2;pipelinerun,taskrun ns-three;--keep=2 --keep-since=300;pipelinerun,taskrun ns-two;--keep-since=300;pipelinerun,taskrun"

	container := getJobContainer(generateAllPruneConfig(configs), "ns", "test-image")
	assert.Equal(t, container[0].Args[1], expected)
}
