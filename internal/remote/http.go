package remote

import (
	"encoding/json"
	"net/http"
)

// Register mounts the remote-control endpoints on mux:
//
//	GET  /control/nowplaying        -> Track JSON (title/artist/playing/…)
//	GET  /control/volume            -> {"volume": 0-100}
//	POST /control/volume {volume:N} -> set, returns {"volume": N}
//	POST /control/{play|pause|toggle|next|prev}
//
// These drive the music source and are independent of the audio transport, so
// the same handlers serve both modes.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("/control/events", Events) // SSE push of now-playing state

	mux.HandleFunc("/control/artwork", func(w http.ResponseWriter, _ *http.Request) {
		data, mime := ResolveArtwork() // high-res from iTunes, else local thumbnail
		if data == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", mime)
		// URL is keyed per-track (?v=…), so the image is safe to cache hard.
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/control/colorthief.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "max-age=86400")
		_, _ = w.Write(colorThiefJS)
	})

	mux.HandleFunc("/control/nowplaying", func(w http.ResponseWriter, _ *http.Request) {
		t, err := NowPlaying()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, t)
	})

	mux.HandleFunc("/control/volume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body struct {
				Volume int `json:"volume"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := SetVolume(body.Volume); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Echo the requested value instead of a fresh read — skips the ~200ms
			// `volume get` spawn so a live drag stays instant. The periodic GET
			// (syncVol) reconciles the real device value when idle.
			vv := body.Volume
			if vv < 0 {
				vv = 0
			} else if vv > 100 {
				vv = 100
			}
			writeJSON(w, map[string]int{"volume": vv})
			return
		}
		v, err := Volume()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int{"volume": v})
	})

	mux.HandleFunc("/control/seek", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Position float64 `json:"position"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := Seek(body.Position); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	for _, cmd := range []string{"play", "pause", "toggle", "next", "prev"} {
		cmd := cmd
		mux.HandleFunc("/control/"+cmd, func(w http.ResponseWriter, _ *http.Request) {
			if err := Send(cmd); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
