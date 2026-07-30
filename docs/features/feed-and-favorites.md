---
title: Feed Uploads And Favorites
type: reference
status: active
---

# Feed, Uploads, And Favorites

The feed API exposes exact indexes. `index=-1` resolves the newest item and
returns `firstIndex` and `lastIndex`; non-negative indexes resolve exactly one
item. The frontend fetches small batches by repeatedly requesting decreasing
indexes and gracefully handles an empty content directory.

Up and Down move between adjacent feed cards and vertically center the selected
card in the viewport. Keyboard navigation follows the retained virtual row
geometry, loads the next item at the current loaded boundary, and yields to
media controls, text entry, dialogs, expanded cards, drawing boards, and
Asteroids after the user explicitly gives the page background keyboard control.

Feed responses include display metadata, safe media URLs, counts, and the latest
comment summary. Cards use `displayName` for visible titles and accessibility
labels, not technical storage names.

## Uploads

The browser and server accept one file per upload. Multiple files, empty files,
path-like names, and known files over 5 GiB are rejected. Filename collisions
use suffixes such as `clip (1).mp4`. Successful uploads update the runtime index,
publish a feed-created event, and cause the initiating client to reload from
the newest item.

Received bytes stream into a scratch file inside the content root and are moved
into place with a rename, so a large upload is written once. The browser
progress bar tracks only the request body, so it reaches 100% before the server
finishes the rename, metadata probe, and index update.

## Favorites

Favorites are an ordered ID array under `feed-ai:favorites`. New IDs are
inserted at the front. Favorites mode maps virtual indexes to this array and
resolves each item with `GET /api/media/{id}`. A `404` removes the stale ID;
unfavoriting a visible item removes it immediately while preserving virtual
index alignment.

The legacy favorite-page backend endpoint is not the source of truth for the
current UI. See [ADR 0006](../decisions/0006-browser-owned-favorites.md).
