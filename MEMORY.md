# MEMORY

This file is for durable project decisions, constraints, and known risks. It is not a changelog; routine implementation steps, small UI tweaks, and verification logs belong in git history or final task notes.

## Product Direction

- Build a modern one-column Instagram-like infinite media feed for local photos and videos.
- `test-content` is the source of test media for v1.
- v1 intentionally does not include uploads, auth, likes, personalization, or a database.
- Comments are required for v1 and must remain filesystem-backed.

## Backend Decisions

- Production uses one Go server. It serves the API, built Svelte SPA, and media files.
- Use the standard Go `net/http` server.
- Keep common Go layout:
  - `cmd/server` for the executable entrypoint.
  - `internal/media` for scanning, sorting, pagination, lookup, and comments storage.
  - `internal/server` for routing, handlers, middleware, SSE, and static serving.
- Media files are scanned from `test-content`, sorted by modification time descending, with filename as the stable tie-breaker.
- Media IDs are server-controlled safe identities. Clients must not provide filesystem paths for media or comments.
- Media serving and comment APIs must validate IDs through the same safe lookup model.
- Feed pagination is cursor-based through `GET /api/feed?cursor=&limit=`.
- The media scanner must ignore internal comment storage such as `test-content/.comments`.

## Comments Decisions

- Comments are stored in server-managed append-friendly text files deterministically associated with media IDs.
- Current storage format is JSON Lines under `test-content/.comments/{mediaID}.jsonl`.
- Creating a comment creates the comment file if needed and appends the new comment.
- Comment text is trimmed and validated; empty comments are rejected.
- Comment parsing must be deterministic and robust against newlines or delimiter characters in user text.
- Feed responses include comment summary data so cards can render the latest 1-2 comments without fetching every full thread.
- Full comment threads are loaded through media-specific comment endpoints.
- Live comment updates use one global SSE stream, not WebSocket.

## Frontend Decisions

- Use Svelte 5 with runes, Vite, Tailwind CSS 4.x, and lucide-svelte icons.
- daisyUI was removed; keep styling in Tailwind classes, scoped component CSS, and the small global theme/component primitives in `web/src/app.css`.
- `App.svelte` is the feed coordinator: pagination, virtualization, measurement, scroll anchoring, overlay state, expanded media state, comments panel state, and SSE subscription.
- Keep presentational pieces split into focused components instead of growing `App.svelte`.
- Feed virtualization keeps only the viewport window plus overscan mounted. Dynamic card height changes above the viewport compensate scroll position manually.
- The app must handle an empty `test-content` directory gracefully.

## Feed Card Layout Decisions

- `FeedCardFrame` owns card geometry and overlay layering.
- Card layers are:
  - media content,
  - optional `contentOverlay`,
  - top overlay stack,
  - bottom overlay stack.
- The top overlay stack contains the persistent media information panel and an optional future top accessory snippet.
- The bottom overlay stack contains an optional bottom accessory snippet and the compact comments preview.
- Top and bottom overlay stacks slide fully from outside the card bounds and do not animate opacity, because opacity animation interacts poorly with `backdrop-filter`.
- Video controls are bottom accessory content. Their visibility and movement are owned by `FeedCardFrame`, not by the controls component.
- Video transient feedback such as play, blocked-play message, speed indicator, and seek feedback is rendered through `contentOverlay`. Video playback state remains owned by `FeedVideoPlayer`.

## Video Player Decisions

- Use a custom feed video player rather than native browser controls.
- The active video player owns keyboard shortcuts; shortcuts should not affect every mounted video.
- Keyboard behavior:
  - Space toggles play/pause; holding Space temporarily plays at 2x.
  - ArrowLeft/ArrowRight seek; holding ArrowRight fast-forwards, holding ArrowLeft rewinds repeatedly.
  - ArrowUp/ArrowDown change playback speed.
- Horizontal wheel/trackpad gestures seek the active video while preserving normal vertical page scrolling.
- Only one mounted video should play at a time; players coordinate through a shared browser event.
- Per-video watch progress, shared volume/mute state, and debug overlay collapsed state are persisted in `localStorage`.
- First-run default video volume is 50% when no saved browser volume exists.
- Safari-specific behavior matters:
  - handle `video.play()` and PiP failures without unhandled promise errors;
  - support Safari PiP fallback where possible;
  - hide the volume slider if programmatic volume control is unsupported;
  - use a small preview-frame nudge after metadata loads so paused videos can show a first frame.

## UI Behavior Decisions

- Expanded media reuses the same image/video DOM node and fixes the media frame to the browser viewport. Do not mount a duplicate media element for fullscreen.
- Expanded media locks background scroll and closes with Escape or the close button.
- Comments open as an in-card overlay over the selected feed card, not as a global fixed side/bottom panel.
- Opening comments should not lock body scrolling because the comments UI is scoped to the card surface.
- Full comments and compact comment previews preserve user line breaks with safe wrapping.
- The comment composer submits with Enter; Shift+Enter inserts a newline; IME composition must not submit prematurely.

## Agent Workflow Constraints

- Agents may start the local server only for short verification checks and must stop it immediately afterward, unless the user explicitly asks to start or keep it running.
- Record only important new decisions, constraints, verification-relevant outcomes, and known issues here. Do not append routine changelog entries.

## Known Environment Notes

- The local Go toolchain previously reported `log/slog` missing from stdlib, so the server used standard `log`.
- Go commands may need sandbox escalation when the Go build cache is outside the workspace.
- Dependency installation may need sandbox escalation for registry access.
