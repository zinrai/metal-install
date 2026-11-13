// metal-install renders installation artifacts for physical-server
// installation and serves them over HTTP.
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
  metal-install version

Run any subcommand with -h for its options.`)
}
