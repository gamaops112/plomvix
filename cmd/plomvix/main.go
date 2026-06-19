// Package main is the entrypoint for the Plomvix database.
//
// Usage:
//
//	plomvix [flags]
//
// Flags:
//
//	--config path   Path to TOML configuration file (default: config.toml)
//	--port number   Override the server listen port (0 = use config file value)
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/plomvix/plomvix/internal/runtime"
)

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "plomvix: %v\n", err)
		os.Exit(2)
	}

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// parseFlags parses CLI flags and returns runtime options. It returns an
// error when flag parsing fails; invalid flag values are reported via
// flag.CommandLine and os.Exit(2) is handled by the standard flag package.
func parseFlags(args []string) (runtime.Options, error) {
	opts := runtime.DefaultOptions()

	fs := flag.NewFlagSet("plomvix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", opts.ConfigPath, "Path to TOML configuration file")
	portOverride := fs.Int("port", 0, "Override server listen port (0 = use config file value)")

	if err := fs.Parse(args); err != nil {
		return runtime.Options{}, err
	}

	opts.ConfigPath = *configPath

	// Port override is stored in options for the runtime to apply after
	// config loading. The runtime merges it into cfg.Server.Port.
	if *portOverride != 0 {
		opts.PortOverride = *portOverride
	}

	return opts, nil
}

func run(opts runtime.Options) error {
	return runtime.RunWithSignals(opts)
}
