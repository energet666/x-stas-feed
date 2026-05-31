<script lang="ts">
  import { LoaderCircle, X } from 'lucide-svelte';
  import CommentThread from './CommentThread.svelte';
  import MediaCard from './MediaCard.svelte';
  import type { Comment, CommentLikeEvent, CommentEvent, MediaItem } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    item,
    loading,
    error,
    username,
    commentEvent,
    commentLikeEvent,
    likePending = false,
    debugToolsEnabled = false,
    onClose,
    onCommentsChanged,
    onCommentLikeChanged,
    onOpenBoardEdit,
    onLike
  }: {
    item: MediaItem | null;
    loading: boolean;
    error: string | null;
    username: string;
    commentEvent: CommentEvent | null;
    commentLikeEvent: CommentLikeEvent | null;
    likePending?: boolean;
    debugToolsEnabled?: boolean;
    onClose: () => void;
    onCommentsChanged: (mediaId: string, comments: Comment[]) => void;
    onCommentLikeChanged: (mediaId: string, commentId: string, likeCount: number) => void;
    onOpenBoardEdit: (mediaId: string) => void;
    onLike: (mediaId: string) => void;
  } = $props();

  let overlayVisible = $state(true);
</script>

<div class="activity-modal-backdrop" role="presentation" onclick={onClose}></div>
<div class="activity-modal" role="dialog" aria-modal="true" aria-label={t.activity.mediaDialog}>
  <header class="activity-modal-header">
    <div class="min-w-0">
      <p class="text-xs font-semibold uppercase text-subtle">{item?.type === 'board' ? t.activity.boardActivity : t.activity.commentActivity}</p>
      <h2 class="truncate text-base font-bold text-primary">{item?.displayName ?? t.common.loadingMedia}</h2>
    </div>
    <button class="ui-icon-button" type="button" aria-label={t.activity.closeMedia} onclick={onClose}>
      <X size={18} />
    </button>
  </header>

  {#if loading}
    <div class="activity-modal-loading">
      <LoaderCircle class="animate-spin text-muted" size={30} />
    </div>
  {:else if error}
    <div class="activity-modal-loading px-6 text-center">
      <p class="text-sm font-semibold text-danger">{error}</p>
    </div>
  {:else if item}
    <div class="activity-modal-body">
      <article class="activity-modal-card ui-media-card overflow-hidden">
        <MediaCard
          {item}
          expanded={false}
          favorite={false}
          ambientActive={true}
          {overlayVisible}
          {likePending}
          {debugToolsEnabled}
          suppressFeedChrome={true}
          onReveal={() => (overlayVisible = true)}
          onKeep={() => (overlayVisible = true)}
          onHide={() => (overlayVisible = false)}
          onToggleFavorite={() => undefined}
          onToggleExpanded={onOpenBoardEdit}
          onOpenComments={() => undefined}
          onLike={() => onLike(item.id)}
        />
      </article>

      <aside class="activity-modal-comments" aria-label={t.comments.forMedia(item.displayName)}>
        <CommentThread
          {item}
          {username}
          {commentEvent}
          {commentLikeEvent}
          {onCommentsChanged}
          {onCommentLikeChanged}
        />
      </aside>
    </div>
  {/if}
</div>

<style>
  .activity-modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: 94;
    background: rgb(0 0 0 / 0.76);
    backdrop-filter: blur(18px) saturate(130%);
    -webkit-backdrop-filter: blur(18px) saturate(130%);
  }

  .activity-modal {
    position: fixed;
    inset: clamp(0.75rem, 3vw, 2rem);
    z-index: 95;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--color-border-glass);
    border-radius: var(--radius-media-card);
    background:
      linear-gradient(180deg, rgb(0 0 0 / 0.66), rgb(0 0 0 / 0.5)),
      var(--background-image-glass-strong);
    color: var(--color-text-primary);
    backdrop-filter: blur(26px) saturate(170%);
    -webkit-backdrop-filter: blur(26px) saturate(170%);
  }

  .activity-modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    border-bottom: 1px solid var(--color-border-glass-soft);
    padding: 0.85rem 1rem;
  }

  .activity-modal-loading {
    display: flex;
    min-height: 20rem;
    flex: 1;
    align-items: center;
    justify-content: center;
  }

  .activity-modal-body {
    display: grid;
    min-height: 0;
    flex: 1;
    grid-template-columns: minmax(0, 1fr) minmax(20rem, 26rem);
    gap: 1px;
    background: var(--color-border-glass-soft);
  }

  .activity-modal-card {
    min-height: 0;
    border: 0;
    border-radius: 0;
    background: var(--color-surface-media);
  }

  .activity-modal-card :global(.media-frame) {
    height: 100%;
    min-height: clamp(28rem, 72vh, 52rem);
  }

  .activity-modal-comments {
    display: flex;
    min-height: 0;
    flex-direction: column;
    background: rgb(0 0 0 / 0.24);
  }

  @media (width < 900px) {
    .activity-modal {
      inset: 0.75rem;
    }

    .activity-modal-body {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: minmax(20rem, 54vh) minmax(16rem, 1fr);
    }

    .activity-modal-card :global(.media-frame) {
      min-height: 20rem;
    }
  }
</style>
