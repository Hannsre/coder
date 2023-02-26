{{- /* Heavily inspired by the Go toolchain formatting. */ -}}
usage: {{.FullUsage}}

{{.Short}}
{{ with .Long}} {{.}} {{ end }}

{{- range $index, $group := optionGroups . }}
{{ with $group.Name }} {{- $group.Name }} Options{{ else -}} Options{{- end -}}:
{{- with $group.Description }} {{- . -}} {{ end }}
    {{- range $index, $option := $group.Options }}
    {{- with flagName $option }}
    --{{- . -}} {{ end }} {{- with $option.FlagShorthand }}, -{{- . -}} {{ end }}
    {{- with envName $option }}, ${{ . }} {{ end }}
    {{- with $option.Default }} (default: {{.}}) {{ end }}
        {{- with $option.Description }}
            {{- "" }}
{{ $desc := wordWrap $option.Description 60 -}} {{- indent $desc 2}}
        {{- end -}}
    {{- end }}
{{- end }}
