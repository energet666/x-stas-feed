<script lang="ts">
  import { Heart, LoaderCircle, MessageCircle, PanelRightOpen, PencilLine, X } from 'lucide-svelte';
  import type { ActivityItem } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    items,
    loading,
    error,
    onSelect
  }: {
    items: ActivityItem[];
    loading: boolean;
    error: string | null;
    onSelect: (item: ActivityItem) => void;
  } = $props();

  let mobileOpen = $state(false);

  $effect(() => {
    if (!mobileOpen) return;

    const previousOverflow = document.body.style.overflow;
    const previousOverscrollBehavior = document.body.style.overscrollBehavior;

    document.body.style.overflow = 'hidden';
    document.body.style.overscrollBehavior = 'none';

    return () => {
      document.body.style.overflow = previousOverflow;
      document.body.style.overscrollBehavior = previousOverscrollBehavior;
    };
  });

  function formatActivityTime(value: string) {
    return new Date(value).toLocaleString();
  }

  function selectActivity(item: ActivityItem) {
    mobileOpen = false;
    onSelect(item);
  }

  function formatBoardAuthors(authors: string[]) {
    const names = authors.filter(Boolean);
    if (names.length === 0) return t.common.guest;
    if (names.length <= 3) return names.join(', ');
    return `${names.slice(0, 3).join(', ')} +${names.length - 3}`;
  }
</script>

<button
  class="activity-mobile-toggle ui-icon-button"
  type="button"
  aria-label={t.activity.open}
  onclick={() => (mobileOpen = true)}
>
  <PanelRightOpen size={18} />
</button>

{#if mobileOpen}
  <button class="activity-mobile-backdrop" type="button" aria-label={t.activity.close} onclick={() => (mobileOpen = false)}></button>
{/if}

<aside class="activity-panel ui-panel ui-panel-side" class:activity-panel-mobile-open={mobileOpen} aria-label={t.activity.label}>
  <button class="activity-close ui-icon-button" type="button" aria-label={t.activity.close} onclick={() => (mobileOpen = false)}>
    <X size={17} />
  </button>

  <div class="activity-list">
    {#if loading && items.length === 0}
      <div class="activity-empty">
        <LoaderCircle class="animate-spin text-muted" size={26} />
      </div>
    {:else if error}
      <div class="activity-empty px-4 text-center">
        <p class="text-sm font-semibold text-danger">{error}</p>
      </div>
    {:else if items.length === 0}
      <div class="activity-empty px-4 text-center">
        <MessageCircle class="text-subtle" size={28} />
        <p class="text-sm font-semibold text-muted">{t.activity.empty}</p>
      </div>
    {:else}
      {#each items as item (item.type === 'comment' ? `comment-${item.comment.id}` : `board-${item.mediaId}`)}
        <button class="activity-row" type="button" title={formatActivityTime(item.type === 'comment' ? item.comment.createdAt : item.updatedAt)} onclick={() => selectActivity(item)}>
          {#if item.type === 'comment'}
            <span class="activity-row-media">
              <MessageCircle size={13} />
              <span class="truncate">{item.mediaDisplayName}</span>
            </span>
            <span class="activity-row-author">
              <span class="truncate font-semibold text-primary">{item.comment.author || t.common.guest}</span>
              <time datetime={item.comment.createdAt}>{formatActivityTime(item.comment.createdAt)}</time>
              {#if item.comment.likeCount > 0}
                <span class="activity-row-likes" aria-label={t.likes.count(item.comment.likeCount)}>
                  <Heart size={13} />
                  <span>{item.comment.likeCount}</span>
                </span>
              {/if}
            </span>
            <span class="activity-row-text">{item.comment.text}</span>
          {:else}
            <span class="activity-row-media">
              <PencilLine size={13} />
              <span class="truncate">{item.boardName}</span>
            </span>
            <span class="activity-row-author">
              <span class="activity-row-author-name truncate font-semibold text-primary" title={item.authors.join(', ') || t.common.guest}>
                {formatBoardAuthors(item.authors)}
              </span>
              <time datetime={item.updatedAt}>{formatActivityTime(item.updatedAt)}</time>
            </span>
            <span class="activity-row-text">
              {t.activity.addedStrokes(item.strokeCount)}
            </span>
          {/if}
        </button>
      {/each}
    {/if}
  </div>
</aside>

<style>
  .activity-mobile-toggle {
    position: fixed;
    right: 0.85rem;
    bottom: 4.25rem;
    z-index: 35;
    display: none;
  }

  .activity-mobile-backdrop {
    position: fixed;
    inset: 0;
    z-index: 34;
    display: none;
    background: rgb(0 0 0 / 0.52);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
  }

  .activity-panel {
    position: sticky;
    top: 1rem;
    align-self: start;
    z-index: 13;
    display: flex;
    width: var(--desktop-activity-rail-width);
    max-height: calc(100vh - 2rem);
    flex-direction: column;
    margin-top: 1rem;
    overflow: hidden;
  }

  .activity-close {
    display: none;
  }

  .activity-list {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 0.5rem;
    -webkit-overflow-scrolling: touch;
  }

  .activity-empty {
    display: flex;
    min-height: 12rem;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
  }

  .activity-row {
    display: flex;
    width: 100%;
    flex-direction: column;
    gap: 0.42rem;
    border-radius: var(--radius-overlay);
    padding: 0.72rem 0.78rem;
    color: var(--color-text-secondary);
    text-align: left;
    transition:
      background 140ms ease,
      transform 140ms ease;
  }

  .activity-row:hover {
    background: var(--color-action-bg);
    transform: translateY(-1px);
  }

  .activity-row-author {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 0.4rem;
    font-size: 0.86rem;
    line-height: 1.28;
  }

  .activity-row-likes {
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
    gap: 0.18rem;
    margin-left: auto;
    color: var(--color-text-muted);
    font-size: 0.78rem;
    font-weight: 700;
    line-height: 1;
  }

  .activity-row-likes :global(svg) {
    fill: color-mix(in srgb, var(--color-text-danger) 22%, transparent);
    stroke: var(--color-text-danger);
  }

  .activity-row-author-name {
    min-width: 0;
  }

  .activity-row-author time {
    position: absolute;
    height: 1px;
    width: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  .activity-row-media {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 0.4rem;
    color: var(--color-text-muted);
    font-size: 0.8rem;
    font-weight: 650;
    line-height: 1.28;
  }

  .activity-row-media :global(svg) {
    flex: 0 0 0.8125rem;
    height: 0.8125rem;
    width: 0.8125rem;
  }

  .activity-row-text {
    display: -webkit-box;
    overflow: hidden;
    color: var(--color-text-secondary);
    font-size: 0.9rem;
    font-weight: 500;
    line-height: 1.45;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    line-clamp: 3;
  }

  @media (width < 1344px) {
    .activity-mobile-toggle {
      display: grid;
    }

    .activity-mobile-backdrop {
      display: block;
    }

    .activity-panel {
      position: fixed;
      top: 0.75rem;
      right: 0.75rem;
      bottom: 0.75rem;
      z-index: 36;
      width: min(21rem, calc(100vw - 1.5rem));
      height: calc(100dvh - 1.5rem);
      max-height: none;
      margin-top: 0;
      transform: translateX(calc(100% + 1rem));
      transition: transform 180ms ease;
    }

    .activity-panel-mobile-open {
      transform: translateX(0);
    }

    .activity-panel-mobile-open .activity-list {
      padding-top: 3.25rem;
    }

    .activity-close {
      display: grid;
      position: absolute;
      top: 0.75rem;
      right: 0.75rem;
      z-index: 1;
    }
  }
</style>
