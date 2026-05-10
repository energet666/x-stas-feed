<script lang="ts">
  import { Pause, Play, Volume2, VolumeX } from 'lucide-svelte';
  import { formatVideoTime } from '../FeedVideoPlayer/utils';

  let {
    paused,
    currentTime,
    duration,
    muted,
    volume,
    progress,
    supportsVolumeControl,
    isDragging = $bindable(false),
    onTogglePlay,
    onSeek,
    onVolumeChange,
    onToggleMute,
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
    isDragging: boolean;
    onTogglePlay: () => void;
    onSeek: (event: Event) => void;
    onVolumeChange: (event: Event) => void;
    onToggleMute: () => void;
    onEnterControls: () => void;
    onLeaveControls: () => void;
    onFinishDragging: () => void;
  } = $props();
</script>

<div
  class="audio-controls flex items-center text-primary"
  onpointerenter={onEnterControls}
  onpointerleave={onLeaveControls}
  onmouseenter={onEnterControls}
  onmouseleave={onLeaveControls}
  role="toolbar"
  aria-label="Audio controls"
  tabindex="-1"
>
  <button class="audio-control-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={paused ? 'Play' : 'Pause'} onclick={onTogglePlay}>
    {#if paused}
      <Play size={18} fill="currentColor" />
    {:else}
      <Pause size={18} fill="currentColor" />
    {/if}
  </button>

  <span class="audio-time shrink-0 font-bold">{formatVideoTime(currentTime)}</span>

  <div class="audio-progress relative flex min-w-16 flex-1 items-center">
    <div class="audio-progress-track w-full overflow-hidden rounded-full">
      <div class="audio-progress-fill h-full" style={`width: ${progress}%`}></div>
    </div>
    <input
      aria-label="Seek audio"
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

  <span class="audio-time audio-time-end shrink-0 font-bold">{formatVideoTime(duration)}</span>

  <button class="audio-control-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={muted ? 'Unmute' : 'Mute'} onclick={onToggleMute}>
    {#if muted || volume === 0}
      <VolumeX size={18} />
    {:else}
      <Volume2 size={18} />
    {/if}
  </button>

  {#if supportsVolumeControl}
    <input
      class="audio-volume shrink-0 cursor-pointer rounded-full"
      aria-label="Volume"
      type="range"
      min="0"
      max="1"
      step="0.05"
      value={muted ? 0 : volume}
      oninput={onVolumeChange}
    />
  {/if}
</div>

<style>
  .audio-controls {
    z-index: 5;
    gap: 0.6rem;
    min-height: 2.5rem;
    width: 100%;
  }

  .audio-control-button {
    color: var(--color-secondary);
    transition:
      background 140ms ease,
      color 140ms ease,
      transform 140ms ease;
  }

  .audio-control-button:hover {
    background: var(--color-button-hover);
    color: var(--color-primary);
    transform: scale(1.04);
  }

  .audio-time {
    width: 2.35rem;
    color: var(--color-muted);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
  }

  .audio-time-end {
    text-align: right;
  }

  .audio-progress {
    height: 1.5rem;
    flex: 1 1 auto;
  }

  .audio-progress-track {
    height: 0.24rem;
    background: var(--color-track-bg);
  }

  .audio-progress-fill {
    border-radius: inherit;
    background: var(--color-track-fill);
  }

  .audio-progress input {
    position: absolute;
    inset: 0;
    width: 100%;
    cursor: pointer;
    opacity: 0;
  }

  .audio-volume {
    width: 3.25rem;
    height: 0.25rem;
    appearance: none;
    background: var(--color-track-bg-strong);
  }

  .audio-volume::-webkit-slider-thumb {
    width: 0.72rem;
    height: 0.72rem;
    appearance: none;
    border-radius: 999px;
    background: var(--color-primary);
    box-shadow: var(--shadow-thumb);
  }

  .audio-volume::-moz-range-thumb {
    width: 0.72rem;
    height: 0.72rem;
    border: 0;
    border-radius: 999px;
    background: var(--color-primary);
    box-shadow: var(--shadow-thumb);
  }

  @media (max-width: 520px) {
    .audio-controls {
      gap: 0.35rem;
    }

    .audio-volume,
    .audio-time-end {
      display: none;
    }
  }
</style>
