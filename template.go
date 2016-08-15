package sqlfmt

const beginTmpl = `
CREATE TABLE {{with .Schema.O}}{{.}}.{{end}}{{.Name.O}} (
`

const colDefTmpl = `
{{- range $i, $c := .Columns}}
    {{$c.Name}} {{$c.FieldType.String | fmtType}}
    {{- if $c.NotNull}} NOT NULL{{end}},
{{- end}}
`

const idxDefTmpl = `
{{- define "idx_cols"}}
	{{- range $i, $col := . }}
		{{- if ne $i 0}}, {{end}}{{.Name}}{{if ne .Length -1}}({{.Length}}){{end}}
	{{- end}}
{{- end}}

{{- range $i, $idx := .Indices}}
    {{- if .Primary}}
    PRIMARY KEY ({{template "idx_cols" .Columns}}),
    {{- else if .Unique}}
    UNIQUE {{.Name.O}}({{template "idx_cols" .Columns}}),
    {{- else}}
    {{if .Fulltext}}FULLTEXT {{end}}INDEX {{.Name.O}}({{template "idx_cols" .Columns}}),
    {{- end}}
{{- end}}
`

const refDefTmpl = `
{{- range $i, $fk := .ForeignKeys}}
    FOREIGN KEY {{.Name.O}}({{range $j, $col := .Cols}}{{if ne $j 0}}, {{end}}{{.}}{{end}})
    {{- "" }} REFERENCES {{with .RefSchema.O}}{{.}}.{{end}}{{.RefTable}}({{range $j, $col := .RefCols}}{{if ne $j 0}}, {{end}}{{.}}{{end}}),
{{- end}}
`

const endTmpl = `
)
{{- with .Engine}} ENGINE={{.}}{{end}}
{{- with .Charset}} CHARACTER SET={{.}}{{end}}
{{- with .Collate}} COLLATE={{.}}{{end}}
{{- with .RowFormat}} ROW_FORMAT={{.}}{{end}}
{{- with .AvgRowLength}} AVG_ROW_LENGTH={{.}}{{end}}
{{- with .KeyBlockSize}} KEY_BLOCK_SIZE={{.}}{{end}}
{{- with .MinRows}} MIN_ROWS={{.}}{{end}}
{{- with .MaxRows}} MAX_ROWS={{.}}{{end}}
{{- with .AutoIncID}} AUTO_INCREMENT={{.}}{{end}}
{{- with .Comment}} COMMENT='{{.}}'{{end}};
`
