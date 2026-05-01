# MEMORY

## 2026-05-01

- Implemented the initial media feed web app from scratch.
- Backend:
  - Go module `feed-ai`.
  - `cmd/server` starts a standard `net/http` server on `:8080` by default.
  - `internal/media` scans `test-content`, filters supported image/video files, sorts by modification time descending, uses filename as tie-breaker, and exposes cursor pagination.
  - `internal/server` serves `GET /api/feed?cursor=&limit=`, safe media URLs under `/media/{id}`, and the built SPA from `web/dist`.
  - Media IDs are URL-safe base64 encodings of relative paths; file serving validates IDs stay inside `test-content`.
- Frontend:
  - Svelte 5 + Vite SPA in `web`.
  - Tailwind CSS 4, daisyUI 5, and lucide-svelte configured.
  - One-column Instagram-like infinite feed with image/video cards, lazy image loading, `video controls preload="metadata"`, loading, empty, error, and end-of-feed states.
- Test media currently exists in `test-content`: several `.mp4` files and one `.png`.
- Verification completed:
  - `go test ./...`
  - `npm run check`
  - `npm run build`
  - Smoke-tested running Go server with `/api/feed?limit=1` and `/`.
- Environment note:
  - `go test` and `go run` needed sandbox escalation because the Go build cache is outside the workspace.
  - `npm install` needed sandbox escalation for registry access.
- Known implementation choice:
  - Used standard `log` instead of `log/slog` because the local Go toolchain reported `log/slog` missing from stdlib.
- Added project process rule:
  - Important intermediate results, decisions, verification outcomes, and known issues must be recorded in `MEMORY.md`.
- Initialized git repository in `/Users/sh/projects/feed-ai`.
- Added `.gitignore` for `.DS_Store`, Go transient outputs, frontend `node_modules`, frontend build output, local `test-content` media, and logs.
- Configured Vite dev server for debugging:
  - Vite runs on fixed port `5173`.
  - `/api` and `/media` proxy to `VITE_API_TARGET`, defaulting to `http://localhost:8080`.
  - Added `web/.env.example` documenting the backend target.
