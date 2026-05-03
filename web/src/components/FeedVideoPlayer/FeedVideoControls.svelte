<script lang="ts">
  import { Pause, PictureInPicture2, Play, Volume2, VolumeX } from 'lucide-svelte';
  import { formatVideoTime } from './utils';

  let {
    paused,
    currentTime,
    duration,
    muted,
    volume,
    progress,
    supportsVolumeControl,
    supportsPip,
    isDragging = $bindable(false),
    onTogglePlay,
    onSeek,
    onVolumeChange,
    onToggleMute,
    onTogglePip,
    onEnterControls,
    onLeaveControls,
    onFinishDragging
  }: {
    paused: boolean;
    currentTime: number;
    duration: number;
    muted: boolean;
    volume: number;
    progress: number;
    supportsVolumeControl: boolean;
    supportsPip: boolean;
    isDragging: boolean;
    onTogglePlay: () => void;
    onSeek: (event: Event) => void;
    onVolumeChange: (event: Event) => void;
    onToggleMute: () => void;
    onTogglePip: () => void;
    onEnterControls: () => void;
    onLeaveControls: () => void;
    onFinishDragging: () => void;
  } = $props();
</script>

<div
  class="video-controls flex items-center text-primary"
  onpointerenter={onEnterControls}
  onpointerleave={onLeaveControls}
  onmouseenter={onEnterControls}
  onmouseleave={onLeaveControls}
  role="toolbar"
  aria-label="Video controls"
  tabindex="-1"
>
  <button class="video-control-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={paused ? 'Play' : 'Pause'} onclick={onTogglePlay}>
    {#if paused}
      <Play size={18} fill="currentColor" />
    {:else}
      <Pause size={18} fill="currentColor" />
    {/if}
  </button>

  <span class="video-time shrink-0 font-bold">{formatVideoTime(currentTime)}</span>

  <div class="video-progress relative flex min-w-16 flex-1 items-center">
    <div class="video-progress-track w-full overflow-hidden rounded-full">
      <div class="video-progress-fill h-full" style={`width: ${progress}%`}></div>
    </div>
    <input
      aria-label="Seek video"
      type="range"
      min="0"
      max={duration || 0}
      step="0.1"
      value={currentTime}
      onpointerdown={() => (isDragging = true)}
      onpointerup={onFinishDragging}
      onmousedown={() => (isDragging = true)}
      onmouseup={onFinishDragging}
      onchange={onFinishDragging}
      oninput={onSeek}
    />
  </div>

  <span class="video-time video-time-end shrink-0 font-bold">{formatVideoTime(duration)}</span>

  <button class="video-control-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={muted ? 'Unmute' : 'Mute'} onclick={onToggleMute}>
    {#if muted || volume === 0}
      <VolumeX size={18} />
    {:else}
      <Volume2 size={18} />
    {/if}
  </button>

  {#if supportsVolumeControl}
    <input
      class="video-volume shrink-0 cursor-pointer rounded-full"
      aria-label="Volume"
      type="range"
      min="0"
      max="1"
      step="0.05"
      value={muted ? 0 : volume}
      oninput={onVolumeChange}
    />
  {/if}

  {#if supportsPip}
    <button class="video-control-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label="Picture in Picture" onclick={onTogglePip}>
      <PictureInPicture2 size={18} />
    </button>
  {/if}
</div>

<style>
  .video-controls {
    z-index: 5;
    gap: 0.6rem;
    min-height: 3.35rem;
    width: 100%;
    padding: 0.65rem 0.8rem;
    border: 1px solid var(--color-glass-border);
    border-radius: 24px;
    background: var(--background-image-glass-overlay);
    box-shadow: var(--shadow-video-controls);
    backdrop-filter: blur(30px) saturate(170%);
    -webkit-backdrop-filter: blur(30px) saturate(170%);
  }

  .video-control-button {
    color: var(--color-secondary);
    transition:
      background 140ms ease,
      color 140ms ease,
      transform 140ms ease;
  }

  .video-control-button:hover {
    background: var(--color-button-hover);
    color: var(--color-primary);
    transform: scale(1.04);
  }

  .video-time {
    width: 2.35rem;
    color: var(--color-muted);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
  }

  .video-time-end {
    text-align: right;
  }

  .video-progress {
    height: 1.5rem;
    flex: 1 1 auto;
  }

  .video-progress-track {
    height: 0.24rem;
    background: var(--color-track-bg);
  }

  .video-progress-fill {
    border-radius: inherit;
    background: var(--color-track-fill);
  }

  .video-progress input {
    position: absolute;
    inset: 0;
    width: 100%;
    cursor: pointer;
    opacity: 0;
  }

  .video-volume {
    width: 3.25rem;
    height: 0.25rem;
    appearance: none;
    background: var(--color-track-bg-strong);
  }

  .video-volume::-webkit-slider-thumb {
    width: 0.72rem;
    height: 0.72rem;
    appearance: none;
    border-radius: 999px;
    background: var(--color-primary);
    box-shadow: var(--shadow-thumb);
  }

  .video-volume::-moz-range-thumb {
    width: 0.72rem;
    height: 0.72rem;
    border: 0;
    border-radius: 999px;
    background: var(--color-primary);
    box-shadow: var(--shadow-thumb);
  }

  @media (max-width: 520px) {
    .video-controls {
      gap: 0.35rem;
      padding: 0.55rem 0.6rem;
    }

    .video-volume,
    .video-time-end {
      display: none;
    }
  }
</style>
