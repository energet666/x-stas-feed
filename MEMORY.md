# MEMORY

This file is for durable project decisions, constraints, and known risks. It is not a changelog; routine implementation steps, small UI tweaks, and verification logs belong in git history or final task notes.

## Product Direction

- Build a modern one-column Instagram-like infinite media feed for local photos, videos, audio, and generic files.
- `test-content` is the source of test media for v1.
- v1 intentionally does not include auth, likes, personalization, or a database.
- Users can upload media through the app; uploads are stored as regular filesystem media in the configured content root.
- Comments are required for v1 and must remain filesystem-backed.

## Backend Decisions

- Production uses one Go server. It serves the API, built Svelte SPA, and media files.
- Use the standard Go `net/http` server.
- Keep common Go layout:
  - `cmd/server` for the executable entrypoint.
  - `internal/media` for scanning, sorting, pagination, lookup, and comments storage.
  - `internal/server` for routing, handlers, middleware, SSE, and static serving.
- Media files are scanned from `test-content`, sorted by modification time descending, with filename as the stable tie-breaker.
- Media IDs are server-controlled opaque SHA-256 hex identities derived from the normalized relative media path. They are fixed-length and not decodable by clients. Clients must not provide filesystem paths for media or comments.
- Media serving and comment APIs must validate IDs through the same safe lookup model.
- Media responses set `Cache-Control: public, max-age=3600`; a `Cache-Control: no-cache` header on media requests is client/browser controlled and can appear during reloads or when DevTools disables cache.
- Main feed indexing is append-only and oldest-to-newest internally: index `0` is the oldest item, the highest index is the newest item, and `GET /api/feed?index=-1` returns the newest item plus `firstIndex`/`lastIndex` bounds. Other non-negative indexes fetch exactly one feed item. The frontend renders the main feed newest-first by requesting decreasing indexes as the user scrolls down.
- Favorites feed pagination uses `POST /api/feed/favorites` with browser-owned ordered media IDs. The server does safe ID lookup against scanned media, ignores stale/missing IDs, preserves the request order, and applies cursor/limit over that ordered ID list.
- The media scanner must ignore internal comment storage such as `test-content/.comments`.
- Media and comment serving uses a long-lived in-memory runtime index initialized once from disk. After startup, out-of-band filesystem media additions/removals are intentionally unsupported until restart; server-managed uploads, comments, comment likes, and media likes update the runtime index directly while preserving filesystem-backed durability.
- Production server logs media scan and runtime index initialization counts/durations, plus server-managed upload/comment/like persistence events, so heavy filesystem operations are visible in stdout.
- Media upload uses `POST /api/uploads` with multipart `files` parts. The server enforces a 1GB request cap, rejects empty/path-like filenames, and writes safe unique filenames directly under the content root.
- Successful uploads are inserted into the runtime media index immediately so the next feed request can show newly uploaded files without a directory rescan.
- Media metadata is filesystem-backed under `test-content/.metadata/{mediaID}.json`, where `mediaID` is the fixed 64-character SHA-256 hex ID. It stores `displayName`, `modifiedAt`, `likeCount`, and cached audio extraction data when available. Existing files fall back to their real filename when no metadata exists.
- Scanning creates a missing metadata JSON for every indexed media/file item using the same baseline fields as upload (`displayName` and `likeCount`), then adds audio extraction fields for audio files when probing succeeds. Existing metadata files are preserved.
- Audio files are first-class feed media with `type: "audio"` for common extensions such as MP3, M4A, AAC, FLAC, WAV, OGA, and Opus. `.ogg` keeps the historical video fallback when probing is unavailable, but `ffprobe` can classify it as audio or video when available.
- Audio tag and duration extraction is best-effort via `ffprobe`; failures must not fail scanning or upload. Extracted audio metadata is cached in the media metadata JSON with a source file signature (`sourceSize` plus `sourceModTimeUnixNano`) and is reused only while that signature matches the current media file.
- Video duration extraction is best-effort via `ffprobe`; failures must not fail scanning or upload. Extracted video duration is cached in the media metadata JSON with the same source file signature and is returned as `durationSeconds` so Safari feed cards do not need to preload video metadata just to render seek controls.
- Embedded audio cover art is extracted best-effort with FFmpeg during upload/scan into `test-content/.covers`, keyed by safe media ID plus file size/mtime. Metadata stores only the generated cover filename, not an absolute path or base64. Feed items expose `coverUrl` only when a cover file was actually saved; otherwise the frontend should render its fallback art. Missing covers, non-audio IDs, and invalid IDs return 404 for the cover endpoint.
- The scanner ignores server-managed internal directories including `.comments`, `.metadata`, `.posters`, and `.covers`.

## Comments Decisions

- Comments are stored in server-managed append-friendly text files deterministically associated with media IDs.
- Current storage format is JSON Lines under `test-content/.comments/{sha256(mediaID)}.jsonl`; legacy `{mediaID}.jsonl` files are intentionally unsupported and can be deleted during development.
- Creating a comment creates the comment file if needed and appends the new comment.
- Comment text is trimmed and validated; empty comments are rejected.
- Comments now include an `author` field. The browser stores the user's chosen nickname in `localStorage`, sends it when creating comments, and the server normalizes untrusted/missing author input to `Guest`.
- Comment parsing must be deterministic and robust against newlines or delimiter characters in user text.
- Feed responses include comment summary data so cards can render the latest 1-2 comments without fetching every full thread.
- Full comment threads are loaded through media-specific comment endpoints.
- Live comment updates use one global SSE stream, not WebSocket.
- Social activity is comment-only for v1. `GET /api/activity?limit=` returns the latest comments across valid scanned media, newest first, and ignores stale comment files for missing media.

## Frontend Decisions

- Use Svelte 5 with runes, Vite, Tailwind CSS 4.x, and lucide-svelte icons.
- The site favicon is a hand-authored SVG served from `web/public/favicon.svg`; it uses a crisp black/cyan pixel-noise pattern.
- daisyUI was removed; keep styling in Tailwind classes, scoped component CSS, and the small global theme/component primitives in `web/src/app.css`.
- `App.svelte` is the feed coordinator: pagination, virtualization, measurement, scroll anchoring, overlay state, expanded media state, comments panel state, and SSE subscription.
- Keep presentational pieces split into focused components instead of growing `App.svelte`.
- Feed virtualization keeps every fetched card in the frontend `items` array as the loaded feed model, computes `rows` as geometry for all loaded items, and mounts only `visibleRows` plus overscan into the DOM. Top and bottom spacer elements are derived from the full `rows` map, not accumulated state. This trades some JS memory for simpler and resize-tolerant scroll math: unloaded DOM cards still keep identity, order, and last known or estimated height in the model. A bottom sentinel loads older decreasing indexes when the loaded geometry does not extend far enough below the viewport.
- Feed pagination uses small pages of 6 items so approaching the end of the loaded set does not append a large batch of new posts at once.
- Viewport updates for virtualization are scheduled through `requestAnimationFrame` so rapid scroll/resize events coalesce into one Svelte state update per frame.
- The app must handle an empty `test-content` directory gracefully.
- Upload UI is a compact header drop-in plus page-level drag-and-drop. After a successful upload, the frontend resets feed pagination and reloads from the first page so newest uploaded media appears at the top.
- Card titles and media accessibility labels use `displayName`, not the technical storage filename.
- Favorites are stored only in browser `localStorage` under `feed-ai:favorites`. New favorites are inserted at the front of the ID array, so the favorites view shows most recently saved media first without server-side sorting.
- The page header owns the all/favorites mode switch. In favorites mode, `App.svelte` requests only the saved favorite IDs through the favorites endpoint and removes an unfavorited visible card immediately while keeping the favorites cursor aligned after ID removal.
- The former full-width page header is now a compact left-side feed controls panel placed above the user profile panel inside one shared left rail; it still owns upload and all/favorites controls and stacks above the feed on smaller screens.

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
- Card overlay `onKeep` must clear the autohide timer instead of rescheduling it. Cursor/focus inside any `.feed-card-panel` stops event propagation and blocks autohide until the cursor leaves that panel, then normal delayed hiding resumes. Root media frame clicks from inside `.feed-card-panel` must call `onKeep` instead of `onReveal`, because panel clicks scheduling a stale hide timer after range interaction cause a brief one-off panel hide.
- Shared audio/video range controls must call the controls enter/keep path on pointer/mouse down and up. Finishing a seek drag while the cursor is still over the controls panel must not schedule a one-off panel hide.
- Shared audio/video seek clicks and pointer drags compute time from pointer coordinates across the visible `.media-playback-progress` bounds instead of trusting the native range value. Native range geometry can offset clicks by thumb width, especially in Safari.
- Video controls are bottom accessory content. Their visibility and movement are owned by `FeedCardFrame`, not by the controls component.
- Audio controls are also bottom accessory content and should match the video controls panel style and overlay motion.
- Horizontal wheel/trackpad seeking must reveal the parent feed card overlay for both audio and video so the seek progress bar is visible during scrubbing. Video's local controls visibility alone is insufficient because `FeedCardFrame` owns bottom accessory visibility.
- Video transient feedback such as play, blocked-play message, speed indicator, and seek feedback is rendered through `contentOverlay`. Video playback state remains owned by `FeedVideoPlayer`.

## Audio Player Decisions

- Use a custom feed audio card rather than native browser controls.
- Audio cards render embedded cover art when `coverUrl` is available; otherwise they show a styled fallback album surface using extracted title/artist/album metadata or the display name.
- Audio playback uses a hidden native `<audio>` element with custom play/pause, seek, time, mute, and volume controls.
- Audio shares the same persisted volume/mute storage and media progress key pattern as video. Only media at least 120 seconds long persists progress.
- Audio and video players coordinate through the shared playback event so starting one mounted player pauses the others.
- Audio cards are focusable keyboard targets: Space toggles playback, and ArrowLeft/ArrowRight seek while preventing page/game shortcut handling from also consuming those keys.

## Video Player Decisions

- Use a custom feed video player rather than native browser controls.
- The active video player owns keyboard shortcuts; shortcuts should not affect every mounted video.
- Keyboard behavior:
  - Space toggles play/pause; holding Space temporarily plays at 2x.
  - ArrowLeft/ArrowRight seek by 1 second on keydown, matching audio cards; holding the key repeats through normal keyboard repeat.
  - ArrowUp/ArrowDown change playback speed.
- Horizontal wheel/trackpad gestures seek the active video while preserving normal vertical page scrolling.
- Only one mounted video should play at a time; players coordinate through a shared browser event.
- Per-video watch progress, shared volume/mute state, and debug overlay collapsed state are persisted in `localStorage`.
- First-run default video volume is 50% when no saved browser volume exists.
- Ambient backgrounds should render as soon as media data is available, not after user interaction, lazy loading, or idle scheduling; video uses `preload="auto"` and draws the ambient canvas immediately on `loadeddata`.
- Video ambient canvas updates dynamically during playback, but draws only every fifth animation frame. Safari can stutter when `drawImage(video)` updates a blurred canvas on every frame, so keep this throttled unless profiling shows enough headroom.
- Idle feed videos use backend-provided `durationSeconds` for duration/seek controls when available and switch to `auto` only while playing or expanded. Safari cards with cached duration keep `preload="none"` until interaction. Avoid client-side preview seeks during scroll, especially in Safari.
- Video preview frames are served as cached JPEG posters through `GET /api/media/{id}/poster?time=`. The frontend chooses the poster time from saved per-video watch progress, falling back to the first frame when the user has not watched that video.
- Poster generation resolves FFmpeg from `tools/ffmpeg/{GOOS}-{GOARCH}/ffmpeg[.exe]` relative to the current working directory or server executable directory before falling back to system `PATH`. Probe metadata resolves FFprobe from the same layout as `ffprobe[.exe]`. Bundled binaries are local ignored files; release packaging should include only the target platform's binaries and retain FFmpeg license/provenance notes with the artifact.
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
- The page background includes an `AsteroidsShip` fixed overlay. The local ship starts parked in the top-left header area but is not rendered until the first ship control, so removing or changing page chrome cannot reveal it early. ArrowLeft/ArrowRight rotate the ship, ArrowUp applies thrust, ArrowDown applies reverse thrust, Space fires one short-lived glowing bullet only when no bullet is already active, and the ship/bullet wraps at viewport edges. First ship control spawns a drifting asteroid; bullets can destroy it, then another respawns after a short delay. Ship-asteroid collisions clear bullets, reset the ship to its parked header position, hide it until the next movement, show the same explosion, and respawn the asteroid. Ship keyboard handling must ignore text entry targets and must yield all ship keys while a video is the active keyboard target, so video Space/arrow shortcuts keep working until page-background click clears video activity.
- Asteroids game sounds are synthesized with Web Audio in `AsteroidsShip` instead of shipped as assets: square-wave laser, sawtooth thrust ticks, asteroid explosions shaped like a softer "dy-dysh" with two low filtered triangle thumps plus a gentle lowpass noise tail, and separate ship-destruction sounds using falling triangle tone and short noise burst. Audio is primed only after keyboard interaction to comply with browser autoplay policy.
- Local ship thrust renders a short-lived local-only smoke trail behind the engine. Smoke particles are visual-only and are not sent over the multiplayer WebSocket.
- Asteroid destruction spawns a local-only visual burst with sparks, rotating debris, and smoke particles. These particles are not synchronized over WebSocket; only the authoritative asteroid-destroyed event position is shared.
- Once the local user first controls the ship, `AsteroidsShip` dispatches `feed-ai:game-started`; `App.svelte` hides the feed, header, sidebar, comments overlays, expanded media, and debug overlay so the background becomes the game surface.
- Comments and other main site async updates stay on SSE. Multiplayer ships use WebSocket at `GET /api/ships/socket`; clients send local ship, active bullet, and active asteroid/comet state every 16ms after first ship control, and the server keeps recent ship state in memory with an 8-second TTL and broadcasts snapshots on the same socket. Remote ships render with the comment nickname, and remote bullets/comets render in the same background layer. Local clients handle ship-ship, remote bullet, and remote comet collisions by clearing bullets, showing an explosion, publishing the reset state, and parking the local ship until the next movement. Shooting a remote comet sends an `asteroid-hit` message; the server validates the reported bullet point against the authoritative latest comet snapshot, removes that comet from broadcasts, suppresses stale owner updates for the destroyed comet id, and broadcasts an `asteroid-destroyed` event so all clients show the explosion and the owner schedules respawn.
- Expanded media locks background scroll and closes with Escape or the close button.
- Each media card top info panel includes a star action to add/remove that media item from browser favorites; the star appears to the left of the fullscreen action.
- Comments open as an in-card overlay over the selected feed card, not as a global fixed side/bottom panel.
- Opening comments should not lock body scrolling because the comments UI is scoped to the card surface.
- When a card is expanded, the media frame is fixed above the page at `z-index: 79`; the comments panel must be rendered through the top-level `.comments-panel-fullscreen` viewport overlay above it, not inside the virtualized card.
- Full comments and compact comment previews preserve user line breaks with safe wrapping.
- Media likes are anonymous one-way increments. Store the server-side `likeCount` in each media item's metadata JSON; do not add client IDs, unlike state, auth, or a separate likes store for v1. Successful likes publish `event: like` over the existing comments SSE stream so other open clients update immediately.
- Comment likes follow the same anonymous one-way increment model. Store each comment's `likeCount` in its JSONL record, rewrite the media comment file atomically when a comment is liked, and publish `event: comment-like` over the existing SSE stream.
- Feed card and comment like controls keep the heart action visible at zero likes, but hide the numeric counter until the count is greater than zero.
- The comment composer submits with Enter; Shift+Enter inserts a newline; IME composition must not submit prematurely.
- Opening comments focuses the composer textarea from the same user-triggered `openComments` path using Svelte `flushSync`, a stable `comment-composer-{mediaID}` element id, and short retries after the click completes; this is needed for Safari, which may ignore delayed textarea focus and leave focus on the nickname input.
- The comment composer textarea intentionally has no placeholder, avoiding visible placeholder flicker during Safari autofocus.
- The comment composer uses `EmojiPanel.svelte`, which wraps lazy-loaded `emoji-mart` with bundled `@emoji-mart/data` and patches the picker's shadow DOM layout so category navigation stays fixed at the bottom. `CommentThread.svelte` owns textarea cursor insertion.
- `EmojiPanel.svelte` lazy-loads `@emoji-mart/data/i18n/ru.json` and passes `locale: 'ru'` plus `i18n` to `emoji-mart`; this localizes picker UI text but does not add Russian searchable emoji keywords by itself.
- Russian emoji search is implemented by lazy-loading `web/src/lib/emoji_ru_keywords.ts`, a generated compact map from Unicode CLDR Russian annotations for all current `emoji-mart` ids plus a small set of conversational aliases, then appending those terms to emoji `keywords` at picker mount time.
- The comment composer layout keeps the emoji trigger in a separate right-side actions column rather than absolutely overlaying it on the resizable textarea.
- A left profile sidebar owns the local comment nickname control and a dice action that generates funny Russian nickname candidates with a random numeric suffix.
- The nickname input must not stay focused on initial page open. `UserSidebar` watches the first 1.5 seconds after mount and blurs only non-user-initiated restored focus on that input, while preserving normal click and keyboard focus afterward.
- On screens below the desktop side-rail breakpoint, the stacked left rail must cap at `40rem`, matching the rendered feed card column inside the feed section's horizontal padding; do not widen it to the section's outer `42rem` max.
- A right social activity overlay shows latest comments across all media. On smaller screens it becomes a right drawer opened by a floating activity button. Clicking an activity comment fetches the corresponding safe media item through `GET /api/media/{id}` and opens a modal media card with the full comment thread and composer.
- The social activity panel intentionally has no manual refresh button in its header; activity should feel live through the existing SSE updates.
- Left and right rail panels plus the feed debug overlay use the `side-glass-panel` backing-layer class for Chrome compatibility: keep the actual `backdrop-filter` on the panel `::before` layer, with panel content above it, instead of filtering the fixed/overflow panel element directly.
- Social activity media-name rows keep their comment icon at a fixed 13px square and prevent flex shrinking so long filenames cannot resize the icon.
- Social activity rows render content name first, then commenter name, then comment text. Comment timestamps are available through the browser tooltip on hover instead of being shown inline.
- The feed supports generic non-hidden files in addition to images and videos. Unknown extensions are indexed as `type: "file"` using the same safe media ID, comments, likes, and `/media/{id}` serving path; dotfiles and hidden/internal dot-directories remain excluded from scanning and ID validation.
- Generic files use a dedicated card surface with file metadata and a download link rather than trying to render the file as visual media. File upload inputs intentionally do not set an `accept` filter.
- Uploaded files preserve the browser-provided source `File.lastModified` in metadata and return it as the item's visible `modifiedAt`; the server file mtime remains the upload time and is still used internally for feed sorting so new uploads appear near the top.

## Drawing Board Decisions

- Drawing boards are a separate entity from media items. They are stored as JSONL files under `test-content/.boards/{boardID}.jsonl`. The first line is JSON board metadata (name, createdAt), subsequent lines are stroke records.
- Board IDs are random 32-character hex strings, stroke IDs are random 24-character hex strings.
- Each stroke record contains: `id`, `tool` (freeform or line), `points` (array of [x,y] coordinate pairs), `color`, `size`, `author`, `createdAt`.
- The `BoardStore` initializes on server startup by reading existing `.boards` JSONL files. It holds all boards and strokes in memory and appends to disk on each new stroke.
- Drawing interaction is only available in fullscreen/expanded mode. В режиме карточки отображается предпросмотр, который синхронизируется в реальном времени.
- **Оптимизация производительности:** Используется система из трех холстов (сетка, буфер завершенных линий, активный штрих). Отрисовка активного штриха — инкрементальная (рисуется только новый сегмент), что обеспечивает стабильные 60 FPS независимо от количества линий.
- **Рендеринг:** Для максимальной производительности удалены тяжелые эффекты `shadowBlur`. Используется чистая быстрая линия.
- **Интерфейс:** Панель инструментов в развернутом режиме расположена вертикально справа. Имеет динамическую прозрачность (60% в покое, 100% с блюром при наведении), чтобы не перекрывать холст и не конфликтовать с панелью комментариев.
- **Координаты:** Математика пересчета координат курсора учитывает «леттербоксинг» (черные поля), возникающий при `object-fit: contain`, обеспечивая точность попадания в пиксель при любых пропорциях окна.
- Each completed stroke is submitted via `POST /api/boards/{id}/strokes` and broadcast over one board-wide SSE stream at `GET /api/boards/events`; clients filter stroke events by `boardId`. Подписка на SSE активна для всех видимых досок в ленте, а не только для развернутой.
- Regular boards are exposed in the feed through `{boardID}.board` placeholder files in the media root. The feed item keeps a normal opaque 64-character media `id` for comments, likes, metadata, favorites, and media lookup, and exposes the drawing board identity separately as `boardId`.
- The frontend no longer prepends virtual board media from `GET /api/boards` on startup; the feed scanner is the single source for board cards, preventing duplicate board rows and split comment/like state.
- New boards are created via `POST /api/boards` triggered by the "Board" button in the header toolbar. The response includes `mediaId`, and the server inserts the placeholder into the runtime media index immediately so the new board can receive comments without a restart.
- **Мастер-доска:** Специальная доска с фиксированным ID `master`, которая создается сервером автоматически и хранится только как `test-content/.boards/master.jsonl`. Её превью всегда отображается в сайдбаре под профилем пользователя, обеспечивая быстрый доступ к общему пространству для рисования из любой части приложения. Для нее не создается `.board` placeholder в media root, потому что она не является элементом основной ленты.
- The media scanner already ignores dot-prefixed directories, so `.boards` is excluded from the media index automatically.
- Expanded drawing boards place their close button at the top-right to match regular expanded media cards.

## Agent Workflow Constraints

- Agents may start the local server only for short verification checks and must stop it immediately afterward, unless the user explicitly asks to start or keep it running.
- Use `make package-win` to rebuild the root Windows package: frontend into `build/feed-ai-win64/web/dist`, Go `server.exe` for Windows amd64, bundled Windows amd64 FFmpeg/FFprobe tools when present, and `build/feed-ai-win64.zip`.
- Record only important new decisions, constraints, verification-relevant outcomes, and known issues here. Do not append routine changelog entries.

## Known Environment Notes

- The local Go toolchain previously reported `log/slog` missing from stdlib, so the server used standard `log`.
- Go commands may need sandbox escalation when the Go build cache is outside the workspace.
- Dependency installation may need sandbox escalation for registry access.
- Upload implementation verified with `go test ./...`, `npm --prefix web run check`, `npm --prefix web run build`, and a short Go server smoke test for `/` plus `/api/feed` on 2026-05-08.
- Favorites implementation verified on 2026-05-09 with `go test ./...`, `npm run check` in `web`, and `npm run build` in `web`.
