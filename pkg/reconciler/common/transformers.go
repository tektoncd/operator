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
	"os"
	"reflect"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

const (
	AnnotationPreserveNS          = "operator.tekton.dev/preserve-namespace"
	AnnotationPreserveRBSubjectNS = "operator.tekton.dev/preserve-rb-subject-namespace"
	PipelinesImagePrefix          = "IMAGE_PIPELINES_"
	TriggersImagePrefix           = "IMAGE_TRIGGERS_"
	AddonsImagePrefix             = "IMAGE_ADDONS_"
	PacImagePrefix                = "IMAGE_PAC_"
	ChainsImagePrefix             = "IMAGE_CHAINS_"
	ManualApprovalGatePrefix      = "IMAGE_MAG_"
	ResultsImagePrefix            = "IMAGE_RESULTS_"
	HubImagePrefix                = "IMAGE_HUB_"

	ArgPrefix   = "arg_"
	ParamPrefix = "param_"

	resultAPIDeployment     = "tekton-results-api"
	resultWatcherDeployment = "tekton-results-watcher"

	runAsNonRootValue              = true
	allowPrivilegedEscalationValue = false
)

// transformers that are common to all components.
func transformers(ctx context.Context, obj v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{
		mf.InjectOwner(obj),
		injectNamespaceConditional(AnnotationPreserveNS, obj.GetSpec().GetTargetNamespace()),
		injectNamespaceCRDWebhookClientConfig(obj.GetSpec().GetTargetNamespace()),
		injectNamespaceCRClusterInterceptorClientConfig(obj.GetSpec().GetTargetNamespace()),
		injectNamespaceClusterRole(obj.GetSpec().GetTargetNamespace()),
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
					if rn.(string) == "tekton-pipelines" {
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

// ReplaceNamespaceInDeploymentEnv replaces namespace in deployment's env var
func ReplaceNamespaceInDeploymentEnv(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || !(u.GetName() == resultAPIDeployment || u.GetName() == resultWatcherDeployment) {
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
			envs[i].Value = strings.ReplaceAll(e.Value, "tekton-pipelines", targetNamespace)
		}
	}
	return envs
}

// ReplaceNamespaceInDeploymentArgs replaces namespace in deployment's args
func ReplaceNamespaceInDeploymentArgs(targetNamespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != resultWatcherDeployment {
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
		if strings.Contains(a, "tekton-pipelines") {
			container.Args[i] = strings.ReplaceAll(a, "tekton-pipelines", targetNamespace)
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
