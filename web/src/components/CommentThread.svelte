<script lang="ts">
  import { tick } from 'svelte';
  import { Heart, LoaderCircle, MessageCircle, Smile } from 'lucide-svelte';
  import EmojiPanel from './EmojiPanel.svelte';
  import {
    createComment,
    createCommentLike,
    fetchComments,
    type Comment,
    type CommentLikeEvent,
    type MediaItem
  } from '../lib/feed';

  let {
    item,
    username,
    commentEvent,
    commentLikeEvent,
    onCommentsChanged,
    onCommentLikeChanged
  }: {
    item: MediaItem | undefined;
    username: string;
    commentEvent: { mediaId: string; comment: Comment } | null;
    commentLikeEvent: CommentLikeEvent | null;
    onCommentsChanged: (mediaId: string, comments: Comment[]) => void;
    onCommentLikeChanged: (mediaId: string, commentId: string, likeCount: number) => void;
  } = $props();

  let comments = $state<Comment[]>([]);
  let loading = $state(false);
  let submitting = $state(false);
  let error = $state<string | null>(null);
  let draft = $state('');
  let activeMediaId = '';
  let pendingCommentLikeCounts = $state<Record<string, number>>({});
  let commentLikeSplashIDs = $state<Record<string, boolean>>({});
  let commentListEl = $state<HTMLDivElement | undefined>(undefined);
  let commentFormEl = $state<HTMLFormElement | undefined>(undefined);
  let commentInputEl = $state<HTMLTextAreaElement | undefined>(undefined);
  let emojiPanelOpen = $state(false);

  const canSubmit = $derived(draft.trim().length > 0 && !submitting && item !== undefined);

  $effect(() => {
    if (!item) {
      activeMediaId = '';
      return;
    }
    if (activeMediaId === item.id) return;

    activeMediaId = item.id;
    comments = Array.isArray(item.comments) ? item.comments : [];
    pendingCommentLikeCounts = {};
    commentLikeSplashIDs = {};
    emojiPanelOpen = false;
    draft = '';
    scrollCommentsToBottom('auto');
    void loadComments(item.id);
  });

  $effect(() => {
    if (!commentEvent) return;
    appendComment(commentEvent.mediaId, commentEvent.comment);
  });

  $effect(() => {
    if (!commentLikeEvent || activeMediaId !== commentLikeEvent.mediaId) return;
    updateCommentLikeCount(commentLikeEvent.commentId, commentLikeEvent.likeCount, true);
  });

  async function loadComments(mediaId: string) {
    loading = true;
    error = null;

    try {
      const response = await fetchComments(mediaId);
      const nextComments = Array.isArray(response.comments) ? response.comments : [];
      comments = nextComments;
      onCommentsChanged(mediaId, nextComments);
      scrollCommentsToBottom('auto');
    } catch (err) {
      error = err instanceof Error ? err.message : 'Unable to load comments';
    } finally {
      loading = false;
    }
  }

  async function submitComment(event: SubmitEvent) {
    event.preventDefault();
    if (!item || !canSubmit) return;

    const text = draft.trim();
    submitting = true;
    error = null;

    try {
      const comment = await createComment(item.id, text, username);
      appendComment(item.id, comment);
      draft = '';
      emojiPanelOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Unable to save comment';
    } finally {
      submitting = false;
    }
  }

  function handleCommentKeydown(event: KeyboardEvent) {
    if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;

    event.preventDefault();
    commentFormEl?.requestSubmit();
  }

  function toggleEmojiPanel(event: MouseEvent) {
    event.preventDefault();
    event.stopPropagation();
    emojiPanelOpen = !emojiPanelOpen;
  }

  async function insertEmoji(emoji: string) {
    const input = commentInputEl;
    if (!input) {
      draft += emoji;
      return;
    }

    const selectionStart = input.selectionStart ?? draft.length;
    const selectionEnd = input.selectionEnd ?? selectionStart;
    draft = `${draft.slice(0, selectionStart)}${emoji}${draft.slice(selectionEnd)}`;
    await tick();
    const cursorPosition = selectionStart + emoji.length;
    input.focus();
    input.setSelectionRange(cursorPosition, cursorPosition);
  }

  function appendComment(mediaId: string, comment: Comment) {
    if (activeMediaId !== mediaId || comments.some((existing) => existing.id === comment.id)) return;

    const nextComments = [...comments, comment];
    comments = nextComments;
    onCommentsChanged(mediaId, nextComments);
    scrollCommentsToBottom('smooth');
  }

  async function likeComment(commentID: string) {
    if (!item) return;

    pendingCommentLikeCounts = {
      ...pendingCommentLikeCounts,
      [commentID]: (pendingCommentLikeCounts[commentID] ?? 0) + 1
    };
    updateCommentLikeCount(commentID, (comments.find((comment) => comment.id === commentID)?.likeCount ?? 0) + 1, true);

    try {
      const result = await createCommentLike(item.id, commentID);
      updateCommentLikeCount(commentID, result.likeCount, true);
    } catch {
      updateCommentLikeCount(
        commentID,
        Math.max(0, (comments.find((comment) => comment.id === commentID)?.likeCount ?? 1) - 1),
        false,
        true
      );
    } finally {
      const remaining = (pendingCommentLikeCounts[commentID] ?? 1) - 1;
      if (remaining > 0) {
        pendingCommentLikeCounts = { ...pendingCommentLikeCounts, [commentID]: remaining };
      } else {
        const nextPendingCounts = { ...pendingCommentLikeCounts };
        delete nextPendingCounts[commentID];
        pendingCommentLikeCounts = nextPendingCounts;
      }
    }
  }

  function updateCommentLikeCount(commentID: string, likeCount: number, animate: boolean, allowDecrease = false) {
    let changed = false;
    let changedLikeCount = likeCount;
    const nextComments = comments.map((comment) => {
      if (comment.id !== commentID) return comment;
      const nextLikeCount = allowDecrease ? Math.max(0, likeCount) : Math.max(comment.likeCount, likeCount);
      if (nextLikeCount === comment.likeCount) return comment;
      changed = true;
      changedLikeCount = nextLikeCount;
      return { ...comment, likeCount: nextLikeCount };
    });
    if (!changed) return;

    comments = nextComments;
    if (activeMediaId) {
      onCommentLikeChanged(activeMediaId, commentID, changedLikeCount);
    }
    if (animate) {
      void animateCommentLikeSplash(commentID);
    }
  }

  async function animateCommentLikeSplash(commentID: string) {
    commentLikeSplashIDs = { ...commentLikeSplashIDs, [commentID]: false };
    await tick();
    commentLikeSplashIDs = { ...commentLikeSplashIDs, [commentID]: true };
  }

  function finishCommentLikeSplash(commentID: string) {
    const nextSplashIDs = { ...commentLikeSplashIDs };
    delete nextSplashIDs[commentID];
    commentLikeSplashIDs = nextSplashIDs;
  }

  function scrollCommentsToBottom(behavior: ScrollBehavior) {
    requestAnimationFrame(() => {
      commentListEl?.scrollTo({ top: commentListEl.scrollHeight, behavior });
    });
  }
</script>

<div bind:this={commentListEl} class="comment-thread-list">
  {#if loading}
    <div class="flex h-40 items-center justify-center">
      <LoaderCircle class="animate-spin text-muted" size={26} />
    </div>
  {:else if comments.length === 0}
    <div class="flex h-40 flex-col items-center justify-center gap-3 text-center">
      <MessageCircle class="text-subtle" size={30} />
      <p class="text-sm font-semibold text-muted">No comments yet</p>
    </div>
  {:else}
    <div class="space-y-3">
      {#each comments as comment (comment.id)}
        <article class="rounded-overlay border border-border-glass-soft bg-action-bg px-3 py-2">
          <div class="comment-meta-row">
            <div class="comment-author-time">
              <span class="comment-author">{comment.author || 'Guest'}</span>
              <time class="comment-time" datetime={comment.createdAt}>
                {new Date(comment.createdAt).toLocaleString()}
              </time>
            </div>
            <button
              class="comment-like-button"
              class:comment-like-button-pending={(pendingCommentLikeCounts[comment.id] ?? 0) > 0}
              type="button"
              aria-label="Like comment"
              onclick={() => likeComment(comment.id)}
            >
              <span class="comment-like-icon-wrap" aria-hidden="true">
                <span class:comment-like-heart-pulse={commentLikeSplashIDs[comment.id] === true}>
                  <Heart size={14} fill="currentColor" />
                </span>
                {#if commentLikeSplashIDs[comment.id] === true}
                  <span class="comment-like-heart-splash" onanimationend={() => finishCommentLikeSplash(comment.id)}>
                    <Heart size={14} fill="currentColor" />
                  </span>
                {/if}
              </span>
              {#if comment.likeCount > 0}
                <span>{comment.likeCount}</span>
              {/if}
            </button>
          </div>
          <p class="comment-text text-sm leading-5 text-secondary">
            {comment.text}
          </p>
        </article>
      {/each}
    </div>
  {/if}

  {#if error}
    <div class="mt-4 rounded-overlay border border-border-glass-soft bg-action-bg px-3 py-2 text-sm font-semibold text-danger">
      {error}
    </div>
  {/if}
</div>

<form bind:this={commentFormEl} class="comment-thread-form" onsubmit={submitComment}>
  <div class="comment-composer">
    <label class="sr-only" for={item ? `comment-composer-${item.id}` : 'comment-composer'}>Add a comment</label>
    <!-- svelte-ignore a11y_autofocus - the comments UI is opened by an explicit user action and should be ready for typing. -->
    <textarea
      id={item ? `comment-composer-${item.id}` : 'comment-composer'}
      autofocus
      bind:this={commentInputEl}
      data-comment-composer={item?.id}
      class="comment-input"
      rows="1"
      maxlength="2000"
      bind:value={draft}
      onkeydown={handleCommentKeydown}
    ></textarea>
    <div class="comment-composer-actions">
      <button
        class="emoji-toggle-button"
        class:emoji-toggle-button-active={emojiPanelOpen}
        type="button"
        aria-label="Open emoji panel"
        aria-expanded={emojiPanelOpen}
        onclick={toggleEmojiPanel}
      >
        <Smile size={18} />
      </button>
    </div>

    {#if emojiPanelOpen}
      <EmojiPanel onSelect={insertEmoji} />
    {/if}
  </div>
</form>

<style>
  .comment-thread-list {
    min-height: 0;
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
  }

  .comment-thread-form {
    border-top: 1px solid var(--color-border-glass-soft);
    padding: 0.75rem;
  }

  .comment-composer {
    position: relative;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: end;
    gap: 0.5rem;
  }

  .comment-input {
    min-height: 2.75rem;
    max-height: 8rem;
    width: 100%;
    resize: vertical;
    border: 1px solid var(--color-border-glass-soft);
    border-radius: 1.25rem;
    background: var(--color-action-bg);
    padding: 0.7rem 0.9rem;
    color: var(--color-text-primary);
    font-size: 0.875rem;
    line-height: 1.35;
    outline: none;
  }

  .comment-input:focus {
    border-color: var(--color-border-glass-hover);
  }

  .comment-composer-actions {
    display: flex;
    height: 2.75rem;
    align-items: center;
  }

  .emoji-toggle-button {
    display: inline-grid;
    width: 1.9rem;
    height: 1.9rem;
    place-items: center;
    border-radius: 999px;
    color: var(--color-text-muted);
    transition:
      background 160ms ease,
      color 160ms ease,
      transform 160ms ease;
  }

  .emoji-toggle-button:hover,
  .emoji-toggle-button-active {
    background: var(--color-action-hover);
    color: var(--color-text-primary);
  }

  .emoji-toggle-button:hover {
    transform: translateY(-1px);
  }

  .comment-text {
    margin-top: 0.45rem;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  .comment-meta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .comment-author-time {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: 0.45rem;
  }

  .comment-author {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text-primary);
    font-size: 0.82rem;
    font-weight: 800;
    line-height: 1.15;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .comment-time {
    flex: 0 0 auto;
    color: var(--color-text-subtle);
    font-size: 0.7rem;
    font-weight: 700;
    line-height: 1.15;
    white-space: nowrap;
  }

  .comment-like-button {
    display: inline-flex;
    min-width: 0.875rem;
    flex: 0 0 auto;
    align-items: center;
    justify-content: flex-end;
    gap: 0.25rem;
    color: color-mix(in srgb, var(--color-text-primary) 82%, #f43f5e 18%);
    font-size: 0.72rem;
    font-weight: 900;
    line-height: 1;
    transition:
      color 160ms ease,
      opacity 160ms ease,
      transform 160ms ease;
  }

  .comment-like-button:hover {
    color: #f43f5e;
    transform: translateY(-1px);
  }

  .comment-like-button-pending {
    opacity: 0.6;
  }

  .comment-like-icon-wrap {
    position: relative;
    display: inline-grid;
    width: 0.875rem;
    height: 0.875rem;
    flex: 0 0 auto;
    place-items: center;
  }

  .comment-like-heart-pulse {
    display: inline-flex;
    animation: comment-like-heart-pulse 260ms cubic-bezier(0.2, 0.85, 0.25, 1.2);
  }

  .comment-like-heart-splash {
    position: absolute;
    inset: 0;
    display: inline-flex;
    color: #f43f5e;
    pointer-events: none;
    transform-origin: center;
    animation: comment-like-heart-splash 520ms ease-out forwards;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
    transform: none;
  }

  @keyframes comment-like-heart-pulse {
    0% {
      transform: scale(1);
    }

    48% {
      transform: scale(1.24);
    }

    100% {
      transform: scale(1);
    }
  }

  @keyframes comment-like-heart-splash {
    0% {
      opacity: 0.72;
      filter: drop-shadow(0 0 0 rgb(244 63 94 / 0));
      transform: scale(1);
    }

    42% {
      opacity: 0.44;
      filter: drop-shadow(0 0 10px rgb(244 63 94 / 0.45));
    }

    100% {
      opacity: 0;
      filter: drop-shadow(0 0 16px rgb(244 63 94 / 0));
      transform: scale(2.6);
    }
  }
</style>
