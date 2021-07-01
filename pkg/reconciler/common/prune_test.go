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
	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPrunableNamespaces(t *testing.T) {
	expected := []string{"ns-one", "ns-two"}
	client := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-api-url"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-api"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-two"}},
	)
	prunableNSList, err := GetPrunableNamespaces(client, context.TODO())
	if err != nil {
		assert.Error(t, err, "unable to get ns list")
	}
	assert.Equal(t, fmt.Sprint(expected), fmt.Sprint(prunableNSList))
}

func TestCreateCronJob(t *testing.T) {
	cronName := "resource-pruner"
	resource := []string{"pipelinerun", "taskrun"}
	cronJob := &v1beta1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: "batch/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cronName,
		},
		Spec: v1beta1.CronJobSpec{
			JobTemplate: v1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{},
				},
			},
		},
	}

	nsObj1 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-one"}}

	pru := v1alpha1.Prune{
		Resources: resource,
		Keep:      2,
		Schedule:  "*/5 * * * *",
	}
	client := fake.NewSimpleClientset(cronJob, nsObj1)
	nsList := []string{"ns-one"}
	if err := createCronJob(client, context.TODO(), pru, nsList[0], nsList, metav1.OwnerReference{}, "some-image"); err != nil {
		t.Error("failed creating cronjob")
	}
	cron, err := client.BatchV1beta1().CronJobs(nsList[0]).Get(context.TODO(), cronName, metav1.GetOptions{})
	if err != nil {
		t.Error("failed getting cronjob")
	}
	if cron.Name != cronName {
		t.Error("cronjob not matched")
	}
	jobName := "pruner-tkn-" + nsList[0]
	nameAfterRemovingRand := cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name[:len(jobName)]
	if nameAfterRemovingRand != jobName {
		t.Error("Job Name not matched")
	}
}
