<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { MediaItem } from '../lib/feed';
  import FeedCardCommentsPreview from './FeedCardCommentsPreview.svelte';
  import FeedCardInfoPanel from './FeedCardInfoPanel.svelte';

  let {
    item,
    expanded,
    favorite,
    ambientActive = true,
    overlayVisible,
    content,
    ambientBackground,
    contentOverlay,
    topAccessory,
    bottomAccessory,
    likePending = false,
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
    ambientActive?: boolean;
    overlayVisible: boolean;
    content: Snippet;
    ambientBackground?: Snippet;
    contentOverlay?: Snippet;
    topAccessory?: Snippet;
    bottomAccessory?: Snippet;
    likePending?: boolean;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  function eventIsInsideCurrentTarget(event: PointerEvent | MouseEvent) {
    const target = event.currentTarget;
    if (!(target instanceof HTMLElement)) return false;
    const bounds = target.getBoundingClientRect();
    return (
      event.clientX >= bounds.left &&
      event.clientX <= bounds.right &&
      event.clientY >= bounds.top &&
      event.clientY <= bounds.bottom
    );
  }

  function keepOverlayFromPanel(event: PointerEvent | MouseEvent | TouchEvent | FocusEvent) {
    event.stopPropagation();
    onKeep();
  }

  function releaseOverlayFromPanel(event: PointerEvent | MouseEvent) {
    event.stopPropagation();
    if (eventIsInsideCurrentTarget(event)) {
      onKeep();
      return;
    }
    onReveal();
  }

  function handleFrameClick(event: MouseEvent) {
    const target = event.target;
    if (target instanceof Element && target.closest('.feed-card-panel')) {
      onKeep();
      return;
    }
    onReveal();
  }
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
  onclick={handleFrameClick}
  onfocusin={onKeep}
  onmouseleave={onHide}
>
  <div class="feed-card-ambient-bg" aria-hidden="true">
    {#if ambientActive}
      {#if ambientBackground}
        {@render ambientBackground()}
      {:else if item.type === 'video'}
        <!-- svelte-ignore a11y_media_has_caption -->
        <video
          src={item.url}
          muted
          loop
          playsinline
          autoplay
          class="ambient-media"
        ></video>
      {:else if item.type === 'image'}
        <img
          src={item.url}
          alt=""
          class="ambient-media"
          decoding="async"
        />
      {/if}
    {/if}
    <div class="feed-card-ambient-grid"></div>
  </div>

  <div class="feed-card-content">
    {@render content()}
  </div>

  {#if contentOverlay}
    <div class="feed-card-content-overlay">
      {@render contentOverlay()}
    </div>
  {/if}

  <div class="feed-card-top-stack feed-card-overlay" class:feed-card-overlay-visible={overlayVisible}>
    <section
      class="feed-card-panel"
      aria-label="Media information"
      onpointerenter={keepOverlayFromPanel}
      onpointermove={keepOverlayFromPanel}
      onpointerdown={keepOverlayFromPanel}
      onpointerleave={releaseOverlayFromPanel}
      onmouseenter={keepOverlayFromPanel}
      onmousemove={keepOverlayFromPanel}
      onmouseleave={releaseOverlayFromPanel}
      ontouchstart={keepOverlayFromPanel}
      onfocusin={keepOverlayFromPanel}
    >
      <FeedCardInfoPanel {item} {expanded} {favorite} {onToggleFavorite} {onToggleExpanded} />
    </section>

    {#if topAccessory}
      <section
        class="feed-card-panel"
        aria-label="Media actions"
        onpointerenter={keepOverlayFromPanel}
        onpointermove={keepOverlayFromPanel}
        onpointerdown={keepOverlayFromPanel}
        onpointerleave={releaseOverlayFromPanel}
        onmouseenter={keepOverlayFromPanel}
        onmousemove={keepOverlayFromPanel}
        onmouseleave={releaseOverlayFromPanel}
        ontouchstart={keepOverlayFromPanel}
        onfocusin={keepOverlayFromPanel}
      >
        {@render topAccessory()}
      </section>
    {/if}
  </div>

  <div class="feed-card-bottom-stack">
    {#if bottomAccessory}
      <div class="feed-card-bottom-accessory" class:feed-card-bottom-accessory-visible={overlayVisible}>
        <div class="feed-card-bottom-accessory-inner">
          <section
            class="feed-card-panel"
            aria-label="Media controls"
            onpointerenter={keepOverlayFromPanel}
            onpointermove={keepOverlayFromPanel}
            onpointerdown={keepOverlayFromPanel}
            onpointerleave={releaseOverlayFromPanel}
            onmouseenter={keepOverlayFromPanel}
            onmousemove={keepOverlayFromPanel}
            onmouseleave={releaseOverlayFromPanel}
            ontouchstart={keepOverlayFromPanel}
            onfocusin={keepOverlayFromPanel}
          >
            {@render bottomAccessory()}
          </section>
        </div>
      </div>
    {/if}

    <section
      class="feed-card-panel"
      aria-label="Comment summary"
      onpointerenter={keepOverlayFromPanel}
      onpointermove={keepOverlayFromPanel}
      onpointerdown={keepOverlayFromPanel}
      onpointerleave={releaseOverlayFromPanel}
      onmouseenter={keepOverlayFromPanel}
      onmousemove={keepOverlayFromPanel}
      onmouseleave={releaseOverlayFromPanel}
      ontouchstart={keepOverlayFromPanel}
      onfocusin={keepOverlayFromPanel}
    >
      <FeedCardCommentsPreview {item} {likePending} {onOpenComments} {onLike} />
    </section>
  </div>
</div>

<style>
  .feed-card-content {
    position: absolute;
    inset: 0;
    z-index: 2;
  }


  .feed-card-content-overlay {
    position: absolute;
    inset: 0;
    z-index: 4;
    pointer-events: none;
  }

  .feed-card-content-overlay :global(button),
  .feed-card-content-overlay :global(input),
  .feed-card-content-overlay :global(select),
  .feed-card-content-overlay :global(textarea),
  .feed-card-content-overlay :global(a) {
    pointer-events: auto;
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
    transform: translateY(calc(-100% - 1.5rem));
  }

  .feed-card-bottom-stack {
    position: absolute;
    right: 0.75rem;
    bottom: 0.75rem;
    left: 0.75rem;
    z-index: 6;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .feed-card-overlay-visible {
    pointer-events: auto;
    transform: translateY(0);
  }

  .feed-card-bottom-accessory {
    display: grid;
    grid-template-rows: 0fr;
    overflow: hidden;
    pointer-events: none;
    transition: grid-template-rows 180ms ease;
  }

  .feed-card-bottom-accessory-visible {
    grid-template-rows: 1fr;
    pointer-events: auto;
  }

  .feed-card-bottom-accessory-inner {
    min-height: 0;
    transform: translateY(0.75rem);
    transition: transform 180ms ease;
  }

  .feed-card-bottom-accessory-visible .feed-card-bottom-accessory-inner {
    transform: translateY(0);
  }

  .feed-card-panel {
    min-width: 0;
    padding: 0.6rem 0.75rem;
    border-radius: var(--radius-overlay);
    background: var(--background-image-glass-overlay);
    box-shadow: var(--shadow-overlay);
    color: var(--color-primary);
    backdrop-filter: blur(10px) saturate(140%);
    -webkit-backdrop-filter: blur(10px) saturate(140%);
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
