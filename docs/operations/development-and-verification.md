---
title: Development And Verification
type: reference
status: active
---

# Development And Verification

Requirements are Go 1.25+, Node.js, and npm. FFmpeg and FFprobe are optional.

## Development

```sh
cd web
npm install
npm run dev
```

Run the API separately:

```sh
GOCACHE=/tmp/feed-ai-go-cache go run ./cmd/server \
  -addr :8080 -content-dir test-content -static-dir web/dist
```

Vite listens on port 5173 and proxies `/api` and `/media` to
`VITE_API_TARGET`, defaulting to `http://localhost:8080`.

## Verification

```sh
GOCACHE=/tmp/feed-ai-go-cache make check
make web-build
```

`make check` runs the documentation checker, `svelte-check`, and all Go tests.
Use `GOCACHE=/tmp/feed-ai-go-cache` in the agent sandbox because the default
macOS cache is not writable.

Agents may run a local server only for short verification and must stop it
immediately afterward unless the user explicitly requests a persistent server.
Builds and tests may require network or sandbox approval when dependencies are
not already available.

## Documentation Workflow

Start at [the knowledge index](../index.md). Update current reference notes in
the same change as behavior. Add an [ADR](../decisions/index.md) only for a
consequential decision. Routine implementation history and verification logs
belong in Git and task results.

