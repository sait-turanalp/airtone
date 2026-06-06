// Package engine runs the proven audio pipeline. It extracts the embedded
// shell/Swift/config engine to the data dir and shells out to it. The Go layer
// orchestrates; it deliberately does NOT reimplement the audio path.
package engine

import (
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	airtone "github.com/sait-turanalp/airtone"
)

// SyncDevice is the Multi-Output device both modes route system audio through.
const SyncDevice = "AirTone Sync"

// Home is the AirTone data dir (override with AIRTONE_HOME).
func Home() string {
	if h := os.Getenv("AIRTONE_HOME"); h != "" {
		return h
	}
	d, _ := os.UserHomeDir()
	return filepath.Join(d, ".airtone")
}

func dir() string { return filepath.Join(Home(), "engine") }

// materialize extracts the embedded engine to <home>/engine, preserving layout
// (engine/scripts/*.sh, engine/assets/*). Idempotent; refreshed each run so an
// upgraded binary always ships its own engine.
func materialize() (string, error) {
	base := dir()
	err := fs.WalkDir(airtone.Engine, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(base, p)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := airtone.Engine.ReadFile(p)
		if err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if filepath.Ext(p) == ".sh" {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
	return base, err
}

// run extracts the engine and executes one of its scripts, sending combined
// output to out (pass os.Stdout for the CLI, a buffer for the TUI).
func run(script string, out io.Writer) error {
	base, err := materialize()
	if err != nil {
		return err
	}
	cmd := exec.Command("/bin/bash", filepath.Join(base, "scripts", script))
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	return cmd.Run()
}

// Setup creates the Multi-Output device and fetches Snapweb.
func Setup(out io.Writer) error { return run("setup.sh", out) }

// Start routes audio and begins streaming.
func Start(out io.Writer) error { return run("start.sh", out) }

// Stop tears down the pipeline and restores the audio output.
func Stop(out io.Writer) error { return run("stop.sh", out) }

// LANIP returns the primary outbound IPv4 (no packets sent).
func LANIP() string {
	c, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}

// CurrentOutput returns the current system audio output device name.
func CurrentOutput() string {
	out, err := exec.Command("SwitchAudioSource", "-c").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SetOutput selects the system audio output device by name.
func SetOutput(device string) error {
	return exec.Command("SwitchAudioSource", "-s", device).Run()
}

// DeviceExists reports whether an audio device with the exact name is present.
func DeviceExists(name string) bool {
	out, err := exec.Command("SwitchAudioSource", "-a").Output()
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) == name {
			return true
		}
	}
	return false
}

// BuiltinOutput returns the name of the built-in speaker output, if found.
// Used as a fallback when the previous output can't be restored.
func BuiltinOutput() string {
	out, err := exec.Command("SwitchAudioSource", "-a").Output()
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(out), "\n") {
		n := strings.TrimSpace(l)
		if n == "" || n == SyncDevice {
			continue
		}
		low := strings.ToLower(n)
		if strings.Contains(low, "speaker") || strings.Contains(low, "hoparl") || strings.Contains(low, "built-in") || strings.Contains(low, "built in") {
			return n
		}
	}
	return ""
}
