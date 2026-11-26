package overlays

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/store"
)

// urlImages is an image source that, like artwork.Images, also fetches
// absolute URLs. Get only accepts TMDB paths.
type urlImages struct {
	data      []byte
	tmdb      []string
	untrusted []string
}

func (f *urlImages) Get(_ context.Context, path string, _ int) ([]byte, string, error) {
	f.tmdb = append(f.tmdb, path)
	if !core.ValidArtworkPath(path) {
		return nil, "", errors.New("invalid path")
	}
	return f.data, "image/png", nil
}

func (f *urlImages) GetUntrustedURL(_ context.Context, rawURL string) ([]byte, string, error) {
	f.untrusted = append(f.untrusted, rawURL)
	return f.data, "image/png", nil
}

// Plugin items supply their own artwork URLs. Overlay posters, backdrops and
// logos must draw from those URLs rather than reject them as TMDB paths.
func TestOverlayArtworkUsesPluginURLs(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenMemory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	img := image.NewRGBA(image.Rect(0, 0, 500, 750))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{200, 40, 40, 255}), image.Point{}, draw.Src)
	var raw bytes.Buffer
	if err = png.Encode(&raw, img); err != nil {
		t.Fatal(err)
	}
	images := &urlImages{data: raw.Bytes()}
	s := New(db, Options{Images: images, Facts: func(context.Context, *core.User, *core.Title) Facts { return Facts{} }})
	if err = s.SaveSettings(ctx, Settings{Enabled: true, Status: true, Quality: true}); err != nil {
		t.Fatal(err)
	}
	show := &core.Title{
		Key:          core.RatingKey(9001000000000001),
		Type:         core.TypeShow,
		Title:        "Plugin Show",
		PosterPath:   "https://posters.example.test/show.png",
		BackdropPath: "https://posters.example.test/backdrop.png",
		LogoPath:     "https://posters.example.test/logo.png",
	}
	u := &core.User{ID: 1}
	if data, _, err := s.Poster(ctx, u, show, "en"); err != nil || len(data) == 0 {
		t.Fatalf("poster: %d bytes, %v", len(data), err)
	}
	if _, _, err := s.Backdrop(ctx, u, show, "en"); err != nil {
		t.Fatalf("backdrop: %v", err)
	}
	if _, _, err := s.Logo(ctx, show); err != nil {
		t.Fatalf("logo: %v", err)
	}
	want := []string{show.PosterPath, show.BackdropPath, show.LogoPath}
	if len(images.untrusted) != len(want) {
		t.Fatalf("untrusted fetches %v, want %v", images.untrusted, want)
	}
	for i := range want {
		if images.untrusted[i] != want[i] {
			t.Fatalf("untrusted fetches %v, want %v", images.untrusted, want)
		}
	}
	if len(images.tmdb) != 0 {
		t.Fatalf("plugin URLs sent to the TMDB path reader: %v", images.tmdb)
	}

	// TMDB paths are unchanged.
	movie := &core.Title{Key: core.MovieKey(1), Type: core.TypeMovie, PosterPath: "/poster.jpg"}
	if _, _, err := s.Poster(ctx, u, movie, "en"); err != nil {
		t.Fatalf("tmdb poster: %v", err)
	}
	if len(images.tmdb) != 1 || images.tmdb[0] != "/poster.jpg" {
		t.Fatalf("tmdb reads %v", images.tmdb)
	}
}
