<script module lang="ts">
  const FEED_VIDEO_CLEAR_ACTIVE_EVENT = 'feed-ai:video-clear-active';
  const FEED_VIDEO_ACTIVE_EVENT = 'feed-ai:video-active';
  let nextPlayerId = 0;
  let activePlayerId: number | undefined = undefined;
</script>

<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import FeedCardFrame from '../FeedCardFrame.svelte';
  import FeedVideoControls from './FeedVideoControls.svelte';
  import FeedVideoOverlay from './FeedVideoOverlay.svelte';
  import { uiText as t } from '../../lib/ui_text';
  import { mediaPosterURL } from '../../lib/feed';
  import type { MediaItem } from '../../lib/feed';
  import {
    AVAILABLE_SPEEDS,
    FEED_VIDEO_PLAY_EVENT,
    FEED_VIDEO_VOLUME_EVENT,
    LONG_PRESS_DELAY_MS,
    MIN_PROGRESS_SAVE_DURATION_SECONDS,
    SEEK_FEEDBACK_ACCUMULATION_MS,
    attachHorizontalSeekWheel,
    canSetVolume,
    clampTime,
    clampVolume,
    clearStoredProgress,
    isEditableTarget,
    readStoredProgress,
    readStoredVolume,
    saveStoredProgress,
    saveStoredVolume,
    supportsPictureInPicture,
    type SafariVideoElement
  } from './utils';
  import { pointerPositionChanged } from '../../lib/pointer_movement';

  const KEYBOARD_FAST_FORWARD_RATE = 5;

  let {
    item,
    expanded,
    favorite,
    ambientActive,
    overlayVisible,
    likePending = false,
    autoplayEnabled = false,
    suppressFeedChrome = false,
    onReveal,
    onKeep,
    onHide,
    onToggleFavorite,
    onToggleExpanded,
    onOpenComments,
    onLike
  }: {
    item: MediaItem;
    expanded: boolean;
    favorite: boolean;
    ambientActive: boolean;
    overlayVisible: boolean;
    likePending?: boolean;
    autoplayEnabled?: boolean;
    suppressFeedChrome?: boolean;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
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
  let temporaryPlaybackRate = $state<number | null>(null);
  let seekFeedbackSide = $state<'left' | 'right' | null>(null);
  let seekFeedbackAmount = $state(10);
  let seekFeedbackTick = $state(0);
  let hideTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let speedTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let clickTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let seekFeedbackTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let spaceTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let arrowRightTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let isSpaceDown = false;
  let isSpaceLongPress = false;
  let isArrowRightDown = false;
  let isArrowRightLongPress = false;
  let lastSeekFeedbackAt = 0;
  let previewFrameRequested = false;
  let progressRestored = false;
  let hasProgressInteraction = false;
  let lastProgressSaveAt = 0;
  let ambientCanvas = $state<HTMLCanvasElement | undefined>(undefined);
  let ambientCanvasWidth = $state(32);
  let ambientCanvasHeight = $state(32);
  let ambientAnimationFrameId: number | undefined = undefined;
  let ambientFrameTick = 0;
  let isSafari = $state(false);
  let posterTime = $state(0);
  let posterCoverVisible = $state(true);
  let metadataWanted = $state(false);
  let hasVideoInteraction = $state(false);
  let hasDecodedFrame = $state(false);
  let autoplayStarted = false;
  let inAutoplayViewport = false;
  let autoplayRequestVersion = 0;
  const playerId = nextPlayerId++;

  const canPersistProgress = $derived(duration >= MIN_PROGRESS_SAVE_DURATION_SECONDS);
  const progress = $derived(duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0);
  const supportsPip = $derived(supportsPictureInPicture(video));
  const videoPreload = $derived(hasVideoInteraction && (expanded || !paused) ? 'auto' : hasVideoInteraction && (metadataWanted || !isSafari) ? 'metadata' : 'none');
  const posterURL = $derived(mediaPosterURL(item.id, posterTime));
  const activePosterURL = $derived(hasVideoInteraction ? undefined : posterURL);
  const activeVideoURL = $derived(hasVideoInteraction ? item.url : undefined);
  const displayedPlaybackRate = $derived(temporaryPlaybackRate ?? userPlaybackRate);

  onMount(() => {
    const userAgent = navigator.userAgent;
    isSafari = /Safari/.test(userAgent) && !/Chrome|Chromium|CriOS|FxiOS|Edg\//.test(userAgent);
    duration = item.durationSeconds ?? 0;
    const storedTime = readStoredProgress(item.id);
    posterTime = storedTime;
    currentTime = storedTime;
    metadataWanted = duration <= 0;
    validateDisplayedProgress();
  });

  function revealControls() {
    showControls = true;
    showCursor = true;
    scheduleControlsHide();
  }

  function revealControlsFromPointer(event: PointerEvent) {
    if (pointerPositionChanged(event)) revealControls();
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
    metadataWanted = true;
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
    autoplayStarted = false;
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

  function markVideoInteraction() {
    hasVideoInteraction = true;
    posterCoverVisible = false;
  }

  async function activateVideoElement() {
    if (hasVideoInteraction) return;
    markVideoInteraction();
    await tick();
  }

  async function togglePlay() {
    if (!video) return;
    autoplayStarted = false;
    setActivePlayer();
    metadataWanted = true;
    markProgressInteraction();
    await activateVideoElement();
    applySavedStartPosition();
    prepareIdleVideoFrame();
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
    duration = Number.isFinite(video.duration) ? video.duration : item.durationSeconds ?? 0;
    applyEffectivePlaybackRate();
    supportsVolumeControl = canSetVolume(video);
    applyStoredVolume();
    if (autoplayStarted) {
      muted = true;
      video.muted = true;
    }
    syncAmbientCanvasSize();
    validateDisplayedProgress();
    applySavedStartPosition();
  }

  function validateDisplayedProgress() {
    if (duration <= 0) return;
    if (!canPersistProgress) {
      currentTime = 0;
      posterTime = 0;
      clearStoredProgress(item.id);
      return;
    }
    if (currentTime <= 0.5) return;
    if (currentTime >= duration - 1) {
      currentTime = 0;
      posterTime = 0;
      clearStoredProgress(item.id);
    }
  }

  function prepareIdleVideoFrame() {
    restoreProgress();
    requestPreviewFrame();
  }

  function applySavedStartPosition() {
    if (!video || !hasVideoInteraction || duration <= 0 || video.currentTime > 0.5) return;
    if (!canPersistProgress) return;
    const storedTime = readStoredProgress(item.id);
    if (storedTime <= 0.5) return;
    if (storedTime >= duration - 1) {
      clearStoredProgress(item.id);
      return;
    }

    try {
      video.currentTime = storedTime;
      currentTime = storedTime;
    } catch {
      // If the browser refuses the seek now, normal playback can continue.
    }
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
      markVideoInteraction();
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
    setActivePlayer();
    markVideoInteraction();
    revealControls();
  }

  async function handleSeek(event: Event) {
    if (!video) return;
    const target = event.target as HTMLInputElement;
    setActivePlayer();
    metadataWanted = true;
    markProgressInteraction();
    await activateVideoElement();
    currentTime = Number(target.value);
    if (!Number.isFinite(currentTime) || duration <= 0) return;
    video.currentTime = currentTime;
    saveProgress();
  }

  function handleVolume(event: Event) {
    if (!video || !supportsVolumeControl) return;
    const target = event.target as HTMLInputElement;
    setActivePlayer();
    autoplayStarted = false;
    markVideoInteraction();
    const nextVolume = Number(target.value);
    applyVolume(nextVolume, nextVolume === 0, true);
  }

  function toggleMute() {
    if (!video) return;
    setActivePlayer();
    autoplayStarted = false;
    markVideoInteraction();
    const nextMuted = !muted;
    const nextVolume = !nextMuted && volume === 0 ? 1 : volume;
    applyVolume(nextVolume, nextMuted, true);
    revealControls();
  }

  async function togglePip() {
    if (!video) return;
    setActivePlayer();
    await activateVideoElement();
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
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_ACTIVE_EVENT));
  }

  function isActivePlayer() {
    return activePlayerId === playerId;
  }

  function clearActivePlayer() {
    if (activePlayerId === playerId) {
      activePlayerId = undefined;
    }
  }

  function releaseActivePlayer() {
    if (!isActivePlayer()) return;
    activePlayerId = undefined;
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_CLEAR_ACTIVE_EVENT));
  }

  function safePlay() {
    if (!video) return;
    autoplayStarted = false;
    metadataWanted = true;
    markProgressInteraction();
    activateVideoElement().then(() => {
      applySavedStartPosition();
      playBlocked = false;
      return video?.play();
    }).catch(() => {
      playBlocked = true;
      showControls = true;
    });
  }

  function pauseAutoplay() {
    autoplayRequestVersion += 1;
    if (!autoplayStarted) return;
    autoplayStarted = false;
    if (video && !video.paused) video.pause();
  }

  async function startAutoplay() {
    if (!autoplayEnabled || !inAutoplayViewport || document.visibilityState === 'hidden' || !video || !video.paused) {
      return;
    }

    const requestVersion = ++autoplayRequestVersion;
    autoplayStarted = true;
    setActivePlayer();
    metadataWanted = true;
    markProgressInteraction();
    await activateVideoElement();
    if (requestVersion !== autoplayRequestVersion || !autoplayEnabled || !inAutoplayViewport || !video) return;

    applySavedStartPosition();
    muted = true;
    video.muted = true;
    playBlocked = false;

    try {
      await video.play();
    } catch {
      if (requestVersion !== autoplayRequestVersion) return;
      autoplayStarted = false;
      playBlocked = true;
      showControls = true;
    }
  }

  function clampPlaybackTime(nextTime: number) {
    const maxTime = duration || video?.duration || 0;
    return clampTime(nextTime, maxTime);
  }

  async function seekBy(seconds: number) {
    if (!video) return;
    setActivePlayer();
    metadataWanted = true;
    const baseTime = !hasVideoInteraction && currentTime > 0.5 ? currentTime : video.currentTime;
    markProgressInteraction();
    await activateVideoElement();
    video.currentTime = clampPlaybackTime(baseTime + seconds);
    currentTime = video.currentTime;
    saveProgress();
    onReveal();
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
    applyEffectivePlaybackRate();
    showSpeed();
  }

  function applyEffectivePlaybackRate() {
    if (video) video.playbackRate = temporaryPlaybackRate ?? userPlaybackRate;
  }

  function setTemporaryPlaybackRate(rate: number) {
    temporaryPlaybackRate = rate;
    applyEffectivePlaybackRate();
    showSpeed();
  }

  function clearTemporaryPlaybackRate(rate: number) {
    if (temporaryPlaybackRate === rate) {
      temporaryPlaybackRate = null;
      applyEffectivePlaybackRate();
    }
  }

  function changePlaybackRate(direction: 1 | -1) {
    const index = Math.max(0, AVAILABLE_SPEEDS.indexOf(userPlaybackRate));
    const nextIndex = Math.max(0, Math.min(AVAILABLE_SPEEDS.length - 1, index + direction));
    if (nextIndex !== index) {
      setPlaybackRate(AVAILABLE_SPEEDS[nextIndex]);
    }
  }

  function handleKeyboard(event: KeyboardEvent, phase: 'down' | 'up') {
    if (phase === 'up') {
      handleKeyUp(event);
      return;
    }

    if (!isActivePlayer() || isEditableTarget(event.target)) return;

    handleKeyDown(event);
  }

  function handleKeyDown(event: KeyboardEvent) {
    if (event.code === 'Space') {
      event.preventDefault();
      if (isSpaceDown) return;
      markVideoInteraction();
      isSpaceDown = true;
      isSpaceLongPress = false;

      spaceTimer = setTimeout(() => {
        if (!video) return;
        isSpaceLongPress = true;
        setTemporaryPlaybackRate(2);
        if (video.paused) safePlay();
      }, LONG_PRESS_DELAY_MS);
      return;
    }

    if (event.shiftKey && (event.code === 'ArrowLeft' || event.code === 'ArrowRight')) {
      event.preventDefault();
      changePlaybackRate(event.code === 'ArrowRight' ? 1 : -1);
      return;
    }

    if (event.code === 'ArrowRight') {
      event.preventDefault();
      if (isArrowRightDown) return;
      isArrowRightDown = true;
      isArrowRightLongPress = false;
      setActivePlayer();

      arrowRightTimer = setTimeout(() => {
        if (!video) return;
        isArrowRightLongPress = true;
        setTemporaryPlaybackRate(KEYBOARD_FAST_FORWARD_RATE);
        if (video.paused) safePlay();
      }, LONG_PRESS_DELAY_MS);
      return;
    }

    if (event.code === 'ArrowLeft') {
      event.preventDefault();
      seekBy(-1);
    }
  }

  function handleKeyUp(event: KeyboardEvent) {
    if (event.code === 'Space') {
      if (!isSpaceDown) return;
      event.preventDefault();
      isSpaceDown = false;
      clearTimeout(spaceTimer);

      if (isSpaceLongPress) {
        clearTemporaryPlaybackRate(2);
      } else {
        void togglePlay();
      }
      return;
    }

    if (event.code === 'ArrowRight' && isArrowRightDown) {
      event.preventDefault();
      isArrowRightDown = false;
      clearTimeout(arrowRightTimer);

      if (isArrowRightLongPress) {
        clearTemporaryPlaybackRate(KEYBOARD_FAST_FORWARD_RATE);
      } else {
        seekBy(1);
      }
    }
  }

  function restoreProgress() {
    if (!video || progressRestored || duration <= 0) return;
    progressRestored = true;
    if (!canPersistProgress) {
      clearStoredProgress(item.id);
      return;
    }

    const storedTime = readStoredProgress(item.id);
    if (storedTime <= 0.5) return;
    if (storedTime >= duration - 1) {
      clearStoredProgress(item.id);
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
    if (!canPersistProgress) {
      clearStoredProgress(item.id);
      return;
    }
    const time = video.currentTime;

    if (!Number.isFinite(time) || time <= 0.5 || time >= duration - 1) {
      clearStoredProgress(item.id);
      return;
    }

    saveStoredProgress(item.id, time, duration);
  }

  $effect(() => {
    if (!container) return;
    return attachHorizontalSeekWheel(container, seekBy);
  });

  $effect(() => {
    if (!container) return;
    const card = container.closest<HTMLElement>('.ui-media-card') ?? container;

    const observer = new IntersectionObserver(
      ([entry]) => {
        inAutoplayViewport = entry?.isIntersecting === true && entry.intersectionRatio >= 1;
        if (inAutoplayViewport) {
          setActivePlayer();
          void startAutoplay();
        } else {
          pauseAutoplay();
          releaseActivePlayer();
        }
      },
      { threshold: 1 }
    );

    observer.observe(card);
    return () => observer.disconnect();
  });

  $effect(() => {
    if (autoplayEnabled) {
      void startAutoplay();
    } else {
      pauseAutoplay();
    }
  });

  $effect(() => {
    window.addEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
    window.addEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);
    window.addEventListener(FEED_VIDEO_CLEAR_ACTIVE_EVENT, clearActivePlayer);

    return () => {
      window.removeEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
      window.removeEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);
      window.removeEventListener(FEED_VIDEO_CLEAR_ACTIVE_EVENT, clearActivePlayer);
    };
  });

  $effect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'hidden') {
        pauseAutoplay();
      } else if (inAutoplayViewport) {
        void startAutoplay();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
  });

  function syncAmbientFrame() {
    drawFrame();
  }

  function startAmbientSync() {
    if (ambientAnimationFrameId || !ambientActive) return;

    const loop = () => {
      if (!video || video.paused || video.ended || !ambientActive) {
        stopAmbientSync();
        return;
      }

      ambientFrameTick += 1;
      if (ambientFrameTick % 5 === 0) {
        drawFrame();
      }

      ambientAnimationFrameId = requestAnimationFrame(loop);
    };

    ambientAnimationFrameId = requestAnimationFrame(loop);
  }

  function stopAmbientSync() {
    if (ambientAnimationFrameId) {
      cancelAnimationFrame(ambientAnimationFrameId);
      ambientAnimationFrameId = undefined;
    }
    ambientFrameTick = 0;
  }

  function drawFrame() {
    if (!video || !ambientCanvas || video.readyState < 2) return;
    syncAmbientCanvasSize();
    const ctx = ambientCanvas.getContext('2d', { alpha: false });
    if (!ctx) return;
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = 'low';
    ctx.drawImage(video, 0, 0, ambientCanvas.width, ambientCanvas.height);
    hasDecodedFrame = true;
  }

  function syncAmbientCanvasSize() {
    if (!video?.videoWidth || !video.videoHeight) return;

    const maxSide = 64;
    const scale = maxSide / Math.max(video.videoWidth, video.videoHeight);
    ambientCanvasWidth = Math.max(1, Math.round(video.videoWidth * scale));
    ambientCanvasHeight = Math.max(1, Math.round(video.videoHeight * scale));
  }

  $effect(() => {
    if (ambientActive && video && duration > 0) {
      drawFrame();
    }
  });

  $effect(() => {
    if (video && ambientCanvas) {
      if (video.readyState >= 2) {
        drawFrame();
      }

      const handler = () => {
        drawFrame();
        if (!paused) startAmbientSync();
      };
      video.addEventListener('loadeddata', handler, { once: true });
      return () => video?.removeEventListener('loadeddata', handler);
    }
  });

  $effect(() => {
    if (ambientActive && !paused) {
      startAmbientSync();
    } else {
      stopAmbientSync();
    }
  });

  onDestroy(() => {
    saveProgress();
    clearTimeout(hideTimer);
    clearTimeout(speedTimer);
    clearTimeout(clickTimer);
    clearTimeout(seekFeedbackTimer);
    clearTimeout(spaceTimer);
    clearTimeout(arrowRightTimer);
    releaseActivePlayer();
    stopAmbientSync();
  });
</script>

<svelte:window
  onkeydown={(event) => handleKeyboard(event, 'down')}
  onkeyup={(event) => handleKeyboard(event, 'up')}
/>

<FeedCardFrame
  {item}
  {expanded}
  {favorite}
  {ambientActive}
  {overlayVisible}
  {likePending}
  {suppressFeedChrome}
  {onReveal}
  {onKeep}
  {onHide}
  {onToggleFavorite}
  {onToggleExpanded}
  {onOpenComments}
  {onLike}
>
  {#snippet ambientBackground()}
    {#if ambientActive}
      {#if paused && !hasVideoInteraction && !hasDecodedFrame}
        <img
          src={posterURL}
          alt=""
          class="ambient-media"
          decoding="async"
        />
      {:else}
        <canvas
          bind:this={ambientCanvas}
          class="ambient-media"
          width={ambientCanvasWidth}
          height={ambientCanvasHeight}
        ></canvas>
      {/if}
    {/if}
  {/snippet}
  {#snippet content()}
    <div
      bind:this={container}
      class="feed-video-player relative z-1 h-full w-full overflow-hidden"
      class:video-cursor-hidden={!showCursor && !isOverControls && !isDragging}
      role="presentation"
      aria-label={t.playback.videoPlayer(item.displayName)}
      onpointermove={revealControlsFromPointer}
      ontouchstart={handleContainerTouch}
      onfocusin={keepControls}
      onpointerleave={hideControls}
    >
      <!-- svelte-ignore a11y_media_has_caption -->
      <video
        bind:this={video}
        class="block h-full w-full object-contain media-content-shadow"
        playsinline
        preload={videoPreload}
        poster={activePosterURL}
        src={activeVideoURL}
        onclick={handleVideoClick}
        onloadedmetadata={syncMetadata}
        ondurationchange={syncMetadata}
        ontimeupdate={() => {
          if (!isDragging && (hasVideoInteraction || !paused)) currentTime = video?.currentTime ?? 0;
          saveProgressThrottled();
        }}
        onplay={() => {
          markProgressInteraction();
          paused = false;
          announcePlayback();
          revealControls();
          startAmbientSync();
        }}
        onseeking={() => {
          syncAmbientFrame();
        }}
        onseeked={() => {
          syncAmbientFrame();
        }}
        onpause={() => {
          paused = true;
          saveProgress();
          revealControls();
          stopAmbientSync();
        }}
        onended={() => {
          paused = true;
          posterTime = 0;
          clearStoredProgress(item.id);
          revealControls();
          stopAmbientSync();
        }}
      ></video>
      {#if posterCoverVisible}
        <img
          src={posterURL}
          alt=""
          class="pointer-events-none absolute inset-0 z-2 block h-full w-full object-contain media-content-shadow"
          decoding="async"
        />
      {/if}
    </div>
  {/snippet}

  {#snippet contentOverlay()}
    <FeedVideoOverlay
      {paused}
      {playBlocked}
      {seekFeedbackSide}
      {seekFeedbackAmount}
      {seekFeedbackTick}
      {showSpeedIndicator}
      userPlaybackRate={displayedPlaybackRate}
      onTogglePlay={togglePlay}
    />
  {/snippet}

  {#snippet bottomAccessory()}
    <FeedVideoControls
      {paused}
      {currentTime}
      {duration}
      {muted}
      {volume}
      {progress}
      {supportsVolumeControl}
      {supportsPip}
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
  {/snippet}
</FeedCardFrame>
