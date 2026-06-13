---
title: SSE And WebSocket Policy
type: decision
status: accepted
date: 2026-06-07
decision: 4
---

# SSE And WebSocket Policy

## Context

Comments, likes, feed announcements, and boards need server-to-browser updates.
Asteroids additionally needs frequent client commands and authoritative world
snapshots.

## Decision

Use shared Server-Sent Event streams for ordinary one-way application updates.
Use WebSocket only for the bidirectional Asteroids protocol.

## Consequences

New ordinary realtime features should extend an appropriate shared SSE stream.
Adding WebSockets elsewhere requires a new architectural decision. Asteroids
must keep command validation and world authority on the server.

