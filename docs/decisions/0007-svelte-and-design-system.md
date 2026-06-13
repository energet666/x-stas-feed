---
title: Svelte 5 And Internal Design System
type: decision
status: accepted
date: 2026-06-03
decision: 7
---

# Svelte 5 And Internal Design System

## Context

The media-heavy UI needs reactive state, focused components, and a distinct
visual language without adopting a generic component theme.

## Decision

Use Svelte 5 runes, TypeScript, Vite, Tailwind CSS 4, and `lucide-svelte`.
Maintain semantic design tokens and `ui-*` primitives in `web/src/app.css`;
keep feature-specific styles scoped. Do not use daisyUI.

## Consequences

New frontend code follows rune patterns and the existing primitives. Shared
visual conventions belong in the global design system only when reused or
product-wide; geometry-heavy feature behavior remains local.

