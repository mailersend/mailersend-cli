package main

import (
	"os"

	"github.com/mailersend/mailersend-cli/cmd"
	"github.com/mailersend/mailersend-cli/internal/output"
)

func main() {
	if err := cmd.Execute(); err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
}
