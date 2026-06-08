# Project Notes for Agents

## Product

Build a modern Instagram-like infinite media feed for local photos, videos, audio, generic files, and drawing boards. Test media files live in `test-content`.

## Architecture

- Use one Go server for production. It serves the API, the built Svelte SPA, and media files.
- Backend code should follow common Go project layout:
  - `cmd/server` for the executable entrypoint.
  - `internal/media` for media scanning, sorting, pagination, and lookup.
  - `internal/server` for HTTP routing, handlers, middleware, and static serving.
- Use the standard Go `net/http` server.
- Agents may start the local server only for their own short verification checks and must stop it immediately afterward, unless the user explicitly asks to keep or start the server.
- In the sandbox, run Go commands with `GOCACHE=/tmp/feed-ai-go-cache`; the default macOS Go cache under `~/Library/Caches/go-build` is not writable and causes `operation not permitted`.
- Do not use WebSocket for regular application features. Asteroids is the explicit exception: its bidirectional real-time protocol uses WebSocket for input commands and authoritative snapshots. Use SSE for other server-to-client async communication.
- The app does not need a database, auth, or personalization for v1. Uploads, media likes, and comment likes are filesystem-backed.
- Comments are required for v1, but must remain filesystem-backed; do not add a database.
- Store comments in server-managed text files whose names are deterministically associated with the media file. Do not trust client-supplied filesystem paths.
- When a new comment is created, the backend should create the corresponding comment file if it does not exist and append the comment there.
- Record all important intermediate results, decisions, verification outcomes, and known issues in `MEMORY.md` as the project evolves.

## Frontend

- Use Svelte 5 with runes. Avoid legacy Svelte practices.
- Use Vite.
- Use Tailwind CSS 4.x. Follow the in-house design system documented in `DESIGN.md`; do not add daisyUI.
- Use lucide icons via the Svelte package.
- Target the main modern browsers.
- The UI should be a modern one-column Instagram-like feed with infinite loading.
- Each media card should show the latest 1-2 comments inline when available.
- Clicking the comments affordance should open a panel with all comments for that media item and a comment input field.

## Media Feed Rules

- Read test media from `test-content`.
- Support photos, videos, audio, generic files, and `.board` drawing cards.
- Sort media by file modification time for feed display, newest first. Use filename as a stable tie-breaker. The current API stores the runtime index oldest-to-newest internally and the frontend renders newest-first by walking indexes downward.
- The feed must handle an empty `test-content` directory gracefully.
- Expose index-based main-feed loading through `GET /api/feed?index=`. `index=-1` returns the newest item and bounds; non-negative indexes fetch exact items.
- Serve media through safe server-controlled URLs, not by trusting arbitrary paths from the client.

## Favorites and Boards

- Favorites are browser-owned under `localStorage` key `feed-ai:favorites`. The current UI resolves saved IDs with `GET /api/media/{id}` and removes stale `404` IDs client-side; do not assume `POST /api/feed/favorites` drives the UI.
- Regular boards are media items represented by root `.board` placeholder files and rendered through the main feed. Stroke history lives under `test-content/.boards/`.
- Image-backed boards use the image media ID with `GET /api/boards/{mediaID}` and receive a direct `board.background.url`, normally `/media/{id}`. The current UI does not fetch `GET /api/boards/{id}/background`.
- The master board is the only non-feed board. It uses fixed ID `master`, lives at `test-content/.boards/master.jsonl`, and is shown from the sidebar.

## Comment Rules

- Comments are tied to media items by the backend using the same safe media identity/lookup model as media serving.
- Expose API endpoints for reading all comments for a media item and creating a new comment.
- Feed responses should include enough comment summary data for the frontend to render the latest 1-2 comments without fetching every full comment thread up front.
- Store comment timestamps and comment text in a simple append-friendly format. Keep parsing deterministic and robust against newlines or delimiter characters in user text.
- Validate and trim submitted comment text. Reject empty comments.
- The comments panel should handle empty, loading, error, and submit-in-progress states gracefully.
