// Package remote lets a phone control the Mac's *source* playback (transport,
// volume, now-playing) independently of how audio is streamed. Transport and
// metadata use the embedded mediaremote-adapter (works with any app, no TCC
// permission, macOS 15.4+/26); master volume uses osascript. Both are invoked
// as short-lived processes.
package remote

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/sait-turanalp/airtone/internal/engine"
)

// adapterTar bundles mediaremote-adapter (Perl script + framework, BSD-3) so the
// feature works offline with no Homebrew. It's a tar.gz because the framework
// contains symlinks, which go:embed can't represent directly.
//
//go:embed mediaremote-adapter.tgz
var adapterTar []byte

// volumeSrc is a tiny CoreAudio helper compiled once at runtime; it sets the
// built-in output volume directly (the "AirTone Sync" aggregate has no master
// volume, so osascript can't be used while streaming).
//
//go:embed volume.swift
var volumeSrc []byte

// remotePage is the standalone control-only page served at /remote.
//
//go:embed remote.html
var remotePage []byte

func dir() string           { return filepath.Join(engine.Home(), "remote", "mediaremote-adapter") }
func plPath() string        { return filepath.Join(dir(), "mediaremote-adapter.pl") }
func fwPath() string        { return filepath.Join(dir(), "MediaRemoteAdapter.framework") }
func volumeSrcPath() string { return filepath.Join(engine.Home(), "remote", "volume.swift") }
func volumeBinPath() string { return filepath.Join(engine.Home(), "remote", "volume") }

var setupOnce sync.Once
var setupErr error

// Warmup extracts the adapter and compiles the volume helper ahead of first
// use. Call it in a goroutine when a server starts so the phone never waits.
func Warmup() error {
	setupOnce.Do(func() { setupErr = setup() })
	return setupErr
}

func ensure() error { return Warmup() }

func setup() error {
	if err := extractAdapter(); err != nil {
		return err
	}
	return buildVolume()
}

// buildVolume writes and compiles the CoreAudio volume helper (once).
func buildVolume() error {
	if err := os.MkdirAll(filepath.Dir(volumeSrcPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(volumeSrcPath(), volumeSrc, 0o644); err != nil {
		return err
	}
	if _, err := os.Stat(volumeBinPath()); err == nil {
		return nil // already compiled
	}
	return exec.Command("swiftc", volumeSrcPath(), "-o", volumeBinPath()).Run()
}

func extractAdapter() error {
	if _, err := os.Stat(plPath()); err == nil {
		return nil // already present
	}
	dest := filepath.Join(engine.Home(), "remote")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(adapterTar))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, h.Name) //nolint:gosec // trusted embedded archive
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.Remove(target)
			if err := os.Symlink(h.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // trusted embedded archive
				f.Close()
				return err
			}
			f.Close()
		}
	}
}

// Track is the subset of now-playing info the UI needs (no bulky artwork).
type Track struct {
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	BundleID string  `json:"bundleIdentifier"`
	Playing  bool    `json:"playing"`
	Duration float64 `json:"duration"`
	Elapsed  float64 `json:"elapsedTime"`
}

// NowPlaying returns the current track (empty Track if nothing is playing).
func NowPlaying() (Track, error) {
	var t Track
	if err := ensure(); err != nil {
		return t, err
	}
	out, err := adapter("get")
	if err != nil {
		return t, err
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return t, nil // nothing playing
	}
	_ = json.Unmarshal(out, &t) // tolerate partial/empty payloads
	return t, nil
}

// transport command name -> MediaRemote command ID (see adapter "send" docs).
var commands = map[string]string{
	"play": "0", "pause": "1", "toggle": "2", "stop": "3", "next": "4", "prev": "5",
}

// Send issues a transport command (play/pause/toggle/next/prev).
func Send(cmd string) error {
	id, ok := commands[cmd]
	if !ok {
		return os.ErrInvalid
	}
	if err := ensure(); err != nil {
		return err
	}
	_, err := adapter("send", id)
	return err
}

// Volume returns the built-in output volume (0-100).
func Volume() (int, error) {
	if err := ensure(); err != nil {
		return 0, err
	}
	out, err := exec.Command(volumeBinPath(), "get").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// SetVolume sets the built-in output volume (clamped 0-100).
func SetVolume(v int) error {
	if err := ensure(); err != nil {
		return err
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return exec.Command(volumeBinPath(), "set", strconv.Itoa(v)).Run()
}

func adapter(args ...string) ([]byte, error) {
	full := append([]string{plPath(), fwPath()}, args...)
	return exec.Command("/usr/bin/perl", full...).Output()
}
