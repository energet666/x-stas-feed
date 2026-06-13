---
title: Browser-Owned Favorites
type: decision
status: accepted
date: 2026-05-31
decision: 6
---

# Browser-Owned Favorites

## Context

V1 has no accounts or personalization backend, but users need a saved-items
view with stable local ordering.

## Decision

Store ordered media IDs in browser `localStorage` under `feed-ai:favorites`.
Resolve items individually through `GET /api/media/{id}` and remove stale IDs
on `404`.

## Consequences

Favorites do not synchronize between browsers and do not require server writes.
The existing favorite-page endpoint is compatibility code, not the current UI's
source of truth.

