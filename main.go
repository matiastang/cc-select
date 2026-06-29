package main

import (
	"os"

	"github.com/cc-select/cc-select/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
