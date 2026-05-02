<script module lang="ts">
  let nextPlayerId = 0;
  let activePlayerId: number | undefined = undefined;
</script>

<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Pause, PictureInPicture2, Play, Volume2, VolumeX } from 'lucide-svelte';
  import {
    AVAILABLE_SPEEDS,
    FEED_VIDEO_PLAY_EVENT,
    FEED_VIDEO_VOLUME_EVENT,
    LONG_PRESS_DELAY_MS,
    SEEK_FEEDBACK_ACCUMULATION_MS,
    TOUCHPAD_SEEK_SENSITIVITY,
    canSetVolume,
    clampTime,
    clampVolume,
    clearStoredProgress,
    formatVideoTime,
    isEditableTarget,
    readStoredProgress,
    readStoredVolume,
    saveStoredProgress,
    saveStoredVolume,
    supportsPictureInPicture,
    type SafariVideoElement
  } from './lib/videoPlayer';

  let {
    mediaId,
    src,
    title
  }: {
    mediaId: string;
    src: string;
    title: string;
  } = $props();

  let video = $state<HTMLVideoElement | undefined>(undefined);
  let container = $state<HTMLDivElement | undefined>(undefined);
  let paused = $state(true);
  let duration = $state(0);
  let currentTime = $state(0);
  let volume = $state(1);
  let muted = $state(false);
  let userPlaybackRate = $state(1);
  let showControls = $state(false);
  let showCursor = $state(true);
  let isDragging = $state(false);
  let isOverControls = $state(false);
  let playBlocked = $state(false);
  let supportsVolumeControl = $state(true);
  let showSpeedIndicator = $state(false);
  let seekFeedbackSide = $state<'left' | 'right' | null>(null);
  let seekFeedbackAmount = $state(10);
  let seekFeedbackTick = $state(0);
  let hideTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let speedTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let clickTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let seekFeedbackTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let spaceTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let arrowTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let rewindTimer: ReturnType<typeof setInterval> | undefined = undefined;
  let isSpaceDown = false;
  let isSpaceLongPress = false;
  let isArrowDown = false;
  let isArrowLongPress = false;
  let arrowRightTemporarilyPlayed = false;
  let lastSeekFeedbackAt = 0;
  let previewFrameRequested = false;
  let progressRestored = false;
  let hasProgressInteraction = false;
  let lastProgressSaveAt = 0;
  const playerId = nextPlayerId++;

  const progress = $derived(duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0);
  const supportsPip = $derived(supportsPictureInPicture(video));

  function revealControls() {
    setActivePlayer();
    showControls = true;
    showCursor = true;
    scheduleControlsHide();
  }

  function scheduleControlsHide() {
    clearTimeout(hideTimer);
    if (!isDragging && !isOverControls) {
      hideTimer = setTimeout(() => {
        if (isDragging || isOverControls) return;
        showControls = false;
        showCursor = false;
      }, 1800);
    }
  }

  function keepControls() {
    setActivePlayer();
    showControls = true;
    showCursor = true;
    clearTimeout(hideTimer);
  }

  function hideControls() {
    if (!isDragging && !isOverControls) {
      showControls = false;
      showCursor = true;
    }
  }

  function enterControls() {
    isOverControls = true;
    keepControls();
  }

  function leaveControls() {
    isOverControls = false;
    revealControls();
  }

  function announcePlayback() {
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_PLAY_EVENT, { detail: { playerId } }));
  }

  function pauseForOtherPlayer(event: Event) {
    const otherPlayerId = (event as CustomEvent<{ playerId: number }>).detail?.playerId;
    if (otherPlayerId === playerId || !video || video.paused) return;
    video.pause();
  }

  function announceVolumeChange() {
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_VOLUME_EVENT, { detail: { playerId, volume, muted } }));
  }

  function applyVolume(nextVolume: number, nextMuted: boolean, announce = false) {
    if (!video) return;
    volume = clampVolume(nextVolume);
    muted = nextMuted;
    if (supportsVolumeControl) {
      video.volume = volume;
    }
    video.muted = muted;
    saveStoredVolume(volume, muted);
    if (announce) announceVolumeChange();
  }

  function applyStoredVolume() {
    const stored = readStoredVolume();
    applyVolume(stored.volume, stored.muted);
  }

  function syncVolumeFromOtherPlayer(event: Event) {
    const detail = (event as CustomEvent<{ playerId: number; volume: number; muted: boolean }>).detail;
    if (!detail || detail.playerId === playerId || !video) return;
    applyVolume(detail.volume, detail.muted);
  }

  async function togglePlay() {
    if (!video) return;
    setActivePlayer();
    markProgressInteraction();
    playBlocked = false;

    try {
      if (video.paused) {
        await video.play();
      } else {
        video.pause();
      }
    } catch {
      playBlocked = true;
      showControls = true;
      return;
    }

    revealControls();
  }

  function syncMetadata() {
    if (!video) return;
    duration = Number.isFinite(video.duration) ? video.duration : 0;
    video.playbackRate = userPlaybackRate;
    supportsVolumeControl = canSetVolume(video);
    applyStoredVolume();
    restoreProgress();
    requestPreviewFrame();
  }

  function requestPreviewFrame() {
    if (!video || previewFrameRequested || !video.paused || duration <= 0 || video.currentTime > 0) return;

    previewFrameRequested = true;
    try {
      video.currentTime = Math.min(0.001, duration);
    } catch {
      previewFrameRequested = false;
    }
  }

  function handleVideoClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (target.closest('.video-controls')) return;
    setActivePlayer();
    clearTimeout(clickTimer);

    if (event.detail === 1) {
      clickTimer = setTimeout(() => {
        void togglePlay();
      }, 220);
      return;
    }

    if (event.detail % 2 === 0) {
      seekBySide(event);
    }
  }

  function finishDragging() {
    isDragging = false;
    revealControls();
  }

  function handleContainerTouch() {
    revealControls();
  }

  function handleSeek(event: Event) {
    if (!video) return;
    const target = event.target as HTMLInputElement;
    markProgressInteraction();
    currentTime = Number(target.value);
    video.currentTime = currentTime;
    saveProgress();
  }

  function handleVolume(event: Event) {
    if (!video || !supportsVolumeControl) return;
    const target = event.target as HTMLInputElement;
    const nextVolume = Number(target.value);
    applyVolume(nextVolume, nextVolume === 0, true);
  }

  function toggleMute() {
    if (!video) return;
    const nextMuted = !muted;
    const nextVolume = !nextMuted && volume === 0 ? 1 : volume;
    applyVolume(nextVolume, nextMuted, true);
    revealControls();
  }

  async function togglePip() {
    if (!video) return;
    const safariVideo = video as SafariVideoElement;

    try {
      if (document.pictureInPictureEnabled && video.requestPictureInPicture) {
        if (document.pictureInPictureElement) {
          await document.exitPictureInPicture();
        } else {
          await video.requestPictureInPicture();
        }
      } else if (safariVideo.webkitSupportsPresentationMode?.('picture-in-picture')) {
        const nextMode = safariVideo.webkitPresentationMode === 'picture-in-picture' ? 'inline' : 'picture-in-picture';
        safariVideo.webkitSetPresentationMode?.(nextMode);
      }
    } catch {
      showControls = true;
    }

    revealControls();
  }

  function setActivePlayer() {
    activePlayerId = playerId;
  }

  function isActivePlayer() {
    return activePlayerId === playerId;
  }

  function safePlay() {
    if (!video) return;
    markProgressInteraction();
    playBlocked = false;
    video.play().catch(() => {
      playBlocked = true;
      showControls = true;
    });
  }

  function clampPlaybackTime(nextTime: number) {
    const maxTime = duration || video?.duration || 0;
    return clampTime(nextTime, maxTime);
  }

  function seekBy(seconds: number) {
    if (!video) return;
    markProgressInteraction();
    video.currentTime = clampPlaybackTime(video.currentTime + seconds);
    currentTime = video.currentTime;
    saveProgress();
    revealControls();
  }

  function seekBySide(event: MouseEvent) {
    if (!video || !container) return;

    const now = Date.now();
    const rect = container.getBoundingClientRect();
    const side = event.clientX < rect.left + rect.width / 2 ? 'left' : 'right';
    const delta = side === 'left' ? -10 : 10;
    const shouldAccumulate = seekFeedbackSide === side && now - lastSeekFeedbackAt <= SEEK_FEEDBACK_ACCUMULATION_MS;

    seekBy(delta);
    seekFeedbackSide = side;
    seekFeedbackAmount = shouldAccumulate ? seekFeedbackAmount + 10 : 10;
    seekFeedbackTick += 1;
    lastSeekFeedbackAt = now;
    clearTimeout(seekFeedbackTimer);
    seekFeedbackTimer = setTimeout(() => {
      seekFeedbackSide = null;
      seekFeedbackAmount = 10;
    }, 420);
  }

  function showSpeed() {
    showSpeedIndicator = true;
    revealControls();
    clearTimeout(speedTimer);
    speedTimer = setTimeout(() => {
      showSpeedIndicator = false;
    }, 650);
  }

  function setPlaybackRate(rate: number) {
    userPlaybackRate = rate;
    if (video) video.playbackRate = rate;
    showSpeed();
  }

  function changePlaybackRate(direction: 1 | -1) {
    const index = Math.max(0, AVAILABLE_SPEEDS.indexOf(userPlaybackRate));
    const nextIndex = Math.max(0, Math.min(AVAILABLE_SPEEDS.length - 1, index + direction));
    if (nextIndex !== index) {
      setPlaybackRate(AVAILABLE_SPEEDS[nextIndex]);
    }
  }

  function handleKeyboard(event: KeyboardEvent, phase: 'down' | 'up') {
    if (!isActivePlayer() || isEditableTarget(event.target)) return;

    if (phase === 'down') {
      handleKeyDown(event);
    } else {
      handleKeyUp(event);
    }
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.code === 'Space') {
      event.preventDefault();
      if (isSpaceDown) return;
      isSpaceDown = true;
      isSpaceLongPress = false;

      spaceTimer = setTimeout(() => {
        if (!video) return;
        isSpaceLongPress = true;
        video.playbackRate = 2;
        if (video.paused) safePlay();
      }, LONG_PRESS_DELAY_MS);
      return;
    }

    if (event.code === 'ArrowUp' || event.code === 'ArrowDown') {
      event.preventDefault();
      changePlaybackRate(event.code === 'ArrowUp' ? 1 : -1);
      return;
    }

    if (event.code === 'ArrowLeft' || event.code === 'ArrowRight') {
      event.preventDefault();
      if (isArrowDown) return;
      isArrowDown = true;
      isArrowLongPress = false;
      arrowRightTemporarilyPlayed = false;
      const isRight = event.code === 'ArrowRight';

      arrowTimer = setTimeout(() => {
        if (!video) return;
        isArrowLongPress = true;
        revealControls();

        if (isRight) {
          video.playbackRate = 16;
          if (video.paused) {
            arrowRightTemporarilyPlayed = true;
            safePlay();
          }
        } else {
          const rewind = () => seekBy(-3);
          rewind();
          rewindTimer = setInterval(rewind, 300);
        }
      }, LONG_PRESS_DELAY_MS);
    }
  }

  function handleKeyUp(event: KeyboardEvent) {
    if (event.code === 'Space') {
      event.preventDefault();
      isSpaceDown = false;
      clearTimeout(spaceTimer);

      if (isSpaceLongPress) {
        if (video) video.playbackRate = userPlaybackRate;
      } else {
        void togglePlay();
      }
      return;
    }

    if (event.code === 'ArrowLeft' || event.code === 'ArrowRight') {
      event.preventDefault();
      if (!isArrowDown) return;

      isArrowDown = false;
      clearTimeout(arrowTimer);
      clearInterval(rewindTimer);

      if (!isArrowLongPress) {
        seekBy(event.code === 'ArrowRight' ? 1 : -1);
      } else if (video) {
        video.playbackRate = userPlaybackRate;
        if (arrowRightTemporarilyPlayed) video.pause();
      }

      arrowRightTemporarilyPlayed = false;
      revealControls();
    }
  }

  function handleWheel(event: WheelEvent) {
    if (Math.abs(event.deltaX) < Math.abs(event.deltaY)) return;
    event.preventDefault();
    setActivePlayer();
    seekBy(-event.deltaX * TOUCHPAD_SEEK_SENSITIVITY);
  }

  function restoreProgress() {
    if (!video || progressRestored || duration <= 0) return;
    progressRestored = true;

    const storedTime = readStoredProgress(mediaId);
    if (storedTime <= 0.5) return;
    if (storedTime >= duration - 1) {
      clearStoredProgress(mediaId);
      return;
    }

    video.currentTime = storedTime;
    currentTime = storedTime;
  }

  function markProgressInteraction() {
    hasProgressInteraction = true;
  }

  function saveProgressThrottled() {
    const now = Date.now();
    if (now - lastProgressSaveAt < 1000) return;
    lastProgressSaveAt = now;
    saveProgress();
  }

  function saveProgress() {
    if (!hasProgressInteraction || !video || duration <= 0) return;
    const time = video.currentTime;

    if (!Number.isFinite(time) || time <= 0.5 || time >= duration - 1) {
      clearStoredProgress(mediaId);
      return;
    }

    saveStoredProgress(mediaId, time, duration);
  }

  $effect(() => {
    if (!container) return;

    const handler = (event: WheelEvent) => handleWheel(event);
    container.addEventListener('wheel', handler, { passive: false });

    return () => {
      container?.removeEventListener('wheel', handler);
    };
  });

  $effect(() => {
    window.addEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
    window.addEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);

    return () => {
      window.removeEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
      window.removeEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);
    };
  });

  onDestroy(() => {
    saveProgress();
    clearTimeout(hideTimer);
    clearTimeout(speedTimer);
    clearTimeout(clickTimer);
    clearTimeout(seekFeedbackTimer);
    clearTimeout(spaceTimer);
    clearTimeout(arrowTimer);
    clearInterval(rewindTimer);
    if (activePlayerId === playerId) activePlayerId = undefined;
  });
</script>

<svelte:window
  onkeydown={(event) => handleKeyboard(event, 'down')}
  onkeyup={(event) => handleKeyboard(event, 'up')}
/>

<div
  bind:this={container}
  class="feed-video-player"
  class:video-cursor-hidden={!showCursor && !isOverControls && !isDragging}
  role="presentation"
  aria-label={`Video player: ${title}`}
  onpointermove={revealControls}
  onpointerenter={revealControls}
  onmousemove={revealControls}
  onmouseenter={revealControls}
  ontouchstart={handleContainerTouch}
  onfocusin={keepControls}
  onmouseleave={hideControls}
  onclick={setActivePlayer}
>
  <!-- svelte-ignore a11y_media_has_caption -->
  <video
    bind:this={video}
    class="h-full w-full bg-black object-contain"
    playsinline
    preload="metadata"
    src={src}
    onclick={handleVideoClick}
    onloadedmetadata={syncMetadata}
    ondurationchange={syncMetadata}
    ontimeupdate={() => {
      if (!isDragging) currentTime = video?.currentTime ?? 0;
      saveProgressThrottled();
    }}
    onplay={() => {
      markProgressInteraction();
      paused = false;
      announcePlayback();
      revealControls();
    }}
    onpause={() => {
      paused = true;
      saveProgress();
      revealControls();
    }}
    onended={() => {
      paused = true;
      clearStoredProgress(mediaId);
      revealControls();
    }}
  ></video>

  {#if seekFeedbackSide}
    {#key `${seekFeedbackSide}-${seekFeedbackTick}`}
      <div
        class="video-seek-feedback"
        class:video-seek-feedback-left={seekFeedbackSide === 'left'}
        class:video-seek-feedback-right={seekFeedbackSide === 'right'}
      >
        <span>{seekFeedbackSide === 'left' ? `-${seekFeedbackAmount}s` : `+${seekFeedbackAmount}s`}</span>
      </div>
    {/key}
  {/if}

  {#if showSpeedIndicator}
    <div class="video-speed-indicator">{userPlaybackRate}x</div>
  {/if}

  {#if paused}
    <button class="video-play-overlay" type="button" aria-label="Play video" onclick={togglePlay}>
      <Play size={26} fill="currentColor" />
    </button>
  {/if}

  {#if playBlocked}
    <div class="video-play-message">Tap play to start video</div>
  {/if}

  <div
    class="video-controls"
    class:video-controls-visible={showControls || isDragging}
    onpointerenter={enterControls}
    onpointerleave={leaveControls}
    onmouseenter={enterControls}
    onmouseleave={leaveControls}
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

    <span class="video-time">{formatVideoTime(currentTime)}</span>

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
        onpointerup={finishDragging}
        onmousedown={() => (isDragging = true)}
        onmouseup={finishDragging}
        onchange={finishDragging}
        oninput={handleSeek}
      />
    </div>

    <span class="video-time video-time-end">{formatVideoTime(duration)}</span>

    <button class="video-control-button" type="button" aria-label={muted ? 'Unmute' : 'Mute'} onclick={toggleMute}>
      {#if muted || volume === 0}
        <VolumeX size={18} />
      {:else}
        <Volume2 size={18} />
      {/if}
    </button>

    {#if supportsVolumeControl}
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
    {/if}

    {#if supportsPip}
      <button class="video-control-button" type="button" aria-label="Picture in Picture" onclick={togglePip}>
        <PictureInPicture2 size={18} />
      </button>
    {/if}
  </div>
</div>

<style>
  .feed-video-player {
    position: relative;
    z-index: 1;
    height: 100%;
    width: 100%;
    overflow: hidden;
    background: black;
  }

  .video-cursor-hidden,
  .video-cursor-hidden video {
    cursor: none;
  }

  .feed-video-player video {
    display: block;
    height: 100%;
    width: 100%;
    object-fit: contain;
  }

  .video-speed-indicator {
    position: absolute;
    top: 4.8rem;
    right: 1rem;
    z-index: 6;
    min-width: 3.1rem;
    padding: 0.45rem 0.7rem;
    border: 1px solid rgb(255 255 255 / 0.2);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 12px 32px rgb(0 0 0 / 0.24);
    color: rgb(255 255 255 / 0.9);
    font-size: 0.8rem;
    font-weight: 800;
    text-align: center;
    backdrop-filter: blur(24px) saturate(170%);
    -webkit-backdrop-filter: blur(24px) saturate(170%);
  }

  .video-seek-feedback {
    position: absolute;
    inset-block: 0;
    z-index: 6;
    display: flex;
    width: 50%;
    align-items: center;
    pointer-events: none;
  }

  .video-seek-feedback-left {
    left: 0;
    justify-content: flex-start;
  }

  .video-seek-feedback-right {
    right: 0;
    justify-content: flex-end;
  }

  .video-seek-feedback span {
    margin: 0 2rem;
    min-width: 4.1rem;
    padding: 0.7rem 0.9rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.56), rgb(10 10 14 / 0.36)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 14px 36px rgb(0 0 0 / 0.26);
    color: rgb(255 255 255 / 0.92);
    font-size: 0.92rem;
    font-weight: 800;
    text-align: center;
    animation: video-seek-feedback-pop 420ms ease-out both;
    backdrop-filter: blur(22px) saturate(170%);
    -webkit-backdrop-filter: blur(22px) saturate(170%);
  }

  @keyframes video-seek-feedback-pop {
    0% {
      opacity: 0;
      transform: scale(0.88);
    }

    22% {
      opacity: 1;
      transform: scale(1);
    }

    100% {
      opacity: 0;
      transform: scale(1.04);
    }
  }

  .video-play-overlay {
    position: absolute;
    top: 50%;
    left: 50%;
    z-index: 4;
    display: grid;
    height: 3.35rem;
    width: 3.35rem;
    place-items: center;
    border: 1px solid rgb(255 255 255 / 0.2);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(255 255 255 / 0.18), rgb(255 255 255 / 0.06)),
      rgb(0 0 0 / 0.16);
    box-shadow: 0 12px 30px rgb(0 0 0 / 0.18);
    color: rgb(255 255 255 / 0.84);
    transform: translate(-50%, -50%);
    backdrop-filter: blur(18px) saturate(150%);
    -webkit-backdrop-filter: blur(18px) saturate(150%);
  }

  .video-play-message {
    position: absolute;
    top: calc(50% + 3.25rem);
    left: 50%;
    z-index: 4;
    width: max-content;
    max-width: calc(100% - 2rem);
    padding: 0.45rem 0.75rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 12px 32px rgb(0 0 0 / 0.24);
    color: rgb(255 255 255 / 0.82);
    font-size: 0.75rem;
    font-weight: 700;
    transform: translateX(-50%);
    backdrop-filter: blur(24px) saturate(170%);
    -webkit-backdrop-filter: blur(24px) saturate(170%);
  }

  .video-controls {
    position: absolute;
    right: max(0.75rem, env(safe-area-inset-right));
    bottom: max(0.75rem, env(safe-area-inset-bottom));
    left: max(0.75rem, env(safe-area-inset-left));
    z-index: 5;
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-height: 3.35rem;
    padding: 0.65rem 0.8rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 24px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.18) inset, 0 20px 48px rgb(0 0 0 / 0.3);
    color: white;
    opacity: 0;
    pointer-events: none;
    transform: translateY(0.75rem);
    backdrop-filter: blur(30px) saturate(170%);
    -webkit-backdrop-filter: blur(30px) saturate(170%);
    transition:
      opacity 180ms ease,
      transform 220ms cubic-bezier(0.32, 0.72, 0, 1);
  }

  .video-controls-visible {
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }

  .video-control-button {
    display: grid;
    height: 2rem;
    width: 2rem;
    flex: 0 0 auto;
    place-items: center;
    border-radius: 999px;
    color: rgb(255 255 255 / 0.86);
    transition:
      background 140ms ease,
      color 140ms ease,
      transform 140ms ease;
  }

  .video-control-button:hover {
    background: rgb(255 255 255 / 0.12);
    color: white;
    transform: scale(1.04);
  }

  .video-time {
    width: 2.35rem;
    flex: 0 0 auto;
    color: rgb(255 255 255 / 0.72);
    font-size: 0.72rem;
    font-variant-numeric: tabular-nums;
    font-weight: 700;
  }

  .video-time-end {
    text-align: right;
  }

  .video-progress {
    position: relative;
    display: flex;
    height: 1.5rem;
    flex: 1 1 auto;
    align-items: center;
    min-width: 4rem;
  }

  .video-progress-track {
    height: 0.24rem;
    width: 100%;
    overflow: hidden;
    border-radius: 999px;
    background: rgb(255 255 255 / 0.2);
  }

  .video-progress-fill {
    height: 100%;
    border-radius: inherit;
    background: rgb(255 255 255 / 0.9);
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
    flex: 0 0 auto;
    cursor: pointer;
    appearance: none;
    border-radius: 999px;
    background: rgb(255 255 255 / 0.25);
  }

  .video-volume::-webkit-slider-thumb {
    width: 0.72rem;
    height: 0.72rem;
    appearance: none;
    border-radius: 999px;
    background: white;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.24);
  }

  .video-volume::-moz-range-thumb {
    width: 0.72rem;
    height: 0.72rem;
    border: 0;
    border-radius: 999px;
    background: white;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.24);
  }

  @media (max-width: 520px) {
    .video-controls {
      right: 0.65rem;
      bottom: 0.65rem;
      left: 0.65rem;
      gap: 0.35rem;
      padding: 0.55rem 0.6rem;
    }

    .video-volume,
    .video-time-end {
      display: none;
    }
  }
</style>
