<script module lang="ts">
  let nextPlayerId = 0;
  let activePlayerId: number | undefined = undefined;
</script>

<script lang="ts">
  import { onDestroy } from 'svelte';
  import { Pause, PictureInPicture2, Play, Volume2, VolumeX } from 'lucide-svelte';

  const availableSpeeds = [1, 1.25, 1.5, 2];
  const longPressDelayMs = 200;
  const seekFeedbackAccumulationMs = 600;
  const touchpadSeekSensitivity = 0.05;

  type SafariVideoElement = HTMLVideoElement & {
    webkitSupportsPresentationMode?: (mode: 'picture-in-picture') => boolean;
    webkitSetPresentationMode?: (mode: 'inline' | 'picture-in-picture') => void;
    webkitPresentationMode?: 'inline' | 'picture-in-picture' | 'fullscreen';
  };

  let {
    src,
    title
  }: {
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
  const playerId = nextPlayerId++;

  const progress = $derived(duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0);
  const supportsPip = $derived.by(() => {
    const safariVideo = video as SafariVideoElement | undefined;
    return Boolean(
      document.pictureInPictureEnabled ||
        safariVideo?.webkitSupportsPresentationMode?.('picture-in-picture')
    );
  });

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

  async function togglePlay() {
    if (!video) return;
    setActivePlayer();
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
    volume = video.volume;
    muted = video.muted;
    video.playbackRate = userPlaybackRate;
    supportsVolumeControl = canSetVolume(video);
  }

  function canSetVolume(element: HTMLVideoElement) {
    const currentVolume = element.volume;
    const testVolume = currentVolume === 1 ? 0.95 : 1;

    try {
      element.volume = testVolume;
      const supported = Math.abs(element.volume - testVolume) < 0.001;
      element.volume = currentVolume;
      return supported;
    } catch {
      return false;
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
    currentTime = Number(target.value);
    video.currentTime = currentTime;
  }

  function handleVolume(event: Event) {
    if (!video || !supportsVolumeControl) return;
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
    playBlocked = false;
    video.play().catch(() => {
      playBlocked = true;
      showControls = true;
    });
  }

  function clampTime(nextTime: number) {
    const maxTime = duration || video?.duration || 0;
    return Math.max(0, Math.min(maxTime, nextTime));
  }

  function seekBy(seconds: number) {
    if (!video) return;
    video.currentTime = clampTime(video.currentTime + seconds);
    currentTime = video.currentTime;
    revealControls();
  }

  function seekBySide(event: MouseEvent) {
    if (!video || !container) return;

    const now = Date.now();
    const rect = container.getBoundingClientRect();
    const side = event.clientX < rect.left + rect.width / 2 ? 'left' : 'right';
    const delta = side === 'left' ? -10 : 10;
    const shouldAccumulate = seekFeedbackSide === side && now - lastSeekFeedbackAt <= seekFeedbackAccumulationMs;

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
    const index = Math.max(0, availableSpeeds.indexOf(userPlaybackRate));
    const nextIndex = Math.max(0, Math.min(availableSpeeds.length - 1, index + direction));
    if (nextIndex !== index) {
      setPlaybackRate(availableSpeeds[nextIndex]);
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
      }, longPressDelayMs);
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
      }, longPressDelayMs);
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

  function isEditableTarget(target: EventTarget | null) {
    if (!(target instanceof HTMLElement)) return false;
    const tagName = target.tagName.toLowerCase();
    return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable;
  }

  function handleWheel(event: WheelEvent) {
    if (Math.abs(event.deltaX) < Math.abs(event.deltaY)) return;
    event.preventDefault();
    setActivePlayer();
    seekBy(-event.deltaX * touchpadSeekSensitivity);
  }

  function formatTime(seconds: number) {
    if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
    const minutes = Math.floor(seconds / 60);
    const rest = Math.floor(seconds % 60);
    return `${minutes}:${String(rest).padStart(2, '0')}`;
  }

  $effect(() => {
    if (!container) return;

    const handler = (event: WheelEvent) => handleWheel(event);
    container.addEventListener('wheel', handler, { passive: false });

    return () => {
      container?.removeEventListener('wheel', handler);
    };
  });

  onDestroy(() => {
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
    }}
    onplay={() => {
      paused = false;
      revealControls();
    }}
    onpause={() => {
      paused = true;
      revealControls();
    }}
    onended={() => {
      paused = true;
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
        onpointerup={finishDragging}
        onmousedown={() => (isDragging = true)}
        onmouseup={finishDragging}
        onchange={finishDragging}
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
