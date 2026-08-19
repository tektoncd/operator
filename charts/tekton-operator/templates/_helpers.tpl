{{/*
Expand the name of the chart.
*/}}
{{- define "tekton-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "tekton-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "tekton-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "tekton-operator.labels" -}}
helm.sh/chart: {{ include "tekton-operator.chart" . }}
app.kubernetes.io/name: {{ include "tekton-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.operator.deployment.customLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels for operator component
*/}}
{{- define "tekton-operator.operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tekton-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
Pod Template labels for operator component
*/}}
{{- define "tekton-operator.operator.podTemplateLabels" -}}
{{- with .Values.operator.deployment.podTemplateCustomLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels for webhook component
*/}}
{{- define "tekton-operator.webhook.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tekton-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: webhook
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "tekton-operator.serviceAccountName" -}}
{{- if .Values.rbac.create }}
{{- default (include "tekton-operator.fullname" .) .Values.rbac.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "tekton-operator.operator-name" -}}
{{- if .Values.operator.operatorName -}}
{{- .Values.operator.operatorName -}}
{{- else -}}
{{- if .Values.openshift.enabled -}}
redhat-openshift-pipelines-operator
{{- else -}}
tekton-operator
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "tekton-operator.controllers" -}}
{{- if .Values.openshift.enabled -}}
tektonconfig,tektonpipeline,tektontrigger,tektonchain,tektonaddon,tektonresult,openshiftpipelinesascode,manualapprovalgate,tektonpruner,tektonscheduler,tektonmulticlusterproxyaae,syncerservice
{{- else -}}
tektonconfig,tektonpipeline,tektontrigger,tektonchain,tektonresult,tektondashboard,manualapprovalgate,tektonpruner,tektonscheduler,tektonmulticlusterproxyaae,openshiftpipelinesascode
{{- end -}}
{{- end -}}

{{- define "tekton-operator.validateTargetNamespace" -}}
{{- if and .Values.openshift.enabled .Values.operator.defaultTargetNamespace (ne .Values.operator.defaultTargetNamespace "openshift-pipelines") -}}
{{- fail (printf "operator.defaultTargetNamespace must be \"openshift-pipelines\" when openshift.enabled=true (got %q). The openshift addon sample pipelines hardcode that namespace, so a custom value breaks them; this is also first-install-only - once the TektonConfig CR exists the operator never re-reads it. To change the target namespace, set spec.targetNamespace on the TektonConfig CR (requires deleting and recreating the CR)." .Values.operator.defaultTargetNamespace) -}}
{{- end -}}
{{- end -}}

{{- define "tekton-operator.operator-image" -}}
{{- $tag := default .Chart.AppVersion .Values.operator.image.tag -}}
{{- $image := "" -}}
{{- if .Values.operator.image.repository -}}
  {{- $image = .Values.operator.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "ghcr.io/tektoncd/operator/operator-1d69a75f22dd094880847eac907fb2c1" -}}
  {{- else -}}
  {{- $image = "ghcr.io/tektoncd/operator/operator-303303c315a48490ba6517859ef65b77" -}}
  {{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}

{{- define "tekton-operator.pruner-image" -}}
{{- if contains "sha256:" .Values.pruner.image.tag -}}
{{- printf "%s@%s" .Values.pruner.image.repository .Values.pruner.image.tag -}}
{{- else -}}
{{- printf "%s:%s" .Values.pruner.image.repository .Values.pruner.image.tag -}}
{{- end -}}
{{- end -}}

{{- define "tekton-operator.webhook-image" -}}
{{- $tag := default .Chart.AppVersion .Values.webhook.image.tag -}}
{{- $image := "" -}}
{{- if .Values.webhook.image.repository -}}
  {{- $image = .Values.webhook.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "ghcr.io/tektoncd/operator/webhook-340ad78e88ca5477447aa144fedfe1a1" -}}
  {{- else -}}
  {{- $image = "ghcr.io/tektoncd/operator/webhook-f2bb711aa8f0c0892856a4cbf6d9ddd8" -}}
  {{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}


{{- define "tekton-operator.kubernetesMinVersionEnv" -}}
{{- if .Values.kubernetesMinVersion }}
- name: KUBERNETES_MIN_VERSION
  value: {{ .Values.kubernetesMinVersion | quote }}
{{- end }}
{{- end }}

{{- define "tekton-operator.webhook-proxy-image" -}}
{{- $tag := default .Chart.AppVersion .Values.webhookProxy.image.tag -}}
{{- $image := "" -}}
{{- if .Values.webhookProxy.image.repository -}}
  {{- $image = .Values.webhookProxy.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "ghcr.io/tektoncd/operator/proxy-webhook-f8f95c9cea9508fe8915ae3d012d15fb" -}}
  {{- else -}}
  {{- $image = "ghcr.io/tektoncd/operator/proxy-webhook-f6167da7bc41b96a27c5529f850e63d1" -}}
  {{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}
