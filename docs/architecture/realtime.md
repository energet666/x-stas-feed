---
title: Realtime Communication
type: reference
status: active
---

# Realtime Communication

Use Server-Sent Events for one-way application updates. WebSocket is reserved
for Asteroids.

## SSE Streams

`GET /api/comments/events` is the global social/feed stream. It carries new
comments, media likes, comment likes, and newly created feed items. The client
updates visible state directly but does not prepend a newly announced card into
the virtual list. It shows a refresh affordance and reloads from the newest
index to avoid viewport jumps.

`GET /api/boards/events` broadcasts persisted stroke and image operations.
Board clients filter events by `mediaId`. The authoring client also ingests its
own operation through SSE, leaving one authoritative state-ingestion path.

Do not add one SSE connection per card or a separate feed stream without
reconsidering browser per-origin connection pressure.

## Asteroids WebSocket

`GET /api/ships/socket` accepts sequenced commands and emits welcome messages
and authoritative snapshots. Physics, collision, score, presence, reconnect
grace, and power-ups remain server-owned. The browser predicts only local
movement and reconciles with acknowledged sequence numbers.

