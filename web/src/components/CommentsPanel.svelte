<script lang="ts">
  import { LoaderCircle, MessageCircle, Send, X } from 'lucide-svelte';
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

  function appendComment(mediaId: string, comment: Comment) {
    if (activeMediaId !== mediaId || comments.some((existing) => existing.id === comment.id)) return;

    const nextComments = [...comments, comment];
    comments = nextComments;
    onCommentsChanged(mediaId, nextComments);
  }
</script>

{#if item}
  <div class="comments-backdrop" role="presentation" onclick={onClose}></div>

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

    <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4">
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
              <p class="text-sm leading-5 text-secondary">
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

    <form class="border-t border-glass-border-soft p-3" onsubmit={submitComment}>
      <div class="flex items-end gap-2">
        <label class="sr-only" for="comment-text">Add a comment</label>
        <textarea
          id="comment-text"
          class="comment-input"
          rows="1"
          maxlength="2000"
          placeholder="Add a comment"
          bind:value={draft}
        ></textarea>
        <button class="glass-icon-button" type="submit" disabled={!canSubmit} aria-label="Send comment">
          {#if submitting}
            <LoaderCircle class="animate-spin" size={17} />
          {:else}
            <Send size={17} />
          {/if}
        </button>
      </div>
    </form>
  </aside>
{/if}

<style>
  .comments-backdrop {
    position: fixed;
    inset: 0;
    z-index: 90;
    background: rgb(0 0 0 / 0.52);
    backdrop-filter: blur(10px) saturate(130%);
    -webkit-backdrop-filter: blur(10px) saturate(130%);
  }

  .comments-panel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: 91;
    display: flex;
    width: min(28rem, 100vw);
    flex-direction: column;
    border-left: 1px solid var(--color-glass-border);
    background: var(--background-image-glass-strong);
    box-shadow: var(--shadow-popover);
    color: var(--color-primary);
    backdrop-filter: blur(34px) saturate(190%);
    -webkit-backdrop-filter: blur(34px) saturate(190%);
  }

  .comment-input {
    min-height: 2.75rem;
    max-height: 8rem;
    flex: 1;
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

  button:disabled {
    cursor: not-allowed;
    opacity: 0.45;
    transform: none;
  }

  @media (width < 720px) {
    .comments-panel {
      top: auto;
      width: 100vw;
      height: min(78vh, 42rem);
      border-top: 1px solid var(--color-glass-border);
      border-left: 0;
      border-radius: 1.5rem 1.5rem 0 0;
    }
  }
</style>
