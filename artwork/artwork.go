// Package artwork turns Silo items into TOP Posters image paths and URLs.
//
// Silo stores what a plugin returns from GetImages and, at serve time, asks the
// plugin that owns the path's scheme to turn it into a URL. The stored path
// therefore never carries the API key: topposters://poster/tmdb/movie-603.
// Changing the key in the plugin settings applies to every stored image at once,
// and the key does not end up in Silo's database.
package artwork

import (
	"fmt"
	"net/url"
	"strings"
)

// Scheme is the image_resolver scheme declared in manifest.json.
const Scheme = "topposters"

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://api.top-posters.com"

// UserAgent identifies the server in the TOP Posters dashboard, like
// jellyfin_client does for the Jellyfin plugin.
const UserAgent = "silo_client"

// Kind is the artwork type in Silo's vocabulary (ImageRecord.kind).
type Kind string

const (
	Poster   Kind = "poster"
	Backdrop Kind = "backdrop"
	Logo     Kind = "logo"
)

// Settings mirrors the plugin's global configuration.
type Settings struct {
	APIKey        string
	BaseURL       string
	Posters       bool
	SeasonPosters bool
	Backdrops     bool
	Logos         bool
}

// DefaultSettings matches the Jellyfin plugin: posters and season posters on,
// logos and backgrounds off (they need Premium and the dashboard toggle).
func DefaultSettings() Settings {
	return Settings{BaseURL: DefaultBaseURL, Posters: true, SeasonPosters: true}
}

// Enabled reports whether the kind is switched on.
func (s Settings) Enabled(kind Kind) bool {
	switch kind {
	case Poster:
		return s.Posters
	case Backdrop:
		return s.Backdrops
	case Logo:
		return s.Logos
	}
	return false
}

// Request is what the plugin knows about the item Silo asks for.
type Request struct {
	ItemType    string            // "movie" or "series"
	ProviderIDs map[string]string // "tmdb", "imdb", "tvdb", ...
	Season      *int              // set for a season gallery
}

// Image is one path to hand back to Silo.
type Image struct {
	Kind   Kind
	Path   string // topposters://...
	Season *int
}

// ItemID returns TOP Posters' id type and id for the item: TMDB first, as in
// the Jellyfin plugin, then IMDb. TMDB ids need the movie-/series- prefix
// because the two TMDB id spaces overlap.
func ItemID(itemType string, ids map[string]string) (idType, id string, ok bool) {
	prefix := ""
	switch itemType {
	case "movie":
		prefix = "movie-"
	case "series", "season":
		prefix = "series-"
	default:
		return "", "", false
	}
	if tmdb := strings.TrimSpace(ids["tmdb"]); tmdb != "" && isDigits(tmdb) {
		return "tmdb", prefix + tmdb, true
	}
	if imdb := strings.TrimSpace(ids["imdb"]); strings.HasPrefix(imdb, "tt") && isDigits(imdb[2:]) {
		return "imdb", imdb, true
	}
	return "", "", false
}

// Images lists the artwork TOP Posters can provide for the request.
func Images(s Settings, req Request) []Image {
	idType, id, ok := ItemID(req.ItemType, req.ProviderIDs)
	if !ok {
		return nil
	}
	if req.Season != nil {
		// Season galleries only ever show posters.
		if !s.SeasonPosters || req.ItemType == "movie" || *req.Season < 0 {
			return nil
		}
		season := *req.Season
		return []Image{{
			Kind:   Poster,
			Path:   Path(Poster, idType, fmt.Sprintf("%s-s%d", id, season)),
			Season: &season,
		}}
	}
	var images []Image
	for _, kind := range []Kind{Poster, Backdrop, Logo} {
		if s.Enabled(kind) {
			images = append(images, Image{Kind: kind, Path: Path(kind, idType, id)})
		}
	}
	return images
}

// Path builds the stored path for an image.
func Path(kind Kind, idType, id string) string {
	return fmt.Sprintf("%s://%s/%s/%s", Scheme, kind, idType, url.PathEscape(id))
}

// Resolve turns a stored path into the API URL, or "" when the path is not
// ours, is malformed, or no API key is configured.
func Resolve(s Settings, path string) string {
	key := strings.TrimSpace(s.APIKey)
	if key == "" {
		return ""
	}
	// The host strips the scheme before calling the resolver
	// (PluginImageResolver groups paths by scheme and sends the bare rest), so
	// "poster/tmdb/movie-603" is the usual form. A full path is accepted too.
	rest := path
	if scheme, bare, found := strings.Cut(path, "://"); found {
		if scheme != Scheme {
			return ""
		}
		rest = bare
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[2] == "" {
		return ""
	}
	kind, idType, id := Kind(parts[0]), parts[1], parts[2]
	if idType != "tmdb" && idType != "imdb" {
		return ""
	}
	var endpoint, ext string
	switch kind {
	case Poster:
		endpoint, ext = "poster", "jpg"
	case Backdrop:
		endpoint, ext = "background", "jpg"
	case Logo:
		endpoint, ext = "logo", "png"
	default:
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s.%s?user_agent=%s",
		base, url.PathEscape(key), idType, endpoint, id, ext, UserAgent)
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
