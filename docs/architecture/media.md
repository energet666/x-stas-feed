---
title: Media Identity And Processing
type: reference
status: active
---

# Media Identity And Processing

Media IDs are fixed-length SHA-256 hexadecimal identities derived from
normalized root filenames. They are opaque to clients and resolve only through
the current runtime index.

## Classification And Ordering

Known image, video, audio, and `.board` extensions receive dedicated media
types. Other non-hidden root files become generic file cards. Hidden files and
dot-directories are excluded.

The runtime slice is ordered oldest to newest. Visible ordering uses filesystem
modification time with filename as a stable tie-breaker, and the frontend walks
indexes downward. Uploaded files retain the browser-provided source
`lastModified` as visible metadata while their server file modification time
keeps the new upload near the top of the feed.

## Optional Processing

FFprobe extracts audio tags and audio/video duration. FFmpeg extracts embedded
audio covers and video posters. The server first checks bundled tools below
`tools/ffmpeg/{GOOS}-{GOARCH}` and then `PATH`.

Probe results are cached with source size and modification-time signatures.
Missing tools, malformed files, or extraction failures are non-fatal. `.ogg`
keeps a video fallback when probing cannot identify whether it is audio or
video.

Non-GIF images are also drawing-board backgrounds. Their canvas dimensions
preserve aspect ratio, normalize area around the default 1200x800 coordinate
space, and account for JPEG EXIF orientations that swap visual dimensions.

