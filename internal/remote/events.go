package remote

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"syscall"
)

// Events streams now-playing changes to the browser via Server-Sent Events,
// driven by the adapter's real-time `stream` mode — so the UI updates the
// instant the Mac's playback changes, with no polling. Heavy fields (artwork)
// are stripped; the per-connection adapter process is killed on disconnect.
func Events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if err := ensure(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	cmd := exec.Command("/usr/bin/perl", plPath(), fwPath(), "stream")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	kill := func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	defer func() { kill(); _ = cmd.Wait() }()
	go func() { <-r.Context().Done(); kill() }() // stop the adapter when the phone disconnects

	// Merge diff/full payloads into running state, emit only the slim subset.
	state := map[string]any{}
	br := bufio.NewReader(stdout)
	for {
		line, rerr := br.ReadBytes('\n')
		if len(line) > 0 {
			var msg struct {
				Payload map[string]any `json:"payload"`
			}
			if json.Unmarshal(line, &msg) == nil && msg.Payload != nil {
				for k, v := range msg.Payload {
					state[k] = v
				}
				slim, _ := json.Marshal(map[string]any{
					"title":       state["title"],
					"artist":      state["artist"],
					"album":       state["album"],
					"playing":     state["playing"],
					"duration":    state["duration"],
					"elapsedTime": state["elapsedTime"],
				})
				if _, werr := fmt.Fprintf(w, "data: %s\n\n", slim); werr != nil {
					return // client gone
				}
				flusher.Flush()
			}
		}
		if rerr != nil {
			return
		}
	}
}
