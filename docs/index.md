---
title: Feed AI Knowledge Base
type: index
status: active
---

# Feed AI Knowledge Base

This directory is the canonical engineering knowledge base for Feed AI. It is
plain Markdown and can be opened directly as an Obsidian vault without plugins.

## Reading Order

1. [Product direction](product/product-direction.md)
2. [System overview](architecture/system-overview.md)
3. The feature or subsystem relevant to the current task
4. [Decision records](decisions/index.md) when the reason behind a constraint matters
5. [Development and verification](operations/development-and-verification.md)

## Architecture

- [System overview](architecture/system-overview.md)
- [Backend and HTTP API](architecture/backend-and-api.md)
- [Frontend architecture](architecture/frontend.md)
- [Filesystem storage](architecture/filesystem-storage.md)
- [Realtime communication](architecture/realtime.md)
- [Media identity and processing](architecture/media.md)

## Features

- [Feed, uploads, and favorites](features/feed-and-favorites.md)
- [Comments, likes, and activity](features/comments-likes-and-activity.md)
- [Drawing boards](features/drawing-boards.md)
- [Media players and cards](features/media-cards-and-players.md)
- [Asteroids](features/asteroids.md)

## Design And Operations

- [Design system](design/design-system.md)
- [Interaction and performance](design/interaction-and-performance.md)
- [Development and verification](operations/development-and-verification.md)
- [Packaging and runtime](operations/packaging-and-runtime.md)
- [Known constraints](operations/known-constraints.md)

## Maintenance

Reference notes describe current behavior only. Update the relevant note in the
same change that alters that behavior. Create an ADR only for a consequential
choice with meaningful alternatives; do not use ADRs as a changelog.

All notes require `title`, `type`, and `status` frontmatter. Decision records
also require `date` and numeric `decision` fields. Use standard relative
Markdown links rather than Obsidian-specific wikilinks.

