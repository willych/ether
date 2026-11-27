package core

import "strings"

// PluginKindSource splits a library source id "plugin:<plugin>:<kind>"
// into its plugin and kind. ok is false for every other source id.
func PluginKindSource(id string) (pluginID, kindKey string, ok bool) {
	rest, found := strings.CutPrefix(id, "plugin:")
	if !found {
		return "", "", false
	}
	// Plugin ids are dotted reverse-DNS names and never contain ':'; the
	// kind is everything after the last ':'.
	i := strings.LastIndexByte(rest, ':')
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

// MixedPluginSources returns the plugin content-kind sources of a
// catalogue library ("plugin:<plugin>:<kind>" ids), in stored order. A
// plugin library's own kind is not a mixed source, so it returns nil for
// every library that is not a catalogue library.
func (l Library) MixedPluginSources() []string {
	if l.Source != LibraryCatalog {
		return nil
	}
	var out []string
	for _, id := range l.Sources {
		if _, _, ok := PluginKindSource(id); ok {
			out = append(out, id)
		}
	}
	return out
}

// CatalogueOnlySources returns lib.Sources without plugin content-kind
// sources: the ids the catalogue store answers for.
func (l Library) CatalogueOnlySources() []string {
	if len(l.Sources) == 0 {
		return l.Sources
	}
	out := make([]string, 0, len(l.Sources))
	for _, id := range l.Sources {
		if _, _, ok := PluginKindSource(id); !ok {
			out = append(out, id)
		}
	}
	return out
}
