---
title: System Overview
type: reference
status: active
---

# System Overview

Production is one Go process using `net/http`. It serves the JSON API, media
bytes, WebSocket/SSE endpoints, and the built Svelte single-page application.

## Repository Boundaries

- `cmd/server`: executable entrypoint and runtime flags.
- `internal/media`: media indexing, metadata, comments, uploads, boards,
  generated artifacts, and game score persistence.
- `internal/server`: HTTP routing, handlers, middleware, SSE hubs, and the
  Asteroids WebSocket transport.
- `internal/game`: authoritative Asteroids simulation.
- `web`: Svelte 5 application built by Vite.
- `test-content`: default local content root and server-managed sidecars.

## Runtime Model

At startup the server initializes drawing boards and lazily builds a long-lived
media index. Media are represented by opaque server-generated IDs. The server
owns every mapping from an ID to a filesystem path.

The browser loads one exact feed index at a time, renders only visible rows, and
uses SSE for ordinary live changes. Asteroids is the sole WebSocket feature
because it needs bidirectional commands and authoritative snapshots.

See [backend and API](backend-and-api.md), [frontend architecture](frontend.md),
[storage](filesystem-storage.md), and [realtime communication](realtime.md).

