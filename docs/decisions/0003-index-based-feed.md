---
title: Index-Based Feed
type: decision
status: accepted
date: 2026-05-31
decision: 3
---

# Index-Based Feed

## Context

The browser needs stable, exact loading for a virtualized newest-first feed
while the runtime library maintains a deterministic ordered slice.

## Decision

Store runtime items oldest to newest. `GET /api/feed?index=-1` resolves the
newest item and bounds; each non-negative index resolves one exact item. The
frontend walks indexes downward.

## Consequences

The frontend owns batching and end detection. Server-managed insertions may
change the highest bound, so live events offer a refresh instead of mutating the
visible virtual list in place.

