SHELL=/bin/bash

all: sql-gen

.PHONY: template clean test

sql-gen: main.go pretty.go template
	go build

%.tmpl.go: %.tmpl
	./gen-template.sh $@ $<

template: sql.tmpl.go

clean:
	rm -f *.tmpl.go sql-gen sql-gen.exe

test: clean sql-gen
	./test.sh
