// Command airtone streams a Mac's system audio to other devices in sync.
//
// This is the CLI/TUI entrypoint. Subcommands are wired up in later phases;
// for now it exposes version and usage so the binary builds and runs.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `airtone — sync your Mac's system audio to your phone

Usage:
  airtone <command>

Commands:
  start     Start the audio bridge
  stop      Stop the audio bridge and restore output
  status    Show live stream and client status
  doctor    Diagnose the setup (devices, ports, permissions)
  setup     Guided first-time setup
  version   Print the version

Run without a command to launch the interactive TUI.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		// TUI is implemented in a later phase; show usage until then.
		fmt.Print(usage)
		return
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("airtone %s\n", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	case "start", "stop", "status", "doctor", "setup":
		fmt.Printf("airtone: %q is not implemented yet\n", args[0])
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "airtone: unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}
