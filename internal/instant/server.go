// Package instant implements AirTone's low-latency "Instant mode" over WebRTC.
//
// Unlike the snapcast path (fixed buffer, sample-accurate multi-device sync),
// WebRTC uses an adaptive jitter buffer (NetEQ) and UDP, so a phone browser
// plays with ~tens of ms latency on a good LAN — at the cost of cross-device
// sync. It stays app-free: the phone just opens a page (scan the QR).
//
// Pipeline: sox (gapless BlackHole capture) | ffmpeg (Opus/Ogg) -> oggreader
// -> a pion Opus track shared by every connected browser.
package instant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"

	"github.com/sait-turanalp/airtone/internal/remote"

	_ "embed"
)

// Port is the HTTP port for the Instant-mode page and signaling.
const Port = 1781

//go:embed instant.html
var pageHTML []byte

// listeners counts currently-connected browsers (for the TUI).
var listeners atomic.Int64

// Listeners returns the number of connected browser clients.
func Listeners() int { return int(listeners.Load()) }

// capturePipeline: gapless capture (sox) piped into a low-delay Opus encoder
// (ffmpeg), muxed as Ogg on stdout for the oggreader.
// One 20ms Opus frame per Ogg page (page_duration matches frame_duration) so
// each page is exactly one Opus packet — required for valid per-packet RTP.
// flush_packets emits each page immediately; sox's real-time capture clock
// paces delivery far more steadily than any wall-clock pacer in Go would.
const capturePipeline = `sox -q -t coreaudio "BlackHole 2ch" -t raw -b 16 -e signed-integer -c 2 -r 48000 - | ` +
	`ffmpeg -hide_banner -loglevel error -f s16le -ar 48000 -ac 2 -i - ` +
	`-c:a libopus -b:a 128k -application lowdelay -frame_duration 20 -page_duration 20000 -flush_packets 1 -f ogg -`

// Run starts the capture pipeline and the HTTP/WebRTC server until ctx is done.
func Run(ctx context.Context, port int) error {
	go func() { _ = remote.Warmup() }() // compile/extract control helpers before the phone connects

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "airtone",
	)
	if err != nil {
		return err
	}

	if err := startCapture(ctx, track); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(pageHTML)
	})
	mux.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		handleOffer(w, r, track)
	})
	remote.Register(mux) // /control/* — drive the Mac's playback from the phone

	srv := &http.Server{Addr: portAddr(port), Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func portAddr(p int) string {
	return ":" + itoa(p)
}

// startCapture launches sox|ffmpeg and pumps Ogg/Opus pages into the track.
func startCapture(ctx context.Context, track *webrtc.TrackLocalStaticSample) error {
	cmd := exec.Command("bash", "-c", capturePipeline)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own group, so we can kill sox+ffmpeg together
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if logf, err := os.Create(filepath.Join(homeDir(), "instant.log")); err == nil {
		cmd.Stderr = logf
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Kill the whole process group when the context ends.
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	// Write each Opus frame to the track as its Ogg page arrives. Delivery is
	// paced by sox's audio capture clock (steadier than a Go wall-clock pacer),
	// and per-page granule deltas give exact sample durations.
	go func() {
		ogg, _, err := oggreader.NewWith(stdout)
		if err != nil {
			return
		}
		var lastGranule uint64
		for {
			data, hdr, err := ogg.ParseNextPage()
			if err != nil {
				return
			}
			samples := hdr.GranulePosition - lastGranule
			lastGranule = hdr.GranulePosition
			dur := time.Duration(samples) * time.Second / 48000
			if werr := track.WriteSample(media.Sample{Data: data, Duration: dur}); werr != nil {
				return
			}
		}
	}()
	return nil
}

// handleOffer completes the WebRTC handshake for one browser (non-trickle).
func handleOffer(w http.ResponseWriter, r *http.Request, track *webrtc.TrackLocalStaticSample) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sender, err := pc.AddTrack(track)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Drain RTCP so the sender's buffers don't fill up.
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, rerr := sender.Read(buf); rerr != nil {
				return
			}
		}
	}()

	counted := false
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		switch s {
		case webrtc.PeerConnectionStateConnected:
			if !counted {
				counted = true
				listeners.Add(1)
			}
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			if counted {
				counted = false
				listeners.Add(-1)
			}
			if s != webrtc.PeerConnectionStateDisconnected {
				_ = pc.Close()
			}
		}
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	<-gatherComplete // non-trickle: ship a complete SDP (LAN host candidates)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pc.LocalDescription())
}

func homeDir() string {
	if h := os.Getenv("AIRTONE_HOME"); h != "" {
		return h
	}
	d, _ := os.UserHomeDir()
	return filepath.Join(d, ".airtone")
}

// itoa avoids pulling strconv just for the port.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [6]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
