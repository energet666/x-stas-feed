<script lang="ts">
  import FeedCardFrame from './FeedCardFrame.svelte';
  import FeedAudioPlayer from './FeedAudioPlayer/FeedAudioPlayer.svelte';
  import FeedVideoPlayer from './FeedVideoPlayer/FeedVideoPlayer.svelte';
  import FileCardContent from './FileCardContent.svelte';
  import DrawingBoardCard from './DrawingBoardCard.svelte';
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
    videoAutoplayEnabled = false,
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
    videoAutoplayEnabled?: boolean;
    suppressFeedChrome?: boolean;
    debugToolsEnabled?: boolean;
    onReveal: (id: string) => void;
    onKeep: (id: string) => void;
    onHide: (id: string) => void;
    onToggleFavorite: (id: string) => void;
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

  const isDrawableImage = $derived(
    item.type === 'image' &&
      item.mimeType !== 'image/gif' &&
      !item.filename.toLowerCase().endsWith('.gif')
  );

  function hideOverlay() {
    onHide(item.id);
  }
</script>

{#if item.type === 'board' || isDrawableImage}
  <DrawingBoardCard
    {item}
    {expanded}
    {favorite}
    {ambientActive}
    {overlayVisible}
    {likePending}
    {username}
    {suppressFeedChrome}
    {debugToolsEnabled}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleFavorite={() => onToggleFavorite(item.id)}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  />
{:else if item.type === 'video'}
  <FeedVideoPlayer
    {item}
    {expanded}
    {favorite}
    {ambientActive}
    {overlayVisible}
    {likePending}
    autoplayEnabled={videoAutoplayEnabled}
    {suppressFeedChrome}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleFavorite={() => onToggleFavorite(item.id)}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  />
{:else if item.type === 'audio'}
  <FeedAudioPlayer
    {item}
    {expanded}
    {favorite}
    {ambientActive}
    {overlayVisible}
    {likePending}
    autoplayEnabled={videoAutoplayEnabled}
    {suppressFeedChrome}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleFavorite={() => onToggleFavorite(item.id)}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  />
{:else if item.type === 'image'}
  <FeedCardFrame
    {item}
    {expanded}
    {favorite}
    {ambientActive}
    {overlayVisible}
    {likePending}
    {suppressFeedChrome}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleFavorite={() => onToggleFavorite(item.id)}
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
{:else}
  <FeedCardFrame
    {item}
    {expanded}
    {favorite}
    ambientActive={true}
    {overlayVisible}
    {likePending}
    {suppressFeedChrome}
    onReveal={revealOverlay}
    onKeep={keepOverlay}
    onHide={hideOverlay}
    onToggleFavorite={() => onToggleFavorite(item.id)}
    onToggleExpanded={() => onToggleExpanded(item.id)}
    onOpenComments={() => onOpenComments(item.id)}
    onLike={() => onLike(item.id)}
  >
    {#snippet ambientBackground()}
      <div class="file-card-ambient ambient-media"></div>
    {/snippet}

    {#snippet content()}
      <FileCardContent {item} />
    {/snippet}
  </FeedCardFrame>
{/if}

<style>
  .file-card-ambient {
    background:
      linear-gradient(90deg, rgb(255 255 255 / 0.04) 1px, transparent 1px),
      linear-gradient(0deg, rgb(255 255 255 / 0.035) 1px, transparent 1px),
      linear-gradient(135deg, rgb(10 14 20), rgb(29 35 44) 52%, rgb(16 22 28));
    background-size: 34px 34px, 34px 34px, auto;
    filter: none;
    transform: none;
  }
</style>
