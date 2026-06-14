---
title: Media Cards And Players
type: reference
status: active
---

# Media Cards And Players

`FeedCardFrame` owns card geometry, ambient composition, expanded mode, and top
and bottom overlay stacks. The lower social chin sits outside the media frame so
likes and comments do not obscure content. Expanded cards use a viewport-fixed
wrapper and raise comments above the media through a top-level overlay.

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
control layout.

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
