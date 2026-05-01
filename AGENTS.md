# Project Notes for Agents

## Product

Build a modern Instagram-like infinite media feed for photos and videos. Test media files live in `test-content`.

## Architecture

- Use one Go server for production. It serves the API, the built Svelte SPA, and media files.
- Backend code should follow common Go project layout:
  - `cmd/server` for the executable entrypoint.
  - `internal/media` for media scanning, sorting, pagination, and lookup.
  - `internal/server` for HTTP routing, handlers, middleware, and static serving.
- Use the standard Go `net/http` server.
- Do not use WebSocket. If async server-to-client communication is needed, use SSE.
- The app does not need a database, uploads, auth, likes, comments, or personalization for v1.
- Record all important intermediate results, decisions, verification outcomes, and known issues in `MEMORY.md` as the project evolves.

## Frontend

- Use Svelte 5 with runes. Avoid legacy Svelte practices.
- Use Vite.
- Use Tailwind CSS 4.x and daisyUI 5.x.
- Use lucide icons via the Svelte package.
- Target the main modern browsers.
- The UI should be a modern one-column Instagram-like feed with infinite loading.

## Media Feed Rules

- Read test media from `test-content`.
- Support photos and videos.
- Sort media by file modification time, newest first. Use filename as a stable tie-breaker.
- The feed must handle an empty `test-content` directory gracefully.
- Expose cursor-based pagination through `GET /api/feed?cursor=&limit=`.
- Serve media through safe server-controlled URLs, not by trusting arbitrary paths from the client.
