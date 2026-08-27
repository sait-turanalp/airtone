// Package doctor diagnoses whether the host is ready to run AirTone: required
// binaries, the macOS version the process-tap API needs, the compiled system
// tap, and Snapweb.
package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sait-turanalp/airtone/internal/engine"
)

// minMacOS is the first release with the CoreAudio process-tap API
// (AudioHardwareCreateProcessTap), which is how AirTone captures system audio.
const minMacOS = "14.2"

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

// atLeast reports whether a dotted version string is >= want, numerically.
// "26.5.2" > "14.2" — a string compare would get that backwards.
func atLeast(have, want string) bool {
	parse := func(s string) []int {
		parts := strings.Split(strings.TrimSpace(s), ".")
		out := make([]int, len(parts))
		for i, p := range parts {
			out[i], _ = strconv.Atoi(p)
		}
		return out
	}
	h, w := parse(have), parse(want)
	for i := range w {
		if i >= len(h) {
			return false
		}
		if h[i] != w[i] {
			return h[i] > w[i]
		}
	}
	return true
}

// Run executes all checks in display order.
func Run() []Check {
	var checks []Check

	checks = append(checks,
		bin("snapserver", "brew install snapcast"),
		bin("snapclient", "brew install snapcast"),
		bin("swiftc", "xcode-select --install"),
	)

	out, err := exec.Command("sw_vers", "-productVersion").Output()
	version := strings.TrimSpace(string(out))
	modern := err == nil && atLeast(version, minMacOS)
	checks = append(checks, Check{
		Name: "macOS " + minMacOS + "+ (audio tap API)", OK: modern,
		Detail: orHint(modern, "found "+version+" — the system tap needs macOS "+minMacOS+" or newer"),
	})

	tap := fileExists(engine.TapBin())
	checks = append(checks, Check{
		Name: "system tap", OK: tap,
		Detail: orHint(tap, "run: airtone setup"),
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
