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
audio covers, video posters, and video scrubbing sprite sheets. The server first
checks bundled tools below `tools/ffmpeg/{GOOS}-{GOARCH}` and then `PATH`.

Probe results are cached with source size and modification-time signatures.
Missing tools, malformed files, or extraction failures are non-fatal. `.ogg`
keeps a video fallback when probing cannot identify whether it is audio or
video.

## Scrubbing Sprite Sheets

One video has one sheet of preview tiles, generated on first request and cached
until the source changes. A sheet holds at most 100 tiles of 160 pixel width in
a ten-column grid, never samples faster than one tile every two seconds, and is
unavailable below five seconds of duration. Tile `i` holds the frame at
`intervalSeconds * (i + 0.5)`, so a client maps a timeline position with
`floor(time / intervalSeconds)`.

The grid is planned against a timeline shortened by half a second, because
ffmpeg emits its last sample only when the source runs half an interval past it
and container duration can overstate the video stream. Video longer than five
minutes decodes keyframes only, which is several times faster at an interval
wide enough to hide the snapping; that pass falls back to a full decode when the
container has no usable keyframe markers. Concurrent requests for one video
share a single generation instead of starting one ffmpeg process each.

Non-GIF images are also drawing-board backgrounds. Their canvas dimensions
preserve aspect ratio, normalize area around the default 1200x800 coordinate
space, and account for JPEG EXIF orientations that swap visual dimensions.

