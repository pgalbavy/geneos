package main

import (
	"github.com/spf13/cobra/doc"

	"wonderland.org/geneos/cmd"
)

func main() {
	// doc.GenManTree(cmd.RootCmd(), nil, "./")
	doc.GenMarkdownTree(cmd.RootCmd(), "./")
}
