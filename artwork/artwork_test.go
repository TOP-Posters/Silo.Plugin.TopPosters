package artwork

import (
	"reflect"
	"testing"
)

func intPtr(value int) *int { return &value }

func TestItemIDPrefersTMDB(t *testing.T) {
	cases := []struct {
		itemType string
		ids      map[string]string
		idType   string
		id       string
		ok       bool
	}{
		{"movie", map[string]string{"tmdb": "603", "imdb": "tt0133093"}, "tmdb", "movie-603", true},
		{"series", map[string]string{"tmdb": "1396"}, "tmdb", "series-1396", true},
		{"movie", map[string]string{"imdb": "tt0133093"}, "imdb", "tt0133093", true},
		{"movie", map[string]string{"tmdb": "abc", "imdb": "tt0133093"}, "imdb", "tt0133093", true},
		{"movie", map[string]string{"tvdb": "81189"}, "", "", false},
		{"movie", map[string]string{"imdb": "nm0000206"}, "", "", false},
		{"episode", map[string]string{"tmdb": "62085"}, "", "", false},
		{"audiobook", map[string]string{"tmdb": "1"}, "", "", false},
	}
	for _, c := range cases {
		idType, id, ok := ItemID(c.itemType, c.ids)
		if idType != c.idType || id != c.id || ok != c.ok {
			t.Errorf("ItemID(%q, %v) = %q, %q, %v; want %q, %q, %v",
				c.itemType, c.ids, idType, id, ok, c.idType, c.id, c.ok)
		}
	}
}

func TestImagesFollowSettings(t *testing.T) {
	req := Request{ItemType: "series", ProviderIDs: map[string]string{"tmdb": "1396"}}

	got := Images(DefaultSettings(), req)
	want := []Image{{Kind: Poster, Path: "topposters://poster/tmdb/series-1396"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default images = %+v, want %+v", got, want)
	}

	all := DefaultSettings()
	all.Backdrops, all.Logos = true, true
	got = Images(all, req)
	want = []Image{
		{Kind: Poster, Path: "topposters://poster/tmdb/series-1396"},
		{Kind: Backdrop, Path: "topposters://backdrop/tmdb/series-1396"},
		{Kind: Logo, Path: "topposters://logo/tmdb/series-1396"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("all images = %+v, want %+v", got, want)
	}

	none := Settings{}
	if got := Images(none, req); len(got) != 0 {
		t.Fatalf("everything off gave %+v", got)
	}
}

func TestSeasonGalleryOffersSeasonPoster(t *testing.T) {
	req := Request{ItemType: "series", ProviderIDs: map[string]string{"tmdb": "1396"}, Season: intPtr(2)}
	got := Images(DefaultSettings(), req)
	if len(got) != 1 || got[0].Path != "topposters://poster/tmdb/series-1396-s2" || *got[0].Season != 2 {
		t.Fatalf("season images = %+v", got)
	}

	specials := Images(DefaultSettings(), Request{ItemType: "series", ProviderIDs: map[string]string{"imdb": "tt0903747"}, Season: intPtr(0)})
	if len(specials) != 1 || specials[0].Path != "topposters://poster/imdb/tt0903747-s0" {
		t.Fatalf("specials = %+v", specials)
	}

	off := DefaultSettings()
	off.SeasonPosters = false
	if got := Images(off, req); len(got) != 0 {
		t.Fatalf("season posters off gave %+v", got)
	}
	if got := Images(DefaultSettings(), Request{ItemType: "movie", ProviderIDs: map[string]string{"tmdb": "603"}, Season: intPtr(1)}); len(got) != 0 {
		t.Fatalf("movie season gave %+v", got)
	}
}

func TestResolve(t *testing.T) {
	s := DefaultSettings()
	s.APIKey = "TP-abc/123"

	cases := map[string]string{
		"topposters://poster/tmdb/movie-603":       "https://api.top-posters.com/TP-abc%2F123/tmdb/poster/movie-603.jpg?user_agent=silo_client",
		"poster/tmdb/movie-603":                    "https://api.top-posters.com/TP-abc%2F123/tmdb/poster/movie-603.jpg?user_agent=silo_client",
		"logo/imdb/tt0133093":                      "https://api.top-posters.com/TP-abc%2F123/imdb/logo/tt0133093.png?user_agent=silo_client",
		"topposters://poster/tmdb/series-1396-s2":  "https://api.top-posters.com/TP-abc%2F123/tmdb/poster/series-1396-s2.jpg?user_agent=silo_client",
		"topposters://backdrop/imdb/tt0133093":     "https://api.top-posters.com/TP-abc%2F123/imdb/background/tt0133093.jpg?user_agent=silo_client",
		"topposters://logo/tmdb/series-1396":       "https://api.top-posters.com/TP-abc%2F123/tmdb/logo/series-1396.png?user_agent=silo_client",
		"tmdb://poster/abc.jpg":                    "",
		"topposters://thumb/tmdb/movie-603":        "",
		"topposters://poster/tvdb/81189":           "",
		"topposters://poster/tmdb/":                "",
		"topposters://poster/tmdb/movie-603/extra": "",
	}
	for path, want := range cases {
		if got := Resolve(s, path); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", path, got, want)
		}
	}

	s.BaseURL = "http://localhost:8081/"
	if got := Resolve(s, "topposters://poster/tmdb/movie-603"); got != "http://localhost:8081/TP-abc%2F123/tmdb/poster/movie-603.jpg?user_agent=silo_client" {
		t.Errorf("custom base = %q", got)
	}

	s.APIKey = " "
	if got := Resolve(s, "topposters://poster/tmdb/movie-603"); got != "" {
		t.Errorf("without a key = %q", got)
	}
}
