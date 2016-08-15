package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/fanyang01/sqlfmt"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(sqlfmt.Format(string(b)))
}
