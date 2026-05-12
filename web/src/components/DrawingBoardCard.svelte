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
    <DrawingBoard {boardId} {expanded} {username} onClose={expanded ? onToggleExpanded : undefined} />
  {/snippet}
</FeedCardFrame>
