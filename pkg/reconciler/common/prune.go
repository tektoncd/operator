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
	"sort"
	"strconv"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	// cron job and service account name
	prunerCronJobName        = "tekton-resource-pruner"
	prunerServiceAccountName = "tekton-resource-pruner"

	// tkn container image via environment key
	prunerContainerImageEnvKey = "IMAGE_JOB_PRUNER_TKN"

	// namespace annotations
	pruneAnnotationSkip             = "operator.tekton.dev/prune.skip"
	pruneAnnotationSchedule         = "operator.tekton.dev/prune.schedule"
	pruneAnnotationKeep             = "operator.tekton.dev/prune.keep"
	pruneAnnotationKeepSince        = "operator.tekton.dev/prune.keep-since"
	pruneAnnotationPrunePerResource = "operator.tekton.dev/prune.prune-per-resource"
	pruneAnnotationResources        = "operator.tekton.dev/prune.resources"
	pruneAnnotationStrategy         = "operator.tekton.dev/prune.strategy"

	// labels used in resources managed by pruner
	pruneCronLabel = "tektonconfig.operator.tekton.dev/pruner"

	// prune strategy used in namespace annotation
	pruneStrategyKeep      = "keep"
	pruneStrategyKeepSince = "keep-since"

	// script to be executed inside container
	prunerCommand = `
	function prune() {
		namespace=$1
		flags=$2
		resources=$3
		prune_per_resource=$4
		updated_flags=$(echo $flags | tr ',' ' ')
		for resource in ${resources//,/ }; do
			echo ""	
			if [[ "$prune_per_resource" == "true" ]]; then
				parent_resource=$(echo $resource | sed "s/run$//")
				resource_names=$(tkn $parent_resource list --namespace=$namespace --no-headers --output=jsonpath={.items[*].metadata.name})
				if [ $? -ne 0 ]; then
					echo "error on getting list of '$parent_resource'"
					error_status=1
					continue
				fi
				if [[ "$resource_names" == "" ]]; then
					echo "there is no '$parent_resource' available in '$namespace'"
					continue
				fi
				for resource_name in $resource_names; do
					echo ""
					target_cmd="tkn $resource delete --$parent_resource=\"$resource_name\" $updated_flags --namespace=$namespace --force"
					echo "\$ ${target_cmd}"
					eval $target_cmd
					if [ $? -ne 0 ]; then
						error_status=1
					fi
				done
			else
				target_cmd="tkn $resource delete $updated_flags --namespace=$namespace --force"
				echo "\$ ${target_cmd}"
				eval $target_cmd
				if [ $? -ne 0 ]; then
					error_status=1
				fi
			fi
		done
	}
	
	error_status=0
	for c in $*; do
		namespace=$(echo $c | cut -d ';' -f 1)
		flags=$(echo $c | cut -d ';' -f 2)
		resources=$(echo $c | cut -d ';' -f 3)
		prune_per_resource=$(echo $c | cut -d ';' -f 4)
		prune $namespace $flags $resources $prune_per_resource
	done
	exit $error_status
	`
)

var (
	// normalize resources
	pruneResourceNameMap = map[string]string{
		"pipelineruns": "pipelinerun",
		"pipelinerun":  "pipelinerun",
		"pr":           "pipelinerun",
		"taskruns":     "taskrun",
		"taskrun":      "taskrun",
		"tr":           "taskrun",
	}
)

type Pruner struct {
	tektonConfig    *v1alpha1.TektonConfig
	kubeClientset   kubernetes.Interface
	tknImage        string
	targetNamespace string
	ownerRef        metav1.OwnerReference
	logger          *zap.SugaredLogger
	ctx             context.Context
}

type pruneConfig struct {
	Schedule         string
	Namespace        string
	Keep             *uint
	KeepSince        *uint
	Resources        []string
	PrunePerResource bool
	TknImage         string
}

func Prune(ctx context.Context, k kubernetes.Interface, tektonConfig *v1alpha1.TektonConfig) error {
	pruner, err := getPruner(ctx, k, tektonConfig)
	if err != nil {
		return err
	}

	return pruner.reconcile()
}

func getPruner(ctx context.Context, k kubernetes.Interface, tektonConfig *v1alpha1.TektonConfig) (*Pruner, error) {
	pruner := &Pruner{
		tektonConfig:    tektonConfig,
		kubeClientset:   k,
		targetNamespace: tektonConfig.Spec.TargetNamespace,
		ownerRef:        *metav1.NewControllerRef(tektonConfig, tektonConfig.GetGroupVersionKind()),
		ctx:             ctx,
		logger:          logging.FromContext(ctx),
	}
	return pruner, nil
}

func (pr *Pruner) reconcile() error {
	// get tkn cli container image name from environment
	tknImageFromEnv := os.Getenv(prunerContainerImageEnvKey)
	if tknImageFromEnv == "" {
		return fmt.Errorf("tkn image '%s' environment variable is not set", prunerContainerImageEnvKey)
	}
	pr.tknImage = tknImageFromEnv

	// reconcile cron jobs
	err := pr.reconcileCronJobs()
	return err
}

func (pr *Pruner) getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{pr.ownerRef}
}

func (pr *Pruner) reconcileCronJobs() error {
	// group prune config by schedule cron expression
	// use schedule cron expression as map key
	// grouping by this way we can limit number of cron jobs
	// example: {"* * * * *": []{}, "*/2 * * * *": []{}}
	pruneConfigsMap := make(map[string][]pruneConfig)

	// verify prune job enabled in TektonConfig CR
	if !pr.tektonConfig.Spec.Pruner.Disabled {
		// collect namespace details
		namespaceList, err := pr.kubeClientset.CoreV1().Namespaces().List(pr.ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		// ignore namespace where pipeline never configured
		ignorePattern := regexp.MustCompile(NamespaceIgnorePattern)
		for _, namespace := range namespaceList.Items {
			if ignorePattern.MatchString(namespace.GetName()) {
				continue
			}

			prunerCfg := pr.getPruneConfig(&namespace)
			if prunerCfg == nil {
				// prune job skipped for this namespace
				// may be annotation skip enabled or some error on the config
				// error details will be printed, if any
				continue
			}

			// add prune config into the map
			if _, found := pruneConfigsMap[prunerCfg.Schedule]; !found {
				pruneConfigsMap[prunerCfg.Schedule] = make([]pruneConfig, 0)
			}
			configSlice := pruneConfigsMap[prunerCfg.Schedule]
			configSlice = append(configSlice, *prunerCfg)
			pruneConfigsMap[prunerCfg.Schedule] = configSlice
		}
	}

	// compute hash for the grouped prune configurations
	computedHashMap := make(map[string]string)
	for schedule := range pruneConfigsMap {
		pruneConfigs := pruneConfigsMap[schedule]
		// order pruneConfigs by namespace to keep a constant hash value
		sort.SliceStable(pruneConfigs, func(i, j int) bool {
			return pruneConfigs[i].Namespace < pruneConfigs[j].Namespace
		})

		computedHash, err := pr.computeHash(pruneConfigs)
		if err != nil {
			pr.logger.Errorw("error on computing hash value, skipping this schedule",
				"schedule", schedule,
				"pruneConfigs", pruneConfigs,
				err,
			)
			continue
		}
		computedHashMap[schedule] = computedHash
	}

	// remove the existing outdated cron jobs
	cronJobsToBeCreated, err := pr.deleteOutdatedCronJobs(computedHashMap)
	if err != nil {
		pr.logger.Errorw("error on deleting outdated cron jobs", err)
		return err
	}

	// create cron jobs that is modified [or] not exists
	pr.createCronJobs(cronJobsToBeCreated, pruneConfigsMap)

	return nil
}

// to compute hash include, pruneConfigs, nodeSelector, toleration, priorityClass
func (pr *Pruner) computeHash(pruneConfigs []pruneConfig) (string, error) {
	// to compute hash additionally include, nodeSelector, tolerations, priorityClassName
	// to update cronjobs if there is a change on those fields
	targetObject := struct {
		PruneConfigs      []pruneConfig
		NodeSelector      map[string]string
		Tolerations       []corev1.Toleration
		PriorityClassName string
		Script            string
	}{
		PruneConfigs:      pruneConfigs,
		NodeSelector:      pr.tektonConfig.Spec.Config.NodeSelector,
		Tolerations:       pr.tektonConfig.Spec.Config.Tolerations,
		PriorityClassName: pr.tektonConfig.Spec.Config.PriorityClassName,
		Script:            prunerCommand,
	}
	return hash.Compute(targetObject)
}

// update prune config from namespace annotations and global pruner config
func (pr *Pruner) getPruneConfig(namespace *corev1.Namespace) *pruneConfig {
	// create prune config and update some values from global config
	// note these values may be replaced with namespace annotations value
	defaultPruneConfig := pr.tektonConfig.Spec.Pruner
	pruneCfg := pruneConfig{
		Namespace:        namespace.GetName(),
		TknImage:         pr.tknImage,
		Schedule:         defaultPruneConfig.Schedule,
		PrunePerResource: defaultPruneConfig.PrunePerResource,
	}

	annotations := namespace.GetAnnotations()

	// asked to skip for this namespace?
	if pr.getMapString(annotations, pruneAnnotationSkip, "") == "true" {
		return nil
	}

	// if the global schedule is disabled and there is no prune schedule annotation present in a namespace
	// skip that namespace
	if defaultPruneConfig.Schedule == "" && pr.getMapString(annotations, pruneAnnotationSchedule, "") == "" {
		return nil
	}

	// update missing values from defaults
	if defaultPruneConfig.Keep == nil && defaultPruneConfig.KeepSince == nil {
		keep := v1alpha1.PrunerDefaultKeep
		defaultPruneConfig.Keep = &keep
	}
	if len(defaultPruneConfig.Resources) == 0 {
		defaultPruneConfig.Resources = v1alpha1.PruningDefaultResources
	}

	// update keep and keep-since based on the strategy
	pruneStrategy := pr.getMapString(annotations, pruneAnnotationStrategy, "")
	// update keep value
	if pruneStrategy == pruneStrategyKeep || pruneStrategy == "" {
		// if value a not found on the namespace annotation, take it from global configuration
		_keep, err := pr.getMapUint(annotations, pruneAnnotationKeep, defaultPruneConfig.Keep)
		if err != nil {
			pr.logger.Errorw("invalid keep value received",
				"keepValue", pr.getMapString(annotations, pruneAnnotationKeep, ""),
				"namespace", namespace.GetName(),
			)
			return nil
		}
		pruneCfg.Keep = _keep
	}
	// update keepSince value
	if pruneStrategy == pruneStrategyKeepSince || pruneStrategy == "" {
		// if value a not found on the namespace annotation, take it from global configuration
		_keepSince, err := pr.getMapUint(annotations, pruneAnnotationKeepSince, defaultPruneConfig.KeepSince)
		if err != nil {
			pr.logger.Errorw("invalid keep-since value received",
				"keepSinceValue", pr.getMapString(annotations, pruneAnnotationKeepSince, ""),
				"namespace", namespace.GetName(),
			)
			return nil
		}
		pruneCfg.KeepSince = _keepSince
	}

	// update schedule
	pruneCfg.Schedule = pr.getMapString(annotations, pruneAnnotationSchedule, defaultPruneConfig.Schedule)

	// update resources
	resourcesString := pr.getMapString(annotations, pruneAnnotationResources, "")
	if resourcesString == "" {
		pruneCfg.Resources = defaultPruneConfig.Resources
	} else {
		resources := strings.Split(resourcesString, ",")
		pruneCfg.Resources = resources
	}

	// update prune-per-resource, if annotation set on this namespace
	prunePerResourceString := pr.getMapString(annotations, pruneAnnotationPrunePerResource, "")
	if prunePerResourceString != "" {
		pruneCfg.PrunePerResource = prunePerResourceString == "true"
	}

	// normalize resource values
	normalizedResources := []string{}
	for _, resource := range pruneCfg.Resources {
		// trim and lowercase the resource
		resource = strings.ToLower(strings.TrimSpace(resource))
		normalizedResource, found := pruneResourceNameMap[resource]
		if !found {
			pr.logger.Errorw("invalid resource value received",
				"resourceValue", resource,
				"namespace", namespace.GetName(),
			)
			continue
		}
		normalizedResources = append(normalizedResources, normalizedResource)
	}
	pruneCfg.Resources = normalizedResources

	// if there is no resource provided, there is no meaning to proceed further
	// return nil, will not be created a cron job for this namespace
	if len(pruneCfg.Resources) == 0 {
		pr.logger.Warnw("there is no resource defined",
			"namespace", namespace.GetName(),
		)
		return nil
	}

	// if keep and keep-since is nil or either one is zero, skip that namespace
	if pruneCfg.Keep == nil && pruneCfg.KeepSince == nil {
		pr.logger.Warnw("flags keep and keep-since can not be nil",
			"namespace", namespace.GetName(),
		)
		return nil
	} else if pruneCfg.Keep != nil && *pruneCfg.Keep == 0 {
		pr.logger.Warnw("flag keep can not be 0",
			"namespace", namespace.GetName(),
		)
		return nil
	} else if pruneCfg.KeepSince != nil && *pruneCfg.KeepSince == 0 {
		pr.logger.Warnw("flag keep-since can not be 0",
			"namespace", namespace.GetName(),
		)
		return nil
	}

	// sort resources to keep a constant hash value
	sort.Strings(pruneCfg.Resources)

	return &pruneCfg
}

func (pr *Pruner) getMapString(data map[string]string, key, defaultValue string) string {
	value, found := data[key]
	if !found {
		return defaultValue
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func (pr *Pruner) getMapUint(data map[string]string, key string, defaultValue *uint) (*uint, error) {
	// break the defaultValue pointer reference
	var defaultValueCloned *uint
	if defaultValue != nil {
		dValue := *defaultValue
		defaultValueCloned = &dValue
	}

	value, found := data[key]
	if !found {
		return defaultValueCloned, nil
	}
	uintValue, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return defaultValueCloned, err
	}

	newValue := uint(uintValue)
	return &newValue, nil
}

// deletes absolute cron jobs and returns cron schedule to be (re)created
func (pr *Pruner) deleteOutdatedCronJobs(computedHashMap map[string]string) (map[string]string, error) {
	// filter only the jobs owned by this operator
	labelsFilter := fmt.Sprintf("%s=true", pruneCronLabel)
	cronJobs, err := pr.kubeClientset.BatchV1().CronJobs(pr.targetNamespace).List(pr.ctx, metav1.ListOptions{LabelSelector: labelsFilter})
	if err != nil {
		return nil, err
	}

	for _, cronJob := range cronJobs.Items {
		markedForDeletion := false
		annotations := cronJob.GetAnnotations()
		existingHash, found := annotations[v1alpha1.LastAppliedHashKey]
		if !found {
			markedForDeletion = true
		}

		// check existing hash availability in comptedHashMap
		if !markedForDeletion {
			hasValidHash := false
			for schedule, computedHash := range computedHashMap {
				// if hash value found. cron job up to date, no action needed
				// and remove it from the computedHashMap
				if computedHash == existingHash {
					delete(computedHashMap, schedule)
					hasValidHash = true
					break
				}
			}
			// hash value not found, mark it for deletion
			if !hasValidHash {
				markedForDeletion = true
			}
		}

		if markedForDeletion {
			pr.logger.Debugw("deleting an outdated cron job",
				"name", cronJob.GetName(),
				"schedule", cronJob.Spec.Schedule,
			)
			err = pr.kubeClientset.BatchV1().CronJobs(pr.targetNamespace).Delete(pr.ctx, cronJob.GetName(), metav1.DeleteOptions{})
			if err != nil {
				pr.logger.Errorw("error on deleting an outdated cron job",
					"name", cronJob.GetName(),
					"namespace", cronJob.GetNamespace(),
					err,
				)
				continue
			}
		}
	}

	return computedHashMap, nil
}

func (pr *Pruner) createCronJobs(cronJobsToBeCreated map[string]string, pruneConfigMap map[string][]pruneConfig) {
	for schedule, computedHash := range cronJobsToBeCreated {
		pruneConfigs, found := pruneConfigMap[schedule]
		if !found {
			pr.logger.Errorw("prune schedule not found",
				"schedule", schedule,
			)
			continue
		}

		prunerCommandArgs := pr.generatePrunerCommandArgs(pruneConfigs)

		// create cron job
		backOffLimit := int32(1)
		failedJobsHistoryLimit := int32(2)
		successfulJobsHistoryLimit := int32(2)
		ttlSecondsAfterFinished := int32(3600)
		runAsNonRoot := true
		allowPrivilegedEscalation := false
		runAsUser := ptr.Int64(65532)
		fsGroup := ptr.Int64(65532)

		// if it is a openshift platform remove the user and fsGroup ids
		// those ids will be allocated dynamically
		if v1alpha1.IsOpenShiftPlatform() {
			runAsUser = nil
			fsGroup = nil
		}

		cronJob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName:    fmt.Sprintf("%s-", prunerCronJobName),
				Namespace:       pr.targetNamespace,
				OwnerReferences: pr.getOwnerReferences(),
				Labels:          map[string]string{v1alpha1.CreatedByKey: v1alpha1.PrunerResourceName, pruneCronLabel: "true"},
				Annotations:     map[string]string{v1alpha1.LastAppliedHashKey: computedHash},
			},
			Spec: batchv1.CronJobSpec{
				Schedule:                   schedule,
				ConcurrencyPolicy:          batchv1.ForbidConcurrent,
				FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
				SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
						BackoffLimit:            &backOffLimit,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name:                     "tkn-pruner",
									Image:                    pr.tknImage,
									Command:                  []string{"/bin/sh", "-c", prunerCommand},
									Args:                     []string{"-s", prunerCommandArgs},
									TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: &allowPrivilegedEscalation,
										Capabilities: &corev1.Capabilities{
											Drop: []corev1.Capability{"ALL"},
										},
									},
								}},
								RestartPolicy:      corev1.RestartPolicyNever,
								ServiceAccountName: prunerServiceAccountName,
								NodeSelector:       pr.tektonConfig.Spec.Config.NodeSelector,
								Tolerations:        pr.tektonConfig.Spec.Config.Tolerations,
								PriorityClassName:  pr.tektonConfig.Spec.Config.PriorityClassName,
								SecurityContext: &corev1.PodSecurityContext{
									RunAsNonRoot: &runAsNonRoot,
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
									RunAsUser: runAsUser,
									FSGroup:   fsGroup,
								},
							},
						},
					},
				},
			},
		}

		// create a cron job
		_, err := pr.kubeClientset.BatchV1().CronJobs(pr.targetNamespace).Create(pr.ctx, cronJob, metav1.CreateOptions{})
		if err != nil {
			pr.logger.Errorw("error on creating a cron job",
				"name", cronJob.GetName(),
				"namespace", cronJob.GetNamespace(),
				err,
			)
		}
	}
}

// generates command arguments to pass it to tkn container
// refer "prunerCommand"(top of this file) constant string to know the actual execution command
// command args format (multiple instance of space separated): namespace;tkn_flag_1,tkn_flag_n;resources;prunePerResource
// NOTE: a space separates each namespace configuration, hence space not allowed in namespace configuration
//
// examples:
// ns-one;--keep=5;pipelinerun;false
//   - $ tkn pipelinerun delete --keep=5 --namespace=ns-one --force
//
// ns-two;--keep=2;taskrun;false
//   - $ tkn taskrun delete --keep=2 --namespace=ns-two --force
//
// ns-three;--keep=4,--keep-since=300;pipelinerun,taskrun;false
//   - $ tkn pipelinerun delete --keep=4 --keep-since=300 --namespace=ns-three --force
//   - $ tkn taskrun delete --keep=4 --keep-since=300 --namespace=ns-three --force
//
// ns-four;--keep=4;pipelinerun,taskrun;true  <= note the "true" - prunePerResource
// resource names will be taken dynamically on the script
//   - $ tkn pipelinerun delete --pipeline="pipeline-one" --keep=5 --namespace=ns-four --force
//   - $ tkn taskrun delete --task="task-one" --keep=5 --namespace=ns-four --force
func (pr *Pruner) generatePrunerCommandArgs(pruneConfigs []pruneConfig) string {
	commands := []string{}
	for _, pruneCfg := range pruneConfigs {
		tknFlagsSlice := []string{}
		if pruneCfg.Keep != nil && *pruneCfg.Keep != 0 {
			tknFlagsSlice = append(tknFlagsSlice, fmt.Sprintf("--keep=%d", *pruneCfg.Keep))
		}
		if pruneCfg.KeepSince != nil && *pruneCfg.KeepSince != 0 {
			tknFlagsSlice = append(tknFlagsSlice, fmt.Sprintf("--keep-since=%d", *pruneCfg.KeepSince))
		}

		tnkFlags := strings.Join(tknFlagsSlice, ",")
		resources := strings.Join(pruneCfg.Resources, ",")

		commands = append(commands,
			fmt.Sprintf(
				"%s;%s;%s;%t",
				pruneCfg.Namespace, tnkFlags, resources, pruneCfg.PrunePerResource,
			),
		)
	}

	// create space separated group of commands
	return strings.Join(commands, " ")
}
