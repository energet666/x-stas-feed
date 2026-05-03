<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { MediaItem } from '../lib/feed';
  import FeedCardCommentsPreview from './FeedCardCommentsPreview.svelte';
  import FeedCardInfoPanel from './FeedCardInfoPanel.svelte';

  let {
    item,
    expanded,
    overlayVisible,
    content,
    topAccessory,
    bottomAccessory,
    onReveal,
    onKeep,
    onHide,
    onToggleExpanded,
    onOpenComments
  }: {
    item: MediaItem;
    expanded: boolean;
    overlayVisible: boolean;
    content: Snippet;
    topAccessory?: Snippet;
    bottomAccessory?: Snippet;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
  } = $props();
</script>

<div
  class="media-frame"
  role="presentation"
  onpointermove={onReveal}
  onpointerenter={onReveal}
  onmousemove={onReveal}
  onmouseenter={onReveal}
  ontouchstart={onReveal}
  onpointerdown={onReveal}
  onclick={onReveal}
  onfocusin={onKeep}
  onmouseleave={onHide}
>
  <div class="feed-card-content">
    {@render content()}
  </div>

  <div class="feed-card-top-stack feed-card-overlay" class:feed-card-overlay-visible={overlayVisible}>
    <section class="feed-card-panel" aria-label="Media information">
      <FeedCardInfoPanel {item} {expanded} {onToggleExpanded} />
    </section>

    {#if topAccessory}
      <section class="feed-card-panel" aria-label="Media actions">
        {@render topAccessory()}
      </section>
    {/if}
  </div>

  <div class="feed-card-bottom-stack feed-card-overlay" class:feed-card-overlay-visible={overlayVisible}>
    {#if bottomAccessory}
      <section class="feed-card-panel feed-card-accessory-panel" aria-label="Media controls">
        {@render bottomAccessory()}
      </section>
    {/if}

    <section class="feed-card-panel" aria-label="Comment summary">
      <FeedCardCommentsPreview {item} {onOpenComments} />
    </section>
  </div>
</div>

<style>
  .feed-card-content {
    position: absolute;
    inset: 0;
    z-index: 1;
  }

  .feed-card-overlay {
    position: absolute;
    right: 0.75rem;
    left: 0.75rem;
    z-index: 6;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    pointer-events: none;
    transition: transform 180ms ease;
  }

  .feed-card-top-stack {
    top: 0.75rem;
    transform: translateY(calc(-100% - 0.75rem));
  }

  .feed-card-bottom-stack {
    bottom: 0.75rem;
    transform: translateY(calc(100% + 0.75rem));
  }

  .feed-card-overlay-visible {
    pointer-events: auto;
    transform: translateY(0);
  }

  .feed-card-panel {
    min-width: 0;
    padding: 0.6rem 0.75rem;
    border-radius: var(--radius-overlay);
    background: var(--background-image-glass-overlay);
    box-shadow: var(--shadow-overlay);
    color: var(--color-primary);
    backdrop-filter: blur(28px) saturate(200%);
    -webkit-backdrop-filter: blur(28px) saturate(200%);
  }

  .feed-card-accessory-panel {
    padding: 0;
    background: transparent;
    box-shadow: none;
    overflow: hidden;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  @media (width < 520px) {
    .feed-card-overlay {
      right: 0.65rem;
      left: 0.65rem;
    }

    .feed-card-top-stack {
      top: 0.65rem;
    }

    .feed-card-bottom-stack {
      bottom: 0.65rem;
    }
  }
</style>
