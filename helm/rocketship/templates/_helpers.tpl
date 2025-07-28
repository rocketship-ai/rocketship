{{/*
Expand the name of the chart.
*/}}
{{- define "rocketship.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "rocketship.fullname" -}}
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
{{- define "rocketship.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "rocketship.labels" -}}
helm.sh/chart: {{ include "rocketship.chart" . }}
{{ include "rocketship.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "rocketship.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rocketship.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Engine labels
*/}}
{{- define "rocketship.engine.labels" -}}
helm.sh/chart: {{ include "rocketship.chart" . }}
{{ include "rocketship.engine.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Engine selector labels
*/}}
{{- define "rocketship.engine.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rocketship.name" . }}-engine
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: engine
{{- end }}

{{/*
Worker labels
*/}}
{{- define "rocketship.worker.labels" -}}
helm.sh/chart: {{ include "rocketship.chart" . }}
{{ include "rocketship.worker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Worker selector labels
*/}}
{{- define "rocketship.worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rocketship.name" . }}-worker
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: worker
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "rocketship.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "rocketship.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Engine image name
*/}}
{{- define "rocketship.engine.image" -}}
{{- $registryName := .Values.global.imageRegistry | default .Values.rocketship.engine.image.registry -}}
{{- $repositoryName := .Values.rocketship.engine.image.repository -}}
{{- $tag := .Values.rocketship.engine.image.tag | default .Chart.AppVersion | toString -}}
{{- if $registryName }}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- else }}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
Worker image name
*/}}
{{- define "rocketship.worker.image" -}}
{{- $registryName := .Values.global.imageRegistry | default .Values.rocketship.worker.image.registry -}}
{{- $repositoryName := .Values.rocketship.worker.image.repository -}}
{{- $tag := .Values.rocketship.worker.image.tag | default .Chart.AppVersion | toString -}}
{{- if $registryName }}
{{- printf "%s/%s:%s" $registryName $repositoryName $tag -}}
{{- else }}
{{- printf "%s:%s" $repositoryName $tag -}}
{{- end }}
{{- end }}

{{/*
Auth database host
*/}}
{{- define "rocketship.auth.database.host" -}}
{{- if .Values.authPostgresql.enabled }}
{{- printf "%s" .Values.authPostgresql.fullnameOverride | default (printf "%s-auth-postgresql" (include "rocketship.fullname" .)) }}
{{- else }}
{{- .Values.auth.database.host }}
{{- end }}
{{- end }}

{{/*
Auth database password secret name
*/}}
{{- define "rocketship.auth.database.secretName" -}}
{{- if .Values.auth.database.existingSecret }}
{{- .Values.auth.database.existingSecret }}
{{- else }}
{{- include "rocketship.fullname" . }}-auth-db
{{- end }}
{{- end }}

{{/*
OIDC secret name
*/}}
{{- define "rocketship.auth.oidc.secretName" -}}
{{- if .Values.auth.oidc.existingSecret }}
{{- .Values.auth.oidc.existingSecret }}
{{- else }}
{{- include "rocketship.fullname" . }}-oidc
{{- end }}
{{- end }}


{{/*
Temporal host
*/}}
{{- define "rocketship.temporal.host" -}}
{{- if .Values.temporal.enabled }}
{{- printf "%s-temporal-frontend:7233" .Release.Name }}
{{- else }}
{{- .Values.temporal.externalHost | default "temporal:7233" }}
{{- end }}
{{- end }}

{{/*
PostgreSQL host
*/}}
{{- define "rocketship.postgresql.host" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- .Values.postgresql.externalHost | default "postgresql" }}
{{- end }}
{{- end }}

{{/*
Elasticsearch host
*/}}
{{- define "rocketship.elasticsearch.host" -}}
{{- if .Values.elasticsearch.enabled }}
{{- printf "%s-elasticsearch-master" .Release.Name }}
{{- else }}
{{- .Values.elasticsearch.externalHost | default "elasticsearch" }}
{{- end }}
{{- end }}