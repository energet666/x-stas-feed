<script lang="ts">
  import { X } from 'lucide-svelte';
  import CommentThread from './CommentThread.svelte';
  import type { Comment, CommentLikeEvent, MediaItem } from '../lib/feed';

  let {
    item,
    username,
    commentEvent,
    commentLikeEvent,
    onClose,
    onCommentsChanged,
    onCommentLikeChanged
  }: {
    item: MediaItem | undefined;
    username: string;
    commentEvent: { mediaId: string; comment: Comment } | null;
    commentLikeEvent: CommentLikeEvent | null;
    onClose: () => void;
    onCommentsChanged: (mediaId: string, comments: Comment[]) => void;
    onCommentLikeChanged: (mediaId: string, commentId: string, likeCount: number) => void;
  } = $props();
</script>

{#if item}
  <aside class="comments-panel" aria-label={`Comments for ${item.displayName}`}>
    <header class="flex items-center justify-between gap-3 border-b border-glass-border-soft px-4 py-3">
      <div class="min-w-0">
        <p class="text-xs font-semibold uppercase text-subtle">Comments</p>
        <h2 class="truncate text-base font-semibold text-primary">{item.displayName}</h2>
      </div>
      <button class="glass-icon-button" type="button" aria-label="Close comments" onclick={onClose}>
        <X size={18} />
      </button>
    </header>

    <CommentThread
      {item}
      {username}
      {commentEvent}
      {commentLikeEvent}
      {onCommentsChanged}
      {onCommentLikeChanged}
    />
  </aside>
{/if}

<style>
  .comments-panel {
    position: absolute;
    inset: 0;
    z-index: 24;
    display: flex;
    width: 100%;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--color-glass-border);
    border-radius: inherit;
    background:
      linear-gradient(180deg, rgb(0 0 0 / 0.62), rgb(0 0 0 / 0.48)),
      var(--background-image-glass-strong);
    box-shadow: var(--shadow-popover);
    color: var(--color-primary);
    backdrop-filter: blur(26px) saturate(170%);
    -webkit-backdrop-filter: blur(26px) saturate(170%);
  }

  @media (width < 720px) {
    .comments-panel {
      border-radius: inherit;
    }
  }
</style>
