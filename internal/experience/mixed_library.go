package experience

import (
	"context"

	"github.com/braedonsaunders/ether/internal/core"
	"github.com/braedonsaunders/ether/internal/frontend"
)

// A mixed library is a show catalogue library (TMDB TV, TheTVDB) whose
// sources also name show-shaped plugin content kinds
// ("plugin:<plugin>:<kind>"). Catalogue titles still come from the
// catalogue store; each plugin kind's items still live in the plugin item
// store and are read through that kind's own plugin library (its backing
// library), which may be hidden. A mixed library page is the two read
// side by side and merged in the requested order, so every item keeps its
// own key, detail, playback, watch state and favourite paths.

type catalogueOnlyKey struct{}

// withCatalogueOnly marks a LibraryItems call as reading only the
// catalogue half of a mixed library.
func withCatalogueOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, catalogueOnlyKey{}, true)
}

func catalogueOnly(ctx context.Context) bool {
	v, _ := ctx.Value(catalogueOnlyKey{}).(bool)
	return v
}

// mixedPluginLibraries returns the active backing plugin libraries for
// lib's plugin content-kind sources, in source order. A source whose
// plugin is missing, disabled or no longer declares the kind adds nothing,
// so a removed source, or an uninstalled plugin, removes its items from
// the mixed library at once.
func (s *Service) mixedPluginLibraries(ctx context.Context, lib core.Library) []core.Library {
	ids := lib.MixedPluginSources()
	if len(ids) == 0 || s.d.PluginLibraries == nil {
		return nil
	}
	libs, err := s.d.PluginLibraries.Libraries(ctx)
	if err != nil {
		return nil
	}
	var out []core.Library
	for _, id := range ids {
		pluginID, kindKey, _ := core.PluginKindSource(id)
		for _, b := range libs {
			if b.Source == core.LibraryPlugin && b.PluginID == pluginID && b.KindKey == kindKey {
				if s.d.PluginLibraries.IsPluginLibraryActive(ctx, b) {
					out = append(out, b)
				}
				break
			}
		}
	}
	return out
}

// mixedLibraryItems answers LibraryItems for a mixed library.
func (s *Service) mixedLibraryItems(ctx context.Context, v *core.User, device core.Device, viewID string, lib core.Library, q frontend.ItemQuery) (core.Page, error) {
	catCtx := withCatalogueOnly(ctx)
	backing := s.mixedPluginLibraries(ctx, lib)
	// A collection opened inside the library is a catalogue collection.
	if len(backing) == 0 || q.CollectionKey != 0 {
		return s.LibraryItems(catCtx, v, device, viewID, q)
	}
	// A query anchored on one item (next/previous, play from here) is
	// answered by the store that holds it.
	if anchor := q.AdjacentTo + q.StartAt; anchor != 0 {
		if !anchor.IsPluginKey() {
			return s.LibraryItems(catCtx, v, device, viewID, q)
		}
		_, owner, err := s.d.PluginLibraries.Ancestors(ctx, anchor)
		if err != nil {
			return core.Page{}, core.ErrNotFound
		}
		for _, b := range backing {
			if b.ID == owner.ID {
				return s.pluginLibraryItems(ctx, v, b, s.sanitizePluginLibraryQuery(ctx, b, q))
			}
		}
		return core.Page{}, core.ErrNotFound
	}
	// Read the first offset+limit items of each half, merge them in order
	// and cut the requested window: the window of the merged list is always
	// inside the union of those prefixes.
	sub := q
	sub.Offset = 0
	if q.Limit > 0 {
		sub.Limit = max(q.Offset, 0) + q.Limit
	}
	// The halves are merged on the client-visible order, so an unsorted
	// request is read in title order on both sides.
	if sub.Sort == "" {
		sub.Sort = core.SortTitle
	}
	var all []core.Title
	total, exact, expanding := 0, true, false
	// A plugin person is credited on no catalogue title, and a catalogue
	// person on no plugin item.
	if q.Filter.Person == 0 || !core.IsPluginPersonID(q.Filter.Person) {
		page, err := s.LibraryItems(catCtx, v, device, viewID, sub)
		if err != nil {
			return core.Page{}, err
		}
		all = append(all, page.Items...)
		total += page.Total
		exact = exact && page.TotalExact
	}
	if !wantsCataloguePerson(q.Filter) {
		for _, b := range backing {
			page, err := s.pluginLibraryItems(ctx, v, b, s.sanitizePluginLibraryQuery(ctx, b, sub))
			if err != nil {
				return core.Page{}, err
			}
			all = append(all, page.Items...)
			total += page.Total
			exact = exact && page.TotalExact
			expanding = expanding || page.Expanding
		}
	}
	if sub.Sort.IsLastViewed() {
		s.sortTitlesByLastViewed(ctx, v, all, sub.Desc)
	} else if err := s.sortTitles(ctx, all, sub); err != nil {
		return core.Page{}, err
	}
	start := min(max(q.Offset, 0), len(all))
	end := len(all)
	if q.Limit > 0 {
		end = min(start+q.Limit, end)
	}
	return core.Page{Items: all[start:end], Total: total, TotalExact: exact, Expanding: expanding}, nil
}
