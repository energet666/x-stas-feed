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

Activity is returned newest first and combines comments for valid indexed media
with the latest persisted operation time for each board. Board activity stores no
separate history: server startup derives one item per board from its final
stroke or image placement, and each new operation replaces that board's
previous in-memory activity item. The frontend applies the same replacement
rule to board SSE events.
Board names are resolved from current feed metadata when the activity response
is built and are not stored in the activity index.
Selecting comment activity opens a media/comments modal. Selecting regular
board activity opens the same media/comments modal for that board; the fixed
master board still opens directly because it is not feed media.
