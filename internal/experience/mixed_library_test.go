package experience

import (
	"context"
	"slices"
	"testing"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/frontend"
)

const onePaceSource = "plugin:com.willych.onepace.catalogue:onepace"

// mixedFakeLibraries serves one plugin library (onepace, id 50) whose
// items are pluginItems, next to the catalogue libraries in libs.
type mixedFakeLibraries struct {
	fakePluginLibraries
	pluginItems []core.Title
	browsed     []core.LibraryID
}

func (f *mixedFakeLibraries) Browse(_ context.Context, lib core.Library, _ frontend.ItemQuery, _ string, limit int) (PluginBrowsePage, error) {
	f.browsed = append(f.browsed, lib.ID)
	if lib.ID != 50 {
		return PluginBrowsePage{Items: []core.Title{}, TotalExact: true}, nil
	}
	items := f.pluginItems
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return PluginBrowsePage{Items: items, Total: len(f.pluginItems), TotalExact: true}, nil
}

func (f *mixedFakeLibraries) Ancestors(_ context.Context, key core.RatingKey) ([]core.Title, core.Library, error) {
	for _, it := range f.pluginItems {
		if it.Key == key {
			lib, _ := f.Library(context.Background(), 50)
			return nil, lib, nil
		}
	}
	return nil, core.Library{}, core.ErrNotFound
}

func mixedLibraryService(t *testing.T, sources []string) (*Service, *mixedFakeLibraries, *[]core.BrowseQuery) {
	t.Helper()
	viewer := &core.User{ID: 1, IsOwner: true}
	tv := core.Presentations{MediaBrowser: &core.MediaBrowserPresentation{CollectionType: "tvshows"}}
	store := &mixedFakeLibraries{
		fakePluginLibraries: fakePluginLibraries{libs: []core.Library{
			{ID: 7, Name: "TV Shows", Source: core.LibraryCatalog, MediaType: core.TypeShow, Sources: sources, Present: tv},
			// The plugin's own library: hidden, so One Pace shows only in TV Shows.
			{ID: 50, Name: "One Pace", Source: core.LibraryPlugin, PluginID: "com.willych.onepace.catalogue", KindKey: "onepace", Present: tv},
		}},
		pluginItems: []core.Title{{Key: core.PluginKey(1), Type: core.TypeShow, Title: "One Pace", PluginAddedAt: 100}},
	}
	catalogue := []core.Title{
		{Key: core.ShowKey(1), Type: core.TypeShow, Title: "Andor"},
		{Key: core.ShowKey(2), Type: core.TypeShow, Title: "Bluey"},
		{Key: core.ShowKey(3), Type: core.TypeShow, Title: "Severance"},
	}
	var queries []core.BrowseQuery
	s := &Service{d: ServiceDeps{
		Active:          func(context.Context, int64) (*core.User, error) { return viewer, nil },
		PluginLibraries: store,
		LibraryPresentation: func(p core.Presentations, mt core.MediaType) (core.MediaType, string, bool) {
			if p.MediaBrowser == nil {
				return "", "", false
			}
			return mt, p.MediaBrowser.CollectionType, true
		},
		Visibility: Visibility{
			SectionFor:      func(core.MediaType) (int, bool) { return 2, true },
			FilterAvailable: func(_ context.Context, titles []core.Title) []core.Title { return titles },
		},
		PageExclusions: func(context.Context, *core.User, core.MediaType) LibraryPageExclusions {
			return LibraryPageExclusions{AvailabilityInSQL: true}
		},
		Browse: func(_ context.Context, q core.BrowseQuery) (core.Page, error) {
			queries = append(queries, q)
			if q.Filter.Person != 0 && core.IsPluginPersonID(q.Filter.Person) {
				t.Fatalf("a plugin person must not be asked of the catalogue")
			}
			items := catalogue
			start := min(max(q.Offset, 0), len(items))
			items = items[start:]
			if q.Limit > 0 && len(items) > q.Limit {
				items = items[:q.Limit]
			}
			return core.Page{Items: items, Total: len(catalogue), TotalExact: true}, nil
		},
	}}
	return s, store, &queries
}

func titlesOf(page core.Page) []string {
	out := make([]string, len(page.Items))
	for i, it := range page.Items {
		out[i] = it.Title
	}
	return out
}

func TestMixedLibraryListsCatalogueAndPluginItems(t *testing.T) {
	ctx := context.Background()
	viewer := &core.User{ID: 1, IsOwner: true}
	s, store, queries := mixedLibraryService(t, []string{"tvdb", onePaceSource})

	page, err := s.LibraryItems(ctx, viewer, core.Device{}, "7", frontend.ItemQuery{Limit: 10, Sort: core.SortTitle})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := titlesOf(page), []string{"Andor", "Bluey", "One Pace", "Severance"}; !slices.Equal(got, want) {
		t.Fatalf("titles = %v, want %v", got, want)
	}
	if page.Total != 4 || !page.TotalExact {
		t.Fatalf("total = %d exact=%v, want 4 exact", page.Total, page.TotalExact)
	}
	if !slices.Contains(store.browsed, core.LibraryID(50)) {
		t.Fatalf("the plugin half must be read through the kind's own library, browsed %v", store.browsed)
	}
	for _, q := range *queries {
		if slices.Contains(q.Filter.CatalogueSources, onePaceSource) {
			t.Fatalf("catalogue browse was asked for a plugin source: %v", q.Filter.CatalogueSources)
		}
	}

	// A page inside the merged list is cut from the merged order, and the
	// unsorted request reads both halves in title order.
	page, err = s.LibraryItems(ctx, viewer, core.Device{}, "7", frontend.ItemQuery{Offset: 2, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := titlesOf(page); !slices.Equal(got, []string{"One Pace"}) || page.Total != 4 {
		t.Fatalf("page 3 = %v total=%d, want [One Pace] of 4", got, page.Total)
	}

	// The count every app shows for the library includes plugin items.
	if n, err := s.LibraryCount(ctx, viewer, "7"); err != nil || n != 4 {
		t.Fatalf("count = %d, %v; want 4", n, err)
	}

	// A catalogue person is credited on no plugin item.
	store.browsed = nil
	page, err = s.LibraryItems(ctx, viewer, core.Device{}, "7", frontend.ItemQuery{Limit: 10, Sort: core.SortTitle, Filter: core.Filter{Person: 1234}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(store.browsed, core.LibraryID(50)) || slices.Contains(titlesOf(page), "One Pace") {
		t.Fatalf("catalogue person page read the plugin half: %v", titlesOf(page))
	}

	// Next/previous from a plugin item is answered by its own store.
	page, err = s.LibraryItems(ctx, viewer, core.Device{}, "7", frontend.ItemQuery{AdjacentTo: core.PluginKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(titlesOf(page), "One Pace") {
		t.Fatalf("adjacent page = %v", titlesOf(page))
	}
}

func TestMixedLibraryWithoutPluginSourceListsCatalogueOnly(t *testing.T) {
	ctx := context.Background()
	viewer := &core.User{ID: 1, IsOwner: true}
	// The owner removed the plugin source: its items leave the library.
	s, store, _ := mixedLibraryService(t, []string{"tmdb-tv"})
	page, err := s.LibraryItems(ctx, viewer, core.Device{}, "7", frontend.ItemQuery{Limit: 10, Sort: core.SortTitle})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := titlesOf(page), []string{"Andor", "Bluey", "Severance"}; !slices.Equal(got, want) || page.Total != 3 {
		t.Fatalf("titles = %v total=%d, want %v of 3", got, page.Total, want)
	}
	if len(store.browsed) != 0 {
		t.Fatalf("plugin store read for a catalogue-only library: %v", store.browsed)
	}

	// The plugin's own library is unchanged: it lists only its own items.
	page, err = s.LibraryItems(ctx, viewer, core.Device{}, "50", frontend.ItemQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := titlesOf(page); !slices.Equal(got, []string{"One Pace"}) {
		t.Fatalf("plugin library titles = %v", got)
	}
}

func TestLibraryPluginSourceHelpers(t *testing.T) {
	plugin, kind, ok := core.PluginKindSource(onePaceSource)
	if !ok || plugin != "com.willych.onepace.catalogue" || kind != "onepace" {
		t.Fatalf("PluginKindSource = %q %q %v", plugin, kind, ok)
	}
	for _, id := range []string{"tmdb-tv", "tvdb", "stremio:p:list", "plugin:", "plugin:p", "plugin:p:"} {
		if _, _, ok := core.PluginKindSource(id); ok {
			t.Fatalf("PluginKindSource(%q) ok", id)
		}
	}
	mixed := core.Library{Source: core.LibraryCatalog, Sources: []string{"tvdb", onePaceSource}}
	if got := mixed.MixedPluginSources(); !slices.Equal(got, []string{onePaceSource}) {
		t.Fatalf("MixedPluginSources = %v", got)
	}
	if got := mixed.CatalogueOnlySources(); !slices.Equal(got, []string{"tvdb"}) {
		t.Fatalf("CatalogueOnlySources = %v", got)
	}
	own := core.Library{Source: core.LibraryPlugin, PluginID: "com.willych.onepace.catalogue", KindKey: "onepace", Sources: []string{onePaceSource}}
	if got := own.MixedPluginSources(); got != nil {
		t.Fatalf("a plugin library is never mixed, got %v", got)
	}
}
