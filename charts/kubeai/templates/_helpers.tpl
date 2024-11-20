{{/*
Expand the name of the chart.
*/}}
{{- define "kubeai.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kubeai.fullname" -}}
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
{{- define "kubeai.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubeai.labels" -}}
helm.sh/chart: {{ include "kubeai.chart" . }}
{{ include "kubeai.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubeai.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubeai.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kubeai.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kubeai.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use for model pods
*/}}
{{- define "models.serviceAccountName" -}}
{{- if .Values.modelServiceAccount.create }}
{{- default (printf "%s-models" (include "kubeai.fullname" .)) .Values.modelServiceAccount.name }}
{{- else }}
{{- default "default" .Values.modelServiceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the aws secret to use
*/}}
{{- define "kubeai.awsSecretName" -}}
{{- if .Values.secrets.aws.create -}}
{{- if .Values.secrets.aws.name -}}
{{- .Values.secrets.aws.name -}}
{{- else }}
{{- (include "kubeai.fullname" .)}}-aws
{{- end}}
{{- else }}
{{- if not .Values.secrets.aws.name -}}
{{ fail "if secrets.aws.create is false, secrets.aws.name is required" }}
{{- end }}
{{- .Values.secrets.aws.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the huggingface secret to use
*/}}
{{- define "kubeai.huggingfaceSecretName" -}}
{{- if .Values.secrets.huggingface.create -}}
{{- if .Values.secrets.huggingface.name -}}
{{- .Values.secrets.huggingface.name -}}
{{- else }}
{{- (include "kubeai.fullname" .)}}-huggingface
{{- end}}
{{- else }}
{{- if not .Values.secrets.huggingface.name -}}
{{ fail "if secrets.huggingface.create is false, secrets.huggingface.name is required" }}
{{- end }}
{{- .Values.secrets.huggingface.name }}
{{- end }}
{{- end }}

{{/*
Set the name of the configmap to use for storing model autoscaling state
*/}}
{{- define "models.autoscalerStateConfigMapName" -}}
{{- default (printf "%s-autoscaler-state" (include "kubeai.fullname" .)) .Values.modelAutoscaling.stateConfigMapName }}
{{- end }}