<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Pause, Play, Volume2, VolumeX } from 'lucide-svelte';
  import { formatVideoTime, type MarkerGroup } from './FeedVideoPlayer/utils';
  import { fetchMediaSprite, mediaSpriteURL, type MediaSprite } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    ariaLabel,
    seekLabel,
    paused,
    currentTime,
    duration,
    muted,
    volume,
    progress,
    markers = [],
    previewMediaId,
    supportsVolumeControl,
    isDragging = $bindable(false),
    extraControls,
    onTogglePlay,
    onSeek,
    onVolumeChange,
    onToggleMute,
    onEnterControls,
    onLeaveControls,
    onFinishDragging
  }: {
    ariaLabel: string;
    seekLabel: string;
    paused: boolean;
    currentTime: number;
    duration: number;
    muted: boolean;
    volume: number;
    progress: number;
    markers?: MarkerGroup[];
    previewMediaId?: string;
    supportsVolumeControl: boolean;
    isDragging: boolean;
    extraControls?: Snippet;
    onTogglePlay: () => void;
    onSeek: (event: Event) => void;
    onVolumeChange: (event: Event) => void;
    onToggleMute: () => void;
    onEnterControls: () => void;
    onLeaveControls: () => void;
    onFinishDragging: () => void;
  } = $props();

  // A portrait tile would tower over the controls, so the frame is scaled down
  // to a common height instead of being shown at its stored size.
  const previewMaxHeight = 108;

  let sprite = $state<MediaSprite | null>(null);
  let spriteURL = $state('');
  let spriteLoaded = $state(false);
  let spriteRequested = false;
  let previewTime = $state(-1);
  let previewOffset = $state(0);
  let trackWidth = $state(0);
  let hoveredMarkerTime = $state(-1);

  const previewScale = $derived(sprite ? Math.min(1, previewMaxHeight / sprite.tileHeight) : 1);
  const previewWidth = $derived(sprite ? sprite.tileWidth * previewScale : 0);
  const previewFrame = $derived.by(() => {
    if (!sprite || !spriteLoaded || previewTime < 0) return null;

    const index = Math.min(sprite.count - 1, Math.max(0, Math.floor(previewTime / sprite.intervalSeconds)));
    const column = index % sprite.columns;
    const row = Math.floor(index / sprite.columns);
    const height = sprite.tileHeight * previewScale;
    // The whole sheet is scaled as one background so a tile keeps its share of
    // the image no matter how far it is scaled down.
    return {
      width: previewWidth,
      height,
      sheetWidth: sprite.columns * previewWidth,
      sheetHeight: sprite.rows * height,
      offsetX: column * previewWidth,
      offsetY: row * height
    };
  });
  const previewLeft = $derived(
    trackWidth > 0 && previewWidth > 0
      ? Math.min(Math.max(previewOffset, previewWidth / 2), Math.max(trackWidth - previewWidth / 2, previewWidth / 2))
      : previewOffset
  );

  // Heavier dots read as moments that more viewers marked.
  function markerSize(count: number) {
    return (0.34 + Math.min(count - 1, 5) * 0.05).toFixed(2);
  }

  // A pixel of a long video spans several seconds, so a pointer near a dot
  // lands exactly on its second instead of wherever the cursor happens to be.
  const markerSnapPixels = 8;

  function trackPosition(event: PointerEvent) {
    const input = event.currentTarget;
    if (!(input instanceof HTMLInputElement) || duration <= 0) return null;
    const progressEl = input.closest('.media-playback-progress');
    if (!(progressEl instanceof HTMLElement)) return null;
    const bounds = progressEl.getBoundingClientRect();
    if (bounds.width <= 0) return null;

    const offset = Math.min(bounds.width, Math.max(0, event.clientX - bounds.left));
    let nearest: MarkerGroup | null = null;
    let nearestDistance = Infinity;
    for (const marker of markers) {
      const distance = Math.abs(Math.min(1, marker.timeSeconds / duration) * bounds.width - offset);
      if (distance < nearestDistance) {
        nearest = marker;
        nearestDistance = distance;
      }
    }

    if (nearest && nearestDistance <= markerSnapPixels) {
      const time = Math.min(nearest.timeSeconds, duration);
      return { width: bounds.width, offset: (time / duration) * bounds.width, time, marker: nearest };
    }
    return { width: bounds.width, offset, time: (offset / bounds.width) * duration, marker: null };
  }

  function loadSprite(mediaId: string) {
    if (spriteRequested) return;
    spriteRequested = true;

    fetchMediaSprite(mediaId)
      .then((sheet) => {
        if (!sheet) return;
        sprite = sheet;
        spriteURL = mediaSpriteURL(mediaId);
        // Nothing is shown until the sheet itself is decoded, so scrubbing
        // never flashes an empty box.
        const image = new Image();
        image.onload = () => (spriteLoaded = true);
        image.src = spriteURL;
      })
      .catch(() => undefined);
  }

  function updatePreview(event: PointerEvent) {
    const position = trackPosition(event);
    hoveredMarkerTime = position?.marker?.timeSeconds ?? -1;
    if (!previewMediaId || !position) {
      previewTime = -1;
      return;
    }

    trackWidth = position.width;
    previewOffset = position.offset;
    previewTime = position.time;
    loadSprite(previewMediaId);
  }

  function hidePreview() {
    previewTime = -1;
    hoveredMarkerTime = -1;
  }

  // A recycled player points at another media, so the cached sheet goes with it.
  $effect(() => {
    previewMediaId;
    sprite = null;
    spriteURL = '';
    spriteLoaded = false;
    spriteRequested = false;
    previewTime = -1;
  });

  function eventIsInsideCurrentTarget(event: PointerEvent | MouseEvent) {
    const target = event.currentTarget;
    if (!(target instanceof HTMLElement)) return false;
    const bounds = target.getBoundingClientRect();
    return (
      event.clientX >= bounds.left &&
      event.clientX <= bounds.right &&
      event.clientY >= bounds.top &&
      event.clientY <= bounds.bottom
    );
  }

  function startRangeInteraction() {
    isDragging = true;
    onEnterControls();
  }

  function applySeekPointerValue(event: PointerEvent) {
    const input = event.currentTarget;
    const position = trackPosition(event);
    if (!(input instanceof HTMLInputElement) || !position) return;
    input.value = String(position.time);
    onSeek(event);
  }

  function startSeekInteraction(event: PointerEvent) {
    const input = event.currentTarget;
    if (!(input instanceof HTMLInputElement)) return;
    startRangeInteraction();
    event.preventDefault();
    input.setPointerCapture(event.pointerId);
    updatePreview(event);
    applySeekPointerValue(event);
  }

  function updateSeekInteraction(event: PointerEvent) {
    updatePreview(event);
    if (!isDragging) return;
    event.preventDefault();
    applySeekPointerValue(event);
  }

  function finishSeekInteraction(event: PointerEvent) {
    // A touch leaves no cursor behind, so its preview goes away with the lift.
    if (event.pointerType !== 'mouse') hidePreview();
    finishRangeInteraction();
  }

  function finishRangeInteraction() {
    onFinishDragging();
    onEnterControls();
  }

  function leaveControls(event: PointerEvent | MouseEvent) {
    if (eventIsInsideCurrentTarget(event)) {
      onEnterControls();
      return;
    }
    onLeaveControls();
  }
</script>

<div
  class="media-playback-controls flex items-center text-fg-primary"
  onpointerenter={onEnterControls}
  onpointerleave={leaveControls}
  onmouseenter={onEnterControls}
  onmouseleave={leaveControls}
  role="toolbar"
  aria-label={ariaLabel}
  tabindex="-1"
>
  <button class="media-playback-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={paused ? t.playback.play : t.playback.pause} onclick={onTogglePlay}>
    {#if paused}
      <Play size={18} fill="currentColor" />
    {:else}
      <Pause size={18} fill="currentColor" />
    {/if}
  </button>

  <span class="media-playback-time shrink-0 font-bold">{formatVideoTime(currentTime)}</span>

  <div class="media-playback-progress relative flex min-w-16 flex-1 items-center">
    {#if previewFrame}
      <div
        class="media-playback-preview pointer-events-none absolute flex flex-col items-center gap-1"
        style={`left: ${previewLeft}px; --preview-width: ${previewFrame.width}px`}
        aria-hidden="true"
      >
        <span
          class="media-playback-preview-frame block overflow-hidden"
          style={`width: ${previewFrame.width}px; height: ${previewFrame.height}px; background-image: url(${spriteURL}); background-size: ${previewFrame.sheetWidth}px ${previewFrame.sheetHeight}px; background-position: -${previewFrame.offsetX}px -${previewFrame.offsetY}px`}
        ></span>
        <span class="media-playback-preview-time">{formatVideoTime(previewTime)}</span>
      </div>
    {/if}
    <div class="media-playback-progress-track w-full overflow-hidden rounded-full">
      <div class="media-playback-progress-fill h-full" style={`width: ${progress}%`}></div>
    </div>
    {#if duration > 0 && markers.length > 0}
      <div class="media-playback-markers pointer-events-none absolute inset-0" aria-hidden="true">
        {#each markers as marker (marker.timeSeconds)}
          <span
            class="media-playback-marker"
            class:media-playback-marker-hovered={marker.timeSeconds === hoveredMarkerTime}
            style={`left: ${Math.min(100, (marker.timeSeconds / duration) * 100)}%; --marker-size: ${markerSize(marker.count)}rem`}
          ></span>
        {/each}
      </div>
    {/if}
    <input
      aria-label={seekLabel}
      type="range"
      min="0"
      max={duration || 0}
      step="0.1"
      value={currentTime}
      onpointerdown={startSeekInteraction}
      onpointermove={updateSeekInteraction}
      onpointerup={finishSeekInteraction}
      onpointerleave={hidePreview}
      onpointercancel={hidePreview}
      onmousedown={startRangeInteraction}
      onmouseup={finishRangeInteraction}
      onchange={finishRangeInteraction}
      oninput={onSeek}
    />
  </div>

  <span class="media-playback-time media-playback-time-end shrink-0 font-bold">{formatVideoTime(duration)}</span>

  <button class="media-playback-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={muted ? t.playback.unmute : t.playback.mute} onclick={onToggleMute}>
    {#if muted || volume === 0}
      <VolumeX size={18} />
    {:else}
      <Volume2 size={18} />
    {/if}
  </button>

  {#if supportsVolumeControl}
    <input
      class="media-playback-volume shrink-0 cursor-pointer rounded-full"
      aria-label={t.playback.volume}
      type="range"
      min="0"
      max="1"
      step="0.05"
      value={muted ? 0 : volume}
      onpointerdown={onEnterControls}
      onpointerup={onEnterControls}
      onmousedown={onEnterControls}
      onmouseup={onEnterControls}
      oninput={onVolumeChange}
    />
  {/if}

  {#if extraControls}
    {@render extraControls()}
  {/if}
</div>

<style>
  .media-playback-controls {
    z-index: 5;
    gap: 0.6rem;
    min-height: 2.5rem;
    width: 100%;
  }

  .media-playback-button,
  :global(.media-playback-button) {
    color: var(--color-fg-secondary);
    transition:
      background 140ms ease,
      color 140ms ease,
      transform 140ms ease;
  }

  .media-playback-button:hover,
  :global(.media-playback-button:hover) {
    background: var(--color-action-hover);
    color: var(--color-fg-primary);
    transform: scale(1.04);
  }

  .media-playback-time {
    width: 2.35rem;
    color: var(--color-fg-muted);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
  }

  .media-playback-time-end {
    text-align: right;
  }

  .media-playback-progress {
    height: 1.5rem;
    flex: 1 1 auto;
  }

  .media-playback-progress-track {
    height: 0.24rem;
    background: var(--color-track-bg);
  }

  .media-playback-progress-fill {
    border-radius: inherit;
    background: var(--color-track-fill);
  }

  .media-playback-preview {
    bottom: calc(100% + 0.15rem);
    z-index: 6;
    transform: translateX(-50%);
  }

  .media-playback-preview-frame {
    border-radius: 0.6rem;
    background-color: var(--color-surface-media);
    background-repeat: no-repeat;
    box-shadow:
      0 0 0 1px var(--color-border-glass),
      var(--shadow-thumb);
  }

  .media-playback-preview-time {
    color: var(--color-fg-primary);
    font-size: 0.68rem;
    font-variant-numeric: tabular-nums;
    font-weight: 700;
    text-shadow: 0 1px 3px rgb(0 0 0 / 0.65);
  }

  .media-playback-marker {
    position: absolute;
    top: 50%;
    width: var(--marker-size, 0.34rem);
    height: var(--marker-size, 0.34rem);
    transform: translate(-50%, -50%);
    border-radius: 999px;
    background: var(--color-track-fill);
    box-shadow: 0 0 0 1px rgb(0 0 0 / 0.45);
    transition:
      width 140ms ease,
      height 140ms ease,
      box-shadow 140ms ease;
  }

  .media-playback-marker-hovered {
    width: calc(var(--marker-size, 0.34rem) + 0.22rem);
    height: calc(var(--marker-size, 0.34rem) + 0.22rem);
    box-shadow:
      0 0 0 1px rgb(0 0 0 / 0.45),
      0 0 0 3px var(--color-action-hover);
  }

  .media-playback-progress input {
    position: absolute;
    inset: 0;
    width: 100%;
    cursor: pointer;
    opacity: 0;
  }

  .media-playback-volume {
    width: 3.25rem;
    height: 0.25rem;
    appearance: none;
    background: var(--color-track-bg-strong);
  }

  .media-playback-volume::-webkit-slider-thumb {
    width: 0.72rem;
    height: 0.72rem;
    appearance: none;
    border-radius: 999px;
    background: var(--color-fg-primary);
    box-shadow: var(--shadow-thumb);
  }

  .media-playback-volume::-moz-range-thumb {
    width: 0.72rem;
    height: 0.72rem;
    border: 0;
    border-radius: 999px;
    background: var(--color-fg-primary);
    box-shadow: var(--shadow-thumb);
  }

  @media (max-width: 520px) {
    .media-playback-controls {
      gap: 0.35rem;
    }

    .media-playback-volume,
    .media-playback-time-end {
      display: none;
    }
  }
</style>
