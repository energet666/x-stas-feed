<script lang="ts">
  import FeedCardFrame from './FeedCardFrame.svelte';
  import DrawingBoard from './DrawingBoard.svelte';
  import type { MediaItem } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    item,
    expanded,
    favorite,
    ambientActive,
    overlayVisible,
    likePending = false,
    username = t.common.guest,
    suppressFeedChrome = false,
    debugToolsEnabled = false,
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
    suppressFeedChrome?: boolean;
    debugToolsEnabled?: boolean;
    onReveal: () => void;
    onKeep: () => void;
    onHide: () => void;
    onToggleFavorite: () => void;
    onToggleExpanded: () => void;
    onOpenComments: () => void;
    onLike: () => void;
  } = $props();

  const mediaId = $derived(item.id);
  const suppressOverlays = $derived(expanded);
  let ambientCanvas = $state<HTMLCanvasElement | undefined>(undefined);

  function openBoardFromPreview(event: MouseEvent) {
    event.stopPropagation();
    onToggleExpanded();
  }
</script>

<FeedCardFrame
  {item}
  {expanded}
  {favorite}
  {ambientActive}
  {overlayVisible}
  {likePending}
  {suppressFeedChrome}
  {suppressOverlays}
  {onReveal}
  {onKeep}
  {onHide}
  {onToggleFavorite}
  {onToggleExpanded}
  {onOpenComments}
  {onLike}
>
  {#snippet ambientBackground()}
    <canvas
      bind:this={ambientCanvas}
      class="ambient-media"
      aria-hidden="true"
    ></canvas>
  {/snippet}

  {#snippet content()}
    {#if expanded}
      <DrawingBoard {mediaId} {expanded} {username} {ambientCanvas} {debugToolsEnabled} onClose={onToggleExpanded} />
    {:else}
      <button
        class="drawing-board-preview-button"
        type="button"
        aria-label={t.board.openDrawingBoardNamed(item.displayName || item.filename)}
        title={t.board.openDrawingBoard}
        onclick={openBoardFromPreview}
      >
        <DrawingBoard {mediaId} {expanded} {username} {ambientCanvas} {debugToolsEnabled} />
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
