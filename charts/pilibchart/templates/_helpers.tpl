{{/*
Expand the name of the chart.
*/}}
{{- define "helper.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "helper.fullname" -}}
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
{{- define "helper.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "helper.labels" -}}
helm.sh/chart: {{ include "helper.chart" . }}
{{ include "helper.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "helper.selectorLabels" -}}
app.kubernetes.io/name: {{ include "helper.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{ with .Values.labels }}
{{- . | toYaml -}}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "helper.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "helper.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "pilibchart.volumes" }}
{{- $fullName := include "helper.fullname" . -}}
volumes:
  {{- if (.Values.iscsi) }}
  {{- range .Values.iscsi }}
  - name: {{ .name }}
    iscsi:
      chapAuthSession: false
      targetPortal: {{ .targetPortal }}
      {{- if (.portals) }}
      portals: {{ .portals }}
      {{- end }}
      iqn: {{ .iqn }}
      lun: {{ .lun }}
      fsType: {{ .fsType }}
      readOnly: {{ .readOnly }}
  {{- end }}
  {{- end }}
  {{- range .Values.pvc }}
  - name: {{ .name }}
    persistentVolumeClaim:
      {{- if .remote }}
      claimName: {{ .name }}
      {{- else }}
      claimName: {{ $fullName }}-{{ .name }}
      {{- end}}
  {{- end }}
  {{- range .Values.configMapVolumes }}
  {{- if .name }}
  - name: {{ .name }}
    configMap:
      name: {{ .name }}
  {{- else }}
  - name: {{ $fullName }}
    configMap:
      name: {{ $fullName }}
  {{- end }}
  {{- end }}
  {{- range .Values.secretVolumes }}
  - name: {{ .name }}
    secret:
      secretName: {{ .secretName }}
  {{- end }}
{{- end}}

{{- define "pilibchart.volumeMounts" }}
{{- $fullName := include "helper.fullname" . -}}
volumeMounts:
  {{- range (concat (list) (default (list) .Values.iscsi) (default (list) .Values.pvc)) }}
  - mountPath: {{ .mountPath }}
    name: {{ .name }}
    {{- with .readOnly }}
    readOnly: {{ . }}
    {{- end }}
    {{- with .subPath }}
    subPath: {{ . }}
    {{- end }}
  {{- end }}
  {{- range .Values.configMapVolumes }}
  {{- $cmvName := "" }}
  {{- if .name }}
  {{- $cmvName = .name }}
  {{- else }}
  {{- $cmvName = $fullName }}
  {{- end }}
  {{- range .mounts }}
  - mountPath: {{ .mountPath }}
    name: {{ $cmvName }}
    {{- with .subPath }}
    subPath: {{ . }}
    {{- end }}
  {{- end }}
  {{- end }}
  {{- range .Values.secretVolumes }}
  - mountPath: {{ .mountPath }}
    name: {{ .name }}
    readOnly: true
    {{- with .subPath }}
    subPath: {{ . }}
    {{- end }}
  {{- end }}
{{- end }}
