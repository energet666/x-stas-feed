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
colors persist in browser storage. The help dialog documents current shortcuts.

## Images And Assets

Dropped images become local placement drafts before persistence. Uploads are
capped at 25 MiB. Asset bytes are SHA-256 content-addressed and shared across
boards; identical bytes reuse storage while creating a new immutable operation.
The asset library combines committed starter-pack images and assets referenced
by board history. Its global endpoints serve only registered assets.

The committed starter pack under `test-content/.boards/sticker-pack` is part of
the repository and Windows package. Its attribution files must remain intact.

## Collaboration

Stroke and image POSTs persist the operation and return through the global board
SSE stream. Every visible board may subscribe for live previews. The frontend
aggregates live board edits into one activity item per board.

