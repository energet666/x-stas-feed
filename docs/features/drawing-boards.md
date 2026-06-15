---
title: Drawing Boards
type: reference
status: active
---

# Drawing Boards

Regular boards are root `.board` placeholder media. Their ordered operation
history lives in `.boards/{rootFilename}.jsonl`. Non-GIF images use their media
ID as image-backed boards without copying the image. The fixed `master` board
lives only at `.boards/master.jsonl` and appears in the sidebar rather than the
feed.

The first JSONL line stores name, creation time, background, and canvas. Later
lines preserve operation order for strokes and fixed images. A stroke contains
tool, normalized points, color, size, opacity, author, and creation time.
Coordinates are finite and rounded to one decimal place but are not clamped to
the canvas. Images include asset identity, placement, rotation, horizontal
mirroring, author, and time.

## Interaction

Feed previews are read-only; editing requires expanded mode. The board uses
separate grid, committed-operation, and active-operation canvases. Drawing is
incremental, while zoom and pan transform only the viewport and never persisted
coordinates.

Boards open in pan mode. Freeform and straight-line tools use press-drag-release.
Plain wheel zooms around the pointer; Ctrl-wheel adjusts brush size while
drawing. Middle-button or Space-drag temporarily pans. Brush settings and custom
colors persist in browser storage. The help dialog documents current shortcuts
and identifies toolbar tools with the same icons used by the toolbar.

Local drawing mode keeps completed strokes and placements from the existing
asset library in the expanded browser session instead of posting them
immediately. The user can undo the latest local operation with the action bar or
Ctrl/Cmd+Z, discard the session, or publish all remaining operations together.
Their layer order is preserved. History mode is unavailable until the local
session is published or discarded. Incoming SSE operations continue to update
the committed layer underneath the local operations.

## Images And Assets

Dropped images are immediately uploaded and registered as reusable assets in
both normal and local drawing modes, then opened as placement drafts at the drop
point. Canceling a placement or local session does not delete the registered
asset. In normal mode, confirming the draft persists the placement immediately;
in local mode, the placement remains local until the operation batch is
published. Uploads are capped at 25 MiB. Asset bytes are SHA-256
content-addressed and shared across boards; identical bytes reuse storage while
creating a new immutable operation. The asset library combines committed
starter-pack images and registered user assets. Its global endpoints serve only
registered assets.

The committed starter pack under `test-content/.boards/sticker-pack` is part of
the repository and Windows package. Its attribution files must remain intact.

## Collaboration

Stroke and image POSTs persist the operation and return through the global board
SSE stream. Every visible board may subscribe for live previews. The server
derives one activity item per board from its latest persisted stroke or image
placement and replaces that item after each new operation.

`POST /api/boards/{id}/operations/batch` validates a complete mixed group of
strokes and existing-asset placements before appending it in order and returning
the created operations. Each persisted operation is then published through the
same global board SSE stream. The stroke-only batch endpoint remains available.

`POST /api/board-assets` accepts one multipart image, stores it content-addressed,
and registers it in the global asset library without adding a board operation or
publishing an SSE event.
