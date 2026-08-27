// Command airtone streams a Mac's system audio to other devices in sync.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/sait-turanalp/airtone/internal/doctor"
	"github.com/sait-turanalp/airtone/internal/engine"
	"github.com/sait-turanalp/airtone/internal/instant"
	"github.com/sait-turanalp/airtone/internal/rpc"
	"github.com/sait-turanalp/airtone/internal/tui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

const httpPort = "1780"

const usage = `airtone — sync your Mac's system audio to your phone

Usage:
  airtone <command>

Commands:
  setup     One-time setup (system tap + Snapweb player)
  party     Multi-device synced playback (~1s delay)  [alias: start]
  instant   Low-latency mode over WebRTC (~tens of ms, single/loose)
  stop      Stop party mode
  status    Show live stream and listener status
  doctor    Diagnose the setup (devices, ports, permissions)
  version   Print the version

Run without a command to launch the interactive TUI.
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "airtone: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("airtone %s\n", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	case "setup":
		exitOnErr(engine.Setup(os.Stdout))
	case "party", "start":
		exitOnErr(engine.Start(os.Stdout))
	case "instant":
		runInstant()
	case "stop":
		exitOnErr(engine.Stop(os.Stdout))
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
	fmt.Printf("  open on your phone: http://%s:%s\n", engine.LANIP(), httpPort)
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

func runInstant() {
	if _, err := os.Stat(engine.TapBin()); err != nil {
		fmt.Fprintln(os.Stderr, "airtone: not set up yet — run: airtone setup")
		os.Exit(1)
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		fmt.Fprintln(os.Stderr, "airtone: instant mode needs ffmpeg — run: brew install ffmpeg")
		os.Exit(1)
	}

	// Catch SIGHUP (terminal close) too, so the capture is torn down on every
	// exit path — not just Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	fmt.Println("AirTone instant mode (WebRTC, low latency)")
	fmt.Printf("  open on your phone: http://%s:%d\n", engine.LANIP(), instant.Port)
	fmt.Println("  play audio on the Mac · press q or Ctrl+C to stop")

	// Single-key 'q' to quit (raw mode). In raw mode Ctrl+C arrives as a byte, so
	// handle 0x03/0x04 here too. Restored to cooked mode before we print/return.
	var restoreTerm func()
	if term.IsTerminal(int(os.Stdin.Fd())) {
		if old, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
			restoreTerm = func() { _ = term.Restore(int(os.Stdin.Fd()), old) }
			go func() {
				b := make([]byte, 1)
				for {
					n, rerr := os.Stdin.Read(b)
					if rerr != nil {
						return
					}
					if n > 0 && (b[0] == 'q' || b[0] == 'Q' || b[0] == 3 || b[0] == 4) {
						stop()
						return
					}
				}
			}()
		}
	}

	err := instant.Run(ctx, instant.Port)
	if restoreTerm != nil {
		restoreTerm() // back to cooked mode before printing
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "airtone: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nstopped.")
}
