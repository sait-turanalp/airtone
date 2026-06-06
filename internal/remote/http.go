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
	mux.HandleFunc("/remote", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(remotePage)
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
		}
		v, err := Volume()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int{"volume": v})
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
