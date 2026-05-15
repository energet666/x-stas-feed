<script lang="ts">
  import { LoaderCircle, MessageCircle, PanelRightOpen, PencilLine, X } from 'lucide-svelte';
  import type { ActivityItem } from '../lib/feed';

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

  function formatActivityTime(value: string) {
    return new Date(value).toLocaleString();
  }

  function selectActivity(item: ActivityItem) {
    mobileOpen = false;
    onSelect(item);
  }

  function formatBoardAuthors(authors: string[]) {
    const names = authors.filter(Boolean);
    if (names.length === 0) return 'Guest';
    if (names.length <= 3) return names.join(', ');
    return `${names.slice(0, 3).join(', ')} +${names.length - 3}`;
  }
</script>

<button
  class="activity-mobile-toggle glass-icon-button"
  type="button"
  aria-label="Open social activity"
  onclick={() => (mobileOpen = true)}
>
  <PanelRightOpen size={18} />
</button>

{#if mobileOpen}
  <button class="activity-mobile-backdrop" type="button" aria-label="Close social activity" onclick={() => (mobileOpen = false)}></button>
{/if}

<aside class="activity-panel glass-panel side-glass-panel" class:activity-panel-mobile-open={mobileOpen} aria-label="Social activity">
  <header class="activity-panel-header">
    <div class="min-w-0">
      <p class="text-xs font-semibold uppercase text-subtle">Social</p>
      <h2 class="truncate text-base font-bold text-primary">Activity</h2>
    </div>
    <div class="flex items-center gap-2">
      <button class="activity-close glass-icon-button" type="button" aria-label="Close social activity" onclick={() => (mobileOpen = false)}>
        <X size={17} />
      </button>
    </div>
  </header>

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
        <p class="text-sm font-semibold text-muted">No activity yet</p>
      </div>
    {:else}
      {#each items as item (item.type === 'comment' ? `comment-${item.comment.id}` : `board-${item.boardId}`)}
        <button class="activity-row" type="button" title={formatActivityTime(item.type === 'comment' ? item.comment.createdAt : item.updatedAt)} onclick={() => selectActivity(item)}>
          {#if item.type === 'comment'}
            <span class="activity-row-media">
              <MessageCircle size={13} />
              <span class="truncate">{item.mediaDisplayName}</span>
            </span>
            <span class="activity-row-author">
              <span class="truncate font-semibold text-primary">{item.comment.author || 'Guest'}</span>
              <time datetime={item.comment.createdAt}>{formatActivityTime(item.comment.createdAt)}</time>
            </span>
            <span class="activity-row-text">{item.comment.text}</span>
          {:else}
            <span class="activity-row-media">
              <PencilLine size={13} />
              <span class="truncate">{item.boardName}</span>
            </span>
            <span class="activity-row-author">
              <span class="activity-row-author-name truncate font-semibold text-primary" title={item.authors.join(', ') || 'Guest'}>
                {formatBoardAuthors(item.authors)}
              </span>
              <time datetime={item.updatedAt}>{formatActivityTime(item.updatedAt)}</time>
            </span>
            <span class="activity-row-text">
              {item.strokeCount === 1 ? 'added 1 stroke' : `added ${item.strokeCount} strokes`}
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
    position: fixed;
    top: 1rem;
    right: max(1rem, calc((100vw - 78rem) / 2));
    z-index: 13;
    display: flex;
    width: 19rem;
    max-height: calc(100vh - 2rem);
    flex-direction: column;
    overflow: hidden;
  }

  .activity-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    border-bottom: 1px solid var(--color-glass-border-soft);
    padding: 1rem;
  }

  .activity-close {
    display: none;
  }

  .activity-list {
    min-height: 0;
    overflow-y: auto;
    padding: 0.5rem;
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
    color: var(--color-secondary);
    text-align: left;
    transition:
      background 140ms ease,
      transform 140ms ease;
  }

  .activity-row:hover {
    background: var(--color-button-bg);
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
    color: var(--color-muted);
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
    color: var(--color-secondary);
    font-size: 0.9rem;
    font-weight: 500;
    line-height: 1.45;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    line-clamp: 3;
  }

  @media (width < 1280px) {
    .activity-mobile-toggle {
      display: grid;
    }

    .activity-mobile-backdrop {
      display: block;
    }

    .activity-panel {
      top: 0.75rem;
      right: 0.75rem;
      bottom: 0.75rem;
      z-index: 36;
      width: min(21rem, calc(100vw - 1.5rem));
      max-height: none;
      transform: translateX(calc(100% + 1rem));
      transition: transform 180ms ease;
    }

    .activity-panel-mobile-open {
      transform: translateX(0);
    }

    .activity-close {
      display: grid;
    }
  }
</style>
