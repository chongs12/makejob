{{/*
MakeJob Helm Chart 通用 helpers
对应 docs/observability-k8s-checklist.md 5.2
*/}}

{{- define "makejob.fullname" -}}
{{- .Release.Name -}}
{{- end -}}

{{- define "makejob.labels" -}}
app.kubernetes.io/name: makejob
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{- define "makejob.selectorLabels" -}}
app.kubernetes.io/name: makejob
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "makejob.serviceName" -}}
{{- .Release.Name }}-{{ .svcName -}}
{{- end -}}

{{- define "makejob.image" -}}
{{- $registry := $.root.Values.global.imageRegistry -}}
{{- if $registry -}}{{ $registry }}/{{- end -}}
makejob/{{ .svcName }}:{{ $.root.Values.global.imageTag }}
{{- end -}}
