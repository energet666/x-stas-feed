---
title: Filesystem-Backed Persistence
type: decision
status: accepted
date: 2026-05-01
decision: 2
---

# Filesystem-Backed Persistence

## Context

Feed AI is a local-first v1 without accounts, database operations, or deployment
infrastructure.

## Decision

Store media as root content files and application state as server-managed JSON
or JSONL sidecars below the content root. Keep an in-memory runtime index and
update it for server-managed mutations.

## Consequences

The repository needs deterministic safe filenames and atomic rewrites where
records change. External filesystem changes require restart. Features requiring
database-style queries, transactions, or user identity are out of scope until
this decision is revisited.

