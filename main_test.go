package main

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"

	"github.com/TOP-Posters/Silo.Plugin.TopPosters/artwork"
)

func entry(t *testing.T, key string, values map[string]any) *pluginv1.ConfigEntry {
	t.Helper()
	value, err := structpb.NewStruct(values)
	if err != nil {
		t.Fatal(err)
	}
	return &pluginv1.ConfigEntry{Key: key, Value: value}
}

func TestManifestLoads(t *testing.T) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.GetPluginId() != "topposters.artwork" || len(manifest.GetCapabilities()) != 2 {
		t.Fatalf("unexpected manifest %+v", manifest)
	}
}

func TestSettingsFromConfig(t *testing.T) {
	got := settingsFromConfig(nil)
	if got != artwork.DefaultSettings() {
		t.Fatalf("no config = %+v", got)
	}

	got = settingsFromConfig([]*pluginv1.ConfigEntry{
		entry(t, "account", map[string]any{"api_key": "  TP-key  ", "base_url": " "}),
		entry(t, "artwork", map[string]any{"posters": false, "logos": true}),
	})
	want := artwork.Settings{APIKey: "TP-key", BaseURL: artwork.DefaultBaseURL, Posters: false, SeasonPosters: true, Logos: true}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

func newServer(settings artwork.Settings) *metadataServer {
	return &metadataServer{store: &settingsStore{settings: settings}}
}

func TestGetImagesNeedsKey(t *testing.T) {
	ids, _ := structpb.NewStruct(map[string]any{"tmdb": "603"})
	response, err := newServer(artwork.DefaultSettings()).GetImages(context.Background(), &pluginv1.GetImagesRequest{ItemType: "movie", ProviderIds: ids})
	if err != nil || len(response.GetImages()) != 0 {
		t.Fatalf("without a key: %v, %v", response, err)
	}
}

func TestGetImagesRecords(t *testing.T) {
	settings := artwork.DefaultSettings()
	settings.APIKey = "TP-key"
	settings.Backdrops, settings.Logos = true, true
	ids, _ := structpb.NewStruct(map[string]any{"tmdb": "603", "imdb": "tt0133093"})

	response, err := newServer(settings).GetImages(context.Background(), &pluginv1.GetImagesRequest{
		ItemType: "movie", ProviderIds: ids, Language: "it",
	})
	if err != nil {
		t.Fatal(err)
	}
	images := response.GetImages()
	if len(images) != 3 {
		t.Fatalf("images = %v", images)
	}
	languages := map[string]string{}
	for _, image := range images {
		languages[image.GetKind()] = image.GetLanguage()
		if image.GetMetadata().GetFields()["rating"].GetNumberValue() <= 0 {
			t.Errorf("%s has no rating", image.GetKind())
		}
	}
	if languages["poster"] != "it" || languages["logo"] != "it" || languages["backdrop"] != "" {
		t.Fatalf("languages = %v", languages)
	}
	if !images[0].GetMetadata().GetFields()["includes_text"].GetBoolValue() {
		t.Fatalf("poster should declare text: %v", images[0])
	}

	season := int32(3)
	seriesIDs, _ := structpb.NewStruct(map[string]any{"tmdb": "1396"})
	response, err = newServer(settings).GetImages(context.Background(), &pluginv1.GetImagesRequest{
		ItemType: "series", ProviderIds: seriesIDs, SeasonNumber: &season,
	})
	if err != nil || len(response.GetImages()) != 1 {
		t.Fatalf("season: %v, %v", response, err)
	}
	if got := response.GetImages()[0]; got.GetUrl() != "topposters://poster/tmdb/series-1396-s3" || got.GetSeasonNumber() != 3 || got.GetLanguage() != "en" {
		t.Fatalf("season image = %v", got)
	}
}

func TestResolveImageURLs(t *testing.T) {
	settings := artwork.DefaultSettings()
	settings.APIKey = "TP-key"
	response, err := newServer(settings).ResolveImageURLs(context.Background(), &pluginv1.ResolveImageURLsRequest{
		Paths: []string{"poster/tmdb/movie-603", "tmdb://poster/x.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	urls := response.GetUrls()
	if urls["poster/tmdb/movie-603"] != "https://api.top-posters.com/TP-key/tmdb/poster/movie-603.jpg?user_agent=silo_client" || urls["tmdb://poster/x.jpg"] != "" {
		t.Fatalf("urls = %v", urls)
	}
}
