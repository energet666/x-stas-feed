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
  const showSocialChin = $derived(showFeedChrome && !expanded);
  const hasTopOverlay = $derived(showFeedChrome || topAccessory !== undefined);
  const hasBottomOverlay = $derived(bottomAccessory !== undefined);
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

<div class="feed-card-frame">
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

      {#if bottomAccessory}
        <div class="feed-card-bottom-stack" class:feed-card-overlay-expanded={expanded}>
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
        </div>
      {/if}
    {/if}
  </div>

  {#if showSocialChin}
    <section class="feed-card-social-chin" aria-label={t.media.commentSummary}>
      <FeedCardCommentsPreview {item} {likePending} {onOpenComments} {onLike} />
    </section>
  {/if}
</div>

<style>
  .feed-card-frame {
    --feed-card-overlay-inset: 0.75rem;
    position: relative;
  }

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
    right: var(--feed-card-overlay-inset);
    left: var(--feed-card-overlay-inset);
    z-index: 6;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    pointer-events: none;
    transition: transform 180ms ease;
  }

  .feed-card-top-stack {
    top: var(--feed-card-overlay-inset);
    transform: translateY(calc(-100% - 1.5rem));
  }

  .feed-card-bottom-stack {
    position: absolute;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 6;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
    padding: 0 var(--feed-card-overlay-inset) var(--feed-card-overlay-inset);
    pointer-events: none;
  }

  .feed-card-overlay-visible {
    pointer-events: auto;
    transform: none;
  }

  .feed-card-bottom-accessory {
    display: block;
    pointer-events: none;
    transform: translateY(calc(100% + var(--feed-card-overlay-inset)));
    transition: transform 180ms ease;
  }

  .feed-card-bottom-accessory-visible {
    pointer-events: auto;
    transform: none;
  }

  .feed-card-bottom-accessory-inner {
    min-height: 0;
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

  .feed-card-social-chin {
    position: relative;
    z-index: 3;
    padding: 0.75rem 0.9rem 0.85rem;
    border-top: 1px solid rgb(255 255 255 / 0.1);
    background:
      linear-gradient(180deg, rgb(255 255 255 / 0.035), rgb(255 255 255 / 0.015)),
      rgb(0 0 0 / 0.72);
    color: var(--color-fg-primary);
  }

  .feed-card-overlay-expanded {
    right: max(0.75rem, calc((100vw - 42rem) / 2));
    left: max(0.75rem, calc((100vw - 42rem) / 2));
  }

  @media (width < 520px) {
    .feed-card-frame {
      --feed-card-overlay-inset: 0.65rem;
    }

    .feed-card-social-chin {
      padding: 0.7rem 0.75rem 0.8rem;
    }
  }
</style>
