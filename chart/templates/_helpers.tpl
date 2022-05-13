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

{{- define "tekton-operator.operator-image" -}}
{{- $tag := default .Chart.AppVersion .Values.operator.image.tag -}}
{{- $image := "" -}}
{{- if .Values.operator.image.repository -}}
  {{- $image = .Values.operator.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/openshift/operator" -}}
{{- else -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/kubernetes/operator" -}}
{{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}

{{- define "tekton-operator.webhook-image" -}}
{{- $tag := default .Chart.AppVersion .Values.webhook.image.tag -}}
{{- $image := "" -}}
{{- if .Values.webhook.image.repository -}}
  {{- $image = .Values.webhook.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/openshift/webhook" -}}
{{- else -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/kubernetes/webhook" -}}
{{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}


{{- define "tekton-operator.webhook-proxy-image" -}}
{{- $tag := default .Chart.AppVersion .Values.webhookProxy.image.tag -}}
{{- $image := "" -}}
{{- if .Values.webhookProxy.image.repository -}}
  {{- $image = .Values.webhookProxy.image.repository }}
{{- else -}}
{{- if .Values.openshift.enabled -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/openshift/proxy-webhook" -}}
{{- else -}}
  {{- $image = "gcr.io/tekton-releases/github.com/tektoncd/operator/cmd/kubernetes/proxy-webhook" -}}
{{- end -}}
{{- end -}}
{{- printf "%s:%s" $image $tag -}}
{{- end -}}
