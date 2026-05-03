<script lang="ts">
  import { Maximize2, X } from 'lucide-svelte';
  import { formatMediaDate } from '../lib/date';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    expanded,
    onToggleExpanded
  }: {
    item: MediaItem;
    expanded: boolean;
    onToggleExpanded: () => void;
  } = $props();
</script>

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
