---
title: Frontend Architecture
type: reference
status: active
---

# Frontend Architecture

The frontend uses Svelte 5 runes, TypeScript, Vite, Tailwind CSS 4, and
`lucide-svelte`. User-facing strings live in `web/src/lib/ui_text.ts`; do not
introduce a full localization framework without a product-scope change.

## Ownership

`App.svelte` coordinates feed loading, virtualization, measurement, scroll
anchoring, global overlays, favorites, comments, activity, uploads, SSE, and
background feature state. Presentational and interaction-heavy behavior stays
in focused components.

The loaded feed model retains every fetched card and its last known or estimated
geometry. Only visible rows plus overscan mount in the DOM. Top and bottom
spacers are derived from the full row map, and viewport work is coalesced with
`requestAnimationFrame`.

## Browser State

Important `localStorage` keys include:

- `feed-ai:favorites`
- `feed-ai:comment-username`
- drawing brush color, size, opacity, and custom colors
- audio/video volume and per-media progress
- feed video autoplay preference
- visitor panel state and page/card appearance preferences
- debug feature switches and the Asteroids resume token

Storage failures must degrade to in-memory behavior. Favorites and appearance
settings are browser-owned and are not treated as backend state.

## UI Composition

Desktop layout is a centered grid with a left control/profile rail, a 42rem feed
column, and a right activity rail. The activity rail becomes a drawer at
narrower widths. Expanded media lock page scrolling; ordinary in-card comments
do not. Fullscreen comments render above expanded media in a top-level overlay.
