---
title: Packaging And Runtime
type: reference
status: active
---

# Packaging And Runtime

`make package-win` builds a Windows amd64 distribution under
`build/feed-ai-win64` and creates `build/feed-ai-win64.zip`. It includes:

- the Vite production bundle;
- `server.exe`;
- the committed drawing-board sticker pack;
- bundled Windows amd64 FFmpeg and FFprobe binaries when locally present.

Production-style local operation builds `web/dist` and starts the Go server
against that directory. The Go server logs startup, scan/index work,
server-managed persistence, requests, and generated media artifacts.

Console logging uses ANSI color unless `NO_COLOR`, `FEED_AI_NO_COLOR=1`, or
`TERM=dumb` disables it. Structured `event key=value` messages are formatted as
readable multiline blocks; ordinary messages remain single-line.

Request logs include method, remote address, path, raw query, status, request
and response byte counts, and duration. Media artifact logs identify cache hits
or generation work.

