// Copyright 2026 Heath Stewart.
// Licensed under the MIT License. See LICENSE.txt in the project root for license information.

package main

import (
	"fmt"
	"os"

	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/heaths/gh-minimize/internal/cmd"
)

func main() {
	os.Exit(run(os.Args[1:], term.FromEnv()))
}

func run(args []string, terminal term.Term) int {
	root := cmd.NewWithTerm(terminal)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(terminal.ErrOut(), err)
		return 1
	}

	return 0
}
