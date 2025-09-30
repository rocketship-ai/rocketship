{{- define "rocketship.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "rocketship.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "rocketship.name" . -}}
{{- if eq $name .Release.Name -}}
{{- $name -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "rocketship.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "rocketship.labels" -}}
helm.sh/chart: {{ include "rocketship.chart" . }}
{{ include "rocketship.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- end -}}

{{- define "rocketship.selectorLabels" -}}
app.kubernetes.io/name: {{ include "rocketship.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "rocketship.engine.fullname" -}}
{{- printf "%s-engine" (include "rocketship.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "rocketship.worker.fullname" -}}
{{- printf "%s-worker" (include "rocketship.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "rocketship.webProxy.fullname" -}}
{{- printf "%s-web-oauth2-proxy" (include "rocketship.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "rocketship.authbroker.fullname" -}}
{{- printf "%s-auth-broker" (include "rocketship.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
