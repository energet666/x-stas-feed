---
title: Known Constraints
type: reference
status: active
---

# Known Constraints

- External content-directory changes are not watched. Restart after manual
  additions, removals, or renames.
- Media probing, poster generation, and cover extraction depend on optional
  FFmpeg tools and intentionally degrade without them.
- Persisted social activity includes comments only; board activity is live and
  session-local.
- Legacy favorite and board listing/background endpoints remain for
  compatibility but are not primary UI data flows.
- Browser preferences and favorites do not synchronize between browsers.
- The local deployment model has no authentication or multi-tenant filesystem
  isolation.
- Very large content collections retain loaded feed metadata and row geometry
  in browser memory even though the DOM is virtualized.
- The emoji picker has Russian UI and generated Russian search keywords rather
  than a general application localization framework.
- The committed board sticker pack is deliberately exposed through `.gitignore`;
  additions and deletions under that pack must be included with related changes.

