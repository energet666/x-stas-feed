---
title: Comments Likes And Activity
type: reference
status: active
---

# Comments, Likes, And Activity

Comments are tied to safe media identity and stored as JSON Lines. Text is
trimmed, required, and capped by the backend. Author input is normalized to a
maximum length and falls back to `Guest`; the browser normally supplies a
persisted or generated Russian nickname.

Cards render the latest one or two comments from feed summaries. Opening the
comments affordance loads the complete thread and provides loading, empty,
error, and submit-in-progress states. Enter submits, Shift+Enter inserts a
newline, and IME composition must not submit.

Media and comment likes are anonymous one-way increments. Media counts live in
metadata JSON. Comment counts live in comment records, so liking a comment
atomically rewrites the thread file. Both publish live events.

Persisted activity is comment-only and returned newest first for valid indexed
media. The frontend merges it with session-only aggregated board activity from
board SSE events. Selecting comment activity opens a media/comments modal;
selecting board activity opens the expanded board.

