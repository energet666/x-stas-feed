<script lang="ts">
  import { MessageCircle, Maximize2, X } from 'lucide-svelte';
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
    onToggleExpanded,
    onOpenComments
  }: {
    item: MediaItem;
    expanded: boolean;
    overlayVisible: boolean;
    onReveal: (id: string) => void;
    onKeep: (id: string) => void;
    onHide: (id: string) => void;
    onToggleExpanded: (id: string) => void;
    onOpenComments: (id: string) => void;
  } = $props();

  let introOverlayVisible = $state(true);

  const latestComment = $derived(item.comments.at(-1));
  const infoPanelVisible = $derived(overlayVisible || introOverlayVisible);

  $effect(() => {
    item.id;
    introOverlayVisible = true;
  });

  function dismissIntroOverlay() {
    introOverlayVisible = false;
  }

  function revealOverlay() {
    dismissIntroOverlay();
    onReveal(item.id);
  }

  function keepOverlay() {
    dismissIntroOverlay();
    onKeep(item.id);
  }

  function hideOverlay() {
    dismissIntroOverlay();
    onHide(item.id);
  }
</script>

<div
  class="media-frame"
  role="presentation"
  onpointermove={revealOverlay}
  onpointerenter={revealOverlay}
  onmousemove={revealOverlay}
  onmouseenter={revealOverlay}
  ontouchstart={revealOverlay}
  onpointerdown={revealOverlay}
  onclick={revealOverlay}
  onfocusin={keepOverlay}
  onmouseleave={hideOverlay}
>
  <div class="card-overlay media-info-panel" class:card-overlay-visible={infoPanelVisible}>
    <div class="flex min-w-0 items-center justify-between gap-3">
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

    {#if !expanded}
      <button
        class="media-comments-compact"
        type="button"
        aria-label={`Open comments for ${item.filename}`}
        onclick={(event) => {
          event.stopPropagation();
          onOpenComments(item.id);
        }}
      >
        <span class="media-comments-count">
          <MessageCircle size={15} />
          <span>{item.commentCount}</span>
        </span>

        <span class="media-comments-preview">
          {#if latestComment}
            <span class="font-semibold text-primary">Guest</span>
            {latestComment.text}
          {:else}
            <span>Add a comment</span>
          {/if}
        </span>
      </button>
    {/if}
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

<style>
  .media-info-panel {
    min-height: 0;
    max-height: min(10rem, calc(100% - 1.5rem));
    align-items: stretch;
    flex-direction: column;
    overflow: hidden;
  }

  .media-comments-compact {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 0.6rem;
    border-top: 1px solid var(--color-glass-border-soft);
    padding-top: 0.55rem;
    color: var(--color-secondary);
    text-align: left;
  }

  .media-comments-count {
    display: inline-flex;
    min-width: 2.75rem;
    align-items: center;
    gap: 0.3rem;
    color: var(--color-muted);
    font-size: 0.75rem;
    font-weight: 800;
    line-height: 1;
  }

  .media-comments-preview {
    display: -webkit-box;
    min-width: 0;
    overflow: hidden;
    color: var(--color-secondary);
    font-size: 0.78rem;
    font-weight: 600;
    line-height: 1.35;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
  }

  .media-comments-compact:hover .media-comments-preview,
  .media-comments-compact:hover .media-comments-count {
    color: var(--color-primary);
  }

  @media (width < 520px) {
    .media-comments-compact {
      align-items: flex-start;
    }

    .media-info-panel {
      max-height: min(9.25rem, calc(100% - 1.5rem));
    }
  }
</style>
