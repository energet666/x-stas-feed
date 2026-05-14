<script lang="ts">
  import FeedCardFrame from './FeedCardFrame.svelte';
  import DrawingBoard from './DrawingBoard.svelte';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    expanded,
    favorite,
    ambientActive,
    overlayVisible,
    likePending = false,
    username = 'Guest',
    onReveal,
    onKeep,
    onHide,
    onToggleFavorite,
    onToggleExpanded,
    onOpenComments,
    onLike
  }: {
    item: MediaItem;
    expanded: boolean;
    favorite: boolean;
    ambientActive: boolean;
    overlayVisible: boolean;
    likePending?: boolean;
    username?: string;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  const boardId = $derived(item.boardId ?? item.id);
  const suppressOverlays = $derived(expanded);

  function openBoardFromPreview(event: MouseEvent) {
    event.stopPropagation();
    onToggleExpanded();
  }
</script>

<FeedCardFrame
  {item}
  {expanded}
  {favorite}
  ambientActive={false}
  {overlayVisible}
  {likePending}
  {suppressOverlays}
  {onReveal}
  {onKeep}
  {onHide}
  {onToggleFavorite}
  {onToggleExpanded}
  {onOpenComments}
  {onLike}
>
  {#snippet content()}
    {#if expanded}
      <DrawingBoard {boardId} {expanded} {username} onClose={onToggleExpanded} />
    {:else}
      <button
        class="drawing-board-preview-button"
        type="button"
        aria-label={`Open drawing board ${item.displayName || item.filename}`}
        title="Open drawing board"
        onclick={openBoardFromPreview}
      >
        <DrawingBoard {boardId} {expanded} {username} />
      </button>
    {/if}
  {/snippet}
</FeedCardFrame>

<style>
  .drawing-board-preview-button {
    display: block;
    width: 100%;
    height: 100%;
    border: 0;
    padding: 0;
    background: transparent;
    cursor: pointer;
    text-align: inherit;
  }

  .drawing-board-preview-button:focus-visible {
    outline: 2px solid rgba(255, 255, 255, 0.78);
    outline-offset: -4px;
  }
</style>
