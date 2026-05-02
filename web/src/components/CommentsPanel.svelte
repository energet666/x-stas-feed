<script lang="ts">
  import { LoaderCircle, MessageCircle, X } from 'lucide-svelte';
  import { createComment, fetchComments, type Comment, type MediaItem } from '../lib/feed';

  let {
    item,
    commentEvent,
    onClose,
    onCommentsChanged
  }: {
    item: MediaItem | undefined;
    commentEvent: { mediaId: string; comment: Comment } | null;
    onClose: () => void;
    onCommentsChanged: (mediaId: string, comments: Comment[]) => void;
  } = $props();

  let comments = $state<Comment[]>([]);
  let loading = $state(false);
  let submitting = $state(false);
  let error = $state<string | null>(null);
  let draft = $state('');
  let activeMediaId = '';
  let commentListEl = $state<HTMLDivElement | undefined>(undefined);
  let commentFormEl = $state<HTMLFormElement | undefined>(undefined);

  const canSubmit = $derived(draft.trim().length > 0 && !submitting && item !== undefined);

  $effect(() => {
    if (!item) {
      activeMediaId = '';
      return;
    }
    if (activeMediaId === item.id) return;

    activeMediaId = item.id;
    comments = item.comments;
    draft = '';
    scrollCommentsToBottom('auto');
    void loadComments(item.id);
  });

  $effect(() => {
    if (!commentEvent) return;
    appendComment(commentEvent.mediaId, commentEvent.comment);
  });

  async function loadComments(mediaId: string) {
    loading = true;
    error = null;

    try {
      const response = await fetchComments(mediaId);
      comments = response.comments;
      onCommentsChanged(mediaId, response.comments);
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
      const comment = await createComment(item.id, text);
      appendComment(item.id, comment);
      draft = '';
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

  function appendComment(mediaId: string, comment: Comment) {
    if (activeMediaId !== mediaId || comments.some((existing) => existing.id === comment.id)) return;

    const nextComments = [...comments, comment];
    comments = nextComments;
    onCommentsChanged(mediaId, nextComments);
    scrollCommentsToBottom('smooth');
  }

  function scrollCommentsToBottom(behavior: ScrollBehavior) {
    requestAnimationFrame(() => {
      commentListEl?.scrollTo({ top: commentListEl.scrollHeight, behavior });
    });
  }
</script>

{#if item}
  <aside class="comments-panel" aria-label={`Comments for ${item.filename}`}>
    <header class="flex items-center justify-between gap-3 border-b border-glass-border-soft px-4 py-3">
      <div class="min-w-0">
        <p class="text-xs font-semibold uppercase text-subtle">Comments</p>
        <h2 class="truncate text-base font-semibold text-primary">{item.filename}</h2>
      </div>
      <button class="glass-icon-button" type="button" aria-label="Close comments" onclick={onClose}>
        <X size={18} />
      </button>
    </header>

    <div bind:this={commentListEl} class="min-h-0 flex-1 overflow-y-auto px-4 py-4">
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
            <article class="rounded-overlay border border-glass-border-soft bg-button-bg px-3 py-2">
              <p class="comment-text text-sm leading-5 text-secondary">
                <span class="font-semibold text-primary">Guest</span>
                {comment.text}
              </p>
              <time class="mt-1 block text-xs font-semibold text-subtle" datetime={comment.createdAt}>
                {new Date(comment.createdAt).toLocaleString()}
              </time>
            </article>
          {/each}
        </div>
      {/if}

      {#if error}
        <div class="mt-4 rounded-overlay border border-glass-border-soft bg-button-bg px-3 py-2 text-sm font-semibold text-danger">
          {error}
        </div>
      {/if}
    </div>

    <form bind:this={commentFormEl} class="border-t border-glass-border-soft p-3" onsubmit={submitComment}>
      <div>
        <label class="sr-only" for="comment-text">Add a comment</label>
        <textarea
          id="comment-text"
          class="comment-input"
          rows="1"
          maxlength="2000"
          placeholder="Add a comment"
          bind:value={draft}
          onkeydown={handleCommentKeydown}
        ></textarea>
      </div>
    </form>
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

  .comment-input {
    min-height: 2.75rem;
    max-height: 8rem;
    width: 100%;
    resize: vertical;
    border: 1px solid var(--color-glass-border-soft);
    border-radius: 1.25rem;
    background: var(--color-button-bg);
    padding: 0.7rem 0.9rem;
    color: var(--color-primary);
    font-size: 0.875rem;
    line-height: 1.35;
    outline: none;
  }

  .comment-input::placeholder {
    color: var(--color-subtle);
  }

  .comment-input:focus {
    border-color: var(--color-glass-border-hover);
  }

  .comment-text {
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
    transform: none;
  }

  @media (width < 720px) {
    .comments-panel {
      border-radius: inherit;
    }
  }
</style>
