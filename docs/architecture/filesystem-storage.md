---
title: Filesystem Storage
type: reference
status: active
---

# Filesystem Storage

All durable application state is stored below the configured content root.
Regular root files are feed content. Dot-prefixed server directories are hidden
from scanning.

## Sidecar Layout

- `.comments/{rootFilename}.jsonl`: comments, one JSON object per line.
- `.metadata/{rootFilename}.json`: display name, visible modified time, media
  like count, and cached probe metadata.
- `.posters/`: generated video poster JPEGs.
- `.covers/`: extracted audio cover JPEGs.
- `.boards/{rootFilename}.jsonl`: board metadata followed by ordered operations.
- `.boards/master.jsonl`: the non-feed master board.
- `.boards/assets/`: content-addressed reusable board images.
- `.boards/sticker-pack/`: committed starter assets and attribution.
- `.game-scores/`: filesystem-backed solo Asteroids scores.

Sidecar names are derived from media records known to the server, never from
arbitrary client paths. JSON and JSON Lines are used so user text may contain
newlines or delimiter characters without ambiguous parsing.

## Consistency Model

The runtime index is initialized from disk and then updated for server-managed
uploads, boards, comments, and likes. Out-of-band additions, removals, or
renames are unsupported until restart. Metadata probing and generated covers or
posters are best effort; failure must not make the underlying media unavailable.

Comment likes rewrite their JSONL file atomically because an existing record
changes. New comments and board operations append to their stores.

