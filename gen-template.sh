#/bin/bash

test -z "$1" -o -z "$2" && exit 1

VARNAME="$2"
VARNAME="${VARNAME%%.tmpl}Template"
read -r -d '' TEMPLATE_FILE <<"EOF"
package main

const VARNAME = `
`
EOF

echo "$TEMPLATE_FILE" | sed "s/VARNAME/$VARNAME/g" | sed "3r $2" > "$1"
