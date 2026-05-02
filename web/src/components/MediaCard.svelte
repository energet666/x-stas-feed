<script lang="ts">
  import { Maximize2, X } from 'lucide-svelte';
  import FeedVideoPlayer from './FeedVideoPlayer/FeedVideoPlayer.svelte';
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
  class="media-frame"
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
      <h2 class="truncate text-sm font-semibold text-primary">{item.filename}</h2>
      <p class="text-xs font-semibold text-muted">{formatMediaDate(item.modifiedAt)}</p>
    </div>
    <button
      class="glass-icon-button"
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
      class="block h-full w-full object-contain"
      src={item.url}
      alt={item.filename}
      loading="lazy"
      decoding="async"
    />
  {/if}
</div>
