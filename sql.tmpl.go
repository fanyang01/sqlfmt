package main

const sqlTemplate = `
{{- $g := . -}}
{{- $t := .Name.O -}}
CREATE TABLE {{with .Schema.O}}{{.}}.{{end}}{{$t}} (
{{- range $i, $c := .Columns}}
    {{$c.Name}} {{$c.FieldType.String}}
    {{- if $c.NotNull}} NOT NULL{{end}},
{{- end}}
{{- range $i, $idx := .Indices}}
    {{- if .Primary}}
    PRIMARY KEY ({{range $j, $col := .Columns}}{{if ne $j 0}}, {{end}}{{.Name}}{{end}}),
    {{- else if .Unique}}
    UNIQUE {{with .Name.O}}{{.}}{{end}}({{range $j, $col := .Columns}}{{if ne $j 0}}, {{end}}{{.Name}}{{end}}),
    {{- else}}
    {{if .Fulltext}}FULLTEXT {{end}}INDEX {{with .Name.O}}{{.}}{{end}}({{range $j, $col := .Columns}}{{if ne $j 0}}, {{end}}{{.Name}}{{end}}),
    {{- end}}
{{- end}}
{{- range $i, $fk := .ForeignKeys}}
    FOREIGN KEY {{.Name.O}}({{range $j, $col := .Cols}}{{if ne $j 0}}, {{end}}{{.}}{{end}})
    {{- "" }} REFERENCES {{with .RefSchema.O}}{{.}}.{{end}}{{.RefTable}}({{range $j, $col := .RefCols}}{{if ne $j 0}}, {{end}}{{.}}{{end}}),
{{- end}}
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
