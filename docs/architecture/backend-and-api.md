---
title: Backend And HTTP API
type: reference
status: active
---

# Backend And HTTP API

The backend uses Go 1.25 and the standard `net/http` router. `cmd/server` accepts
`-addr`, `-content-dir`, and `-static-dir`; their defaults are `:8080`,
`test-content`, and `web/dist`.

## Main HTTP Surface

Feed and media:

- `GET /api/feed?index=-1` returns the newest item and index bounds.
- `GET /api/feed?index=N` returns exactly one item.
- `GET /api/media/{id}` returns one safe media record.
- `GET /media/{id}` serves media bytes with a one-hour public cache policy.
- `GET /api/media/{id}/poster?time=T` and `/cover` serve generated artifacts.
- `POST /api/uploads` accepts one multipart `files` part, plus optional
  `modifiedAt`, with a 5 GiB request limit.
- `GET /api/activity?limit=N` returns persisted comment activity, capped at 100.

Comments and likes:

- `GET|POST /api/media/{id}/comments`
- `POST /api/media/{id}/comments/{commentID}/likes`
- `POST /api/media/{id}/likes`
- `GET /api/comments/events`

Boards:

- `POST /api/boards`
- `GET /api/boards/{id}`
- `POST /api/boards/{id}/strokes`
- `POST /api/boards/{id}/images`
- `GET /api/boards/events`
- `GET /api/board-assets` and `GET /api/board-assets/{assetID}`

`GET /api/boards`, `GET /api/boards/{id}/background`,
`GET /api/boards/{id}/assets/{assetID}`, and
`POST /api/feed/favorites` remain compatibility surfaces but are not the
primary current UI paths.

Asteroids:

- `GET /api/ships/socket` upgrades to WebSocket.
- `GET /api/ships/scores` returns the server-written solo leaderboard.

## Safety And Mutation Rules

Clients never submit filesystem paths. Media, comment, board, cover, poster, and
asset handlers resolve server-controlled IDs through their owning stores.
Uploads reject empty or path-like names, allocate OS-style collision suffixes,
and enter the runtime index immediately. Empty content directories and missing
optional generated artifacts are normal states.

