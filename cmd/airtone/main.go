// Command airtone streams a Mac's system audio to other devices in sync.
package main

import (
	"fmt"
	"net"
	"os"

	"github.com/sait-turanalp/airtone/internal/doctor"
	"github.com/sait-turanalp/airtone/internal/engine"
	"github.com/sait-turanalp/airtone/internal/rpc"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const httpPort = "1780"

const usage = `airtone — sync your Mac's system audio to your phone

Usage:
  airtone <command>

Commands:
  setup     One-time setup (Multi-Output device + Snapweb player)
  start     Start the audio bridge
  stop      Stop the audio bridge and restore output
  status    Show live stream and listener status
  doctor    Diagnose the setup (devices, ports, permissions)
  version   Print the version

Run without a command to launch the interactive TUI (coming soon).
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		return
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("airtone %s\n", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	case "setup":
		exitOnErr(engine.Setup())
	case "start":
		exitOnErr(engine.Start())
	case "stop":
		exitOnErr(engine.Stop())
	case "status":
		runStatus()
	case "doctor":
		runDoctor()
	default:
		fmt.Fprintf(os.Stderr, "airtone: unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "airtone: %v\n", err)
		os.Exit(1)
	}
}

func runStatus() {
	st, err := rpc.GetStatus(rpc.DefaultAddr)
	if err != nil {
		fmt.Println("AirTone is not running. Start it with: airtone start")
		return
	}
	fmt.Println("AirTone status")
	for _, s := range st.Streams {
		fmt.Printf("  stream %q: %s\n", s.ID, s.Status)
	}
	var connected []rpc.Client
	for _, c := range st.Clients {
		if c.Connected {
			connected = append(connected, c)
		}
	}
	if len(connected) == 0 {
		fmt.Println("  listeners: none yet (open the URL below on your phone)")
	} else {
		fmt.Println("  listeners:")
		for _, c := range connected {
			muted := ""
			if c.Muted {
				muted = ", muted"
			}
			fmt.Printf("    - %s (vol %d%%%s)\n", c.Name, c.Percent, muted)
		}
	}
	fmt.Printf("  open on your phone: http://%s:%s\n", lanIP(), httpPort)
}

func runDoctor() {
	checks := doctor.Run()
	fmt.Println("AirTone doctor")
	for _, c := range checks {
		mark := "✓"
		if !c.OK {
			mark = "✗"
		}
		line := fmt.Sprintf("  %s %s", mark, c.Name)
		if !c.OK && c.Detail != "" {
			line += " — " + c.Detail
		}
		fmt.Println(line)
	}
	if doctor.OK(checks) {
		fmt.Println("All good ✅  Run: airtone start")
		return
	}
	fmt.Println("Some checks failed. Fix the items above, then re-run airtone doctor.")
	os.Exit(1)
}

// lanIP returns the primary outbound IPv4 without sending packets.
func lanIP() string {
	c, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}
