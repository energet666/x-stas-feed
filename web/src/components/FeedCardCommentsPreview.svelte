<script lang="ts">
  import { tick } from 'svelte';
  import { Heart, MessageCircle } from 'lucide-svelte';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    likePending = false,
    onOpenComments,
    onLike
  }: {
    item: MediaItem;
    likePending?: boolean;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  const latestComment = $derived(item.comments.at(-1));
  let previousLikeCount: number | undefined = undefined;
  let likeSplashActive = $state(false);

  $effect(() => {
    const nextLikeCount = item.likeCount;
    if (previousLikeCount !== undefined && nextLikeCount > previousLikeCount) {
      void animateLikeSplash();
    }
    previousLikeCount = nextLikeCount;
  });

  async function animateLikeSplash() {
    likeSplashActive = false;
    await tick();
    likeSplashActive = true;
  }
</script>

<div class="feed-card-comments-row">
  <button
    class="feed-card-like-button"
    class:feed-card-like-button-pending={likePending}
    type="button"
    aria-label={`Like ${item.displayName}`}
    onclick={(event) => {
      event.stopPropagation();
      onLike();
    }}
  >
    <span class="feed-card-like-icon-wrap" aria-hidden="true">
      <span class:feed-card-like-heart-pulse={likeSplashActive}>
        <Heart size={16} fill="currentColor" />
      </span>
      {#if likeSplashActive}
        <span class="feed-card-like-heart-splash" onanimationend={() => (likeSplashActive = false)}>
          <Heart size={16} fill="currentColor" />
        </span>
      {/if}
    </span>
    {#if item.likeCount > 0}
      <span>{item.likeCount}</span>
    {/if}
  </button>

  <button
    class="feed-card-comments-preview"
    type="button"
    aria-label={`Open comments for ${item.displayName}`}
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
</div>

<style>
  .feed-card-comments-row {
    display: flex;
    min-width: 0;
    width: 100%;
    align-items: center;
    gap: 0.65rem;
  }

  .feed-card-like-button {
    display: inline-flex;
    min-width: 1rem;
    position: relative;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    border-radius: 999px;
    color: color-mix(in srgb, var(--color-primary) 88%, #f43f5e 12%);
    font-size: 0.78rem;
    font-weight: 900;
    line-height: 1;
    transition:
      color 160ms ease,
      opacity 160ms ease,
      transform 160ms ease;
  }

  .feed-card-like-button:hover {
    color: #f43f5e;
    transform: translateY(-1px);
  }

  .feed-card-like-button-pending {
    opacity: 0.6;
  }

  .feed-card-like-icon-wrap {
    position: relative;
    display: inline-grid;
    width: 1rem;
    height: 1rem;
    flex: 0 0 auto;
    place-items: center;
  }

  .feed-card-like-heart-pulse {
    display: inline-flex;
    animation: like-heart-pulse 260ms cubic-bezier(0.2, 0.85, 0.25, 1.2);
  }

  .feed-card-like-heart-splash {
    position: absolute;
    inset: 0;
    display: inline-flex;
    color: #f43f5e;
    pointer-events: none;
    transform-origin: center;
    animation: like-heart-splash 520ms ease-out forwards;
  }

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
    .feed-card-comments-row {
      align-items: flex-start;
    }

    .feed-card-comments-preview {
      align-items: flex-start;
    }
  }

  @keyframes like-heart-pulse {
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

  @keyframes like-heart-splash {
    0% {
      opacity: 0.72;
      filter: drop-shadow(0 0 0 rgb(244 63 94 / 0));
      transform: scale(1);
    }

    42% {
      opacity: 0.44;
      filter: drop-shadow(0 0 12px rgb(244 63 94 / 0.45));
    }

    100% {
      opacity: 0;
      filter: drop-shadow(0 0 18px rgb(244 63 94 / 0));
      transform: scale(2.8);
    }
  }
</style>
