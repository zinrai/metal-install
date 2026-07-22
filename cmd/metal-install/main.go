// metal-install is a single binary that exposes three subcommands:
//
//	metal-install server   - HTTP API for per-node artifact generation
//	metal-install render   - generate per-node artifacts for one spec
//	metal-install snapshot - generate artifacts for every machine x OS
//	                         combination declared in compatibility.yml
//
// All three subcommands share the same internal/render package, so a
// given (machine, os, spec) input always produces the same output
// regardless of which subcommand is invoked.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "server":
		return serverCmd(rest)
	case "render":
		return renderCmd(rest)
	case "snapshot":
		return snapshotCmd(rest)
	case "version":
		printVersion()
		return nil
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown subcommand: %s", cmd)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  metal-install server   -data-dir DIR -state-dir DIR [-listen ADDR]
  metal-install render   -data-dir DIR -spec FILE -output-dir DIR
  metal-install snapshot -data-dir DIR -output-dir DIR

Run any subcommand with -h for its options.`)
}
