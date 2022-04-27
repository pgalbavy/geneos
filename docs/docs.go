package main

import (
	"github.com/spf13/cobra/doc"

	"wonderland.org/geneos/cmd"
)

func main() {
	doc.GenMarkdownTree(cmd.RootCmd(), "./")
}
