# TOP Posters for Silo

Artwork provider for [Silo](https://siloserver.org). Movie and series posters
with ratings, season posters, transparent title logos and backgrounds come from
your TOP Posters API key. Their look follows your TOP Posters dashboard, where
the server appears as the **Silo Server** device: its settings apply to everyone
who watches through it, because Silo caches the images and serves the same ones
to every client.

> Prototype. Silo is pre-release and its plugin API may still change.

## How it works

The plugin is an artwork-only metadata provider (`lookup_images`). It never
identifies an item: TMDB or TVDB match the title, and TOP Posters uses the TMDB
ID they found (or the IMDb ID when there is no TMDB ID).

Images are stored in Silo as `topposters://poster/tmdb/movie-603` and turned into
API URLs only when Silo downloads them, through the plugin's `image_resolver`.
The API key therefore never ends up in Silo's database, and changing it applies
to every image at once. Silo caches each image as WebP at several widths.

| Silo artwork | TOP Posters endpoint | Default |
|---|---|---|
| Movie and series poster | `poster/{id}.jpg` | on |
| Season poster (season image picker) | `poster/{id}-sN.jpg` | on |
| Background | `background/{id}.jpg` | off (Premium) |
| Logo | `logo/{id}.png` | off (Premium) |

## Set up

1. Download the binary for your server from the
   [latest release](https://github.com/TOP-Posters/Silo.Plugin.TopPosters/releases/latest):
   `plugin-linux-amd64` for most servers and NAS, `plugin-linux-arm64` for ARM
   devices and Docker on Apple silicon, `plugin-darwin-arm64` or
   `plugin-freebsd-*` for Silo installed directly on macOS or FreeBSD. Install
   it in **Admin > Integrations > Plugins > Install from a file**.
2. Open the plugin settings, paste the API key and choose the artwork types.
   Logos and backgrounds also need to be enabled in the TOP Posters dashboard.
3. New libraries list TOP Posters right after local NFO files and before
   TMDB/TVDB for movies, series and seasons. For a library created before the
   plugin was installed, open **Admin > Libraries**, edit the library and in
   **Provider Priority** move **TOP Posters** above TMDB and TVDB at those
   levels. Silo takes each image from the first provider in the chain that has
   one.
4. Refresh the library metadata.

## Limits

- **Season posters** appear in the season image picker but are not applied by a
  refresh: Silo takes season posters on refresh only from the provider that
  matched the series.
- **Episode thumbnails** are not available: Silo asks only the matching
  provider for episode stills.
- **Locked images**: an item whose image an admin picked or uploaded keeps it.
  Choose the TOP Posters image in **Edit Metadata > Images** to switch.
- **Rating updates**: Silo keeps its cached copy until the item's metadata is
  refreshed.

## Build and test

Go is not needed locally; the targets run in the `golang` image when `go` is
missing.

```bash
make test
make build-all   # dist/plugin-<os>-<arch> for every platform Silo supports
```

## Release

Raise `"version"` in `manifest.json` in a commit on `main`, then push the tag
`v<version>` on that commit. The release workflow builds the binaries with
Silo's shared `plugin-release.yml`, signs them and publishes them with
`checksums.txt` and `plugin.sigstore.json`.
