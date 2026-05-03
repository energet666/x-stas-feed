<script lang="ts">
  import { MessageCircle } from 'lucide-svelte';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    onOpenComments
  }: {
    item: MediaItem;
    onOpenComments: () => void;
  } = $props();

  const latestComment = $derived(item.comments.at(-1));
</script>

<button
  class="feed-card-comments-preview"
  type="button"
  aria-label={`Open comments for ${item.filename}`}
  onclick={(event) => {
    event.stopPropagation();
    onOpenComments();
  }}
>
  <span class="feed-card-comments-count">
    <MessageCircle size={15} />
    <span>{item.commentCount}</span>
  </span>

  <span class="feed-card-comments-text">
    {#if latestComment}
      <span class="font-semibold text-primary">{latestComment.author || 'Guest'}</span>
      {latestComment.text}
    {:else}
      <span>Add a comment</span>
    {/if}
  </span>
</button>

<style>
  .feed-card-comments-preview {
    display: flex;
    min-width: 0;
    width: 100%;
    align-items: center;
    gap: 0.6rem;
    color: var(--color-secondary);
    text-align: left;
  }

  .feed-card-comments-count {
    display: inline-flex;
    min-width: 2.75rem;
    align-items: center;
    gap: 0.3rem;
    color: var(--color-muted);
    font-size: 0.75rem;
    font-weight: 800;
    line-height: 1;
  }

  .feed-card-comments-text {
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

  .feed-card-comments-preview:hover .feed-card-comments-count,
  .feed-card-comments-preview:hover .feed-card-comments-text {
    color: var(--color-primary);
  }

  @media (width < 520px) {
    .feed-card-comments-preview {
      align-items: flex-start;
    }
  }
</style>
