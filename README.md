# Feed AI

Feed AI is a local, Instagram-style media feed for photos, videos, audio, files, and collaborative drawing boards. The app scans media from a filesystem directory, serves it through safe server-controlled IDs, and keeps comments, likes, metadata, and drawing history in text files next to the content.

## Features

- One-column infinite feed with virtualized rendering.
- Images, GIFs, videos, audio files, generic files, and `.board` drawing cards in the same main feed.
- Uploads from the browser into the configured content root.
- Filesystem-backed comments with author names, likes, inline previews, full comment panel, and live updates through SSE.
- Filesystem-backed media likes, metadata, audio covers, video posters, and drawing-board strokes.
- Favorites stored in browser `localStorage` and rendered in the user's saved order.
- Russian UI strings in `web/src/lib/ui_text.ts`.
- Dark glass media-first design system documented in
  [`docs/design/design-system.md`](docs/design/design-system.md).

## Stack

- Backend: Go `net/http`, entrypoint in `cmd/server`.
- Frontend: Svelte 5, Vite, Tailwind CSS 4, lucide-svelte.
- Storage: plain files under the media content directory. There is no database.

## Repository Layout

```text
cmd/server/          Go server entrypoint
internal/media/      Media scan, lookup, pagination, comments, metadata, boards
internal/server/     HTTP routes, handlers, middleware, static serving, SSE
web/                 Svelte SPA
test-content/        Local test media and server-managed sidecar data
tools/ffmpeg/        Optional bundled FFmpeg/FFprobe binaries for packaging
docs/                Engineering knowledge vault and architecture decisions
```

## Requirements

- Go 1.25 or newer.
- Node.js and npm.
- Optional: FFmpeg and FFprobe for video posters, video duration, audio duration, and embedded cover extraction. The server first checks `tools/ffmpeg/{GOOS}-{GOARCH}/` and then falls back to `PATH`.

## Development

Install frontend dependencies once:

```sh
cd web
npm install
```

Run the Go API server:

```sh
go run ./cmd/server -addr :8080 -content-dir test-content -static-dir web/dist
```

In another terminal, run the Vite dev server:

```sh
cd web
npm run dev
```

Open `http://localhost:5173`. Vite proxies `/api` and `/media` to `http://localhost:8080` by default. To point Vite at another backend:

```sh
VITE_API_TARGET=http://localhost:18080 npm run dev
```

## Production-Style Local Run

Build the SPA and serve it from the Go server:

```sh
cd web
npm run build
cd ..
go run ./cmd/server -addr :8080 -content-dir test-content -static-dir web/dist
```

Open `http://localhost:8080`.

## Verification

Run all current checks:

```sh
make check
```

This runs:

```sh
go run ./tools/docscheck
cd web && npm run check
go test ./...
```

Build only the frontend:

```sh
make web-build
```

Create a Windows x64 package:

```sh
make package-win
```

The Windows package is written under `build/feed-ai-win64` and zipped as `build/feed-ai-win64.zip`. The starter board sticker pack is copied into the packaged `test-content/.boards/sticker-pack` directory. If local Windows FFmpeg binaries are present under `tools/ffmpeg/windows-amd64/`, they are copied into the package.

## Content Directory

By default, the server reads and writes under `test-content`.

Supported root media types include:

- Images: `.avif`, `.gif`, `.jpeg`, `.jpg`, `.png`, `.webp`
- Video: `.m4v`, `.mov`, `.mp4`, `.ogg`, `.ogv`, `.webm`
- Audio: `.aac`, `.flac`, `.m4a`, `.mp3`, `.oga`, `.opus`, `.wav`
- Drawing boards: `.board`
- Other non-hidden files are indexed as generic file cards.

Server-managed internal directories are hidden from the feed:

```text
test-content/.comments/   JSONL comment files
test-content/.metadata/   JSON metadata and like counts
test-content/.posters/    Cached video poster JPEGs
test-content/.covers/     Extracted audio cover JPEGs
test-content/.boards/     JSONL drawing-board metadata and strokes
```

Media IDs are opaque SHA-256 hex IDs derived by the backend from normalized root filenames. Clients must use IDs returned by the API and must not send filesystem paths.

## API Overview

Current main-feed loading is index-based:

- `GET /api/feed?index=-1` returns the newest item plus `firstIndex` and `lastIndex`.
- `GET /api/feed?index=N` returns the exact item at index `N`.
- Internally, index `0` is the oldest item and `lastIndex` is the newest item. The frontend walks indexes downward to render newest-first infinite loading.

Other feed and media endpoints:

- `GET /api/activity?limit=30`
- `GET /api/media/{id}`
- `GET /media/{id}`
- `GET /api/media/{id}/poster?time=0`
- `GET /api/media/{id}/cover`
- `POST /api/uploads` multipart form with one `files` part and an optional `modifiedAt` part in Unix milliseconds. Uploads are capped at 5 GiB.

Favorites are currently a frontend-owned view. The browser stores media IDs in `localStorage` under `feed-ai:favorites`; newly favorited items are inserted at the front of that list. When the user opens favorites, the frontend walks that saved list in order and resolves each card with `GET /api/media/{id}`. Missing/stale IDs are removed client-side when the server returns `404`. The older `POST /api/feed/favorites` backend endpoint still exists, but the current Svelte UI does not use it for rendering favorites.

Comments and likes:

- `GET /api/media/{id}/comments`
- `POST /api/media/{id}/comments` with `{ "text": "...", "author": "..." }`
- `POST /api/media/{id}/comments/{commentID}/likes`
- `POST /api/media/{id}/likes`
- `GET /api/comments/events` for SSE comment, like, activity, and new-feed-item events

Drawing boards:

- `POST /api/boards` with `{ "name": "..." }`, which creates a root `.board` placeholder and inserts it into the media feed
- `GET /api/boards/{id}`
- `POST /api/boards/{id}/strokes`
- `GET /api/boards/events` for SSE stroke events

Regular boards are feed media. They are discovered from root `.board` placeholder files, returned by the same `GET /api/feed?index=` flow as other media, and store their stroke history in `test-content/.boards/`. Image-backed boards also use their media ID with the board endpoints. `GET /api/boards/{id}` returns board metadata, strokes, and any background URL needed by the client; current image-backed boards point directly at the root media URL (`/media/{id}`), so the UI does not fetch a separate board background endpoint. The master board is the exception: it uses the fixed ID `master`, lives only in `test-content/.boards/master.jsonl`, and is shown from the sidebar rather than as a main-feed item. The older `GET /api/boards` list endpoint and `GET /api/boards/{id}/background` background endpoint still exist in the backend, but the current Svelte UI does not use them to render boards.

Asteroids:

- `GET /api/ships/socket` upgrades to the game WebSocket. The server owns the `1600x900` simulation and accepts only sequenced input/action commands.
- `GET /api/ships/scores` returns the filesystem-backed solo leaderboard. Completed timed solo rounds are saved by the server; client score submission is not exposed.
- The welcome message includes a resume token that restores the same player for up to 10 seconds after a disconnect.
- The authoritative world periodically spawns collectible boosts: shield, triple shot, rapid fire, overdrive, and nova. Snapshots include boost positions and active player effects; pickup, duration, weapon behavior, shield absorption, and nova destruction are all resolved by the server. Nova clears current asteroids and destroys every active enemy ship, bypassing shields.

## Notes for Contributors

- Keep production as one Go server that serves the API, media files, and built SPA.
- Keep WebSocket limited to Asteroids; other one-way live updates use SSE.
- Keep comments and other v1 state filesystem-backed; do not add a database.
- Start engineering work from [`docs/index.md`](docs/index.md).
- Keep UI work aligned with the documented design system and existing Svelte 5
  rune patterns.
- Update current reference notes with behavior changes; use ADRs only for
  consequential architectural decisions.
