<script lang="ts">
  import { LoaderCircle, MessageCircle, PanelRightOpen, RefreshCw, X } from 'lucide-svelte';
  import type { ActivityItem } from '../lib/feed';

  let {
    items,
    loading,
    error,
    onReload,
    onSelect
  }: {
    items: ActivityItem[];
    loading: boolean;
    error: string | null;
    onReload: () => void;
    onSelect: (item: ActivityItem) => void;
  } = $props();

  let mobileOpen = $state(false);

  function selectActivity(item: ActivityItem) {
    mobileOpen = false;
    onSelect(item);
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

<aside class="activity-panel glass-panel" class:activity-panel-mobile-open={mobileOpen} aria-label="Social activity">
  <header class="activity-panel-header">
    <div class="min-w-0">
      <p class="text-xs font-semibold uppercase text-subtle">Activity</p>
      <h2 class="truncate text-base font-bold text-primary">Latest comments</h2>
    </div>
    <div class="flex items-center gap-2">
      <button class="glass-icon-button" type="button" aria-label="Reload activity" disabled={loading} onclick={onReload}>
        {#if loading}
          <LoaderCircle class="animate-spin" size={17} />
        {:else}
          <RefreshCw size={17} />
        {/if}
      </button>
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
        <p class="text-sm font-semibold text-muted">No comments yet</p>
      </div>
    {:else}
      {#each items as item (item.comment.id)}
        <button class="activity-row" type="button" onclick={() => selectActivity(item)}>
          <span class="activity-row-meta">
            <span class="truncate font-extrabold text-primary">{item.comment.author || 'Guest'}</span>
            <time datetime={item.comment.createdAt}>{new Date(item.comment.createdAt).toLocaleString()}</time>
          </span>
          <span class="activity-row-media">
            <MessageCircle size={13} />
            <span class="truncate">{item.mediaDisplayName}</span>
          </span>
          <span class="activity-row-text">{item.comment.text}</span>
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
    top: 5.25rem;
    right: max(1rem, calc((100vw - 78rem) / 2));
    z-index: 13;
    display: flex;
    width: 18rem;
    max-height: calc(100vh - 6.25rem);
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
    gap: 0.35rem;
    border-radius: var(--radius-overlay);
    padding: 0.65rem 0.7rem;
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

  .activity-row-meta {
    display: flex;
    min-width: 0;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.5rem;
    font-size: 0.76rem;
    line-height: 1.2;
  }

  .activity-row-meta time {
    flex: 0 0 auto;
    color: var(--color-subtle);
    font-size: 0.66rem;
    font-weight: 700;
    white-space: nowrap;
  }

  .activity-row-media {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 0.35rem;
    color: var(--color-muted);
    font-size: 0.72rem;
    font-weight: 800;
    line-height: 1.2;
  }

  .activity-row-text {
    display: -webkit-box;
    overflow: hidden;
    color: var(--color-secondary);
    font-size: 0.8rem;
    font-weight: 600;
    line-height: 1.35;
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
