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
	"os"
	"regexp"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	tektonSA         = "tekton-pipelines-controller"
	CronName         = "resource-pruner"
	JobsTKNImageName = "IMAGE_JOB_PRUNER_TKN"
	ownerAPIVer      = "operator.tekton.dev/v1alpha1"
	ownerKind        = "TektonConfig"
)

func Prune(k kubernetes.Interface, ctx context.Context, tC *v1alpha1.TektonConfig) error {

	if len(tC.Spec.Pruner.Resources) == 0 || tC.Spec.Pruner.Schedule == "" {
		return checkAndDelete(k, ctx, tC.Spec.TargetNamespace)
	}

	tknImage := os.Getenv(JobsTKNImageName)
	if tknImage == "" {
		return fmt.Errorf("%s environment variable not found", JobsTKNImageName)
	}
	pru := tC.Spec.Pruner
	logger := logging.FromContext(ctx)
	ownerRef := v1.OwnerReference{
		APIVersion: ownerAPIVer,
		Kind:       ownerKind,
		Name:       tC.Name,
		UID:        tC.ObjectMeta.UID,
	}

	pruningNamespaces, err := GetPrunableNamespaces(k, ctx)
	if err != nil {
		return err
	}
	if err := createCronJob(k, ctx, pru, tC.Spec.TargetNamespace, pruningNamespaces, ownerRef, tknImage); err != nil {
		logger.Error("failed to create cronjob ", err)

	}

	return nil
}

func GetPrunableNamespaces(k kubernetes.Interface, ctx context.Context) ([]string, error) {
	nsList, err := k.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var allNameSpaces []string
	re := regexp.MustCompile(NamespaceIgnorePattern)
	for _, ns := range nsList.Items {
		if ignore := re.MatchString(ns.GetName()); ignore {
			continue
		}
		allNameSpaces = append(allNameSpaces, ns.Name)
	}
	return allNameSpaces, nil
}

func createCronJob(k kubernetes.Interface, ctx context.Context, pru v1alpha1.Prune, targetNs string, pruningNs []string, oR v1.OwnerReference, tknImage string) error {
	pruneContainers := getPruningContainers(pru.Resources, pruningNs, *pru.Keep, tknImage)
	backOffLimit := int32(3)
	ttlSecondsAfterFinished := int32(3600)
	cj := &v1beta1.CronJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: "batch/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:            CronName,
			OwnerReferences: []v1.OwnerReference{oR},
		},
		Spec: v1beta1.CronJobSpec{
			Schedule:          pru.Schedule,
			ConcurrencyPolicy: "Forbid",
			JobTemplate: v1beta1.JobTemplateSpec{

				Spec: batchv1.JobSpec{
					TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
					BackoffLimit:            &backOffLimit,

					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:         pruneContainers,
							RestartPolicy:      "OnFailure",
							ServiceAccountName: tektonSA,
						},
					},
				},
			},
		},
	}

	if _, err := k.BatchV1beta1().CronJobs(targetNs).Create(ctx, cj, v1.CreateOptions{}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			if _, err := k.BatchV1beta1().CronJobs(targetNs).Update(ctx, cj, v1.UpdateOptions{}); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}

func getPruningContainers(resources, namespaces []string, keep uint, tknImage string) []corev1.Container {
	containers := []corev1.Container{}
	for _, ns := range namespaces {
		cmdArgs := deleteCommand(resources, keep, ns)
		jobName := SimpleNameGenerator.RestrictLengthWithRandomSuffix("pruner-tkn-" + ns)
		container := corev1.Container{
			Name:                     jobName,
			Image:                    tknImage,
			Command:                  []string{"/bin/sh", "-c"},
			Args:                     []string{cmdArgs},
			TerminationMessagePolicy: "FallbackToLogsOnError",
		}
		containers = append(containers, container)
	}

	return containers
}

func deleteCommand(resources []string, keep uint, ns string) string {
	var cmdArgs string
	for _, res := range resources {
		cmd := "tkn " + strings.ToLower(res) + " delete --keep=" + fmt.Sprint(keep) + " -n " + ns + " -f ; "
		cmdArgs = cmdArgs + cmd
	}
	return cmdArgs
}

func checkAndDelete(k kubernetes.Interface, ctx context.Context, targetNamespace string) error {
	if _, err := k.BatchV1beta1().CronJobs(targetNamespace).Get(ctx, CronName, v1.GetOptions{}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		} else {
			return err
		}
	}

	//if there is no error it means cron is exists, but no prune in config it means delete it
	return k.BatchV1beta1().CronJobs(targetNamespace).Delete(ctx, CronName, v1.DeleteOptions{})
}
