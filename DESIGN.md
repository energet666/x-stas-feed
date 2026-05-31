# Feed AI Design System

## Overview

The frontend uses a lightweight in-house design system built on Tailwind CSS 4. It is CSS-first: shared tokens and reusable primitives live in `web/src/app.css`, while feature-specific layout and interaction styling stays in scoped Svelte component CSS.

daisyUI is not part of the current system. Do not add daisyUI themes or component classes unless the product direction changes deliberately.

## Tokens

Define shared tokens in the `@theme` block in `web/src/app.css`.

Use semantic token names for new code:

- Page and surfaces: `--color-page`, `--color-surface-media`, `--color-surface-overlay`
- Text: `--color-text-primary`, `--color-text-secondary`, `--color-text-muted`, `--color-text-subtle`, `--color-text-danger`
- Borders: `--color-border-glass`, `--color-border-glass-soft`, `--color-border-glass-hover`
- Actions: `--color-action-bg`, `--color-action-hover`, `--color-action-hover-strong`
- Radii: `--radius-media-card`, `--radius-panel`, `--radius-overlay`, `--radius-control`, `--radius-toolbar`

Historical aliases such as `--color-primary`, `--color-muted`, `--color-button-bg`, `--color-glass-border`, and `--radius-card` remain available for compatibility. Prefer the semantic names above when adding or refactoring code.

Tailwind utilities generated from these tokens are valid. In markup, concise compatibility utilities such as `text-primary`, `text-muted`, and `text-danger` are acceptable for readability. Prefer newer semantic utilities such as `bg-action-bg` and `border-border-glass-soft` when using action or border tokens directly.

## Primitives

Use the `ui-*` primitives for new shared UI:

- `ui-nav`: top or rail navigation glass surface
- `ui-panel`: standard glass panel
- `ui-panel-side`: lighter side/debug panel treatment
- `ui-media-card`: media card shell with the card backing layer
- `ui-button`: text or icon+text pill button
- `ui-button-danger`: destructive button variant, composed with `ui-button`
- `ui-pill`: passive compact pill
- `ui-icon-button`: square icon button
- `ui-icon-button-danger`: destructive icon button variant, composed with `ui-icon-button`
- `ui-overlay-panel`: overlay panel above media
- `ui-overlay-panel-visible`: visible state for overlay panels

Existing `glass-*` and `card-overlay*` classes are compatibility aliases for the same primitives. New component markup should use `ui-*` names. Local feature classes such as `feed-card-overlay` can keep their domain-specific names when they are not reusable design-system primitives.

## Component Styling Rules

Keep reusable visual language in `web/src/app.css` when at least two components need it or when it defines a product-wide convention.

Keep scoped styles inside a Svelte component when the rules are feature-specific, geometry-heavy, or tied to local interaction state. Drawing-board controls, media-player internals, virtual-feed geometry, and game HUD styling can stay local unless a pattern repeats elsewhere.

Use lucide-svelte icons for standard actions. Buttons that are only actions should generally be icon buttons with accessible labels; reserve text buttons for commands where the label carries important meaning.

Avoid reintroducing external component themes. The app's visual identity is a dark, media-first glass interface with restrained borders, minimal external shadows, and readable overlays over media content.
