<script module lang="ts">
  let nextPlayerId = 0;
  let activePlayerId: number | undefined = undefined;
</script>

<script lang="ts">
  import { onDestroy } from 'svelte';
  import FeedVideoControls from './FeedVideoControls.svelte';
  import FeedVideoOverlay from './FeedVideoOverlay.svelte';
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
  } from './utils';

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
  class="feed-video-player relative h-full w-full overflow-hidden"
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
    class="block h-full w-full object-contain"
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

  <FeedVideoOverlay
    {paused}
    {playBlocked}
    {seekFeedbackSide}
    {seekFeedbackAmount}
    {seekFeedbackTick}
    {showSpeedIndicator}
    {userPlaybackRate}
    onTogglePlay={togglePlay}
  />

  <FeedVideoControls
    {paused}
    {currentTime}
    {duration}
    {muted}
    {volume}
    {progress}
    {supportsVolumeControl}
    {supportsPip}
    {showControls}
    bind:isDragging
    onTogglePlay={togglePlay}
    onSeek={handleSeek}
    onVolumeChange={handleVolume}
    onToggleMute={toggleMute}
    onTogglePip={togglePip}
    onEnterControls={enterControls}
    onLeaveControls={leaveControls}
    onFinishDragging={finishDragging}
  />
</div>

<style>
  .feed-video-player {
    z-index: 1;
  }

  .video-cursor-hidden,
  .video-cursor-hidden video {
    cursor: none;
  }

  /* Controls and overlays styles were moved to their respective components */
</style>
