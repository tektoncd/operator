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
	"strconv"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	tektonSA         = "tekton-pipelines-controller"
	CronName         = "tekton-resource-pruner"
	JobsTKNImageName = "IMAGE_JOB_PRUNER_TKN"
	pruneSkip        = "operator.tekton.dev/prune.skip"
	pruneResources   = "operator.tekton.dev/prune.resources"
	pruneStrategy    = "operator.tekton.dev/prune.strategy"
	pruneKeep        = "operator.tekton.dev/prune.keep"
	pruneKeepSince   = "operator.tekton.dev/prune.keep-since"
	pruneSchedule    = "operator.tekton.dev/prune.schedule"
	pruneCronLabel   = "tektonconfig.operator.tekton.dev/pruner"
	keep             = "keep"
	keepSince        = "keep-since"
)

type Pruner struct {
	kc       kubernetes.Interface
	tknImage string
}

type pruneConfigPerNS struct {
	config v1alpha1.Prune
}

// all the namespaces of default and the annotationbased
type pruningNs struct {
	commonScheduleNs map[string]*pruneConfigPerNS
	uniqueScheduleNS map[string]*pruneConfigPerNS
}

func Prune(ctx context.Context, k kubernetes.Interface, tC *v1alpha1.TektonConfig) error {
	pruner := &Pruner{kc: k}

	//to remove cronjob created by older version of operator.
	oldCronName := CronName[7:] + "-" + tC.Spec.TargetNamespace
	if err := pruner.checkAndDeleteCron(ctx, oldCronName, tC.Spec.TargetNamespace); err != nil {
		return err
	}

	//may be reconciled by removing pruning spec from tektonConfig
	if pruner.removedFromTektonConfig(tC.Spec.Pruner) {
		return pruner.checkAndDeleteCron(ctx, CronName, tC.Spec.TargetNamespace)
	}

	//may be reconciled by removing/adding annotation on a Namespace
	// if schedule is removed or prune.skip is added we delete pre-existing cron.
	annotationRemovedNamespaces, err := pruner.checkAnnotationsRemovedNamespaces(ctx, tC.Spec.TargetNamespace)
	if err != nil {
		return err
	}
	if len(annotationRemovedNamespaces) > 0 {
		for _, ns := range annotationRemovedNamespaces {
			if err := pruner.checkAndDeleteCron(ctx, CronName, ns); err != nil {
				return err
			}
		}
	}

	tknImageFromEnv := os.Getenv(JobsTKNImageName)
	if tknImageFromEnv == "" {
		return fmt.Errorf("%s environment variable not found", JobsTKNImageName)
	}
	pruner.tknImage = tknImageFromEnv
	logger := logging.FromContext(ctx)
	ownerRef := *v1.NewControllerRef(tC, tC.GetGroupVersionKind())

	// for the default config from the tektonconfig
	pruningNamespaces, err := prunableNamespaces(ctx, k, tC.Spec.Pruner)
	if err != nil {
		return err
	}

	if pruningNamespaces.commonScheduleNs != nil && len(pruningNamespaces.commonScheduleNs) > 0 {
		jobs := pruner.createAllJobContainers(pruningNamespaces.commonScheduleNs)

		if err := pruner.createCronJob(ctx, tC.Spec.TargetNamespace, tC.Spec.Pruner.Schedule, jobs, ownerRef); err != nil {
			logger.Error("failed to create cronjob ", err)
		}
	}

	if pruningNamespaces.uniqueScheduleNS != nil {
		for ns, con := range pruningNamespaces.uniqueScheduleNS {
			jobs := pruner.createJobContainers(con, ns)

			if err := pruner.createCronJob(ctx, ns, con.config.Schedule, jobs, ownerRef); err != nil {
				logger.Errorf("failed to create cronjob in ns %s:", ns, err)
			}
		}
	}

	return nil
}

func prunableNamespaces(ctx context.Context, k kubernetes.Interface, defaultPruneConfig v1alpha1.Prune) (pruningNs, error) {
	nsList, err := k.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		return pruningNs{}, err
	}
	var prunableNs pruningNs
	commonSchedule := make(map[string]*pruneConfigPerNS)
	uniqueSchedule := make(map[string]*pruneConfigPerNS)
	re := regexp.MustCompile(NamespaceIgnorePattern)
	for _, ns := range nsList.Items {
		if ignore := re.MatchString(ns.GetName()); ignore {
			continue
		}
		nsAnnotations := ns.GetAnnotations()
		//skip all the namespaces if annotated with prune skip
		if nsAnnotations[pruneSkip] != "" && nsAnnotations[pruneSkip] == "true" {
			continue
		}
		pc := &pruneConfigPerNS{
			config: v1alpha1.Prune{},
		}

		if nsAnnotations[pruneResources] != "" {
			pc.config.Resources = strings.Split(nsAnnotations[pruneResources], ",")
		} else {
			pc.config.Resources = defaultPruneConfig.Resources
		}

		// if strategy not specified at the annotation level then config strategy is taken by default
		if nsAnnotations[pruneStrategy] == keep {
			if nsAnnotations[pruneKeep] != "" {
				keep, _ := strconv.Atoi(nsAnnotations[pruneKeep])
				uintKeep := uint(keep)
				pc.config.Keep = &uintKeep
				pc.config.KeepSince = nil
			} else if defaultPruneConfig.Keep != nil {
				pc.config.Keep = defaultPruneConfig.Keep
			}
		}

		if nsAnnotations[pruneStrategy] == keepSince {
			if nsAnnotations[pruneKeepSince] != "" {
				keepsince, _ := strconv.Atoi(nsAnnotations[pruneKeepSince])
				uintKeepSince := uint(keepsince)
				pc.config.KeepSince = &uintKeepSince
				pc.config.Keep = nil
			} else if defaultPruneConfig.KeepSince != nil {
				pc.config.KeepSince = defaultPruneConfig.KeepSince
			}
		}
		// is strategy not specified take the strategy from the tektonconfig and value from annotations
		if _, ok := nsAnnotations[pruneStrategy]; !ok {
			if defaultPruneConfig.Keep != nil {
				if nsAnnotations[pruneKeep] != "" {
					pc.config.Keep = stringToUint(nsAnnotations[pruneKeep])
				} else {
					pc.config.Keep = defaultPruneConfig.Keep
				}
			}
			if defaultPruneConfig.KeepSince != nil {
				if nsAnnotations[pruneKeepSince] != "" {
					pc.config.KeepSince = stringToUint(nsAnnotations[pruneKeepSince])
				} else {
					pc.config.KeepSince = defaultPruneConfig.KeepSince
				}
			}
		}

		// if a different schedule then create a new cronJob
		if nsAnnotations[pruneSchedule] != "" {
			if nsAnnotations[pruneSchedule] != defaultPruneConfig.Schedule {
				pc.config.Schedule = nsAnnotations[pruneSchedule]
				uniqueSchedule[ns.Name] = pc
				delete(commonSchedule, ns.Name)
				continue
			}
		}

		commonSchedule[ns.Name] = pc
	}
	prunableNs.commonScheduleNs = commonSchedule
	prunableNs.uniqueScheduleNS = uniqueSchedule
	return prunableNs, nil
}

func (pruner *Pruner) createAllJobContainers(nsConfig map[string]*pruneConfigPerNS) []corev1.Container {
	var containers []corev1.Container
	for ns, con := range nsConfig {
		jobContainers := pruner.createJobContainers(con, ns)
		containers = append(containers, jobContainers...)
	}
	return containers
}

func (pruner *Pruner) createJobContainers(nsConfig *pruneConfigPerNS, ns string) []corev1.Container {
	var containers []corev1.Container

	cmdArgs := pruneCommand(nsConfig, ns)
	containerName := SimpleNameGenerator.RestrictLengthWithRandomSuffix("pruner-tkn-" + ns)
	container := corev1.Container{
		Name:                     containerName,
		Image:                    pruner.tknImage,
		Command:                  []string{"/bin/sh", "-c"},
		Args:                     []string{cmdArgs},
		TerminationMessagePolicy: "FallbackToLogsOnError",
	}
	containers = append(containers, container)

	return containers
}

func (pruner *Pruner) createCronJob(ctx context.Context, targetNs, schedule string, pruneContainers []corev1.Container, oR v1.OwnerReference) error {
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
			Labels:          map[string]string{pruneCronLabel: "true"},
		},
		Spec: v1beta1.CronJobSpec{
			Schedule:          schedule,
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

	if _, err := pruner.kc.BatchV1beta1().CronJobs(targetNs).Create(ctx, cj, v1.CreateOptions{}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			if _, err := pruner.kc.BatchV1beta1().CronJobs(targetNs).Update(ctx, cj, v1.UpdateOptions{}); err != nil {
				return err
			}
		}
		return err
	}
	return nil
}

func pruneCommand(pru *pruneConfigPerNS, ns string) string {
	var cmdArgs string
	for _, resource := range pru.config.Resources {
		res := strings.TrimSpace(resource)
		var keepCmd string
		if pru.config.Keep != nil {
			keepCmd = "--keep=" + fmt.Sprint(*pru.config.Keep)
		}
		if pru.config.Keep == nil && pru.config.KeepSince != nil {
			keepCmd = "--keep-since=" + fmt.Sprint(*pru.config.KeepSince)
		}
		cmd := "tkn " + strings.ToLower(res) + " delete " + keepCmd + " -n=" + ns + " -f ; "
		cmdArgs = cmdArgs + cmd
	}
	return cmdArgs
}

func (pruner *Pruner) checkAndDeleteCron(ctx context.Context, cronName, ns string) error {
	if _, err := pruner.kc.BatchV1beta1().CronJobs(ns).Get(ctx, cronName, v1.GetOptions{}); err != nil {
		if err != nil && errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	//if there is no error it means cron is exists, but no prune in config it means delete it
	return pruner.kc.BatchV1beta1().CronJobs(ns).Delete(ctx, cronName, v1.DeleteOptions{})
}

func (pruner *Pruner) removedFromTektonConfig(prune v1alpha1.Prune) bool {
	if len(prune.Resources) == 0 || prune.Schedule == "" {
		return true
	}
	return false
}

func (pruner *Pruner) checkAnnotationsRemovedNamespaces(ctx context.Context, targetNs string) ([]string, error) {
	var namespaces []string
	var opts = v1.ListOptions{
		LabelSelector: fmt.Sprint(pruneCronLabel),
	}
	cronJobs, err := pruner.kc.BatchV1beta1().CronJobs("").List(ctx, opts)
	if err != nil {
		if errors.IsNotFound(err) {
			return namespaces, nil
		}
		return namespaces, err
	}
	for _, cronjob := range cronJobs.Items {
		cronNameSpace := cronjob.ObjectMeta.Namespace
		ns, err := pruner.kc.CoreV1().Namespaces().Get(ctx, cronNameSpace, v1.GetOptions{})
		if err != nil {
			return namespaces, err
		}
		ano := ns.GetAnnotations()
		if _, ok := ano[pruneSchedule]; !ok || ano[pruneSkip] == "true" {
			if cronNameSpace == targetNs {
				continue
			}
			namespaces = append(namespaces, cronNameSpace)
		}
	}
	return namespaces, err
}

func stringToUint(num string) *uint {
	keep, _ := strconv.Atoi(num)
	uintKeep := uint(keep)
	return &uintKeep
}
