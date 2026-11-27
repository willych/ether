package plugins

import "github.com/braedonsaunders/ether/internal/core"

// ShowShaped reports whether a content kind is shaped like a TV show, so
// it can join a show catalogue library (TMDB TV, TheTVDB) as a source next
// to catalogue titles. It needs exactly three levels in parent order, all
// of video media: a root with role "show", a middle level with role
// "season" or no role (a season-like grouping, such as an arc), and a
// playable leaf with role "episode". The kind must also declare a library,
// because its items are stored and browsed through that library.
func (k MediaKindMeta) ShowShaped() bool {
	if !k.HasLibrary() || len(k.Levels) != 3 {
		return false
	}
	show, season, episode := k.Levels[0], k.Levels[1], k.Levels[2]
	if show.Role != "show" || episode.Role != "episode" || !episode.Playable {
		return false
	}
	if season.Role != "" && season.Role != "season" {
		return false
	}
	if season.Members || episode.Members || show.Members {
		return false
	}
	if k.LevelParent(season.Key) != show.Key || k.LevelParent(episode.Key) != season.Key {
		return false
	}
	for _, lv := range k.Levels {
		if k.EffectiveLevelMedia(lv.Key, core.MediaClassVideo) != core.MediaClassVideo {
			return false
		}
	}
	return true
}
