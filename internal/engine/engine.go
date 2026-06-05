// Package engine runs the proven audio pipeline. It extracts the embedded
// shell/Swift/config engine to the data dir and shells out to it. The Go layer
// orchestrates; it deliberately does NOT reimplement the audio path.
package engine

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	airtone "github.com/sait-turanalp/airtone"
)

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

// run extracts the engine and executes one of its scripts, streaming output.
func run(script string) error {
	base, err := materialize()
	if err != nil {
		return err
	}
	cmd := exec.Command("/bin/bash", filepath.Join(base, "scripts", script))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

// Setup creates the Multi-Output device and fetches Snapweb.
func Setup() error { return run("setup.sh") }

// Start routes audio and begins streaming.
func Start() error { return run("start.sh") }

// Stop tears down the pipeline and restores the audio output.
func Stop() error { return run("stop.sh") }
