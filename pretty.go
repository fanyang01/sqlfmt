package sqlfmt

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/kr/text"
)

func pretty(seg map[string]string) string {
	for _, name := range []string{"ref-def", "idx-def", "col-def"} {
		segment := seg[name]
		if strings.HasSuffix(segment, ",") {
			seg[name] = segment[:len(segment)-1]
			break
		}
	}

	var (
		buf  = new(bytes.Buffer)
		w    = new(tabwriter.Writer)
		trim = strings.TrimSpace
	)
	w.Init(buf, 0, 4, 2, ' ', 0)
	for _, line := range fieldsLine(seg["col-def"]) {
		words := strings.Fields(line)
		fmt.Fprintf(w, "%s\t%s\n", words[0], strings.Join(words[1:], " "))
	}
	w.Flush()
	seg["col-def"] = buf.String()

	buf.Reset()
	w.Init(buf, 0, 4, 1, ' ', 0)

	for _, line := range fieldsLine(seg["idx-def"]) {
		line = trim(line)
		for _, prefix := range []string{
			"PRIMARY KEY", "INDEX", "KEY",
			"UNIQUE INDEX", "UNIQUE KEY", "UNIQUE",
			"FULLTEXT INDEX", "FULLTEXT KEY", "FULLTEXT",
		} {
			if strings.HasPrefix(line, prefix) {
				line = strings.TrimPrefix(line, prefix)
				fmt.Fprintf(w, "%s\t%s\n", prefix, trim(line))
				break
			}
		}
	}
	for _, line := range fieldsLine(seg["ref-def"]) {
		prefix := "FOREIGN KEY"
		rest := strings.TrimPrefix(trim(line), prefix)
		fmt.Fprintf(w, "FOREIGN KEY\t%s\n", trim(rest))
	}
	w.Flush()
	seg["key-def"] = buf.String()

	buf.Reset()
	fmt.Fprintln(buf, seg["begin"])
	fmt.Fprint(buf, text.Indent(seg["col-def"], "    "))
	fmt.Fprint(buf, text.Indent(seg["key-def"], "    "))
	fmt.Fprintln(buf, seg["end"])
	return buf.String()
}

func fieldsLine(s string) []string {
	lineField := func(r rune) bool { return r == '\n' }
	return strings.FieldsFunc(s, lineField)
}
