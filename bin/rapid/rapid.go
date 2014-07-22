package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
)

var (
	schemaURL = kingpin.Arg("schema-url", "URL endpoint for Rapid schema.").Required().URL()
)

func main() {
	kingpin.CommandLine.Help = `Generate Go code for rapid JSON clients. See https://github.com/alecthomas/rapid
for details.`
	kingpin.Parse()

	fmt.Printf("%s\n", *schemaURL)
}
