<script module lang="ts">
  const FEED_AUDIO_PLAYER_PREFIX = 'audio-';
  let nextPlayerId = 0;
</script>

<script lang="ts">
  import { Disc3 } from 'lucide-svelte';
  import { onDestroy, onMount } from 'svelte';
  import FeedCardFrame from '../FeedCardFrame.svelte';
  import AudioControls from './FeedAudioControls.svelte';
  import { uiText as t } from '../../lib/ui_text';
  import type { MediaItem } from '../../lib/feed';
  import {
    FEED_VIDEO_PLAY_EVENT,
    FEED_VIDEO_VOLUME_EVENT,
    attachHorizontalSeekWheel,
    canSetVolume,
    clampTime,
    clearStoredProgress,
    clampVolume,
    isEditableTarget,
    readStoredProgress,
    readStoredVolume,
    saveStoredProgress,
    saveStoredVolume
  } from '../FeedVideoPlayer/utils';

  let {
    item,
    expanded,
    favorite,
    ambientActive,
    overlayVisible,
    likePending = false,
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
    suppressFeedChrome?: boolean;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  let audio = $state<HTMLAudioElement | undefined>(undefined);
  let paused = $state(true);
  let duration = $state(0);
  let currentTime = $state(0);
  let volume = $state(1);
  let muted = $state(false);
  let supportsVolumeControl = $state(true);
  let isDragging = $state(false);
  let isOverControls = $state(false);
  let playBlocked = $state(false);
  let coverFailed = $state(false);
  let container = $state<HTMLDivElement | undefined>(undefined);
  let lastProgressSaveAt = 0;
  const playerId = `${FEED_AUDIO_PLAYER_PREFIX}${nextPlayerId++}`;

  const tags = $derived(item.audioTags ?? {});
  const title = $derived(tags.title || stripExtension(item.displayName));
  const artist = $derived(tags.artist || tags.albumArtist || '');
  const albumLine = $derived([tags.album, tags.date].filter(Boolean).join(' · '));
  const progress = $derived(duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0);
  const showCover = $derived(Boolean(item.coverUrl && !coverFailed));

  onMount(() => {
    duration = item.durationSeconds ?? 0;
    currentTime = readStoredProgress(item.id);
  });

  function stripExtension(name: string) {
    const dot = name.lastIndexOf('.');
    return dot > 0 ? name.slice(0, dot) : name;
  }

  function announcePlayback() {
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_PLAY_EVENT, { detail: { playerId } }));
  }

  function pauseForOtherPlayer(event: Event) {
    const otherPlayerId = (event as CustomEvent<{ playerId: string }>).detail?.playerId;
    if (otherPlayerId === playerId || !audio || audio.paused) return;
    audio.pause();
  }

  function announceVolumeChange() {
    window.dispatchEvent(new CustomEvent(FEED_VIDEO_VOLUME_EVENT, { detail: { playerId, volume, muted } }));
  }

  function applyVolume(nextVolume: number, nextMuted: boolean, announce = false) {
    if (!audio) return;
    volume = clampVolume(nextVolume);
    muted = nextMuted;
    if (supportsVolumeControl) {
      audio.volume = volume;
    }
    audio.muted = muted;
    saveStoredVolume(volume, muted);
    if (announce) announceVolumeChange();
  }

  function applyStoredVolume() {
    const stored = readStoredVolume();
    applyVolume(stored.volume, stored.muted);
  }

  function syncVolumeFromOtherPlayer(event: Event) {
    const detail = (event as CustomEvent<{ playerId: string; volume: number; muted: boolean }>).detail;
    if (!detail || detail.playerId === playerId || !audio) return;
    applyVolume(detail.volume, detail.muted);
  }

  function syncMetadata() {
    if (!audio) return;
    duration = Number.isFinite(audio.duration) ? audio.duration : item.durationSeconds ?? 0;
    supportsVolumeControl = canSetVolume(audio);
    applyStoredVolume();
    if (currentTime > 0.5 && currentTime < duration - 1 && audio.currentTime <= 0.5) {
      try {
        audio.currentTime = currentTime;
      } catch {
        // Browsers can reject early seeks until enough metadata is available.
      }
    }
  }

  async function togglePlay() {
    if (!audio) return;
    playBlocked = false;
    if (audio.paused) {
      try {
        if (currentTime > 0.5 && audio.currentTime <= 0.5) {
          audio.currentTime = currentTime;
        }
        await audio.play();
      } catch {
        playBlocked = true;
      }
    } else {
      audio.pause();
    }
    onReveal();
  }

  function handleCardClick(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (target.closest('.audio-controls, .feed-card-panel, button, a, input, textarea, select')) return;
    container?.focus();
    void togglePlay();
  }

  function isAudioKeyboardTarget(target: EventTarget | null) {
    if (isEditableTarget(target)) return false;
    return target instanceof HTMLElement && !target.closest('.audio-controls, button, a');
  }

  function handleKeyboard(event: KeyboardEvent) {
    if (!isAudioKeyboardTarget(event.target)) return;

    if (event.code === 'Space') {
      event.preventDefault();
      event.stopPropagation();
      if (!event.repeat) void togglePlay();
      return;
    }

    if (event.code === 'ArrowLeft' || event.code === 'ArrowRight') {
      event.preventDefault();
      event.stopPropagation();
      seekBy(event.code === 'ArrowRight' ? 1 : -1);
    }
  }

  function handleSeek(event: Event) {
    if (!audio) return;
    const target = event.target as HTMLInputElement;
    currentTime = Number(target.value);
    if (!Number.isFinite(currentTime) || duration <= 0) return;
    audio.currentTime = currentTime;
    saveProgress();
  }

  function seekBy(seconds: number) {
    if (!audio) return;
    const maxTime = duration || audio.duration || 0;
    if (maxTime <= 0) return;
    audio.currentTime = clampTime(audio.currentTime + seconds, maxTime);
    currentTime = audio.currentTime;
    saveProgress();
    onReveal();
  }

  function handleVolume(event: Event) {
    if (!audio || !supportsVolumeControl) return;
    const target = event.target as HTMLInputElement;
    const nextVolume = Number(target.value);
    applyVolume(nextVolume, nextVolume === 0, true);
  }

  function toggleMute() {
    if (!audio) return;
    const nextMuted = !muted;
    const nextVolume = !nextMuted && volume === 0 ? 1 : volume;
    applyVolume(nextVolume, nextMuted, true);
    onReveal();
  }

  function saveProgressThrottled() {
    const now = Date.now();
    if (now - lastProgressSaveAt < 1000) return;
    lastProgressSaveAt = now;
    saveProgress();
  }

  function saveProgress() {
    if (!audio || duration <= 0) return;
    saveStoredProgress(item.id, audio.currentTime, duration);
  }

  function finishDragging() {
    isDragging = false;
    onReveal();
  }

  function enterControls() {
    isOverControls = true;
    onKeep();
  }

  function leaveControls() {
    isOverControls = false;
    onReveal();
  }

  $effect(() => {
    window.addEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
    window.addEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);

    return () => {
      window.removeEventListener(FEED_VIDEO_PLAY_EVENT, pauseForOtherPlayer);
      window.removeEventListener(FEED_VIDEO_VOLUME_EVENT, syncVolumeFromOtherPlayer);
    };
  });

  $effect(() => {
    if (!container) return;
    return attachHorizontalSeekWheel(container, seekBy);
  });

  onDestroy(() => {
    saveProgress();
  });
</script>

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
      {#if showCover}
        <img src={item.coverUrl} alt="" class="ambient-media" decoding="async" onerror={() => (coverFailed = true)} />
      {:else}
        <div class="audio-ambient ambient-media"></div>
      {/if}
    {/if}
  {/snippet}

  {#snippet content()}
    <div
      bind:this={container}
      class="audio-card-surface"
      class:audio-card-surface-playing={!paused}
      role="button"
      aria-label={t.playback.audioPlayer(item.displayName)}
      aria-pressed={!paused}
      tabindex="0"
      onpointermove={onReveal}
      onpointerenter={onReveal}
      onmousemove={onReveal}
      onmouseenter={onReveal}
      onclick={handleCardClick}
      onkeydown={handleKeyboard}
      ontouchstart={onReveal}
      onfocusin={onKeep}
      onmouseleave={() => {
        if (!isOverControls && !isDragging) onHide();
      }}
    >
      <audio
        bind:this={audio}
        preload="metadata"
        src={item.url}
        onloadedmetadata={syncMetadata}
        ondurationchange={syncMetadata}
        ontimeupdate={() => {
          if (!isDragging) currentTime = audio?.currentTime ?? 0;
          saveProgressThrottled();
        }}
        onplay={() => {
          paused = false;
          announcePlayback();
          onReveal();
        }}
        onpause={() => {
          paused = true;
          saveProgress();
          onReveal();
        }}
        onended={() => {
          paused = true;
          currentTime = 0;
          clearStoredProgress(item.id);
          onReveal();
        }}
      ></audio>

      <div class="audio-art-wrap">
        <div class="audio-art">
          {#if showCover}
            <img src={item.coverUrl} alt="" decoding="async" onerror={() => (coverFailed = true)} />
          {:else}
            <div class="audio-art-fallback" aria-hidden="true">
              <Disc3 class="audio-disc" size={150} strokeWidth={1.2} />
            </div>
          {/if}
        </div>
      </div>

      <div class="audio-copy">
        <p class="audio-kicker">{t.playback.audio}</p>
        <h3 title={title}>{title}</h3>
        {#if artist}
          <p class="audio-artist">{artist}</p>
        {/if}
        {#if albumLine}
          <p class="audio-album">{albumLine}</p>
        {/if}
        {#if playBlocked}
          <p class="audio-error">{t.playback.browserBlocked}</p>
        {/if}
      </div>
    </div>
  {/snippet}

  {#snippet bottomAccessory()}
    <AudioControls
      {paused}
      {currentTime}
      {duration}
      {muted}
      {volume}
      {progress}
      {supportsVolumeControl}
      bind:isDragging
      onTogglePlay={togglePlay}
      onSeek={handleSeek}
      onVolumeChange={handleVolume}
      onToggleMute={toggleMute}
      onEnterControls={enterControls}
      onLeaveControls={leaveControls}
      onFinishDragging={finishDragging}
    />
  {/snippet}
</FeedCardFrame>

<style>
  .audio-card-surface {
    position: relative;
    display: grid;
    height: 100%;
    min-height: 34rem;
    overflow: hidden;
    grid-template-rows: auto auto;
    align-content: center;
    place-items: center;
    gap: clamp(1.4rem, 3.5vw, 2.2rem);
    padding: clamp(4.4rem, 12vw, 6rem) clamp(1.25rem, 5vw, 3rem) clamp(7.5rem, 14vw, 8.5rem);
    background: rgb(12 16 22);
    color: var(--color-text-primary);
  }

  .audio-card-surface::before {
    position: absolute;
    inset: -75%;
    z-index: 0;
    content: "";
    background:
      radial-gradient(circle at 28% 20%, rgb(134 239 172 / 0.28), transparent 28%),
      radial-gradient(circle at 78% 72%, rgb(56 189 248 / 0.18), transparent 32%),
      linear-gradient(140deg, rgb(12 16 22), rgb(28 34 42) 48%, rgb(8 12 18));
    transform-origin: center;
    will-change: transform;
    animation: audio-background-spin 48s linear infinite;
    animation-play-state: paused;
  }

  .audio-card-surface > :global(*) {
    position: relative;
    z-index: 1;
  }

  .audio-card-surface-playing::before {
    animation-play-state: running;
  }

  @keyframes audio-background-spin {
    from {
      transform: rotate(0deg) scale(1.1);
    }

    to {
      transform: rotate(360deg) scale(1.1);
    }
  }

  .audio-ambient {
    background:
      radial-gradient(circle at 30% 24%, rgb(134 239 172 / 0.7), transparent 35%),
      radial-gradient(circle at 78% 70%, rgb(56 189 248 / 0.42), transparent 38%),
      linear-gradient(135deg, rgb(15 23 20), rgb(8 13 20));
  }

  .audio-art-wrap {
    display: grid;
    width: min(64%, 22rem);
    max-width: min(100%, 44vh);
    aspect-ratio: 1;
    place-items: center;
  }

  .audio-art {
    position: relative;
    display: grid;
    width: 100%;
    height: 100%;
    overflow: hidden;
    place-items: center;
    border: 1px solid rgb(255 255 255 / 0.16);
    border-radius: 0.8rem;
    background: rgb(255 255 255 / 0.08);
    box-shadow: 0 1.5rem 4rem rgb(0 0 0 / 0.36);
  }

  .audio-art img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .audio-art-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background:
      linear-gradient(90deg, rgb(255 255 255 / 0.035) 1px, transparent 1px),
      linear-gradient(0deg, rgb(255 255 255 / 0.03) 1px, transparent 1px),
      linear-gradient(135deg, rgb(20 27 31), rgb(13 19 28));
    background-size: 30px 30px, 30px 30px, auto;
    color: rgb(187 247 208);
  }

  :global(.audio-disc) {
    opacity: 0.72;
  }

  .audio-copy {
    display: grid;
    width: min(100%, 32rem);
    justify-items: center;
    gap: 0.3rem;
    text-align: center;
  }

  .audio-kicker {
    color: rgb(187 247 208);
    font-size: 0.72rem;
    font-weight: 850;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  h3 {
    display: -webkit-box;
    max-height: 3.9em;
    overflow: hidden;
    overflow-wrap: anywhere;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    font-size: clamp(1.35rem, 4vw, 2.05rem);
    font-weight: 850;
    line-height: 1.08;
    letter-spacing: 0;
  }

  .audio-artist,
  .audio-album,
  .audio-error {
    max-width: 100%;
    overflow-wrap: anywhere;
    font-size: 0.92rem;
    font-weight: 750;
  }

  .audio-artist {
    color: var(--color-text-primary);
  }

  .audio-album {
    color: var(--color-text-muted);
  }

  .audio-error {
    color: rgb(252 165 165);
  }

  @media (max-width: 520px) {
    .audio-card-surface {
      padding-right: 1rem;
      padding-left: 1rem;
    }

    .audio-art-wrap {
      width: min(72%, 18rem);
      max-width: min(100%, 38vh);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .audio-card-surface-playing::before {
      animation-play-state: paused;
    }
  }
</style>
