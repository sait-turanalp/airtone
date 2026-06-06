package remote

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The MediaRemote thumbnail is tiny and looks bad scaled up. ResolveArtwork
// upgrades it: it looks the track up in the iTunes catalog and fetches a
// high-resolution cover, falling back to the local thumbnail when there's no
// match or no internet. Results are cached per track.

type artEntry struct {
	data []byte
	mime string
}

// artCall lets concurrent lookups for the same track share one fetch
// (singleflight): the prefetch and the phone's request don't both hit iTunes.
type artCall struct {
	wg   sync.WaitGroup
	data []byte
	mime string
}

var (
	artMu       sync.Mutex
	artCache    = map[string]artEntry{} // ONLY high-res hits — see resolveFor
	artInflight = map[string]*artCall{}
)

// ResolveArtwork returns the best available cover for the current track.
func ResolveArtwork() ([]byte, string) {
	t, _ := NowPlaying()
	return resolveFor(t.Title, t.Artist, t.BundleID)
}

// PrefetchArtwork warms the cache for a track ahead of the phone requesting it
// (called when the now-playing stream reports a track change).
func PrefetchArtwork(title, artist, bundle string) { _, _ = resolveFor(title, artist, bundle) }

func resolveFor(title, artist, bundle string) ([]byte, string) {
	if title == "" {
		data, mime, _ := Artwork()
		return data, mime
	}
	key := strings.ToLower(title + "\x00" + artist)

	artMu.Lock()
	if e, ok := artCache[key]; ok { // high-res, already resolved
		artMu.Unlock()
		return e.data, e.mime
	}
	if call, ok := artInflight[key]; ok { // someone else is fetching — wait & share
		artMu.Unlock()
		call.wg.Wait()
		return call.data, call.mime
	}
	call := &artCall{}
	call.wg.Add(1)
	artInflight[key] = call
	artMu.Unlock()

	data, mime, hiRes := fetchArtwork(title, artist, bundle)
	call.data, call.mime = data, mime
	call.wg.Done()

	artMu.Lock()
	delete(artInflight, key)
	// Cache ONLY high-res. A thumbnail fallback means iTunes failed (no match,
	// timeout, or rate-limit) — don't poison the cache, so the next track change
	// retries and can still upgrade to high-res.
	if hiRes && data != nil {
		artCache[key] = artEntry{data, mime}
	}
	artMu.Unlock()
	return data, mime
}

// browserArtMinBytes separates a real browser cover (e.g. a YouTube video
// thumbnail, ~10KB+) from the app-icon fallback (e.g. the Chrome logo, ~4KB)
// that MediaRemote returns when a page sets no media artwork.
const browserArtMinBytes = 6000

// fetchArtwork prefers the high-res iTunes cover; hiRes reports whether it got one.
func fetchArtwork(title, artist, bundle string) (data []byte, mime string, hiRes bool) {
	if d, m := itunesArtwork(artist, title); d != nil {
		return d, m, true
	}
	d, m, _ := Artwork() // a music app embeds the cover; a browser gives a thumbnail or its app icon
	// For browsers, suppress ONLY the tiny app-icon fallback (so the placeholder
	// shows) but keep a genuine thumbnail like YouTube's video cover.
	if isBrowserBundle(bundle) && len(d) < browserArtMinBytes {
		return nil, "", false
	}
	return d, m, false
}

// isBrowserBundle reports whether the now-playing source is a web browser, whose
// MediaRemote artwork is typically the app icon rather than real cover art.
func isBrowserBundle(id string) bool {
	id = strings.ToLower(id)
	for _, b := range []string{"chrome", "safari", "firefox", "edge", "brave", "opera", "vivaldi", "chromium", "thebrowser", "webkit"} {
		if strings.Contains(id, b) {
			return true
		}
	}
	return false
}

var httpClient = &http.Client{Timeout: 4 * time.Second}

// itunesArtwork finds a high-res cover via the public iTunes Search API.
func itunesArtwork(artist, title string) ([]byte, string) {
	// YouTube auto-channels suffix the artist with "- Topic"; drop it for matching.
	artist = strings.TrimSpace(strings.TrimSuffix(artist, "- Topic"))
	term := strings.TrimSpace(artist + " " + title)
	if term == "" {
		return nil, ""
	}
	q := "https://itunes.apple.com/search?media=music&entity=song&limit=1&term=" + url.QueryEscape(term)
	resp, err := httpClient.Get(q)
	if err != nil {
		return nil, ""
	}
	defer resp.Body.Close()
	var r struct {
		Results []struct {
			ArtworkURL100 string `json:"artworkUrl100"`
		} `json:"results"`
	}
	if json.NewDecoder(resp.Body).Decode(&r) != nil || len(r.Results) == 0 || r.Results[0].ArtworkURL100 == "" {
		return nil, ""
	}
	// iTunes lets you request any size by editing the dimensions in the URL.
	hi := strings.Replace(r.Results[0].ArtworkURL100, "100x100bb", "1000x1000bb", 1)
	ir, err := httpClient.Get(hi)
	if err != nil {
		return nil, ""
	}
	defer ir.Body.Close()
	if ir.StatusCode != http.StatusOK {
		return nil, ""
	}
	data, err := io.ReadAll(io.LimitReader(ir.Body, 5<<20))
	if err != nil || len(data) == 0 {
		return nil, ""
	}
	mime := ir.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}
	return data, mime
}
