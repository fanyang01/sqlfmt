package main

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
)

func pretty(sql string) string {
	lines := strings.Split(sql, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if EndWith(lines[i], ',', " \t") {
			lines[i] = strings.TrimRight(lines[i], "\t ,")
			break
		}
	}

	writer := new(bytes.Buffer)
	w := new(tabwriter.Writer)
	w.Init(writer, 0, 4, 2, ' ', 0)
	buf := new(bytes.Buffer)

	isBegin := func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "CREATE TABLE")
	}
	isKeyDef := func(s string) bool {
		for _, prefix := range []string{
			"PRIMARY KEY", "INDEX", "UNIQUE", "FOREIGN KEY",
		} {
			if strings.HasPrefix(strings.TrimSpace(s), prefix) {
				return true
			}
		}
		return false
	}
	isEnd := func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), ")")
	}
	splitKeyDef := func(s string) (string, string) {
		words := strings.Fields(s)
		switch words[0] {
		case "FOREIGN", "PRIMARY":
			return strings.Join(words[:2], " "), strings.Join(words[2:], " ")
		default:
			return words[0], strings.Join(words[1:], " ")
		}
	}
	const (
		posOut = iota
		posColDef
		posKeyDef
	)
	pos := posOut
	for _, line := range lines {
		if pos == posOut && isBegin(line) {
			fmt.Fprintln(buf, line)
			pos = posColDef
			continue
		} else if pos == posColDef && (isKeyDef(line) || isEnd(line)) {
			w.Flush()
			for _, l := range FieldsLine(writer.String()) {
				fmt.Fprintf(buf, "    %s\n", l)
			}
			if isEnd(line) {
				fmt.Fprintln(buf, line)
				pos = posOut
				continue
			}
			writer.Reset()
			w.Init(writer, 0, 4, 1, ' ', 0)
			pos = posKeyDef
		} else if pos == posKeyDef && isEnd(line) {
			w.Flush()
			for _, l := range FieldsLine(writer.String()) {
				fmt.Fprintf(buf, "    %s\n", l)
			}
			fmt.Fprintln(buf, line)
			pos = posOut
			continue
		}

		if pos == posOut {
			fmt.Fprintln(buf, line)
			continue
		} else if pos == posColDef {
			words := strings.Fields(line)
			fmt.Fprintf(w, "%s\t%s\n", words[0], strings.Join(words[1:], " "))
		} else if pos == posKeyDef {
			p0, p1 := splitKeyDef(line)
			fmt.Fprintf(w, "%s\t%s\n", p0, p1)
		}
	}

	return buf.String()
}

func EndWith(s string, c rune, skip string) bool {
	found := false
	strings.LastIndexFunc(s, func(r rune) bool {
		if strings.IndexRune(skip, r) >= 0 {
			return false
		}
		found = (r == c)
		return true
	})
	return found
}

func FieldsLine(s string) []string {
	lineField := func(r rune) bool { return r == '\n' }
	return strings.FieldsFunc(s, lineField)
}
