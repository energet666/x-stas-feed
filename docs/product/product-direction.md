---
title: Product Direction
type: reference
status: active
---

# Product Direction

Feed AI is a local-first, Instagram-like media feed for photos, GIFs, videos,
audio, generic files, and collaborative drawing boards. It is designed for a
single local deployment backed by a filesystem content directory.

## Current Product

- A modern one-column, virtualized, infinite feed ordered newest first.
- Browser uploads of one file at a time into the configured content root.
- Inline comment summaries, full comment threads, anonymous media and comment
  likes, and live social activity.
- Browser-owned favorites shown in the order they were saved.
- Drawing on regular boards and non-GIF images, including reusable placed image
  assets and live collaboration.
- A server-authoritative Asteroids mode that can also render behind the feed.
- A Russian user interface with internal engineering documentation in English.

## V1 Boundaries

- No accounts, authentication, authorization, or database.
- No server-side personalization or synchronization of browser preferences.
- Persistence is filesystem-backed and intended for a trusted local operator.
- Likes are anonymous one-way increments; there is no unlike or per-user state.
- The main content directory is scanned at startup. Server-managed mutations
  update the in-memory index; arbitrary external mutations require restart.
- Modern evergreen browsers are the target.

The implementation constraints behind these boundaries are documented in
[system overview](../architecture/system-overview.md) and the
[filesystem persistence ADR](../decisions/0002-filesystem-backed-persistence.md).

