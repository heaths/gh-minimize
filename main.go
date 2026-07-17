// Copyright 2026 Heath Stewart.
// Licensed under the MIT License. See LICENSE.txt in the project root for license information.

package main

import (
	"fmt"
	"os"

	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/heaths/gh-minimize/internal/cmd"
)

func main() {
	os.Exit(run(os.Args[1:], iostreams.System()))
}

func run(args []string, streams *iostreams.IOStreams) int {
	root := cmd.NewWithIO(streams)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(streams.ErrOut, err)
		return 1
	}

	return 0
}
