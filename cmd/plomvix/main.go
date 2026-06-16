// Package main is the entrypoint for the Plomvix database.
package main

import (
	"fmt"
	"os"

	"github.com/plomvix/plomvix/internal/runtime"
)

func main() {
	if err := run(runtime.DefaultOptions()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts runtime.Options) error {
	return runtime.RunWithSignals(opts)
}
