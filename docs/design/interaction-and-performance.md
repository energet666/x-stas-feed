---
title: Interaction And Performance
type: reference
status: active
---

# Interaction And Performance

Preserve browser-specific interaction workarounds unless the underlying issue
is reverified:

- Avoid no-op transforms on visible blurred overlays in Firefox.
- Keep unprefixed and `-webkit-` backdrop-filter declarations. Vite's Tailwind
  optimization remains disabled because it previously dropped the unprefixed
  property in production CSS.
- Focus the comment composer synchronously from the opening user gesture, with
  short retries for Safari.
- Use pointer geometry for audio/video seek tracks.
- Stop board file-drop events before they reach the page upload target.
- Keep wheel scrolling inside board help and asset panels from becoming board
  zoom while preserving native panel scrolling.

## Performance Invariants

- Virtualize feed DOM while retaining loaded item geometry in memory.
- Schedule scroll/resize state updates through `requestAnimationFrame`.
- Render board strokes incrementally without `shadowBlur`.
- Keep background canvas internal resolution near one megapixel, idle animation
  near 24 FPS, and short 60 FPS scheduling only during scroll response.
- Respect reduced motion and document visibility.
- Use an opaque background canvas and avoid unnecessary local Asteroids RAF work.
- Keep glass effects user/debug switchable because `backdrop-filter` can
  dominate Chrome GPU cost even at low blur radii.

After changes to card effects, particles, boards, or media controls, verify in
Chrome, Firefox, and Safari when practical.

