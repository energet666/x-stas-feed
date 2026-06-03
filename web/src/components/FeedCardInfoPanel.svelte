<script lang="ts">
  import { Maximize2, Star, X } from 'lucide-svelte';
  import { formatMediaDate } from '../lib/date';
  import type { MediaItem } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    item,
    expanded,
    favorite,
    onToggleFavorite,
    onToggleExpanded
  }: {
    item: MediaItem;
    expanded: boolean;
    favorite: boolean;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
  } = $props();
</script>

<div class="flex min-w-0 items-center justify-between gap-3">
  <div class="min-w-0">
    <h2 class="truncate text-sm font-semibold text-fg-primary">{item.displayName}</h2>
    <p class="text-xs font-semibold text-fg-muted">{formatMediaDate(item.modifiedAt)}</p>
  </div>
  <div class="flex shrink-0 items-center gap-2">
    <button
      class="ui-icon-button favorite-button"
      class:favorite-button-active={favorite}
      type="button"
      aria-label={favorite ? t.media.removeFromFavorites : t.media.addToFavorites}
      title={favorite ? t.media.removeFromFavorites : t.media.addToFavorites}
      onclick={(event) => {
        event.stopPropagation();
        onToggleFavorite();
      }}
    >
      <Star size={16} fill={favorite ? 'currentColor' : 'none'} />
    </button>
    <button
      class="ui-icon-button"
      type="button"
      aria-label={expanded ? t.media.closeFullscreen : t.media.openFullscreen}
      title={expanded ? t.media.closeFullscreen : t.media.openFullscreen}
      onclick={(event) => {
        event.stopPropagation();
        onToggleExpanded();
      }}
    >
      {#if expanded}
        <X size={17} />
      {:else}
        <Maximize2 size={16} />
      {/if}
    </button>
  </div>
</div>

<style>
  .favorite-button-active {
    border-color: rgb(250 204 21 / 0.45);
    color: rgb(253 224 71);
  }
</style>
