/*
Copyright 2021 The Tekton Authors

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
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// RemoveRunAsUser will remove RunAsUser from all container in a deployment
func RemoveRunAsUser() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			if c.SecurityContext != nil {
				// Remove runAsUser
				c.SecurityContext.RunAsUser = nil
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

// RemoveRunAsGroup will remove runAsGroup from all container in a deployment
func RemoveRunAsGroup() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			if c.SecurityContext != nil {
				// Remove runAsGroup
				c.SecurityContext.RunAsGroup = nil
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

// RemoveFsGroup will remove FsGroup in a deployment
func RemoveFsGroup(obj string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" {
			return nil
		}

		d := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d)
		if err != nil {
			return err
		}

		if d.Name == obj {
			if d.Spec.Template.Spec.SecurityContext != nil {
				d.Spec.Template.Spec.SecurityContext = nil
			}

			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
			if err != nil {
				return err
			}
			u.SetUnstructuredContent(unstrObj)

			return nil
		}

		return nil
	}
}

// AddDeploymentRestrictedPSA will add the default restricted spec on Deployment to remove errors/warning
func AddDeploymentRestrictedPSA() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		runAsNonRoot := true
		allowPrivilegedEscalation := false

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
		d.Spec.Template.Spec.SecurityContext.RunAsNonRoot = &runAsNonRoot

		for i := range d.Spec.Template.Spec.Containers {
			c := &d.Spec.Template.Spec.Containers[i]
			if c.SecurityContext == nil {
				c.SecurityContext = &corev1.SecurityContext{}
			}
			c.SecurityContext.AllowPrivilegeEscalation = &allowPrivilegedEscalation
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

// AddJobRestrictedPSA will add the default restricted spec on Job to remove errors/warning
func AddJobRestrictedPSA() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		runAsNonRoot := true
		allowPrivilegedEscalation := false

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
		jb.Spec.Template.Spec.SecurityContext.RunAsNonRoot = &runAsNonRoot

		for i := range jb.Spec.Template.Spec.Containers {
			c := &jb.Spec.Template.Spec.Containers[i]
			if c.SecurityContext == nil {
				c.SecurityContext = &corev1.SecurityContext{}
			}
			c.SecurityContext.AllowPrivilegeEscalation = &allowPrivilegedEscalation
			c.SecurityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
		}

		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jb)
		if err != nil {
			return err
		}
		u.SetUnstructuredContent(unstrObj)
		return nil
	}
}
