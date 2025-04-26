{{/*
Expand the name of the chart.
*/}}
{{- define "kube-jit-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kube-jit-api.fullname" -}}
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
{{- define "kube-jit-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kube-jit-api.labels" -}}
helm.sh/chart: {{ include "kube-jit-api.chart" . }}
{{ include "kube-jit-api.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kube-jit-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kube-jit-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kube-jit-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kube-jit-api.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Allowed cluster configMap keys
*/}}
{{- define "allowedClusterKeys" -}}
name
host
ca
insecure
tokenSecret
type
projectID
region
{{- end -}}

{{/*
Allowed roles configMap keys
*/}}
{{- define "allowedRoleKeys" -}}
name
{{- end -}}

{{/*
Allowed approver teams configMap keys
*/}}
{{- define "allowedApproverTeamKeys" -}}
name
id
{{- end -}}

{{/*
Used for configMap key validation
*/}}
{{- define "has" -}}
{{- $slice := splitList "\n" (index . 0 | trim) -}}
{{- $value := index . 1 -}}
{{- $found := false -}}
{{- range $slice }}
  {{- if eq . $value }}
    {{- $found = true }}
  {{- end }}
{{- end }}
{{- if not $found }}
  {{- fail (printf "Key %s not found in allowed keys: %v" $value $slice) }}
{{- end }}
{{- $found }}
{{- end -}}

{{/*
Check if a value is in a list of allowed values.
*/}}
{{- define "isValidType" -}}
{{- $validTypes := list "gke" "generic" -}}
{{- $type := . -}}
{{- if has $type $validTypes -}}
true
{{- else -}}
false
{{- end -}}
{{- end }}
