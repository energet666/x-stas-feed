<script lang="ts">
  import FeedCardFrame from './FeedCardFrame.svelte';
  import FeedVideoPlayer from './FeedVideoPlayer/FeedVideoPlayer.svelte';
  import type { MediaItem } from '../lib/feed';

  let {
    item,
    expanded,
    overlayVisible,
    onReveal,
    onKeep,
    onHide,
    onToggleExpanded,
    onOpenComments
  }: {
    item: MediaItem;
    expanded: boolean;
    overlayVisible: boolean;
    onReveal: (id: string) => void;
    onKeep: (id: string) => void;
    onHide: (id: string) => void;
    onToggleExpanded: (id: string) => void;
    onOpenComments: (id: string) => void;
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
    {overlayVisible}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
  />
{:else}
  <FeedCardFrame
    {item}
    {expanded}
    {overlayVisible}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
  >
    {#snippet content()}
      <img
        class="block h-full w-full object-contain"
        src={item.url}
        alt={item.filename}
        loading="lazy"
        decoding="async"
      />
    {/snippet}
  </FeedCardFrame>
{/if}
