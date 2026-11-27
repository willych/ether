package app

import (
	"context"
	"strings"
	"testing"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/libraries"
	"github.com/braedonsaunders/ether/internal/plugins"
)

// installShowShapedPlugin installs a content plugin with a show-shaped
// kind (show / arc / episode, like One Pace) and a flat kind (clips).
func installShowShapedPlugin(t *testing.T, a *App) {
	t.Helper()
	ctx := context.Background()
	manifest := []byte(`{
		"id": "community.example.onepace",
		"version": "1.0.0",
		"name": "One Pace",
		"description": "Show-shaped plugin content for mixed library tests",
		"ether": {
			"protocol": 1,
			"kind": "content",
			"summary": "Show-shaped plugin content",
			"runtime": {"kind": "hosted", "entry": "dist/plugin.js"},
			"capabilities": ["catalog_provider", "kind_resolver"],
			"contributes": {
				"mediaKinds": [{
					"key": "onepace",
					"label": "One Pace",
					"media": "video",
					"library": {"name": "One Pace"},
					"levels": [
						{"key": "show", "label": "Show", "role": "show"},
						{"key": "arc", "label": "Arc"},
						{"key": "episode", "label": "Episode", "role": "episode", "playable": true}
					],
					"present": {"plex": {"base": "show"}, "mediaBrowser": {"collectionType": "tvshows"}},
					"sync": {"mode": "push"}
				}, {
					"key": "clips",
					"label": "Clips",
					"media": "video",
					"library": {"name": "Clips"},
					"levels": [{"key": "clip", "label": "Clip", "playable": true}],
					"present": {"plex": {"base": "movie"}},
					"sync": {"mode": "push"}
				}]
			}
		}
	}`)
	preview, err := a.pluginsRT.mgr.PreviewInstall(ctx, plugins.Source{
		Kind:     "studio",
		Manifest: manifest,
		Files:    map[string][]byte{"dist/plugin.js": []byte(`function handle(req) { return {status: 200, body: {status: "ok"}}; }`)},
	})
	if err != nil {
		t.Fatalf("PreviewInstall: %v", err)
	}
	if _, err := a.pluginsRT.mgr.Install(ctx, preview, plugins.InstallOptions{AcceptAddedPermissions: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
}

func TestShowShapedPluginKindJoinsShowCatalogueLibrary(t *testing.T) {
	a := setupTestApp(t, true)
	ctx := context.Background()
	installShowShapedPlugin(t, a)
	const onePace = "plugin:community.example.onepace:onepace"
	const clips = "plugin:community.example.onepace:clips"

	own, err := a.libraries.UpsertPluginLibrary(ctx, "community.example.onepace", "onepace", libraries.PluginLibraryDecl{Name: "One Pace"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.libraries.UpsertPluginLibrary(ctx, "community.example.onepace", "clips", libraries.PluginLibraryDecl{Name: "Clips"}); err != nil {
		t.Fatal(err)
	}
	tv, err := a.libraries.Create(ctx, "Anime", core.TypeShow, nil, "", core.Presentations{})
	if err != nil {
		t.Fatal(err)
	}
	movies, err := a.libraries.Create(ctx, "Films", core.TypeMovie, nil, "", core.Presentations{})
	if err != nil {
		t.Fatal(err)
	}

	if ref, err := a.resolveCatalogueSource(ctx, onePace); err != nil || ref.typ != "show" || !ref.showShaped {
		t.Fatalf("show-shaped kind resolved to %+v, %v; want a show", ref, err)
	}
	if ref, err := a.resolveCatalogueSource(ctx, clips); err != nil || ref.typ != "plugin:"+clips || ref.showShaped {
		t.Fatalf("flat kind resolved to %+v, %v; want its own type", ref, err)
	}

	cases := []struct {
		name    string
		lib     core.Library
		sources []string
		wantErr string
	}{
		{"tmdb tv plus show-shaped kind", tv, []string{"tmdb-tv", onePace}, ""},
		{"show-shaped kind alone", tv, []string{onePace}, "keep at least one catalogue source"},
		{"flat kind in a tv library", tv, []string{"tmdb-tv", clips}, "shaped like a TV show"},
		{"show-shaped kind in a movie library", movies, []string{"tmdb-movies", onePace}, "same media type"},
		{"plugin library keeps its own kind", own, []string{onePace}, ""},
		{"plugin library takes no catalogue source", own, []string{onePace, "tmdb-tv"}, "plugin libraries use their own content kind"},
		{"unknown kind", tv, []string{"tmdb-tv", "plugin:community.example.onepace:gone"}, "no longer offers the content kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := a.validateCatalogueSources(ctx, tc.lib, tc.sources)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validate %v: %v", tc.sources, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("validate %v = %v, want error containing %q", tc.sources, err, tc.wantErr)
			}
		})
	}

	// The saved sources round-trip, and the catalogue half never sees the
	// plugin source as catalogue membership.
	if err := a.libraries.SetSources(ctx, tv.ID, []string{"tvdb", onePace}); err != nil {
		t.Fatal(err)
	}
	saved, err := a.libraries.Get(ctx, tv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.MixedPluginSources(); len(got) != 1 || got[0] != onePace {
		t.Fatalf("mixed sources = %v", got)
	}

	// With the kind's own library removed there is nowhere to read its
	// items from, so the source can't be added.
	if err := a.libraries.Delete(ctx, own.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.validateCatalogueSources(ctx, tv, []string{"tmdb-tv", onePace}); err == nil || !strings.Contains(err.Error(), "library for this content was removed") {
		t.Fatalf("validate after removing the plugin library = %v", err)
	}
}
