<script lang="ts">
  import { Pause, PictureInPicture2, Play, Volume2, VolumeX } from 'lucide-svelte';

  let {
    src,
    title
  }: {
    src: string;
    title: string;
  } = $props();

  let video = $state<HTMLVideoElement | undefined>(undefined);
  let paused = $state(true);
  let duration = $state(0);
  let currentTime = $state(0);
  let volume = $state(1);
  let muted = $state(false);
  let showControls = $state(false);
  let isDragging = $state(false);
  let hideTimer: ReturnType<typeof setTimeout> | undefined = undefined;

  const progress = $derived(duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0);

  function revealControls() {
    showControls = true;
    clearTimeout(hideTimer);
    if (!paused && !isDragging) {
      hideTimer = setTimeout(() => {
        showControls = false;
      }, 1800);
    }
  }

  function keepControls() {
    showControls = true;
    clearTimeout(hideTimer);
  }

  async function togglePlay() {
    if (!video) return;
    if (video.paused) {
      await video.play();
    } else {
      video.pause();
    }
    revealControls();
  }

  function handleSeek(event: Event) {
    if (!video) return;
    const target = event.target as HTMLInputElement;
    currentTime = Number(target.value);
    video.currentTime = currentTime;
  }

  function handleVolume(event: Event) {
    if (!video) return;
    const target = event.target as HTMLInputElement;
    volume = Number(target.value);
    video.volume = volume;
    muted = volume === 0;
    video.muted = muted;
  }

  function toggleMute() {
    if (!video) return;
    muted = !muted;
    video.muted = muted;
    if (!muted && volume === 0) {
      volume = 1;
      video.volume = volume;
    }
    revealControls();
  }

  async function togglePip() {
    if (!video || !document.pictureInPictureEnabled) return;
    if (document.pictureInPictureElement) {
      await document.exitPictureInPicture();
    } else {
      await video.requestPictureInPicture();
    }
    revealControls();
  }

  function formatTime(seconds: number) {
    if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
    const minutes = Math.floor(seconds / 60);
    const rest = Math.floor(seconds % 60);
    return `${minutes}:${String(rest).padStart(2, '0')}`;
  }
</script>

<div
  class="feed-video-player"
  role="group"
  aria-label={`Video player: ${title}`}
  onpointermove={revealControls}
  onpointerenter={revealControls}
  onfocusin={keepControls}
  onmouseleave={() => {
    if (!paused && !isDragging) showControls = false;
  }}
>
  <!-- svelte-ignore a11y_media_has_caption -->
  <video
    bind:this={video}
    class="h-full w-full bg-black object-contain"
    playsinline
    preload="metadata"
    src={src}
    {title}
    onclick={togglePlay}
    onloadedmetadata={() => {
      duration = video?.duration ?? 0;
      volume = video?.volume ?? 1;
      muted = video?.muted ?? false;
    }}
    ontimeupdate={() => {
      if (!isDragging) currentTime = video?.currentTime ?? 0;
    }}
    onplay={() => {
      paused = false;
      revealControls();
    }}
    onpause={() => {
      paused = true;
      showControls = true;
    }}
    onended={() => {
      paused = true;
      showControls = true;
    }}
  ></video>

  {#if paused}
    <button class="video-play-overlay" type="button" aria-label="Play video" onclick={togglePlay}>
      <Play size={34} fill="currentColor" />
    </button>
  {/if}

  <div
    class="video-controls"
    class:video-controls-visible={showControls || paused || isDragging}
    onpointerenter={keepControls}
    onpointerleave={revealControls}
    role="toolbar"
    aria-label="Video controls"
    tabindex="-1"
  >
    <button class="video-control-button" type="button" aria-label={paused ? 'Play' : 'Pause'} onclick={togglePlay}>
      {#if paused}
        <Play size={18} fill="currentColor" />
      {:else}
        <Pause size={18} fill="currentColor" />
      {/if}
    </button>

    <span class="video-time">{formatTime(currentTime)}</span>

    <div class="video-progress">
      <div class="video-progress-track">
        <div class="video-progress-fill" style={`width: ${progress}%`}></div>
      </div>
      <input
        aria-label="Seek video"
        type="range"
        min="0"
        max={duration || 0}
        step="0.1"
        value={currentTime}
        onpointerdown={() => (isDragging = true)}
        onpointerup={() => {
          isDragging = false;
          revealControls();
        }}
        oninput={handleSeek}
      />
    </div>

    <span class="video-time video-time-end">{formatTime(duration)}</span>

    <button class="video-control-button" type="button" aria-label={muted ? 'Unmute' : 'Mute'} onclick={toggleMute}>
      {#if muted || volume === 0}
        <VolumeX size={18} />
      {:else}
        <Volume2 size={18} />
      {/if}
    </button>

    <input
      class="video-volume"
      aria-label="Volume"
      type="range"
      min="0"
      max="1"
      step="0.05"
      value={muted ? 0 : volume}
      oninput={handleVolume}
    />

    <button class="video-control-button" type="button" aria-label="Picture in Picture" onclick={togglePip}>
      <PictureInPicture2 size={18} />
    </button>
  </div>
</div>
