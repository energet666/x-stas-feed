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
- Agents may start the local server only for their own short verification checks and must stop it immediately afterward, unless the user explicitly asks to keep or start the server.
- Do not use WebSocket. If async server-to-client communication is needed, use SSE.
- The app does not need a database, uploads, auth, likes, or personalization for v1.
- Comments are required for v1, but must remain filesystem-backed; do not add a database.
- Store comments in server-managed text files whose names are deterministically associated with the media file. Do not trust client-supplied filesystem paths.
- When a new comment is created, the backend should create the corresponding comment file if it does not exist and append the comment there.
- Record all important intermediate results, decisions, verification outcomes, and known issues in `MEMORY.md` as the project evolves.

## Frontend

- Use Svelte 5 with runes. Avoid legacy Svelte practices.
- Use Vite.
- Use Tailwind CSS 4.x and daisyUI 5.x.
- Use lucide icons via the Svelte package.
- Target the main modern browsers.
- The UI should be a modern one-column Instagram-like feed with infinite loading.
- Each media card should show the latest 1-2 comments inline when available.
- Clicking the comments affordance should open a panel with all comments for that media item and a comment input field.

## Media Feed Rules

- Read test media from `test-content`.
- Support photos and videos.
- Sort media by file modification time, newest first. Use filename as a stable tie-breaker.
- The feed must handle an empty `test-content` directory gracefully.
- Expose cursor-based pagination through `GET /api/feed?cursor=&limit=`.
- Serve media through safe server-controlled URLs, not by trusting arbitrary paths from the client.

## Comment Rules

- Comments are tied to media items by the backend using the same safe media identity/lookup model as media serving.
- Expose API endpoints for reading all comments for a media item and creating a new comment.
- Feed responses should include enough comment summary data for the frontend to render the latest 1-2 comments without fetching every full comment thread up front.
- Store comment timestamps and comment text in a simple append-friendly format. Keep parsing deterministic and robust against newlines or delimiter characters in user text.
- Validate and trim submitted comment text. Reject empty comments.
- The comments panel should handle empty, loading, error, and submit-in-progress states gracefully.
