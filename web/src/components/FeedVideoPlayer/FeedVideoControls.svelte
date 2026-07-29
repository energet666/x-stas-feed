<script lang="ts">
  import { PictureInPicture2, Sparkles } from 'lucide-svelte';
  import MediaPlaybackControls from '../MediaPlaybackControls.svelte';
  import type { MarkerGroup } from './utils';
  import { uiText as t } from '../../lib/ui_text';

  let {
    paused,
    currentTime,
    duration,
    muted,
    volume,
    progress,
    markers = [],
    supportsVolumeControl,
    supportsPip,
    markerPending = false,
    isDragging = $bindable(false),
    onTogglePlay,
    onSeek,
    onVolumeChange,
    onToggleMute,
    onTogglePip,
    onAddMarker,
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
    markers?: MarkerGroup[];
    supportsVolumeControl: boolean;
    supportsPip: boolean;
    markerPending?: boolean;
    isDragging: boolean;
    onTogglePlay: () => void;
    onSeek: (event: Event) => void;
    onVolumeChange: (event: Event) => void;
    onToggleMute: () => void;
    onTogglePip: () => void;
    onAddMarker: () => void;
    onEnterControls: () => void;
    onLeaveControls: () => void;
    onFinishDragging: () => void;
  } = $props();
</script>

<MediaPlaybackControls
  ariaLabel={t.playback.videoControls}
  seekLabel={t.playback.seekVideo}
  {paused}
  {currentTime}
  {duration}
  {muted}
  {volume}
  {progress}
  {markers}
  {supportsVolumeControl}
  bind:isDragging
  {onTogglePlay}
  {onSeek}
  {onVolumeChange}
  {onToggleMute}
  {onEnterControls}
  {onLeaveControls}
  {onFinishDragging}
>
  {#snippet extraControls()}
    <button
      class="media-playback-button grid size-8 shrink-0 place-items-center rounded-full"
      type="button"
      aria-label={t.playback.addMarker}
      title={t.playback.addMarker}
      disabled={markerPending}
      onclick={onAddMarker}
    >
      <Sparkles size={18} />
    </button>

    {#if supportsPip}
      <button class="media-playback-button grid size-8 shrink-0 place-items-center rounded-full" type="button" aria-label={t.playback.pip} onclick={onTogglePip}>
        <PictureInPicture2 size={18} />
      </button>
    {/if}
  {/snippet}
</MediaPlaybackControls>
