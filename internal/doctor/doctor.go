// Package doctor diagnoses whether the host is ready to run AirTone:
// required binaries, the BlackHole driver, the Multi-Output device, and Snapweb.
package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sait-turanalp/airtone/internal/engine"
)

// Check is a single diagnostic result.
type Check struct {
	Name   string
	OK     bool
	Detail string // hint shown when not OK (or extra info)
}

func bin(name, hint string) Check {
	if _, err := exec.LookPath(name); err == nil {
		return Check{Name: name, OK: true}
	}
	return Check{Name: name, OK: false, Detail: hint}
}

// audioDeviceExists reports whether SwitchAudioSource lists a device by exact name.
func audioDeviceExists(name string) bool {
	out, err := exec.Command("SwitchAudioSource", "-a").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// Run executes all checks in display order.
func Run() []Check {
	var checks []Check

	checks = append(checks,
		bin("snapserver", "brew install snapcast"),
		bin("sox", "brew install sox"),
		bin("SwitchAudioSource", "brew install switchaudio-osx"),
		bin("swift", "install Xcode Command Line Tools"),
	)

	bh := audioDeviceExists("BlackHole 2ch")
	checks = append(checks, Check{
		Name: "BlackHole 2ch driver", OK: bh,
		Detail: orHint(bh, "brew install --cask blackhole-2ch (needs admin + reboot)"),
	})

	dev := audioDeviceExists("AirTone Sync")
	checks = append(checks, Check{
		Name: "AirTone Sync device", OK: dev,
		Detail: orHint(dev, "run: airtone setup"),
	})

	snapweb := fileExists(filepath.Join(engine.Home(), "snapweb", "index.html"))
	checks = append(checks, Check{
		Name: "Snapweb player", OK: snapweb,
		Detail: orHint(snapweb, "run: airtone setup"),
	})

	return checks
}

// OK reports whether every check passed.
func OK(checks []Check) bool {
	for _, c := range checks {
		if !c.OK {
			return false
		}
	}
	return true
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func orHint(ok bool, hint string) string {
	if ok {
		return ""
	}
	return hint
}
