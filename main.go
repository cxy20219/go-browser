package main

import (
	"github.com/browserless/go-cli-browser/cmd"
	_ "github.com/browserless/go-cli-browser/cmd/commands"
)

func main() {
	cmd.Execute()
}
