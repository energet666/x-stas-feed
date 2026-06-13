# Project Instructions for Agents

Start with [`docs/index.md`](docs/index.md), then read only the reference notes
and ADRs relevant to the task.

## Mandatory Constraints

- Production is one Go `net/http` server serving the API, media, realtime
  endpoints, and built Svelte SPA.
- Keep backend ownership under `cmd/server`, `internal/media`,
  `internal/server`, and `internal/game`.
- Use filesystem-backed persistence. Do not add a database, authentication, or
  server-side personalization without an explicit architecture change.
- Never trust client-supplied filesystem paths. Resolve media, comments, boards,
  and assets through server-controlled identity and lookup.
- Use SSE for normal server-to-client updates. Asteroids is the only WebSocket
  exception.
- Use Svelte 5 runes, Vite, Tailwind CSS 4, existing `ui-*` design primitives,
  and `lucide-svelte`. Do not add daisyUI.
- Keep user-facing strings in `web/src/lib/ui_text.ts` unless localization scope
  is deliberately changed.
- In the sandbox, run Go commands with
  `GOCACHE=/tmp/feed-ai-go-cache`.
- Start a local server only for short verification and stop it immediately
  afterward unless the user asks to keep it running.
- Preserve and commit the complete current
  `test-content/.boards/sticker-pack/` when related project changes add or
  remove pack files.

## Documentation Contract

- Reference notes under `docs/` describe current behavior only.
- Update the relevant reference note in the same change that alters behavior.
- Create an ADR only for a consequential decision with meaningful alternatives.
- Keep routine history and verification logs in Git and task results, not in the
  knowledge base.
- Run `GOCACHE=/tmp/feed-ai-go-cache make check` before completion.
