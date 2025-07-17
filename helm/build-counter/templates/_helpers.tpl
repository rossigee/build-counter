{{/*
Expand the name of the chart.
*/}}
{{- define "build-counter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "build-counter.fullname" -}}
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
{{- define "build-counter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "build-counter.labels" -}}
helm.sh/chart: {{ include "build-counter.chart" . }}
{{ include "build-counter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "build-counter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "build-counter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "build-counter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "build-counter.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the ConfigMap to use for lightweight mode
*/}}
{{- define "build-counter.configMapName" -}}
{{- if .Values.storage.configmap.name }}
{{- .Values.storage.configmap.name }}
{{- else }}
{{- include "build-counter.fullname" . }}
{{- end }}
{{- end }}

{{/*
Create the namespace for ConfigMap in lightweight mode
*/}}
{{- define "build-counter.configMapNamespace" -}}
{{- if .Values.storage.configmap.namespace }}
{{- .Values.storage.configmap.namespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Determine storage mode args
*/}}
{{- define "build-counter.storageArgs" -}}
{{- if eq .Values.storage.mode "lightweight" }}
- --lightweight
{{- end }}
{{- end }}

{{/*
Environment variables for storage configuration
*/}}
{{- define "build-counter.storageEnv" -}}
{{- if eq .Values.storage.mode "database" }}
{{- if .Values.storage.database.url }}
- name: DATABASE_URL
  value: {{ .Values.storage.database.url | quote }}
{{- else if .Values.storage.database.secretName }}
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: {{ .Values.storage.database.secretName }}
      key: {{ .Values.storage.database.secretKey }}
{{- end }}
{{- else if eq .Values.storage.mode "lightweight" }}
- name: NAMESPACE
  value: {{ include "build-counter.configMapNamespace" . | quote }}
- name: CONFIGMAP_NAME
  value: {{ include "build-counter.configMapName" . | quote }}
{{- end }}
{{- end }}

{{/*
Environment variables for OpenTelemetry
*/}}
{{- define "build-counter.otelEnv" -}}
{{- if .Values.opentelemetry.enabled }}
{{- if .Values.opentelemetry.endpoint }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ .Values.opentelemetry.endpoint | quote }}
{{- end }}
{{- if .Values.opentelemetry.serviceName }}
- name: OTEL_SERVICE_NAME
  value: {{ .Values.opentelemetry.serviceName | quote }}
{{- end }}
{{- if .Values.opentelemetry.serviceVersion }}
- name: OTEL_SERVICE_VERSION
  value: {{ .Values.opentelemetry.serviceVersion | quote }}
{{- end }}
{{- if .Values.opentelemetry.insecure }}
- name: OTEL_EXPORTER_OTLP_INSECURE
  value: "true"
{{- end }}
{{- end }}
{{- end }}

{{/*
Image name
*/}}
{{- define "build-counter.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.image.registry -}}
{{- $repository := .Values.image.repository -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion -}}
{{- if $registry -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- else -}}
{{- printf "%s:%s" $repository $tag -}}
{{- end -}}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "build-counter.imagePullSecrets" -}}
{{- with .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}