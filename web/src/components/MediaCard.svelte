<script lang="ts">
  import FeedCardFrame from './FeedCardFrame.svelte';
  import FeedVideoPlayer from './FeedVideoPlayer/FeedVideoPlayer.svelte';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    expanded,
    ambientActive,
    overlayVisible,
    likePending = false,
    onReveal,
    onKeep,
    onHide,
    onToggleExpanded,
    onOpenComments,
    onLike
  }: {
    item: MediaItem;
    expanded: boolean;
    ambientActive: boolean;
    overlayVisible: boolean;
    likePending?: boolean;
    onReveal: (id: string) => void;
    onKeep: (id: string) => void;
    onHide: (id: string) => void;
    onToggleExpanded: (id: string) => void;
    onOpenComments: (id: string) => void;
    onLike: (id: string) => void;
  } = $props();

  function revealOverlay() {
    onReveal(item.id);
  }

  function keepOverlay() {
    onKeep(item.id);
  }

  function hideOverlay() {
    onHide(item.id);
  }
</script>

{#if item.type === 'video'}
  <FeedVideoPlayer
    {item}
    {expanded}
    {ambientActive}
    {overlayVisible}
    {likePending}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  />
{:else}
  <FeedCardFrame
    {item}
    {expanded}
    {ambientActive}
    {overlayVisible}
    {likePending}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  >
    {#snippet content()}
      <img
        class="block h-full w-full object-contain media-content-shadow"
        src={item.url}
        alt={item.displayName}
        loading="lazy"
        decoding="async"
      />
    {/snippet}
  </FeedCardFrame>
{/if}
