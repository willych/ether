package plugins

import "testing"

func TestMediaKindShowShaped(t *testing.T) {
	lib := KindLibraryDecl{Name: "One Pace"}
	show := KindLevel{Key: "show", Label: "Show", Role: "show"}
	arc := KindLevel{Key: "arc", Label: "Arc"}
	ep := KindLevel{Key: "episode", Label: "Episode", Role: "episode", Playable: true}
	cases := []struct {
		name string
		kind MediaKindMeta
		want bool
	}{
		{"show / arc / episode", MediaKindMeta{Media: "video", Library: lib, Levels: []KindLevel{show, arc, ep}}, true},
		{"media defaults to video", MediaKindMeta{Library: lib, Levels: []KindLevel{show, arc, ep}}, true},
		{"season role", MediaKindMeta{Library: lib, Levels: []KindLevel{show, {Key: "season", Label: "Season", Role: "season"}, ep}}, true},
		{"no library", MediaKindMeta{Media: "video", Levels: []KindLevel{show, arc, ep}}, false},
		{"no show role", MediaKindMeta{Library: lib, Levels: []KindLevel{{Key: "show", Label: "Show"}, arc, ep}}, false},
		{"no middle level", MediaKindMeta{Library: lib, Levels: []KindLevel{show, ep}}, false},
		{"middle level with another role", MediaKindMeta{Library: lib, Levels: []KindLevel{show, {Key: "album", Label: "Album", Role: "album"}, ep}}, false},
		{"episode not playable", MediaKindMeta{Library: lib, Levels: []KindLevel{show, arc, {Key: "episode", Label: "Episode", Role: "episode"}}}, false},
		{"music kind", MediaKindMeta{Media: "music", Library: lib, Levels: []KindLevel{show, arc, ep}}, false},
		{"episode parented to the show", MediaKindMeta{Library: lib, Levels: []KindLevel{show, arc, {Key: "episode", Label: "Episode", Role: "episode", Playable: true, Parent: "show"}}}, false},
	}
	for _, tc := range cases {
		if got := tc.kind.ShowShaped(); got != tc.want {
			t.Errorf("%s: ShowShaped = %v, want %v", tc.name, got, tc.want)
		}
	}
}
