<script lang="ts">
  import { Maximize2, X } from 'lucide-svelte';
  import FeedVideoPlayer from '../FeedVideoPlayer.svelte';
  import { formatMediaDate } from '../lib/date';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    expanded,
    overlayVisible,
    onReveal,
    onKeep,
    onHide,
    onToggleExpanded
  }: {
    item: MediaItem;
    expanded: boolean;
    overlayVisible: boolean;
    onReveal: (id: string) => void;
    onKeep: (id: string) => void;
    onHide: (id: string) => void;
    onToggleExpanded: (id: string) => void;
  } = $props();
</script>

<div
  class="media-frame bg-black"
  role="presentation"
  onpointermove={() => onReveal(item.id)}
  onpointerenter={() => onReveal(item.id)}
  onmousemove={() => onReveal(item.id)}
  onmouseenter={() => onReveal(item.id)}
  ontouchstart={() => onReveal(item.id)}
  onpointerdown={() => onReveal(item.id)}
  onclick={() => onReveal(item.id)}
  onfocusin={() => onKeep(item.id)}
  onmouseleave={() => onHide(item.id)}
>
  <div class="card-overlay" class:card-overlay-visible={overlayVisible}>
    <div class="min-w-0">
      <h2 class="truncate text-sm font-semibold text-white">{item.filename}</h2>
      <p class="text-xs font-semibold text-white/62">{formatMediaDate(item.modifiedAt)}</p>
    </div>
    <button
      class="card-overlay-action"
      type="button"
      aria-label={expanded ? 'Close fullscreen media' : 'Open media fullscreen'}
      onclick={(event) => {
        event.stopPropagation();
        onToggleExpanded(item.id);
      }}
    >
      {#if expanded}
        <X size={17} />
      {:else}
        <Maximize2 size={16} />
      {/if}
    </button>
  </div>

  {#if item.type === 'video'}
    <FeedVideoPlayer mediaId={item.id} src={item.url} title={item.filename} />
  {:else}
    <img
      class="h-full w-full bg-black object-contain"
      src={item.url}
      alt={item.filename}
      loading="lazy"
      decoding="async"
    />
  {/if}
</div>

<style>
  .media-frame {
    position: relative;
    height: clamp(28rem, 76vh, 46rem);
    width: 100%;
    overflow: hidden;
  }

  .media-frame > img {
    display: block;
    height: 100%;
    width: 100%;
    object-fit: contain;
  }

  .card-overlay {
    position: absolute;
    top: 0.75rem;
    right: 0.75rem;
    left: 0.75rem;
    z-index: 6;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    min-height: 3.5rem;
    padding: 0.6rem 0.75rem;
    border: 0;
    border-radius: 20px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.16) inset, 0 14px 34px rgb(0 0 0 / 0.22);
    opacity: 0;
    transform: translateY(-1rem);
    backdrop-filter: blur(34px) saturate(220%);
    -webkit-backdrop-filter: blur(34px) saturate(220%);
    transition:
      opacity 180ms ease,
      transform 180ms ease;
  }

  .card-overlay h2 {
    color: white;
    text-shadow: 0 1px 8px rgb(0 0 0 / 0.34);
  }

  .card-overlay p {
    color: rgb(255 255 255 / 0.72);
  }

  .card-overlay-action {
    display: grid;
    height: 2.25rem;
    width: 2.25rem;
    flex: 0 0 auto;
    place-items: center;
    border: 1px solid rgb(255 255 255 / 0.16);
    border-radius: 999px;
    background: rgb(255 255 255 / 0.08);
    color: rgb(255 255 255 / 0.82);
    transition:
      background 140ms ease,
      border-color 140ms ease,
      color 140ms ease,
      transform 140ms ease;
  }

  .card-overlay-action:hover {
    border-color: rgb(255 255 255 / 0.28);
    background: rgb(255 255 255 / 0.14);
    color: white;
    transform: scale(1.04);
  }

  .card-overlay-visible {
    opacity: 1;
    transform: translateY(0);
  }

  @media (max-width: 520px) {
    .media-frame {
      height: clamp(24rem, 72vh, 40rem);
    }
  }
</style>
