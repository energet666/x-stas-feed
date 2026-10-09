---
title: Media Cards And Players
type: reference
status: active
---

# Media Cards And Players

`FeedCardFrame` owns card geometry, ambient composition, expanded mode, and top
and bottom overlay stacks. The lower social chin sits outside the media frame so
likes and comments do not obscure content. Expanded cards use a viewport-fixed
wrapper and raise comments above the media through a top-level overlay. While a
feed card, drawing board, or activity media modal occupies the viewport, page
background animation is paused without changing the visitor's saved appearance
preference, then resumes when the fullscreen layer closes.

Overlay visibility is coordinated by the frame. Interacting with a panel or
range control clears auto-hide timers. Visible overlay transforms must become
`none` so Firefox can apply nested backdrop blur. Range seeking uses pointer
coordinates over the rendered track rather than relying on browser-native range
geometry.

## Video

Video cards provide playback, seek, volume, mute, fullscreen, poster, buffered
state, and per-media progress persistence. Only one feed video is active at a
time. Browser-owned autoplay is off by default. When enabled, a video starts
with the stored volume and mute state after its entire feed card enters the
viewport and pauses as soon as the card is no longer fully visible; cards
taller than the viewport do not autoplay. Manual controls remain available.
Duration from backend metadata avoids requiring Safari to preload solely for
control layout. Ambient sampling follows the visible frames: while playback runs
in picture-in-picture the card keeps its last ambient frame and stops sampling,
and it resumes on return to the card.

Viewers mark interesting moments with the controls button or `M`. A marker is
the whole second plus the visitor nickname, with no identifier and no browser
state. Markers are shared, so every viewer sees them as dots over the seek
track; one second is one dot, and the dot grows with the number of viewers who
marked that second. Video items carry their markers in the feed payload, and
new markers arrive over the social SSE stream, including for the marking
client, so the appearing dot is the only confirmation; a duplicate or failed
marker stays silent. One nickname marks one second once: the backend returns
the existing marker without creating or publishing anything, and the player
refuses the repeat locally.

A pointer within a few pixels of a dot snaps to it: the dot is highlighted, the
preview shows that marker's second, and a click or drag seeks exactly there,
because one pixel of a long video spans several seconds.

Pointing at the seek track shows the frame at that position above the cursor
with its timecode. Frames come from one cached sprite sheet per video, so
scrubbing costs a single request and no seeking; the sheet is requested on the
first hover and nothing is drawn until it decodes, which leaves plain seeking
for short videos that have no sheet. The preview stays inside the track at both
ends, portrait frames are scaled down to a common height, and a touch drag shows
the preview until the finger lifts.

Up and Down remain reserved for feed navigation. A fully visible video becomes
the active player even when autoplay is disabled; explicit interaction also
activates it immediately. Playback speed changes with Shift+Right and
Shift+Left. Its unmodified Left and Right, Space, and long-press shortcuts keep
their playback behavior.

## Audio

Audio cards use extracted tags, duration, and optional embedded cover art, with
a designed fallback when extraction is unavailable. Controls share the media
playback primitives and persist volume/progress in browser storage. Like video,
a fully visible audio card becomes the active keyboard player. When autoplay is
enabled, it starts playing on activation and pauses after leaving the fully
visible viewport.

## Generic Files And Images

Generic files render metadata and a download action rather than being forced
into a media renderer. Animated GIFs remain ordinary image cards. Other images
render through the drawing-board card so they can be annotated in expanded
mode.
