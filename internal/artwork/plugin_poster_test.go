package artwork

import (
	"bytes"
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/braedonsaunders/ether/internal/config"
	"github.com/braedonsaunders/ether/internal/core"
)

// A plugin show has its own poster URL and no TMDB identity. With a poster
// provider chosen, the provider cannot match it, so GetPoster must serve the
// plugin's poster instead of treating the URL as a TMDB path and failing.
func TestGetPosterFallsBackToPluginPosterURL(t *testing.T) {
	poster := testPNG(t, color.RGBA{R: 200, G: 40, B: 40, A: 255})
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/posters/show.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(poster)
	}))
	defer srv.Close()
	p := config.Posters{Enabled: true, Source: "rpdb", APIKey: "key"}
	im := NewImages(ImagesOptions{
		Dir:     t.TempDir(),
		Guard:   testGuard(map[string]*httptest.Server{"posters.example.test": srv}),
		Posters: func() config.Posters { return p },
	})
	defer im.Close()
	show := &core.Title{Type: core.TypeShow, PosterPath: "http://posters.example.test/posters/show.png"}
	data, ct, err := im.GetPoster(context.Background(), show, show.PosterPath, 500)
	if err != nil {
		t.Fatalf("plugin poster: %v", err)
	}
	if !bytes.Equal(data, poster) || ct != "image/png" {
		t.Fatalf("got %d bytes (%s), want the plugin's poster", len(data), ct)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
	// A TMDB path still goes through the TMDB path check.
	if _, _, err := im.GetPoster(context.Background(), &core.Title{Type: core.TypeShow}, "not-a-path", 500); err == nil {
		t.Fatal("invalid TMDB path accepted")
	}
}
