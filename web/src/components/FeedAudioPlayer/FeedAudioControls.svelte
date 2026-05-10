<script lang="ts">
  import MediaPlaybackControls from '../MediaPlaybackControls.svelte';

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
  ariaLabel="Audio controls"
  seekLabel="Seek audio"
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
