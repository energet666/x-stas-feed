<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { MediaItem } from '../lib/feed';
  import FeedCardCommentsPreview from './FeedCardCommentsPreview.svelte';
  import FeedCardInfoPanel from './FeedCardInfoPanel.svelte';
  import { uiText as t } from '../lib/ui_text';

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
    suppressFeedChrome = false,
    suppressOverlays = false,
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
    suppressFeedChrome?: boolean;
    suppressOverlays?: boolean;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  const showFeedChrome = $derived(!suppressOverlays && !suppressFeedChrome);
  const hasTopOverlay = $derived(showFeedChrome || topAccessory !== undefined);
  const hasBottomOverlay = $derived(showFeedChrome || bottomAccessory !== undefined);
  const showOverlayLayer = $derived(!suppressOverlays && (hasTopOverlay || hasBottomOverlay));

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
    if (suppressOverlays) return;
    event.stopPropagation();
    onKeep();
  }

  function releaseOverlayFromPanel(event: PointerEvent | MouseEvent) {
    if (suppressOverlays) return;
    event.stopPropagation();
    if (eventIsInsideCurrentTarget(event)) {
      onKeep();
      return;
    }
    onReveal();
  }

  function handleFrameClick(event: MouseEvent) {
    if (suppressOverlays) return;
    const target = event.target;
    if (target instanceof Element && target.closest('.feed-card-panel')) {
      onKeep();
      return;
    }
    onReveal();
  }

  function revealOverlay() {
    if (!suppressOverlays) onReveal();
  }

  function keepOverlay() {
    if (!suppressOverlays) onKeep();
  }

  function hideOverlay() {
    if (!suppressOverlays) onHide();
  }
</script>

<div
  class="media-frame"
  role="presentation"
  class:media-frame-overlays-suppressed={suppressOverlays}
  onpointermove={revealOverlay}
  onpointerenter={revealOverlay}
  onmousemove={revealOverlay}
  onmouseenter={revealOverlay}
  ontouchstart={revealOverlay}
  onpointerdown={revealOverlay}
  onclick={handleFrameClick}
  onfocusin={keepOverlay}
  onmouseleave={hideOverlay}
>
  <div class="feed-card-ambient-bg" aria-hidden="true">
    {#if ambientActive}
      {#if ambientBackground}
        {@render ambientBackground()}
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

  {#if showOverlayLayer}
    {#if hasTopOverlay}
      <div
        class="feed-card-top-stack feed-card-overlay"
        class:feed-card-overlay-visible={overlayVisible}
        class:feed-card-overlay-expanded={expanded}
      >
        {#if showFeedChrome}
          <section
            class="feed-card-panel"
            aria-label={t.media.information}
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
        {/if}

        {#if topAccessory}
          <section
            class="feed-card-panel"
            aria-label={t.media.actions}
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
    {/if}

    {#if hasBottomOverlay}
      <div class="feed-card-bottom-stack" class:feed-card-overlay-expanded={expanded}>
        {#if bottomAccessory}
          <div class="feed-card-bottom-accessory" class:feed-card-bottom-accessory-visible={overlayVisible}>
            <div class="feed-card-bottom-accessory-inner">
              <section
                class="feed-card-panel"
                aria-label={t.media.controls}
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

        {#if showFeedChrome}
          <section
            class="feed-card-panel"
            aria-label={t.media.commentSummary}
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
        {/if}
      </div>
    {/if}
  {/if}
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
    transform: none;
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
    transform: none;
  }

  .feed-card-panel {
    min-width: 0;
    padding: 0.6rem 0.75rem;
    border-radius: var(--radius-overlay);
    background: var(--background-image-glass-overlay);
    box-shadow: var(--shadow-overlay);
    color: var(--color-fg-primary);
    backdrop-filter: blur(10px) saturate(140%);
    -webkit-backdrop-filter: blur(10px) saturate(140%);
  }

  .feed-card-overlay-expanded {
    right: max(0.75rem, calc((100vw - 42rem) / 2));
    left: max(0.75rem, calc((100vw - 42rem) / 2));
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
