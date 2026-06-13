---
title: Design System
type: reference
status: active
---

# Design System

The frontend uses a CSS-first internal design system built with Tailwind CSS 4.
Shared tokens and reusable primitives live in `web/src/app.css`; feature-specific
geometry and interaction styling stays in scoped Svelte CSS. Do not add daisyUI.

## Tokens

Use semantic `@theme` tokens:

- Page and surfaces: `--color-page`, `--color-surface-media`,
  `--color-surface-overlay`
- Text: `--color-fg-primary`, `--color-fg-secondary`, `--color-fg-muted`,
  `--color-fg-subtle`, `--color-fg-danger`
- Borders: `--color-border-glass`, `--color-border-glass-soft`,
  `--color-border-glass-hover`
- Actions: `--color-action-bg`, `--color-action-hover`,
  `--color-action-hover-strong`
- Radii: media card, panel, overlay, control, and toolbar radii

Do not restore historical generic color aliases or `glass-*` class names.

## Shared Primitives

- `ui-nav`, `ui-panel`, and `ui-panel-side`
- `ui-media-card`
- `ui-button` and `ui-button-danger`
- `ui-pill`
- `ui-icon-button` and `ui-icon-button-danger`
- `ui-overlay-panel` and `ui-overlay-panel-visible`

Use Lucide icons for standard actions. Keep local classes when behavior is
specific to one feature and does not define a reusable visual convention.

## Visual Direction

The product is a dark, media-first glass interface with restrained borders,
minimal external shadows, readable overlays, and ambient card backgrounds.
Side rails use lighter glass than media overlays. A user-selectable daylight
background uses graphite geometry instead of cosmos particles. Global
scrollbars share one dark-theme treatment across Firefox and WebKit browsers.

