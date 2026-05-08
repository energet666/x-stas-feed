# MEMORY

This file is for durable project decisions, constraints, and known risks. It is not a changelog; routine implementation steps, small UI tweaks, and verification logs belong in git history or final task notes.

## Product Direction

- Build a modern one-column Instagram-like infinite media feed for local photos and videos.
- `test-content` is the source of test media for v1.
- v1 intentionally does not include auth, likes, personalization, or a database.
- Users can upload photos and videos through the app; uploads are stored as regular filesystem media in the configured content root.
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
- Media responses set `Cache-Control: public, max-age=3600`; a `Cache-Control: no-cache` header on media requests is client/browser controlled and can appear during reloads or when DevTools disables cache.
- Feed pagination is cursor-based through `GET /api/feed?cursor=&limit=`.
- The media scanner must ignore internal comment storage such as `test-content/.comments`.
- Media upload uses `POST /api/uploads` with multipart `files` parts. The server enforces a 1GB request cap, accepts only the same supported photo/video extensions used by scanning, rejects empty/path-like/unsupported filenames, and writes safe unique filenames directly under the content root.
- Successful uploads invalidate the media scan cache immediately so the next feed request can show newly uploaded files without waiting for the scan TTL.
- Media metadata is filesystem-backed under `test-content/.metadata/{mediaID}.json`. It currently stores `displayName`, preserving user-facing names with spaces/Cyrillic while physical uploaded filenames remain safe unique server-generated names. Existing files fall back to their real filename when no metadata exists.

## Comments Decisions

- Comments are stored in server-managed append-friendly text files deterministically associated with media IDs.
- Current storage format is JSON Lines under `test-content/.comments/{mediaID}.jsonl`.
- Creating a comment creates the comment file if needed and appends the new comment.
- Comment text is trimmed and validated; empty comments are rejected.
- Comments now include an `author` field. The browser stores the user's chosen nickname in `localStorage`, sends it when creating comments, and the server normalizes untrusted/missing author input to `Guest`.
- Comment parsing must be deterministic and robust against newlines or delimiter characters in user text.
- Feed responses include comment summary data so cards can render the latest 1-2 comments without fetching every full thread.
- Full comment threads are loaded through media-specific comment endpoints.
- Live comment updates use one global SSE stream, not WebSocket.

## Frontend Decisions

- Use Svelte 5 with runes, Vite, Tailwind CSS 4.x, and lucide-svelte icons.
- daisyUI was removed; keep styling in Tailwind classes, scoped component CSS, and the small global theme/component primitives in `web/src/app.css`.
- `App.svelte` is the feed coordinator: pagination, virtualization, measurement, scroll anchoring, overlay state, expanded media state, comments panel state, and SSE subscription.
- Keep presentational pieces split into focused components instead of growing `App.svelte`.
- Feed virtualization keeps only the viewport window plus two overscan cards before and after it mounted. Dynamic card height changes above the viewport compensate scroll position manually.
- Feed pagination uses small pages of 6 items so approaching the end of the loaded set does not append a large batch of new posts at once.
- Viewport updates for virtualization are scheduled through `requestAnimationFrame` so rapid scroll/resize events coalesce into one Svelte state update per frame.
- The app must handle an empty `test-content` directory gracefully.
- Upload UI is a compact header drop-in plus page-level drag-and-drop. After a successful upload, the frontend resets feed pagination and reloads from the first page so newest uploaded media appears at the top.
- Card titles and media accessibility labels use `displayName`, not the technical storage filename.

## Feed Card Layout Decisions

- `FeedCardFrame` owns card geometry and overlay layering.
- Feed card shells keep their page-background glass effect through a `::before` backing layer. Do not put `backdrop-filter` directly on `.glass-card`, because nested overlay panels need to blur their own backdrop rather than inheriting a flattened card backdrop.
- Added a subtle 45-degree diagonal grid pattern to the glass card background backing layer. Large gradient highlights were removed, but symmetrical 1px accents were added to the top and bottom edges for a balanced, premium look.
- Card visual effects are being restored in stages for scroll performance testing. Ambient media layer and ambient blur are enabled; card backing pattern, glass transparency, shell backdrop blur, media drop shadow, and shell inset shadows remain disabled. Global background particles are enabled again and may react to scroll only through a clamped, eased draw offset; direct scroll-velocity mutation caused visible jitter in Safari.
- Card layers are:
  - media content,
  - optional `contentOverlay`,
  - top overlay stack,
  - bottom overlay stack.
- The top overlay stack contains the persistent media information panel and an optional future top accessory snippet.
- The bottom overlay stack contains an optional bottom accessory snippet and the compact comments preview.
- Compact comments preview is persistent and remains visible at the bottom of each card. Bottom accessory content such as video controls expands above it and does not control comment preview visibility.
- Top and bottom overlay stacks slide fully from outside the card bounds and do not animate opacity, because opacity animation interacts poorly with `backdrop-filter`. External shadows were removed from all UI components for a cleaner look in the dark theme, and a safe offset of 1.5rem is used when hidden to ensure no subpixel artifacts remain visible at the card edges.
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
- Ambient backgrounds should render as soon as media data is available, not after user interaction, lazy loading, or idle scheduling; video uses `preload="auto"` and draws the ambient canvas immediately on `loadeddata`.
- Video ambient canvas updates dynamically during playback, but draws only every fifth animation frame. Safari can stutter when `drawImage(video)` updates a blurred canvas on every frame, so keep this throttled unless profiling shows enough headroom.
- Idle feed videos use metadata preload for duration/seek controls and switch to `auto` only while playing or expanded. Avoid client-side preview seeks during scroll, especially in Safari.
- Video preview frames are served as cached JPEG posters through `GET /api/media/{id}/poster?time=`. The frontend chooses the poster time from saved per-video watch progress, falling back to the first frame when the user has not watched that video.
- Idle video ambient backgrounds use the same poster JPEG as the video element instead of seeking/loading the video.
- After any user interaction with a mounted video card, remove the poster cover, remove the video `poster` attribute, and stop using poster ambient for that card lifetime. From that point, the real video element/canvas owns visuals.
- Video ambient canvas dimensions track the source video aspect ratio; keeping a square canvas makes `object-fit: cover` crop live ambient differently from poster ambient.
- In Safari, mounted idle videos lazily load metadata immediately so duration and seek controls are available, but metadata load must not seek the video for a preview frame; posters remain responsible for idle visuals.
- Video controls display saved watch progress immediately from `localStorage` so the seekbar matches the poster before playback starts; this display state must not seek the video until the user plays or manually seeks.
- Idle `timeupdate` events from metadata/preload must not overwrite the display-only saved progress before user interaction; Chrome can emit these with `video.currentTime` still at 0.
- Manual seeking while paused must not request a fresh poster. Once the user interacts with a video, seek the real video element; poster generation is only for scroll/idle optimization.
- First manual seek from a saved-progress poster must use the displayed saved progress as its base time, not `video.currentTime`, because the real video element may still be at 0 until the first interaction.
- A video becomes the active keyboard target only after explicit interaction with that player or its controls. Hover, pointer movement, and focus used only to reveal controls must not activate video keyboard handling. Pointer-down on empty page background dispatches a clear-active-video event.
- Save and reuse per-video watch progress only for videos at least 120 seconds long. Short videos always use first-frame posters so partially watched shorts do not expand the server poster cache on later visits.
- Ambient media activation is observer-driven: mounted overscan cards prepare their ambient background and idle video preview seek only when they approach the viewport, rather than immediately at mount.
- The debug overlay includes a persisted card background mode switch: `simple` disables ambient card backgrounds, `ambient` uses the observer-driven ambient preparation path. This switch must not disable normal video preview/progress seeking.
- Safari and Chrome both use the blurred ambient media card background; video ambient canvas is prepared through the same observer-driven ambient path as the rest of the card background.
- Safari-specific behavior matters:
  - handle `video.play()` and PiP failures without unhandled promise errors;
  - support Safari PiP fallback where possible;
  - hide the volume slider if programmatic volume control is unsupported;
  - use a small preview-frame nudge after metadata loads so paused videos can show a first frame.

## UI Behavior Decisions

- Expanded media reuses the same image/video DOM node and fixes the media frame to the browser viewport. Do not mount a duplicate media element for fullscreen.
- The page background includes an `AsteroidsShip` fixed overlay. It starts parked horizontally in the top-left header area on a lower layer so the sticky header overlays it. ArrowLeft/ArrowRight rotate the ship, ArrowUp applies thrust, ArrowDown applies reverse thrust, Space fires one short-lived glowing bullet only when no bullet is already active, and the ship/bullet wraps at viewport edges. First ship control spawns a drifting asteroid; bullets can destroy it, then another respawns after a short delay. Ship-asteroid collisions clear bullets, reset the ship to its parked header position, show the same explosion, and respawn the asteroid. Ship keyboard handling must ignore text entry targets and must yield all ship keys while a video is the active keyboard target, so video Space/arrow shortcuts keep working until page-background click clears video activity.
- Asteroids game sounds are synthesized with Web Audio in `AsteroidsShip` instead of shipped as assets: square-wave laser, sawtooth thrust ticks, asteroid explosions shaped like a softer "dy-dysh" with two low filtered triangle thumps plus a gentle lowpass noise tail, and separate ship-destruction sounds using falling triangle tone and short noise burst. Audio is primed only after keyboard interaction to comply with browser autoplay policy.
- Local ship thrust renders a short-lived local-only smoke trail behind the engine. Smoke particles are visual-only and are not sent over the multiplayer WebSocket.
- Asteroid destruction spawns a local-only visual burst with sparks, rotating debris, and smoke particles. These particles are not synchronized over WebSocket; only the authoritative asteroid-destroyed event position is shared.
- Once the local user first controls the ship, `AsteroidsShip` dispatches `feed-ai:game-started`; `App.svelte` hides the feed, header, sidebar, comments overlays, expanded media, and debug overlay so the background becomes the game surface.
- Comments and other main site async updates stay on SSE. Multiplayer ships use WebSocket at `GET /api/ships/socket`; clients send local ship, active bullet, and active asteroid/comet state every 16ms after first ship control, and the server keeps recent ship state in memory with an 8-second TTL and broadcasts snapshots on the same socket. Remote ships render with the comment nickname, and remote bullets/comets render in the same background layer. Local clients handle ship-ship, remote bullet, and remote comet collisions by clearing bullets, showing an explosion, publishing the reset state, and parking the local ship until the next movement. Shooting a remote comet sends an `asteroid-hit` message; the server validates the reported bullet point against the authoritative latest comet snapshot, removes that comet from broadcasts, suppresses stale owner updates for the destroyed comet id, and broadcasts an `asteroid-destroyed` event so all clients show the explosion and the owner schedules respawn.
- Expanded media locks background scroll and closes with Escape or the close button.
- Comments open as an in-card overlay over the selected feed card, not as a global fixed side/bottom panel.
- Opening comments should not lock body scrolling because the comments UI is scoped to the card surface.
- Full comments and compact comment previews preserve user line breaks with safe wrapping.
- Media likes are anonymous one-way increments. Store the server-side `likeCount` in each media item's metadata JSON; do not add client IDs, unlike state, auth, or a separate likes store for v1. Successful likes publish `event: like` over the existing comments SSE stream so other open clients update immediately.
- The comment composer submits with Enter; Shift+Enter inserts a newline; IME composition must not submit prematurely.
- Opening comments focuses the composer textarea from the same user-triggered `openComments` path using Svelte `flushSync`, a stable `comment-composer-{mediaID}` element id, and short retries after the click completes; this is needed for Safari, which may ignore delayed textarea focus and leave focus on the nickname input.
- The comment composer textarea intentionally has no placeholder, avoiding visible placeholder flicker during Safari autofocus.
- A left profile sidebar owns the local comment nickname control and a dice action that generates funny Russian nickname candidates with a random numeric suffix.

## Agent Workflow Constraints

- Agents may start the local server only for short verification checks and must stop it immediately afterward, unless the user explicitly asks to start or keep it running.
- Use `make package-win` to rebuild the root Windows package: frontend into `build/feed-ai-win64/web/dist`, Go `server.exe` for Windows amd64, and `build/feed-ai-win64.zip`.
- Record only important new decisions, constraints, verification-relevant outcomes, and known issues here. Do not append routine changelog entries.

## Known Environment Notes

- The local Go toolchain previously reported `log/slog` missing from stdlib, so the server used standard `log`.
- Go commands may need sandbox escalation when the Go build cache is outside the workspace.
- Dependency installation may need sandbox escalation for registry access.
- Upload implementation verified with `go test ./...`, `npm --prefix web run check`, `npm --prefix web run build`, and a short Go server smoke test for `/` plus `/api/feed` on 2026-05-08.
