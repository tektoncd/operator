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
	"reflect"
	"sort"
	"strconv"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/yaml"
)

const (
	AnnotationPreserveNS          = "operator.tekton.dev/preserve-namespace"
	AnnotationPreserveRBSubjectNS = "operator.tekton.dev/preserve-rb-subject-namespace"
	ImageRegistryOverride         = "TEKTON_REGISTRY_OVERRIDE"
	PipelinesImagePrefix          = "IMAGE_PIPELINES_"
	TriggersImagePrefix           = "IMAGE_TRIGGERS_"
	AddonsImagePrefix             = "IMAGE_ADDONS_"
	PacImagePrefix                = "IMAGE_PAC_"
	ChainsImagePrefix             = "IMAGE_CHAINS_"
	ManualApprovalGatePrefix      = "IMAGE_MAG_"
	PrunerImagePrefix             = "IMAGE_PRUNER_"
	SchedulerImagePrefix          = "IMAGE_SCHEDULER_"
	ResultsImagePrefix            = "IMAGE_RESULTS_"
	HubImagePrefix                = "IMAGE_HUB_"
	DashboardImagePrefix          = "IMAGE_DASHBOARD_"

	DefaultTargetNamespace = "tekton-pipelines"

	ArgPrefix   = "arg_"
	ParamPrefix = "param_"

	runAsNonRootValue              = true
	allowPrivilegedEscalationValue = false
	pipelinesControllerDeployment  = "tekton-pipelines-controller"
)

// transformers that are common to all components.
func transformers(ctx context.Context, obj v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		mf.InjectOwner(obj),
		injectNamespaceConditional(AnnotationPreserveNS, obj.GetSpec().GetTargetNamespace()),
		injectNamespaceCRDWebhookClientConfig(obj.GetSpec().GetTargetNamespace()),
		injectNamespaceCRClusterInterceptorClientConfig(obj.GetSpec().GetTargetNamespace()),
		injectNamespaceClusterRole(obj.GetSpec().GetTargetNamespace()),
		ReplaceNamespaceInWebhookNamespaceSelector(obj.GetSpec().GetTargetNamespace()),
		AddDeploymentRestrictedPSA(),
	}
}

// TODO for now added here but planning to refactor so that we can avoid openshift specific changes as part of common
func roleBindingTransformers(ctx context.Context, obj v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		mf.InjectOwner(obj),
		injectNamespaceRoleBindingConditional(AnnotationPreserveNS,
			AnnotationPreserveRBSubjectNS, obj.GetSpec().GetTargetNamespace()),
	}
}

// Transform will mutate the passed-by-reference manifest with one
// transformed by platform, common, and any extra passed in
func Transform(ctx context.Context, manifest *mf.Manifest, instance v1alpha1.TektonComponent, extra ...mf.Transformer) error {
	logger := logging.FromContext(ctx)
	logger.Debug("Transforming manifest")

	roleBindingManifest := manifest.Filter(mf.Any(mf.ByKind("RoleBinding")))
	remainingManifest := manifest.Filter(mf.Not(mf.Any(mf.ByKind("RoleBinding"))))

	transformers := transformers(ctx, instance)
	transformers = append(transformers, extra...)

	t1 := roleBindingTransformers(ctx, instance)

	remainingManifest, err := remainingManifest.Transform(transformers...)
	if err != nil {
		return err
	}
	roleBindingManifest, err = roleBindingManifest.Transform(t1...)
	if err != nil {
		return err
	}
	*manifest = remainingManifest.Append(roleBindingManifest)
	return nil
}

func injectNamespaceConditional(preserveNamespace, targetNamespace string) mf.Transformer {
	tf := mf.InjectNamespace(targetNamespace)
	return func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		val, ok := annotations[preserveNamespace]
		if ok && val == "true" {
			return nil
		}
		return tf(u)
	}
}

func injectNamespaceCRDWebhookClientConfig(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "customresourcedefinition" {
			return nil
		}
		service, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "conversion", "webhookClientConfig", "service")
		if !found || err != nil {
			return err
		}
		m := service.(map[string]interface{})
		if _, ok := m["namespace"]; ok {
			m["namespace"] = targetNamespace
		}
		return nil
	}
}

func injectNamespaceCRClusterInterceptorClientConfig(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "clusterinterceptor" {
			return nil
		}
		service, found, err := unstructured.NestedFieldNoCopy(u.Object, "spec", "clientConfig", "service")
		if !found || err != nil {
			return err
		}
		m := service.(map[string]interface{})
		if _, ok := m["namespace"]; ok {
			m["namespace"] = targetNamespace
		}
		return nil
	}
}

// ImagesFromEnv will provide map of key value.
func ImagesFromEnv(prefix string) map[string]string {
	images := map[string]string{}
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}

		keyValue := strings.Split(env, "=")
		name := strings.TrimPrefix(keyValue[0], prefix)
		url := keyValue[1]
		images[name] = url
	}

	return images
}

// ImageRegistryDomainOverride will add or override the registry used in the image list
func ImageRegistryDomainOverride(images map[string]string) map[string]string {
	registry := os.Getenv(ImageRegistryOverride)
	if registry == "" {
		return images
	} else {
		for key, imageName := range images {
			parts := strings.Split(imageName, "/")
			if len(parts) > 1 {
				// if image has registry part, replace it
				images[key] = registry + "/" + strings.Join(parts[1:], "/")
			} else {
				// if image does not have registry part, add it
				images[key] = registry + "/" + imageName
			}
		}
		return images
	}
}

// ToLowerCaseKeys converts key value to lower cases.
func ToLowerCaseKeys(keyValues map[string]string) map[string]string {
	newMap := map[string]string{}

	for k, v := range keyValues {
		key := strings.ToLower(k)
		newMap[key] = v
	}

	return newMap
}

// DeploymentImages replaces container and args images.
func DeploymentImages(images map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		containers := d.Spec.Template.Spec.Containers
		replaceContainerImages(containers, images)
		initContainers := d.Spec.Template.Spec.InitContainers
		replaceContainerImages(initContainers, images)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// StatefulSetImages replaces container and args images.
func StatefulSetImages(images map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" {
			return nil
		}

		s := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, s)
		if err != nil {
			return err
		}

		containers := s.Spec.Template.Spec.Containers
		replaceContainerImages(containers, images)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// JobImages replaces container and args images.
func JobImages(images map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Job" {
			return nil
		}

		jb := &batchv1.Job{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, jb)
		if err != nil {
			return err
		}

		containers := jb.Spec.Template.Spec.Containers
		replaceContainerImages(containers, images)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func replaceContainerImages(containers []corev1.Container, images map[string]string) {
	for i, container := range containers {
		name := formKey("", container.Name)
		if url, exist := images[name]; exist {
			containers[i].Image = url
		}

		replaceContainersArgsImage(&container, images)
	}
}

func replaceContainersArgsImage(container *corev1.Container, images map[string]string) {
	for a, arg := range container.Args {
		if argVal, hasArg := SplitsByEqual(arg); hasArg {
			argument := formKey(ArgPrefix, argVal[0])
			if url, exist := images[argument]; exist {
				container.Args[a] = argVal[0] + "=" + url
			}
			continue
		}

		argument := formKey(ArgPrefix, arg)
		if url, exist := images[argument]; exist {
			container.Args[a+1] = url
		}
	}
}

func formKey(prefix, arg string) string {
	argument := strings.ToLower(arg)
	if prefix != "" {
		argument = prefix + argument
	}
	return strings.ReplaceAll(argument, "-", "_")
}

func SplitsByEqual(arg string) ([]string, bool) {
	values := strings.Split(arg, "=")
	if len(values) == 2 {
		return values, true
	}

	return values, false
}

// TaskImages replaces step and params images.
func TaskImages(ctx context.Context, images map[string]string) mf.Transformer {
	logger := logging.FromContext(ctx)
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ClusterTask" && u.GetKind() != "Task" {
			return nil
		}

		steps, found, err := unstructured.NestedSlice(u.Object, "spec", "steps")
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		replaceStepsImages(steps, images, logger)
		err = unstructured.SetNestedField(u.Object, steps, "spec", "steps")
		if err != nil {
			return err
		}

		params, found, err := unstructured.NestedSlice(u.Object, "spec", "params")
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		replaceParamsImage(params, images, logger)
		return unstructured.SetNestedField(u.Object, params, "spec", "params")
	}
}

// StepActionImages replaces spec images.
func StepActionImages(ctx context.Context, images map[string]string) mf.Transformer {
	logger := logging.FromContext(ctx)
	return func(u *unstructured.Unstructured) error {
		stepActionSpec, found, err := unstructured.NestedMap(u.Object, "spec")
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		replaceStepActionImages(stepActionSpec, images, u.GetName(), logger)
		return unstructured.SetNestedMap(u.Object, stepActionSpec, "spec")
	}
}

func replaceStepActionImages(stepActionSpec map[string]interface{}, override map[string]string, name string, logger *zap.SugaredLogger) {
	name = formKey("", name)
	image, found := override[name]
	if !found || image == "" {
		logger.Debugf("Image not found in stepaction %s action skip", name)
		return
	}
	// Replace the image in the stepActionSpec if the key exists.
	if _, ok := stepActionSpec["image"]; ok {
		logger.Debugf("replacing image with %s", image)
		stepActionSpec["image"] = image
	}
}

func replaceStepsImages(steps []interface{}, override map[string]string, logger *zap.SugaredLogger) {
	for _, s := range steps {
		step := s.(map[string]interface{})
		name, ok := step["name"].(string)
		if !ok {
			logger.Debugf("Unable to get the step %v step", s)
			continue
		}

		name = formKey("", name)
		image, found := override[name]
		if !found || image == "" {
			logger.Debugf("Image not found step %s action skip", name)
			continue
		}
		step["image"] = image
	}
}

func replaceParamsImage(params []interface{}, override map[string]string, logger *zap.SugaredLogger) {
	for _, p := range params {
		param := p.(map[string]interface{})
		name, ok := param["name"].(string)
		if !ok {
			logger.Debugf("Unable to get the pram %v param", p)
			continue
		}

		name = formKey(ParamPrefix, name)
		image, found := override[name]
		if !found || image == "" {
			logger.Debugf("Image not found step %s action skip", name)
			continue
		}
		param["default"] = image
	}
}

func injectNamespaceRoleBindingConditional(preserveNS, preserveRBSubjectNS, targetNamespace string) mf.Transformer {
	tf := injectNamespaceRoleBindingSubjects(targetNamespace)

	return func(u *unstructured.Unstructured) error {
		annotations := u.GetAnnotations()
		val, ok := annotations[preserveNS]
		if !(ok && val == "true") {
			u.SetNamespace(targetNamespace)
		}
		val, ok = annotations[preserveRBSubjectNS]
		if ok && val == "true" {
			return nil
		}
		return tf(u)
	}
}

func injectNamespaceRoleBindingSubjects(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "rolebinding" {
			return nil
		}
		subjects, found, err := unstructured.NestedFieldNoCopy(u.Object, "subjects")
		if !found || err != nil {
			return err
		}
		for _, subject := range subjects.([]interface{}) {
			m := subject.(map[string]interface{})
			if _, ok := m["namespace"]; ok {
				m["namespace"] = targetNamespace
			}
		}
		return nil
	}
}

func injectNamespaceClusterRole(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "clusterrole" {
			return nil
		}
		rules, found, err := unstructured.NestedFieldNoCopy(u.Object, "rules")
		if !found || err != nil {
			return err
		}
		for _, rule := range rules.([]interface{}) {
			m := rule.(map[string]interface{})
			resources, ok := m["resources"]
			if !ok || len(resources.([]interface{})) == 0 {
				continue
			}
			containsNamespaceResource := false
			for _, resource := range resources.([]interface{}) {
				if strings.HasPrefix(resource.(string), "namespaces") {
					containsNamespaceResource = true
				}
			}
			resourceNames, ok := m["resourceNames"]
			if containsNamespaceResource && ok {
				nm := []interface{}{}
				for _, rn := range resourceNames.([]interface{}) {
					if rn.(string) == DefaultTargetNamespace {
						nm = append(nm, targetNamespace)
					} else {
						nm = append(nm, rn)
					}
				}
				m["resourceNames"] = nm
			}
		}
		return nil
	}
}

// ReplaceNamespaceInDeploymentEnv replaces any instance of the default namespace string in the given deployments' env var
func ReplaceNamespaceInDeploymentEnv(deploymentNames []string, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || !slices.Contains(deploymentNames, u.GetName()) {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		container := d.Spec.Template.Spec.Containers[0]
		d.Spec.Template.Spec.Containers[0].Env = replaceNamespaceInDBAddress(container.Env, targetNamespace)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func replaceNamespaceInDBAddress(envs []corev1.EnvVar, targetNamespace string) []corev1.EnvVar {
	req := []string{"DB_ADDR", "TEKTON_RESULTS_API_SERVICE"}

	for i, e := range envs {
		if slices.Contains(req, e.Name) {
			envs[i].Value = strings.ReplaceAll(e.Value, DefaultTargetNamespace, targetNamespace)
		}
	}
	return envs
}

// ReplaceNamespaceInDeploymentArgs replaces any instance of the default namespace in the given deployments' args
func ReplaceNamespaceInDeploymentArgs(deploymentNames []string, targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || !slices.Contains(deploymentNames, u.GetName()) {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		container := d.Spec.Template.Spec.Containers[0]
		replaceNamespaceInContainerArg(&container, targetNamespace)

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

func replaceNamespaceInContainerArg(container *corev1.Container, targetNamespace string) {
	for i, a := range container.Args {
		if strings.Contains(a, DefaultTargetNamespace) {
			container.Args[i] = strings.ReplaceAll(a, DefaultTargetNamespace, targetNamespace)
		}
	}
}

// AddConfigMapValues will loop on the interface (should be a struct) and add the fields in to configMap
// the key will be the json tag of the struct field
func AddConfigMapValues(configMapName string, prop interface{}) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" || u.GetName() != configMapName || prop == nil {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		values := reflect.ValueOf(prop)
		// if the given properties is not struct type, do not proceed
		if values.Kind() != reflect.Struct {
			return nil
		}

		for index := 0; index < values.NumField(); index++ {
			key := values.Type().Field(index).Tag.Get("json")
			if strings.Contains(key, ",") {
				key = strings.Split(key, ",")[0]
			}

			if key == "" {
				continue
			}

			element := values.Field(index)
			if element.Kind() == reflect.Ptr {
				if element.IsNil() {
					continue
				}
				// empty string value will not be included in the following switch statement
				// however, *string pointer can have empty("") string
				// so copying the actual string value to the configMap, it can be a empty string too
				if value, ok := element.Interface().(*string); ok {
					if value != nil {
						cm.Data[key] = *value
					}
				}
				// extract the actual element from the pointer
				element = values.Field(index).Elem()
			}

			if !element.IsValid() {
				continue
			}

			_value := ""
			switch element.Kind() {
			case reflect.Bool:
				_value = strconv.FormatBool(element.Bool())

			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				_value = strconv.FormatInt(element.Int(), 10)

			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				_value = strconv.FormatUint(element.Uint(), 10)

			case reflect.Float32, reflect.Float64:
				_value = strconv.FormatFloat(element.Float(), 'f', 6, 64)

			case reflect.String:
				_value = element.String()

			case reflect.Struct:
				out, err := yaml.Marshal(element.Interface())
				if err != nil {
					return fmt.Errorf("failed to marshal struct field %s: %v", key, err)
				}
				_value = string(out)
			}

			if _value != "" {
				cm.Data[key] = _value
			}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// InjectLabelOnNamespace will add a label on tekton-pipelines and
// openshift-pipelines namespace
func InjectLabelOnNamespace(label string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "namespace" {
			return nil
		}

		labels := u.GetLabels()
		arr := strings.Split(label, "=")
		labels[arr[0]] = arr[1]
		u.SetLabels(labels)

		return nil
	}
}

func AddConfiguration(config v1alpha1.Config) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		d.Spec.Template.Spec.NodeSelector = config.NodeSelector
		d.Spec.Template.Spec.Tolerations = config.Tolerations
		d.Spec.Template.Spec.PriorityClassName = config.PriorityClassName

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// AddDeploymentRestrictedPSA will add the default restricted spec on Deployment to remove errors/warning
func AddDeploymentRestrictedPSA() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		if d.Spec.Template.Spec.SecurityContext == nil {
			d.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}

		if d.Spec.Template.Spec.SecurityContext.RunAsNonRoot == nil {
			d.Spec.Template.Spec.SecurityContext.RunAsNonRoot = ptr.Bool(runAsNonRootValue)
		}

		if d.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
			d.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			}
		}

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			if c.SecurityContext == nil {
				c.SecurityContext = &corev1.SecurityContext{}
			}
			c.SecurityContext.AllowPrivilegeEscalation = ptr.Bool(allowPrivilegedEscalationValue)
			c.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// AddStatefulSetRestrictedPSA will add the default restricted spec on StatefulSet to remove errors/warning
func AddStatefulSetRestrictedPSA() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if strings.ToLower(u.GetKind()) != "statefulset" {
			return nil
		}

		s := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, s)
		if err != nil {
			return err
		}

		if s.Spec.Template.Spec.SecurityContext == nil {
			s.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}

		if s.Spec.Template.Spec.SecurityContext.RunAsNonRoot == nil {
			s.Spec.Template.Spec.SecurityContext.RunAsNonRoot = ptr.Bool(runAsNonRootValue)
		}

		if s.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
			s.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			}
		}

		for i := range s.Spec.Template.Spec.Containers {
			c := &s.Spec.Template.Spec.Containers[i]
			if c.SecurityContext == nil {
				c.SecurityContext = &corev1.SecurityContext{}
			}
			c.SecurityContext.AllowPrivilegeEscalation = ptr.Bool(allowPrivilegedEscalationValue)
			c.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(s)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// AddJobRestrictedPSA will add the default restricted spec on Job to remove errors/warning
func AddJobRestrictedPSA() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Job" {
			return nil
		}

		jb := &batchv1.Job{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, jb)
		if err != nil {
			return err
		}

		if jb.Spec.Template.Spec.SecurityContext == nil {
			jb.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
		}

		if jb.Spec.Template.Spec.SecurityContext.RunAsNonRoot == nil {
			jb.Spec.Template.Spec.SecurityContext.RunAsNonRoot = ptr.Bool(runAsNonRootValue)
		}

		if jb.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
			jb.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			}
		}

		for i := range jb.Spec.Template.Spec.Containers {
			c := &jb.Spec.Template.Spec.Containers[i]
			if c.SecurityContext == nil {
				c.SecurityContext = &corev1.SecurityContext{}
			}
			if c.SecurityContext.AllowPrivilegeEscalation == nil {
				c.SecurityContext.AllowPrivilegeEscalation = ptr.Bool(allowPrivilegedEscalationValue)
			}
			if c.SecurityContext.Capabilities == nil {
				c.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
			}
		}
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// CopyConfigMap will copy all the values from the passed configmap to the configmap
// in the manifest and any extra fields will be added in the manifest
func CopyConfigMap(configMapName string, expectedValues map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		kind := strings.ToLower(u.GetKind())
		if kind != "configmap" {
			return nil
		}
		if u.GetName() != configMapName || len(expectedValues) == 0 {
			return nil
		}

		cm := &corev1.ConfigMap{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, cm)
		if err != nil {
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}

		for key, value := range expectedValues {
			// updates values , if the key is found,
			// adds key and value, if the key is not found
			cm.Data[key] = value
		}
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

func ReplaceDeploymentArg(deploymentName, existingArg, newArg string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}
		if u.GetName() != deploymentName {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i, arg := range d.Spec.Template.Spec.Containers[0].Args {
			if arg == existingArg {
				d.Spec.Template.Spec.Containers[0].Args[i] = newArg
			}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}

// replaces the namespace in serviceAccount
func ReplaceNamespaceInServiceAccount(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ServiceAccount" {
			return nil
		}

		// update namespace
		u.SetNamespace(targetNamespace)

		return nil
	}
}

// replaces the namespace in clusterRoleBinding
func ReplaceNamespaceInClusterRoleBinding(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ClusterRoleBinding" {
			return nil
		}

		crb := &rbacv1.ClusterRoleBinding{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, crb)
		if err != nil {
			return err
		}

		// update namespace
		for index := range crb.Subjects {
			crb.Subjects[index].Namespace = targetNamespace
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)
		return nil
	}
}

// updates "metadata.namespace" and under "spec"
// TODO: we have different transformer for each kind
// TODO: replaces all the existing transformers(used to update namespace) with this.
func ReplaceNamespace(newNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// update metadata.namespace for all the resources
		// this change will be updated in cluster wide resource too
		// there is no effect on updating namespace on cluster wide resource
		u.SetNamespace(newNamespace)

		switch u.GetKind() {
		case "ClusterRoleBinding":
			crb := &rbacv1.ClusterRoleBinding{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, crb)
			if err != nil {
				return err
			}

			// update namespace
			for index := range crb.Subjects {
				crb.Subjects[index].Namespace = newNamespace
			}

			obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crb)
			if err != nil {
				return err
			}
			u.SetUnstructuredContent(obj)
		}

		return nil
	}
}

// AddSecretData adds the given data and annotations to the Secret object.
func AddSecretData(data map[string][]byte, annotations map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// If input data is empty, do not transform
		if len(data) == 0 {
			return nil
		}

		// Check if the resource is a Secret
		if u.GetKind() != "Secret" {
			return nil
		}

		// Convert unstructured to Secret
		secret := &corev1.Secret{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, secret)
		if err != nil {
			return err
		}

		// Update the Secret's data only if it is nil or empty
		if len(secret.Data) == 0 {
			secret.Data = data
		}

		// Update the Secret's annotations
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		for key, value := range annotations {
			secret.Annotations[key] = value
		}

		// Convert back to unstructured
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
		if err != nil {
			return err
		}

		// Update the original unstructured object
		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// ConvertDeploymentToStatefulSet converts a Deployment to a StatefulSet with given parameters
func ConvertDeploymentToStatefulSet(controllerName, serviceName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != controllerName {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		ss := &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: appsv1.SchemeGroupVersion.Group + "/" + appsv1.SchemeGroupVersion.Version,
			},
			ObjectMeta: d.ObjectMeta,
			Spec: appsv1.StatefulSetSpec{
				Selector:    d.Spec.Selector,
				ServiceName: serviceName,
				Template:    d.Spec.Template,
				Replicas:    d.Spec.Replicas,
				UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
					Type: appsv1.RollingUpdateStatefulSetStrategyType,
				},
			},
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ss)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// AddStatefulEnvVars adds environment variables to the statefulset based on given parameters
func AddStatefulEnvVars(controllerName, serviceName, statefulServiceEnvVar, controllerOrdinalEnvVar string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "StatefulSet" || u.GetName() != controllerName {
			return nil
		}

		ss := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, ss)
		if err != nil {
			return err
		}

		newEnvVars := []corev1.EnvVar{
			{
				Name:  statefulServiceEnvVar,
				Value: serviceName,
			},
			{
				Name: controllerOrdinalEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		}

		if len(ss.Spec.Template.Spec.Containers) > 0 {
			ss.Spec.Template.Spec.Containers[0].Env = append(ss.Spec.Template.Spec.Containers[0].Env, newEnvVars...)
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ss)
		if err != nil {
			return err
		}

		u.SetUnstructuredContent(unstrObj)

		return nil
	}
}

// updates performance flags/args into deployment and container given as args
// and leader election config as pod labels into a Deployment, ensuring that any changes trigger a rollout.
// It also updates the replica count if specified in the performanceSpec.
func UpdatePerformanceFlagsInDeploymentAndLeaderConfigMap(performanceSpec *v1alpha1.PerformanceProperties, leaderConfig, deploymentName, containerName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != deploymentName {
			return nil
		}

		// holds the flags needs to be added in the container args section
		flags := map[string]interface{}{}

		// convert struct to map with json tag
		// so that, we can map the arguments as is
		if err := StructToMap(&performanceSpec.DeploymentPerformanceArgs, &flags); err != nil {
			return err
		}

		// if there is no flags to update, return from here
		if len(flags) == 0 {
			return nil
		}

		// convert unstructured object to deployment
		dep := &appsv1.Deployment{}
		err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, dep)
		if err != nil {
			return err
		}

		// include config-leader-election data into deployment pod label
		// so that pods will be recreated, if there is a change in "buckets"
		leaderElectionConfigMapData := map[string]interface{}{}
		if err = StructToMap(&performanceSpec.PerformanceLeaderElectionConfig, &leaderElectionConfigMapData); err != nil {
			return err
		}
		podLabels := dep.Spec.Template.Labels
		if podLabels == nil {
			podLabels = map[string]string{}
		}
		// sort data keys in an order, to get the consistent hash value in installerset
		labelKeys := getSortedKeys(leaderElectionConfigMapData)
		for _, key := range labelKeys {
			value := leaderElectionConfigMapData[key]
			labelKey := fmt.Sprintf("%s.data.%s", leaderConfig, key)
			podLabels[labelKey] = fmt.Sprintf("%v", value)
		}
		dep.Spec.Template.Labels = podLabels

		// update replicas, if available
		if performanceSpec.Replicas != nil {
			dep.Spec.Replicas = ptr.Int32(*performanceSpec.Replicas)
		}

		// include it in the pods label, that will recreate all the pods, if there is a change in replica count
		if dep.Spec.Replicas != nil {
			dep.Spec.Template.Labels["deployment.spec.replicas"] = fmt.Sprintf("%d", *dep.Spec.Replicas)
		}

		// sort flag keys in an order, to get the consistent hash value in installerset
		flagKeys := getSortedKeys(flags)
		// update performance arguments into target container
		for containerIndex, container := range dep.Spec.Template.Spec.Containers {
			if container.Name != containerName {
				continue
			}
			for _, flagKey := range flagKeys {
				// update the arg name with "-" prefix
				expectedArg := fmt.Sprintf("-%s", flagKey)
				argStringValue := fmt.Sprintf("%v", flags[flagKey])

				// skip deprecated disable-ha flag if not pipelinesControllerDeployment
				// should be removed when the flag is removed from pipelines controller
				// we can use this logic incase we need to skip it for other controllers as well here
				if deploymentName != pipelinesControllerDeployment && flagKey == "disable-ha" {
					continue
				}

				argUpdated := false
				for argIndex, existingArg := range container.Args {
					if strings.HasPrefix(existingArg, expectedArg) {
						container.Args[argIndex] = fmt.Sprintf("%s=%s", expectedArg, argStringValue)
						argUpdated = true
						break
					}
				}
				if !argUpdated {
					container.Args = append(container.Args, fmt.Sprintf("%s=%s", expectedArg, argStringValue))
				}
			}
			dep.Spec.Template.Spec.Containers[containerIndex] = container
		}

		// convert deployment to unstructured object
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(dep)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(obj)

		return nil
	}
}

// sort keys in an order, to get the consistent hash value in installerset
func getSortedKeys(input map[string]interface{}) []string {
	keys := []string{}
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// replaces the namespace in ValidatingWebhookConfiguration namespaceSelector
func ReplaceNamespaceInWebhookNamespaceSelector(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if !strings.EqualFold(u.GetKind(), "ValidatingWebhookConfiguration") {
			return nil
		}
		// Accept either spec.webhooks (some older patterns) or top-level webhooks (current API structure).
		webhooks, foundSpec, errSpec := unstructured.NestedSlice(u.Object, "spec", "webhooks")
		pathIsSpec := true
		if errSpec != nil || !foundSpec {
			// Fallback to top-level
			webhooksTop, foundTop, errTop := unstructured.NestedSlice(u.Object, "webhooks")
			if errTop != nil || !foundTop {
				// Nothing to transform
				return nil
			}
			webhooks = webhooksTop
			pathIsSpec = false
		}
		changed := false
		for i := range webhooks {
			wh, ok := webhooks[i].(map[string]interface{})
			if !ok {
				continue
			}
			nsSel, okNs := wh["namespaceSelector"].(map[string]interface{})
			if !okNs {
				continue
			}
			matchExprs, okExpr := nsSel["matchExpressions"].([]interface{})
			if !okExpr {
				continue
			}
			for j := range matchExprs {
				expr, ok := matchExprs[j].(map[string]interface{})
				if !ok {
					continue
				}
				values, okVals := expr["values"].([]interface{})
				if !okVals || len(values) == 0 {
					continue
				}
				for k := range values {
					valStr, ok := values[k].(string)
					if !ok || targetNamespace == "" {
						continue
					}
					if strings.Contains(valStr, DefaultTargetNamespace) {
						newVal := strings.ReplaceAll(valStr, DefaultTargetNamespace, targetNamespace)
						if newVal != valStr {
							values[k] = newVal
							changed = true
						}
					}
				}
				expr["values"] = values
				matchExprs[j] = expr
			}
			nsSel["matchExpressions"] = matchExprs
			wh["namespaceSelector"] = nsSel
			webhooks[i] = wh
		}
		if changed {
			if pathIsSpec {
				_ = unstructured.SetNestedSlice(u.Object, webhooks, "spec", "webhooks")
			} else {
				// Top-level assignment
				u.Object["webhooks"] = webhooks
			}
		}
		return nil
	}
}
