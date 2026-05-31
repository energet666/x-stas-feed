<script lang="ts">
  import MediaPlaybackControls from '../MediaPlaybackControls.svelte';
  import { uiText as t } from '../../lib/ui_text';

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

<MediaPlaybackControls
  ariaLabel={t.playback.audioControls}
  seekLabel={t.playback.seekAudio}
  {paused}
  {currentTime}
  {duration}
  {muted}
  {volume}
  {progress}
  {supportsVolumeControl}
  bind:isDragging
  {onTogglePlay}
  {onSeek}
  {onVolumeChange}
  {onToggleMute}
  {onEnterControls}
  {onLeaveControls}
  {onFinishDragging}
/>
