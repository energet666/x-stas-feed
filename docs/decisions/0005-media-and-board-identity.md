---
title: Server-Controlled Media And Board Identity
type: decision
status: accepted
date: 2026-05-31
decision: 5
---

# Server-Controlled Media And Board Identity

## Context

Clients need stable references for media, comments, likes, and boards without
being allowed to choose filesystem paths.

## Decision

Derive opaque SHA-256 media IDs from normalized root filenames and resolve them
through the server index. Regular `.board` files and image-backed boards use
their media ID for feed, comments, likes, and board APIs. Only the non-feed
master board uses the fixed ID `master`.

## Consequences

All handlers must use server-owned lookup before filesystem access. Renaming a
root file changes its identity. Supporting board JSONL files cannot recreate a
missing root `.board` feed item.

