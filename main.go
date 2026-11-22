// Command plugin is the TOP Posters artwork provider for Silo.
//
// It is an artwork-only enrichment provider (lookup_images): it never
// identifies an item. Search and GetMetadata answer empty, and GetImages works
// from the TMDB or IMDb ID a primary provider such as TMDB already resolved.
// Images are stored as topposters:// paths and turned into API URLs at serve
// time by the image_resolver capability, so the API key stays in the plugin
// settings.
package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	publicmanifest "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/manifest"
	"github.com/Silo-Server/silo-plugin-sdk/pkg/pluginsdk/runtime"

	"github.com/TOP-Posters/Silo.Plugin.TopPosters/artwork"
)

// version is set at build time via -ldflags "-X main.version=...".
var version string

//go:embed manifest.json
var manifestJSON []byte

// imageRating lifts TOP Posters artwork over an image already on the item.
// Silo replaces stored artwork on a scheduled refresh only with a rated image
// (applyBestImages); within a language tier the first provider in chain order
// still wins, so the library must list TOP Posters above TMDB.
const imageRating = 10

type settingsStore struct {
	mu       sync.RWMutex
	settings artwork.Settings
}

func (s *settingsStore) get() artwork.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

func (s *settingsStore) set(settings artwork.Settings) {
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
}

type runtimeServer struct {
	pluginv1.UnimplementedRuntimeServer
	manifest *pluginv1.PluginManifest
	store    *settingsStore
}

func (s *runtimeServer) GetManifest(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
	return &pluginv1.GetManifestResponse{Manifest: s.manifest}, nil
}

func (s *runtimeServer) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	s.store.set(settingsFromConfig(req.GetConfig()))
	return &pluginv1.ConfigureResponse{}, nil
}

type metadataServer struct {
	pluginv1.UnimplementedMetadataProviderServer
	pluginv1.UnimplementedImageResolverServer
	store *settingsStore
}

// Search answers nothing: identity belongs to the primary provider.
func (s *metadataServer) Search(context.Context, *pluginv1.SearchMetadataRequest) (*pluginv1.SearchMetadataResponse, error) {
	return &pluginv1.SearchMetadataResponse{}, nil
}

// The host calls every MetadataProvider method on every provider in the
// chain. TOP Posters has only artwork, so the rest answer empty: an error
// would add a warning to every match and refresh.

func (s *metadataServer) GetMetadata(context.Context, *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	return &pluginv1.GetMetadataResponse{}, nil
}

func (s *metadataServer) GetPersonDetail(context.Context, *pluginv1.GetPersonDetailRequest) (*pluginv1.GetPersonDetailResponse, error) {
	return &pluginv1.GetPersonDetailResponse{}, nil
}

func (s *metadataServer) GetSeasons(context.Context, *pluginv1.GetSeasonsRequest) (*pluginv1.GetSeasonsResponse, error) {
	return &pluginv1.GetSeasonsResponse{}, nil
}

func (s *metadataServer) GetEpisodes(context.Context, *pluginv1.GetEpisodesRequest) (*pluginv1.GetEpisodesResponse, error) {
	return &pluginv1.GetEpisodesResponse{}, nil
}

func (s *metadataServer) GetImages(_ context.Context, req *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	settings := s.store.get()
	if strings.TrimSpace(settings.APIKey) == "" {
		return &pluginv1.GetImagesResponse{}, nil
	}
	request := artwork.Request{
		ItemType:    req.GetItemType(),
		ProviderIDs: stringMap(req.GetProviderIds()),
	}
	if req.SeasonNumber != nil {
		season := int(req.GetSeasonNumber())
		request.Season = &season
	}
	response := &pluginv1.GetImagesResponse{}
	for _, image := range artwork.Images(settings, request) {
		record := &pluginv1.ImageRecord{
			Kind:     string(image.Kind),
			Url:      image.Path,
			Language: imageLanguage(image.Kind, req.GetLanguage()),
			Metadata: imageMetadata(image.Kind),
		}
		if image.Season != nil {
			season := int32(*image.Season)
			record.SeasonNumber = &season
		}
		response.Images = append(response.Images, record)
	}
	return response, nil
}

// imageLanguage tags posters and logos with the library language, the first
// tier Silo tries for artwork with text. Backgrounds stay language-neutral,
// the tier Silo prefers for backdrops.
func imageLanguage(kind artwork.Kind, language string) string {
	if kind == artwork.Backdrop {
		return ""
	}
	language = strings.TrimSpace(language)
	if language == "" {
		return "en"
	}
	return language
}

func imageMetadata(kind artwork.Kind) *structpb.Struct {
	fields := map[string]any{"rating": float64(imageRating)}
	if kind == artwork.Poster {
		fields["includes_text"] = true
	}
	value, err := structpb.NewStruct(fields)
	if err != nil {
		panic(err)
	}
	return value
}

func (s *metadataServer) ResolveImageURL(_ context.Context, req *pluginv1.ResolveImageURLRequest) (*pluginv1.ResolveImageURLResponse, error) {
	return &pluginv1.ResolveImageURLResponse{Url: artwork.Resolve(s.store.get(), req.GetPath())}, nil
}

func (s *metadataServer) ResolveImageURLs(_ context.Context, req *pluginv1.ResolveImageURLsRequest) (*pluginv1.ResolveImageURLsResponse, error) {
	settings := s.store.get()
	urls := make(map[string]string, len(req.GetPaths()))
	for _, path := range req.GetPaths() {
		urls[path] = artwork.Resolve(settings, path)
	}
	return &pluginv1.ResolveImageURLsResponse{Urls: urls}, nil
}

// settingsFromConfig reads the "account" and "artwork" entries declared in
// manifest.json. A switch the admin never saved keeps its default.
func settingsFromConfig(entries []*pluginv1.ConfigEntry) artwork.Settings {
	settings := artwork.DefaultSettings()
	for _, entry := range entries {
		if entry == nil || entry.GetValue() == nil {
			continue
		}
		values := entry.GetValue().AsMap()
		switch entry.GetKey() {
		case "account":
			if key, ok := values["api_key"].(string); ok {
				settings.APIKey = strings.TrimSpace(key)
			}
			if base, ok := values["base_url"].(string); ok && strings.TrimSpace(base) != "" {
				settings.BaseURL = strings.TrimSpace(base)
			}
		case "artwork":
			readBool(values, "posters", &settings.Posters)
			readBool(values, "season_posters", &settings.SeasonPosters)
			readBool(values, "backdrops", &settings.Backdrops)
			readBool(values, "logos", &settings.Logos)
		}
	}
	return settings
}

func readBool(values map[string]any, key string, target *bool) {
	if value, ok := values[key].(bool); ok {
		*target = value
	}
}

func stringMap(value *structpb.Struct) map[string]string {
	result := make(map[string]string)
	if value == nil {
		return result
	}
	for key, raw := range value.AsMap() {
		if text, ok := raw.(string); ok && text != "" {
			result[key] = text
		}
	}
	return result
}

func main() {
	manifest, err := loadManifest()
	if err != nil {
		panic(err)
	}
	store := &settingsStore{settings: artwork.DefaultSettings()}
	metadata := &metadataServer{store: store}
	runtime.Serve(runtime.ServeConfig{
		Servers: runtime.CapabilityServers{
			Runtime:          &runtimeServer{manifest: manifest, store: store},
			MetadataProvider: metadata,
			ImageResolver:    metadata,
		},
	})
}

func loadManifest() (*pluginv1.PluginManifest, error) {
	manifest, err := publicmanifest.Load(manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest: %w", err)
	}
	if version != "" {
		manifest.Version = version
	}
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	binary, err := os.ReadFile(executablePath)
	if err != nil {
		return nil, fmt.Errorf("read executable %q: %w", executablePath, err)
	}
	checksum := sha256.Sum256(binary)
	manifest.Checksum = hex.EncodeToString(checksum[:])
	return manifest, nil
}
