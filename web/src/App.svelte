<script lang="ts">
  import { flushSync, onMount } from 'svelte';
  import { AlertCircle, LoaderCircle, PanelRightOpen, Rocket, Star, Upload } from 'lucide-svelte';
  import ActivityMediaModal from './components/ActivityMediaModal.svelte';
  import AsteroidsShip from './components/AsteroidsShip.svelte';
  import BackgroundParticles from './components/BackgroundParticles.svelte';
  import CommentsPanel from './components/CommentsPanel.svelte';
  import EmptyFeedState from './components/EmptyFeedState.svelte';
  import FeedDebugOverlay from './components/FeedDebugOverlay.svelte';
  import FeedError from './components/FeedError.svelte';
  import FeedHeader from './components/FeedHeader.svelte';
  import MediaCard from './components/MediaCard.svelte';
  import SocialActivityPanel from './components/SocialActivityPanel.svelte';
  import UserSidebar from './components/UserSidebar.svelte';
  import DrawingBoard from './components/DrawingBoard.svelte';
  import { boardEvents } from './lib/board_events.svelte';
  import { debugToolsStorageKey, readDebugToolsEnabled } from './lib/debug';
  import { fallbackUsername, randomUsername } from './lib/usernames';
  import { uiText as t } from './lib/ui_text';
  import {
    commentEventsURL,
    createBoard,
    createLike,
    fetchActivity,
    fetchBoard,
    fetchFeedItem,
    fetchMediaItem,
    maxUploadBytes,
    uploadMedia,
    type ActivityItem,
    type BoardInfo,
    type Comment,
    type CommentLikeEvent,
    type CommentEvent,
    type FeedItemCreatedEvent,
    type IndexedFeedItem,
    type LikeEvent,
    type MediaItem,
    type StrokeEvent
  } from './lib/feed';

  const activityLimit = 30;
  const estimatedCardHeight = 760;
  const itemGap = 16;
  const overscanRows = 2;
  const preloadAheadPx = 1600;
  const usernameStorageKey = 'feed-ai:comment-username';
  const cardBackgroundModeStorageKey = 'feed-ai:card-background-mode';
  const pageBackgroundModeStorageKey = 'feed-ai:page-background-mode';
  const pageBackgroundEnabledStorageKey = 'feed-ai:page-background-enabled';
  const backgroundParticlesEnabledStorageKey = 'feed-ai:background-particles-enabled';
  const asteroidsEnabledStorageKey = 'feed-ai:asteroids-enabled';
  const glassEffectsEnabledStorageKey = 'feed-ai:glass-effects-enabled';
  const favoritesStorageKey = 'feed-ai:favorites';
  const clearActiveVideoEvent = 'feed-ai:video-clear-active';
  const gameStartedEvent = 'feed-ai:game-started';
  const gameExitedEvent = 'feed-ai:game-exited';
  const backgroundKeyboardFocusEvent = 'feed-ai:background-keyboard-focus';

  type CardBackgroundMode = 'simple' | 'ambient';
  type PageBackgroundMode = 'cosmos' | 'daylight';
  type GlassEffectsMode = 'off' | 'full';
  type FeedMode = 'all' | 'favorites';
  type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';

  type FeedRow = {
    item: MediaItem;
    top: number;
    height: number;
  };

  function isDrawableMediaItem(item: MediaItem | null | undefined): item is MediaItem {
    return !!item && (
      item.type === 'board' ||
      (item.type === 'image' && item.mimeType !== 'image/gif' && !item.filename.toLowerCase().endsWith('.gif'))
    );
  }

  type FeedItemLookup = {
    result?: IndexedFeedItem;
    staleFavoriteIDs?: string[];
  };

  let items = $state<MediaItem[]>([]);
  let firstFeedIndex = $state<number | undefined>(undefined);
  let lastFeedIndex = $state<number | undefined>(undefined);
  let topFeedIndex = $state<number | undefined>(undefined);
  let bottomFeedIndex = $state<number | undefined>(undefined);
  let loading = $state(false);
  let initialLoaded = $state(false);
  let error = $state<string | null>(null);
  let sentinel = $state<HTMLDivElement | undefined>(undefined);
  let listEl = $state<HTMLElement | undefined>(undefined);
  let scrollY = $state(0);
  let viewportHeight = $state(0);
  let listTop = $state(0);
  let bottomSentinelTop = $state<number | undefined>(undefined);
  let measuredHeights = $state<Record<string, number>>({});
  let debugCollapsed = $state(false);
  let debugToolsEnabled = $state(false);
  let activeOverlayID = $state<string | null>(null);
  let expandedItemID = $state<string | null>(null);
  let commentsPanelItemID = $state<string | null>(null);
  let activityItems = $state<ActivityItem[]>([]);
  let activityLoading = $state(false);
  let activityError = $state<string | null>(null);
  let selectedActivityMedia = $state<MediaItem | null>(null);
  let activityMediaLoading = $state(false);
  let activityMediaError = $state<string | null>(null);
  let activityModalOpen = $state(false);
  let latestCommentEvent = $state<CommentEvent | null>(null);
  let latestCommentLikeEvent = $state<CommentLikeEvent | null>(null);
  let username = $state<string>(fallbackUsername);
  let usernameStorageReady = $state(false);
  let cardBackgroundMode = $state<CardBackgroundMode>('ambient');
  let pageBackgroundMode = $state<PageBackgroundMode>('cosmos');
  let cardBackgroundModeStorageReady = $state(false);
  let backgroundParticlesEnabled = $state(true);
  let asteroidsEnabled = $state(true);
  let glassEffectsMode = $state<GlassEffectsMode>('off');
  let backgroundLayersStorageReady = $state(false);
  let favoriteIDs = $state<string[]>([]);
  let favoritesStorageReady = $state(false);
  let feedMode = $state<FeedMode>('all');
  let gameActive = $state(false);
  let uploadStatus = $state<UploadStatus>('idle');
  let uploadMessage = $state<string>(t.upload.action);
  let uploadProgress = $state<number | null>(null);
  let pageDragActive = $state(false);
  let pageDragHasMultipleFiles = $state(false);
  let uploadFeedRefreshPending = false;
  let ambientReadyIDs = $state<Record<string, boolean>>({});
  let pendingLikeCounts = $state<Record<string, number>>({});
  let overlayHideTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let uploadStatusTimer: number | undefined = undefined;
  let viewportFrameID: number | undefined = undefined;
  let loadMoreFrameID: number | undefined = undefined;
  let commentEvents: EventSource | undefined = undefined;
  let feedRequestVersion = 0;
  let masterBoardExpanded = $state(false);
  let activityBoardExpandedID = $state<string | null>(null);
  let newFeedItemCount = $state(0);
  let activityPanelOpen = $state(false);
  let unsubscribeBoardActivity: (() => void) | undefined = undefined;
  const pendingBoardActivityFetches = new Set<string>();

  const isFavoriteMode = $derived(feedMode === 'favorites');
  const hasMore = $derived(
    !initialLoaded || (bottomFeedIndex !== undefined && firstFeedIndex !== undefined && bottomFeedIndex > firstFeedIndex)
  );
  const isEmpty = $derived(initialLoaded && items.length === 0 && !error);
  const viewportStart = $derived(Math.max(0, scrollY - listTop));
  const viewportEnd = $derived(viewportStart + viewportHeight);
  const rows = $derived.by<FeedRow[]>(() => {
    let top = 0;
    return items.map((item) => {
      const height = (measuredHeights[item.id] ?? estimatedCardHeight) + itemGap;
      const row = { item, top, height };
      top += height;
      return row;
    });
  });
  const totalHeight = $derived(rows.reduce((total, row) => total + row.height, 0));
  const firstVisibleIndex = $derived.by(() => {
    if (rows.length === 0) {
      return -1;
    }

    const firstVisible = rows.findIndex((row) => row.top + row.height >= viewportStart);
    if (firstVisible === -1) {
      return rows.length - 1;
    }

    return firstVisible;
  });
  const lastVisibleIndex = $derived.by(() => {
    if (firstVisibleIndex < 0) {
      return -1;
    }

    let lastVisible = firstVisibleIndex;
    while (lastVisible + 1 < rows.length && rows[lastVisible + 1].top <= viewportEnd) {
      lastVisible += 1;
    }

    return lastVisible;
  });
  const visibleStartIndex = $derived(firstVisibleIndex >= 0 ? Math.max(0, firstVisibleIndex - overscanRows) : -1);
  const visibleEndIndex = $derived(
    lastVisibleIndex >= 0 ? Math.min(rows.length - 1, lastVisibleIndex + overscanRows) : -1
  );
  const visibleRows = $derived(
    visibleStartIndex >= 0 ? rows.slice(visibleStartIndex, visibleEndIndex + 1) : []
  );
  const topSpacer = $derived(visibleRows[0]?.top ?? 0);
  const bottomSpacer = $derived.by(() => {
    const last = visibleRows.at(-1);
    if (!last) return 0;
    return Math.max(0, totalHeight - last.top - last.height);
  });
  const loadedBottom = $derived(totalHeight);
  const unloadedAfter = $derived(Math.max(0, items.length - visibleEndIndex - 1));
  const unloadedBefore = $derived(Math.max(0, visibleStartIndex));
  const measuredCount = $derived(Object.keys(measuredHeights).length);
  const commentUsername = $derived(username.trim() || fallbackUsername);
  const favoriteIDSet = $derived(new Set(favoriteIDs));
  const commentsPanelItem = $derived(items.find((item) => item.id === commentsPanelItemID));
  const commentsPanelFullscreen = $derived(
    commentsPanelItem !== undefined && commentsPanelItemID === expandedItemID
  );

  onMount(() => {
    document.documentElement.classList.toggle('safari-browser', isSafariBrowser());
    debugCollapsed = readStoredDebugCollapsed();
    debugToolsEnabled = readDebugToolsEnabled();
    username = readStoredUsername();
    usernameStorageReady = true;
    cardBackgroundMode = readStoredCardBackgroundMode();
    pageBackgroundMode = readStoredPageBackgroundMode();
    cardBackgroundModeStorageReady = true;
    backgroundParticlesEnabled = readStoredBackgroundLayerEnabled(backgroundParticlesEnabledStorageKey);
    asteroidsEnabled = readStoredBackgroundLayerEnabled(asteroidsEnabledStorageKey);
    glassEffectsMode = readStoredGlassEffectsMode();
    backgroundLayersStorageReady = true;
    favoriteIDs = readStoredFavoriteIDs();
    favoritesStorageReady = true;
    updateViewport();
    window.addEventListener('scroll', scheduleViewportUpdate, { passive: true });
    window.addEventListener('resize', scheduleViewportUpdate);
    window.addEventListener('storage', syncDebugToolsEnabled);
    window.addEventListener('focus', syncDebugToolsEnabled);
    document.addEventListener('visibilitychange', syncDebugToolsEnabled);
    window.addEventListener('pointerdown', updateBackgroundKeyboardFocus, { capture: true });
    window.addEventListener(gameStartedEvent, activateGameMode);
    window.addEventListener(gameExitedEvent, deactivateGameMode);
    subscribeToCommentEvents();
    subscribeToBoardActivity();
    void loadPage();
    void loadActivity();

    return () => {
      document.documentElement.classList.remove('safari-browser');
      document.documentElement.classList.remove('no-glass-effects');
      clearTimeout(overlayHideTimer);
      clearTimeout(uploadStatusTimer);
      if (viewportFrameID !== undefined) {
        cancelAnimationFrame(viewportFrameID);
      }
      if (loadMoreFrameID !== undefined) {
        cancelAnimationFrame(loadMoreFrameID);
      }
      commentEvents?.close();
      unsubscribeBoardActivity?.();
      window.removeEventListener('scroll', scheduleViewportUpdate);
      window.removeEventListener('resize', scheduleViewportUpdate);
      window.removeEventListener('storage', syncDebugToolsEnabled);
      window.removeEventListener('focus', syncDebugToolsEnabled);
      document.removeEventListener('visibilitychange', syncDebugToolsEnabled);
      window.removeEventListener('pointerdown', updateBackgroundKeyboardFocus, { capture: true });
      window.removeEventListener(gameStartedEvent, activateGameMode);
      window.removeEventListener(gameExitedEvent, deactivateGameMode);
    };
  });

  $effect(() => {
    if (!sentinel) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          void loadPage();
        }
      },
      { rootMargin: `0px 0px ${preloadAheadPx}px 0px` }
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  });

  $effect(() => {
    const previousOverflow = document.body.style.overflow;
    if (expandedItemID || activityModalOpen || masterBoardExpanded || activityBoardExpandedID) {
      document.body.style.overflow = 'hidden';
    }

    return () => {
      document.body.style.overflow = previousOverflow;
    };
  });

  $effect(() => {
    if (!usernameStorageReady) return;
    persistUsername(username);
  });

  $effect(() => {
    if (!cardBackgroundModeStorageReady) return;
    persistCardBackgroundMode(cardBackgroundMode);
    persistPageBackgroundMode(pageBackgroundMode);
  });

  $effect(() => {
    if (!backgroundLayersStorageReady) return;
    persistBackgroundLayerEnabled(backgroundParticlesEnabledStorageKey, backgroundParticlesEnabled);
    persistBackgroundLayerEnabled(asteroidsEnabledStorageKey, asteroidsEnabled);
    persistGlassEffectsMode(glassEffectsMode);
    document.documentElement.classList.toggle('no-glass-effects', glassEffectsMode === 'off');
  });

  $effect(() => {
    if (!favoritesStorageReady) return;
    persistFavoriteIDs(favoriteIDs);
  });

  async function loadPage() {
    if (loading || (initialLoaded && !hasMore)) return;

    if (isFavoriteMode && favoriteIDs.length === 0) {
      initialLoaded = true;
      error = null;
      return;
    }

    const requestVersion = feedRequestVersion;
    const requestMode = feedMode;
    const requestFavoriteIDs = favoriteIDs;
    const requestIndex = bottomFeedIndex === undefined ? -1 : bottomFeedIndex - 1;
    loading = true;
    error = null;

    try {
      const lookup = await fetchIndexedFeedItem(requestMode, requestIndex, requestFavoriteIDs);
      if (requestVersion !== feedRequestVersion) return;
      if (lookup.staleFavoriteIDs?.length) {
        const staleFavoriteIDSet = new Set(lookup.staleFavoriteIDs);
        favoriteIDs = favoriteIDs.filter((id) => !staleFavoriteIDSet.has(id));
        syncFavoriteFeedIndexesFromItems();
      }
      if (!lookup.result) {
        initialLoaded = true;
        return;
      }

      const result = lookup.result;
      firstFeedIndex = result.firstIndex;
      lastFeedIndex = result.lastIndex;
      topFeedIndex = topFeedIndex === undefined ? result.index : topFeedIndex;
      bottomFeedIndex = result.index;
      items = [...items, result.item];
      initialLoaded = true;
      scheduleViewportUpdate();
      scheduleLoadMoreCheck();
    } catch (err) {
      if (requestVersion !== feedRequestVersion) return;
      if (!initialLoaded && !isFavoriteMode && err instanceof Error && err.message === 'feed item not found') {
        initialLoaded = true;
        return;
      }
      error = err instanceof Error ? err.message : t.feed.loadFallback;
      initialLoaded = true;
    } finally {
      if (requestVersion === feedRequestVersion) {
        loading = false;
      }
    }
  }

  async function fetchIndexedFeedItem(
    mode: FeedMode,
    index: number,
    requestFavoriteIDs: string[]
  ): Promise<FeedItemLookup> {
    if (mode === 'all') {
      return { result: await fetchFeedItem(index) };
    }

    return fetchFavoriteIndexedItem(index, requestFavoriteIDs);
  }

  async function fetchFavoriteIndexedItem(index: number, ids: string[]): Promise<FeedItemLookup> {
    if (ids.length === 0) return {};

    const resolvedIndex = index === -1 ? ids.length - 1 : index;
    if (resolvedIndex < 0 || resolvedIndex >= ids.length) return {};

    let localIndex = ids.length - 1 - resolvedIndex;
    const staleIDs: string[] = [];

    while (localIndex < ids.length) {
      const id = ids[localIndex];

      try {
        const item = await fetchMediaItem(id);
        const compactedLength = ids.length - staleIDs.length;
        const compactedLocalIndex = localIndex - staleIDs.length;
        return {
          result: {
            index: compactedLength - 1 - compactedLocalIndex,
            firstIndex: 0,
            lastIndex: compactedLength - 1,
            item
          },
          staleFavoriteIDs: staleIDs
        };
      } catch (err) {
        if (isHTTPStatusError(err, 404)) {
          staleIDs.push(id);
          localIndex += 1;
          continue;
        }
        throw err;
      }
    }

    return { staleFavoriteIDs: staleIDs };
  }

  function isHTTPStatusError(err: unknown, status: number) {
    return err instanceof Error && (err as Error & { status?: number }).status === status;
  }

  async function loadActivity() {
    activityLoading = true;
    activityError = null;

    try {
      const response = await fetchActivity({ limit: activityLimit });
      const boardActivityItems = activityItems.filter((item) => item.type === 'board');
      activityItems = sortActivityItems([...response.items, ...boardActivityItems]).slice(0, activityLimit);
    } catch (err) {
      activityError = err instanceof Error ? err.message : t.activity.loadFallback;
    } finally {
      activityLoading = false;
    }
  }

  function retry() {
    void loadPage();
  }

  function scrollFeedToTop() {
    window.scrollTo({ top: 0, left: 0, behavior: 'smooth' });
  }

  async function handleUploadFiles(files: File[]) {
    const uploadFiles = files.filter((file) => file.size > 0);
    if (uploadStatus === 'uploading') return;
    if (uploadFiles.length === 0) {
      setUploadStatus('error', t.upload.noFiles, null);
      return;
    }
    if (uploadFiles.length > 1) {
      setUploadStatus('error', t.upload.oneFileOnly, null);
      return;
    }

    const uploadFile = uploadFiles[0];
    if (uploadFile.size > maxUploadBytes) {
      setUploadStatus('error', t.upload.fileTooLarge(formatFileSize(maxUploadBytes)), null);
      return;
    }
    setUploadStatus('uploading', uploadFile.name, 0);
    uploadFeedRefreshPending = true;

    try {
      const result = await uploadMedia(uploadFile, (progress) => {
        uploadProgress = progress.percent;
      });
      const uploadedCount = result.items.length;
      const errorCount = result.errors?.length ?? 0;
      if (uploadedCount === 0) {
        throw new Error(result.errors?.[0]?.error ?? t.upload.noneUploaded);
      }

      setUploadStatus(
        errorCount > 0 ? 'error' : 'success',
        errorCount > 0 ? t.upload.uploadedWithErrors(uploadedCount, errorCount) : t.upload.uploaded(uploadedCount),
        null
      );
      resetFeedState();
      await loadPage();
      scheduleViewportUpdate();
    } catch (err) {
      setUploadStatus('error', err instanceof Error ? err.message : t.upload.failed, null);
    } finally {
      uploadFeedRefreshPending = false;
    }
  }

  async function handleCreateBoard(name: string) {
    const board = await createBoard(name);
    const boardItem = boardInfoToMediaItem(board);
    if (!boardItem) throw new Error(t.board.mediaItemMissing);
    resetFeedState();
    await loadPage();
    scheduleViewportUpdate();
    window.scrollTo({ top: 0, left: 0, behavior: 'smooth' });
  }

  function boardInfoToMediaItem(board: BoardInfo): MediaItem | null {
    if (!board.mediaId) return null;
    return {
      id: board.mediaId,
      filename: board.filename || board.name,
      displayName: board.name,
      type: 'board',
      url: '',
      mimeType: '',
      size: 0,
      modifiedAt: board.createdAt,
      comments: [],
      commentCount: 0,
      likeCount: 0
    };
  }

  function setUploadStatus(status: UploadStatus, message: string, progress: number | null) {
    clearTimeout(uploadStatusTimer);
    uploadStatus = status;
    uploadMessage = message;
    uploadProgress = progress;

    if (status === 'success' || status === 'error') {
      uploadStatusTimer = window.setTimeout(() => {
        uploadStatus = 'idle';
        uploadMessage = t.upload.action;
        uploadProgress = null;
      }, 3200);
    }
  }

  function resetFeedState() {
    feedRequestVersion += 1;
    items = [];
    firstFeedIndex = undefined;
    lastFeedIndex = undefined;
    topFeedIndex = undefined;
    bottomFeedIndex = undefined;
    loading = false;
    initialLoaded = false;
    error = null;
    bottomSentinelTop = undefined;
    measuredHeights = {};
    ambientReadyIDs = {};
    pendingLikeCounts = {};
    commentsPanelItemID = null;
    expandedItemID = null;
    activityModalOpen = false;
    selectedActivityMedia = null;
    activityMediaLoading = false;
    activityMediaError = null;
    activeOverlayID = null;
    newFeedItemCount = 0;
  }

  async function refreshFeedFromTop() {
    resetFeedState();
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
    updateViewport();
    await loadPage();
  }

  function updateViewport() {
    viewportFrameID = undefined;
    scrollY = window.scrollY;
    viewportHeight = window.innerHeight;
    if (listEl) {
      listTop = listEl.getBoundingClientRect().top + window.scrollY;
    }
    bottomSentinelTop = sentinel?.getBoundingClientRect().top;
  }

  function scheduleViewportUpdate() {
    if (viewportFrameID !== undefined) return;
    viewportFrameID = requestAnimationFrame(updateViewport);
  }

  function scheduleLoadMoreCheck() {
    if (loadMoreFrameID !== undefined) return;
    loadMoreFrameID = requestAnimationFrame(() => {
      loadMoreFrameID = undefined;
      if (isSentinelInPreloadRange()) {
        void loadPage();
      }
    });
  }

  function isSentinelInPreloadRange() {
    if (!sentinel || loading || error || (initialLoaded && !hasMore)) return false;
    return sentinel.getBoundingClientRect().top <= window.innerHeight + preloadAheadPx;
  }

  function isSafariBrowser() {
    const userAgent = navigator.userAgent;
    return /Safari/.test(userAgent) && !/Chrome|Chromium|CriOS|FxiOS|Edg\//.test(userAgent);
  }

  function measureCard(node: HTMLElement, id: string) {
    const update = () => {
      const height = Math.ceil(node.getBoundingClientRect().height);
      if (height > 0 && measuredHeights[id] !== height) {
        const previousHeight = measuredHeights[id] ?? estimatedCardHeight;
        const rowTop = rowTopForID(id);
        const delta = height - previousHeight;
        measuredHeights = { ...measuredHeights, [id]: height };
        if (delta !== 0 && rowTop !== undefined && rowTop + previousHeight <= viewportStart) {
          window.scrollBy({ top: delta, left: 0, behavior: 'instant' });
          updateViewport();
        }
      }
    };
    const observer = new ResizeObserver(update);

    update();
    observer.observe(node);

    return {
      update(nextID: string) {
        id = nextID;
        update();
      },
      destroy() {
        observer.disconnect();
      }
    };
  }

  function prepareAmbient(node: HTMLElement, id: string) {
    const markReady = () => {
      if (!ambientReadyIDs[id]) {
        ambientReadyIDs = { ...ambientReadyIDs, [id]: true };
      }
    };

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          markReady();
          observer.disconnect();
        }
      },
      { rootMargin: '720px 0px 720px 0px' }
    );

    observer.observe(node);

    return {
      update(nextID: string) {
        id = nextID;
        if (ambientReadyIDs[id]) {
          observer.disconnect();
        }
      },
      destroy() {
        observer.disconnect();
      }
    };
  }

  function rowTopForID(id: string) {
    return rows.find((row) => row.item.id === id)?.top;
  }

  function revealCardOverlay(id: string) {
    activeOverlayID = id;
    scheduleCardOverlayHide(id);
  }

  function keepCardOverlay(id: string) {
    activeOverlayID = id;
    clearTimeout(overlayHideTimer);
  }

  function hideCardOverlay(id: string) {
    if (activeOverlayID === id) {
      activeOverlayID = null;
    }
  }

  function scheduleCardOverlayHide(id: string) {
    clearTimeout(overlayHideTimer);
    overlayHideTimer = setTimeout(() => {
      if (activeOverlayID === id) {
        activeOverlayID = null;
      }
    }, 1800);
  }

  function toggleDebugCollapsed() {
    debugCollapsed = !debugCollapsed;
    try {
      window.localStorage.setItem('feed-ai:debug-collapsed', String(debugCollapsed));
    } catch {
      // Ignore storage failures; debug UI should still be usable.
    }
  }

  function readStoredDebugCollapsed() {
    try {
      return window.localStorage.getItem('feed-ai:debug-collapsed') === 'true';
    } catch {
      return false;
    }
  }

  function syncDebugToolsEnabled(event?: StorageEvent | Event) {
    if (event instanceof StorageEvent && event.key !== debugToolsStorageKey) return;
    if (document.visibilityState === 'hidden') return;
    debugToolsEnabled = readDebugToolsEnabled();
  }

  function persistUsername(nextUsername: string) {
    try {
      const storedUsername = nextUsername.trim();
      if (storedUsername) {
        window.localStorage.setItem(usernameStorageKey, storedUsername);
      } else {
        window.localStorage.removeItem(usernameStorageKey);
      }
    } catch {
      // Ignore storage failures; comments can still be submitted with the in-memory name.
    }
  }

  function persistCardBackgroundMode(nextMode: CardBackgroundMode) {
    try {
      window.localStorage.setItem(cardBackgroundModeStorageKey, nextMode);
    } catch {
      // Ignore storage failures; the in-memory debug setting still applies.
    }
  }

  function persistPageBackgroundMode(nextMode: PageBackgroundMode) {
    try {
      window.localStorage.setItem(pageBackgroundModeStorageKey, nextMode);
    } catch {
      // Ignore storage failures; the in-memory background setting still applies.
    }
  }

  function persistBackgroundLayerEnabled(storageKey: string, enabled: boolean) {
    try {
      window.localStorage.setItem(storageKey, String(enabled));
    } catch {
      // Ignore storage failures; the in-memory debug setting still applies.
    }
  }

  function persistFavoriteIDs(nextIDs: string[]) {
    try {
      window.localStorage.setItem(favoritesStorageKey, JSON.stringify(nextIDs));
    } catch {
      // Ignore storage failures; the in-memory favorites list still applies.
    }
  }

  function readStoredUsername() {
    try {
      const storedUsername = window.localStorage.getItem(usernameStorageKey)?.trim();
      return storedUsername || randomUsername();
    } catch {
      return randomUsername();
    }
  }

  function readStoredCardBackgroundMode(): CardBackgroundMode {
    try {
      return window.localStorage.getItem(cardBackgroundModeStorageKey) === 'simple' ? 'simple' : 'ambient';
    } catch {
      return 'ambient';
    }
  }

  function readStoredPageBackgroundMode(): PageBackgroundMode {
    try {
      return window.localStorage.getItem(pageBackgroundModeStorageKey) === 'daylight' ? 'daylight' : 'cosmos';
    } catch {
      return 'cosmos';
    }
  }

  function readStoredBackgroundLayerEnabled(storageKey: string) {
    try {
      const storedValue = window.localStorage.getItem(storageKey);
      if (storedValue !== null) return storedValue !== 'false';
      return window.localStorage.getItem(pageBackgroundEnabledStorageKey) !== 'false';
    } catch {
      return true;
    }
  }

  function persistGlassEffectsMode(mode: GlassEffectsMode) {
    try {
      window.localStorage.setItem(glassEffectsEnabledStorageKey, mode);
    } catch {
      // Ignore storage failures; the in-memory debug setting still applies.
    }
  }

  function readStoredGlassEffectsMode(): GlassEffectsMode {
    try {
      const storedValue = window.localStorage.getItem(glassEffectsEnabledStorageKey);
      if (storedValue === 'off' || storedValue === 'full') return storedValue;
      if (storedValue === 'soft') return 'off';
      if (storedValue === 'false') return 'off';
      if (storedValue === 'true') return 'full';
      return 'off';
    } catch {
      return 'off';
    }
  }

  function resetDebugSwitches() {
    cardBackgroundMode = 'ambient';
    pageBackgroundMode = 'cosmos';
    backgroundParticlesEnabled = true;
    asteroidsEnabled = true;
    glassEffectsMode = 'off';
  }

  function readStoredFavoriteIDs() {
    try {
      const rawValue = window.localStorage.getItem(favoritesStorageKey);
      if (!rawValue) return [];
      const parsedValue = JSON.parse(rawValue);
      if (!Array.isArray(parsedValue)) return [];
      const ids = parsedValue.filter((id): id is string => typeof id === 'string' && id.length > 0);
      return Array.from(new Set(ids));
    } catch {
      return [];
    }
  }

  function toggleFavoriteMode() {
    feedMode = isFavoriteMode ? 'all' : 'favorites';
    resetFeedState();
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
    updateViewport();
    void loadPage();
  }

  function showAllMedia() {
    if (!isFavoriteMode) return;
    feedMode = 'all';
    resetFeedState();
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
    updateViewport();
    void loadPage();
  }

  function toggleFavorite(id: string) {
    if (favoriteIDSet.has(id)) {
      removeFavorite(id);
      return;
    }

    favoriteIDs = [id, ...favoriteIDs.filter((favoriteID) => favoriteID !== id)];
  }

  function removeFavorite(id: string) {
    favoriteIDs = favoriteIDs.filter((favoriteID) => favoriteID !== id);
    if (!isFavoriteMode) return;

    feedRequestVersion += 1;
    loading = false;
    items = items.filter((item) => item.id !== id);
    measuredHeights = omitRecordKey(measuredHeights, id);
    ambientReadyIDs = omitRecordKey(ambientReadyIDs, id);
    pendingLikeCounts = omitRecordKey(pendingLikeCounts, id);
    closeItemState(id);
    syncFavoriteFeedIndexesFromItems();
    if (favoriteIDs.length === 0) {
      initialLoaded = true;
    }
    scheduleViewportUpdate();
    if (items.length === 0 && favoriteIDs.length > 0) {
      initialLoaded = false;
      void loadPage();
    }
  }

  function syncFavoriteFeedIndexesFromItems() {
    if (!isFavoriteMode) return;
    if (favoriteIDs.length === 0) {
      firstFeedIndex = undefined;
      lastFeedIndex = undefined;
      topFeedIndex = undefined;
      bottomFeedIndex = undefined;
      return;
    }

    firstFeedIndex = 0;
    lastFeedIndex = favoriteIDs.length - 1;

    const visibleIndexes = items
      .map((item) => favoriteIDs.indexOf(item.id))
      .filter((index) => index >= 0)
      .map((index) => favoriteIDs.length - 1 - index);

    if (visibleIndexes.length === 0) {
      topFeedIndex = undefined;
      bottomFeedIndex = undefined;
      return;
    }

    topFeedIndex = Math.max(...visibleIndexes);
    bottomFeedIndex = Math.min(...visibleIndexes);
  }

  function closeItemState(id: string) {
    if (commentsPanelItemID === id) commentsPanelItemID = null;
    if (expandedItemID === id) expandedItemID = null;
    if (activeOverlayID === id) activeOverlayID = null;
  }

  function omitRecordKey<T>(record: Record<string, T>, key: string) {
    if (!(key in record)) return record;
    const nextRecord = { ...record };
    delete nextRecord[key];
    return nextRecord;
  }

  function toggleMasterBoard() {
    masterBoardExpanded = !masterBoardExpanded;
    if (masterBoardExpanded) {
      removeBoardActivity('master');
      expandedItemID = null;
      commentsPanelItemID = null;
      selectedActivityMedia = null;
      activityBoardExpandedID = null;
    }
  }

  function openActivityBoard(mediaId: string) {
    removeBoardActivity(mediaId);
    activityBoardExpandedID = mediaId;
    masterBoardExpanded = false;
    activityModalOpen = false;
    selectedActivityMedia = null;
    activityMediaLoading = false;
    activityMediaError = null;
    expandedItemID = null;
    commentsPanelItemID = null;
  }

  function removeBoardActivity(mediaId: string) {
    activityItems = activityItems.filter((item) => item.type !== 'board' || item.mediaId !== mediaId);
  }

  function editedBoardId() {
    if (masterBoardExpanded) return 'master';
    if (activityBoardExpandedID) return activityBoardExpandedID;

    const expandedItem = expandedItemID ? items.find((item) => item.id === expandedItemID) : undefined;
    return isDrawableMediaItem(expandedItem) ? expandedItem.id : null;
  }

  function openActivityBoardFromMedia(mediaId: string) {
    if (!selectedActivityMedia || selectedActivityMedia.id !== mediaId || !isDrawableMediaItem(selectedActivityMedia)) return;
    openActivityBoard(selectedActivityMedia.id);
  }

  function closeActivityBoard() {
    activityBoardExpandedID = null;
  }

  function toggleExpandedItem(id: string) {
    const nextExpandedID = expandedItemID === id ? null : id;
    expandedItemID = nextExpandedID;
    if (nextExpandedID) {
      const item = items.find((candidate) => candidate.id === nextExpandedID);
      if (isDrawableMediaItem(item)) {
        removeBoardActivity(item.id);
      }
    }
    revealCardOverlay(id);
  }

  function closeExpandedItem() {
    expandedItemID = null;
  }

  function openComments(id: string) {
    flushSync(() => {
      commentsPanelItemID = id;
    });
    focusCommentsComposer(id, 0);
    window.setTimeout(() => focusCommentsComposer(id, 0), 0);
    window.setTimeout(() => focusCommentsComposer(id, 0), 80);
  }

  function closeComments() {
    commentsPanelItemID = null;
  }

  async function openActivityMedia(activityItem: ActivityItem) {
    if (activityItem.type === 'board') {
      removeBoardActivity(activityItem.mediaId);

      if (activityItem.mediaId === 'master') {
        masterBoardExpanded = true;
        activityBoardExpandedID = null;
        activityModalOpen = false;
        selectedActivityMedia = null;
        expandedItemID = null;
        commentsPanelItemID = null;
        return;
      }

      openActivityBoard(activityItem.mediaId);
      return;
    }

    activityModalOpen = true;
    selectedActivityMedia = null;
    activityMediaLoading = true;
    activityMediaError = null;

    try {
      selectedActivityMedia = await fetchMediaItem(activityItem.mediaId);
    } catch (err) {
      activityMediaError = err instanceof Error ? err.message : t.activity.mediaLoadFallback;
    } finally {
      activityMediaLoading = false;
    }
  }

  function closeActivityModal() {
    activityModalOpen = false;
    selectedActivityMedia = null;
    activityMediaLoading = false;
    activityMediaError = null;
  }

  function focusCommentsComposer(id: string, attempt: number) {
    const textarea = document.getElementById(`comment-composer-${id}`) as HTMLTextAreaElement | null;
    if (document.activeElement instanceof HTMLElement && document.activeElement !== textarea) {
      document.activeElement.blur();
    }
    textarea?.focus({ preventScroll: true });
    textarea?.setSelectionRange(textarea.value.length, textarea.value.length);

    if (textarea && document.activeElement !== textarea && attempt < 3) {
      window.setTimeout(() => focusCommentsComposer(id, attempt + 1), 60);
    }
  }

  function updateItemComments(mediaId: string, comments: Comment[]) {
    items = items.map((item) =>
      item.id === mediaId
        ? {
            ...item,
            comments: comments.slice(-2),
            commentCount: comments.length
          }
        : item
    );
    if (selectedActivityMedia?.id === mediaId) {
      selectedActivityMedia = {
        ...selectedActivityMedia,
        comments: comments.slice(-2),
        commentCount: comments.length
      };
    }
  }

  function appendItemComment(mediaId: string, comment: Comment) {
    items = items.map((item) => {
      if (item.id !== mediaId || item.comments.some((existing) => existing.id === comment.id)) {
        return item;
      }

      return {
        ...item,
        comments: [...item.comments, comment].slice(-2),
        commentCount: item.commentCount + 1
      };
    });
    if (selectedActivityMedia?.id === mediaId && !selectedActivityMedia.comments.some((existing) => existing.id === comment.id)) {
      selectedActivityMedia = {
        ...selectedActivityMedia,
        comments: [...selectedActivityMedia.comments, comment].slice(-2),
        commentCount: selectedActivityMedia.commentCount + 1
      };
    }
  }

  function updateItemLikeCount(mediaId: string, likeCount: number) {
    items = items.map((item) =>
      item.id === mediaId
        ? {
            ...item,
            likeCount: Math.max(item.likeCount, likeCount)
          }
        : item
    );
    if (selectedActivityMedia?.id === mediaId) {
      selectedActivityMedia = {
        ...selectedActivityMedia,
        likeCount: Math.max(selectedActivityMedia.likeCount, likeCount)
      };
    }
  }

  function updateItemCommentLikeCount(mediaId: string, commentId: string, likeCount: number) {
    items = items.map((item) =>
      item.id === mediaId
        ? {
            ...item,
            comments: item.comments.map((comment) =>
              comment.id === commentId
                ? {
                    ...comment,
                    likeCount: Math.max(comment.likeCount, likeCount)
                  }
                : comment
            )
          }
        : item
    );
    activityItems = activityItems.map((item) =>
      item.type === 'comment' && item.mediaId === mediaId && item.comment.id === commentId
        ? {
            ...item,
            comment: {
              ...item.comment,
              likeCount: Math.max(item.comment.likeCount, likeCount)
            }
          }
        : item
    );
    if (selectedActivityMedia?.id === mediaId) {
      selectedActivityMedia = {
        ...selectedActivityMedia,
        comments: selectedActivityMedia.comments.map((comment) =>
          comment.id === commentId
            ? {
                ...comment,
                likeCount: Math.max(comment.likeCount, likeCount)
              }
            : comment
        )
      };
    }
  }

  async function likeItem(mediaId: string) {
    pendingLikeCounts = { ...pendingLikeCounts, [mediaId]: (pendingLikeCounts[mediaId] ?? 0) + 1 };
    items = items.map((item) =>
      item.id === mediaId
        ? {
            ...item,
            likeCount: item.likeCount + 1
          }
        : item
    );

    try {
      const result = await createLike(mediaId);
      updateItemLikeCount(mediaId, result.likeCount);
    } catch {
      items = items.map((item) =>
        item.id === mediaId
          ? {
              ...item,
              likeCount: Math.max(0, item.likeCount - 1)
            }
          : item
      );
    } finally {
      const remaining = (pendingLikeCounts[mediaId] ?? 1) - 1;
      if (remaining > 0) {
        pendingLikeCounts = { ...pendingLikeCounts, [mediaId]: remaining };
      } else {
        const nextPendingCounts = { ...pendingLikeCounts };
        delete nextPendingCounts[mediaId];
        pendingLikeCounts = nextPendingCounts;
      }
    }
  }

  function prependActivityItem(activityItem: ActivityItem) {
    const nextItems = [
      activityItem,
      ...activityItems.filter((item) => activityItemKey(item) !== activityItemKey(activityItem))
    ];
    activityItems = sortActivityItems(nextItems).slice(0, activityLimit);
  }

  function activityItemKey(item: ActivityItem) {
    return item.type === 'comment' ? `comment-${item.comment.id}` : `board-${item.mediaId}`;
  }

  function activityItemTime(item: ActivityItem) {
    return Date.parse(item.type === 'comment' ? item.comment.createdAt : item.updatedAt) || 0;
  }

  function sortActivityItems(nextItems: ActivityItem[]) {
    return [...nextItems].sort((a, b) => {
      const timeDelta = activityItemTime(b) - activityItemTime(a);
      if (timeDelta !== 0) return timeDelta;
      return activityItemKey(a).localeCompare(activityItemKey(b));
    });
  }

  async function prependActivityFromComment(event: CommentEvent) {
    const item =
      items.find((candidate) => candidate.id === event.mediaId) ??
      (selectedActivityMedia?.id === event.mediaId ? selectedActivityMedia : undefined);

    if (item) {
      prependActivityItem({
        type: 'comment',
        mediaId: item.id,
        mediaDisplayName: item.displayName,
        mediaType: item.type,
        comment: event.comment
      });
      return;
    }

    try {
      const mediaItem = await fetchMediaItem(event.mediaId);
      prependActivityItem({
        type: 'comment',
        mediaId: mediaItem.id,
        mediaDisplayName: mediaItem.displayName,
        mediaType: mediaItem.type,
        comment: event.comment
      });
    } catch {
      // Stale media can disappear between the SSE event and metadata fetch.
    }
  }

  function upsertBoardActivity(event: StrokeEvent, boardName: string) {
    const existing = activityItems.find((item) => item.type === 'board' && item.mediaId === event.mediaId);
    const author = event.stroke.author || t.common.guest;
    const authors = existing?.type === 'board'
      ? [...existing.authors.filter((existingAuthor) => existingAuthor !== author), author]
      : [author];
    const updatedItem: ActivityItem = {
      type: 'board',
      mediaId: event.mediaId,
      boardName,
      strokeCount: existing?.type === 'board' ? existing.strokeCount + 1 : 1,
      authors,
      lastAuthor: author,
      updatedAt: event.stroke.createdAt
    };

    prependActivityItem(updatedItem);
  }

  function boardMediaItem(mediaId: string) {
    return items.find((item) => isDrawableMediaItem(item) && item.id === mediaId);
  }

  function handleBoardActivity(event: StrokeEvent) {
    if (event.mediaId === editedBoardId()) {
      removeBoardActivity(event.mediaId);
      return;
    }

    const item = boardMediaItem(event.mediaId);
    if (item) {
      upsertBoardActivity(event, item.displayName || item.filename || t.common.board);
      return;
    }

    if (event.mediaId === 'master') {
      upsertBoardActivity(event, t.common.masterBoard);
      return;
    }

    upsertBoardActivity(event, t.common.board);
    if (pendingBoardActivityFetches.has(event.mediaId)) return;
    pendingBoardActivityFetches.add(event.mediaId);

    void fetchBoard(event.mediaId)
      .then((data) => {
        activityItems = activityItems.map((activityItem) =>
          activityItem.type === 'board' && activityItem.mediaId === event.mediaId
            ? {
                ...activityItem,
                boardName: data.board.name || activityItem.boardName
              }
            : activityItem
        );
      })
      .catch(() => {
        // Stale board events can outlive a board fetch.
      })
      .finally(() => {
        pendingBoardActivityFetches.delete(event.mediaId);
      });
  }

  function subscribeToBoardActivity() {
    unsubscribeBoardActivity?.();
    unsubscribeBoardActivity = boardEvents.subscribe((event) => {
      handleBoardActivity(event);
    });
  }

  function handleFeedItemCreated(event: FeedItemCreatedEvent) {
    if (uploadFeedRefreshPending) return;
    if (feedMode !== 'all' || !initialLoaded) return;
    if (items.some((item) => item.id === event.item.id)) return;

    const baselineTopIndex = topFeedIndex ?? lastFeedIndex;
    if (baselineTopIndex === undefined || event.index <= baselineTopIndex) return;

    newFeedItemCount = Math.max(newFeedItemCount, event.index - baselineTopIndex);
  }

  function subscribeToCommentEvents() {
    commentEvents?.close();
    commentEvents = new EventSource(commentEventsURL());

    commentEvents.addEventListener('comment', (event) => {
      try {
        const nextEvent = JSON.parse(event.data) as CommentEvent;
        latestCommentEvent = nextEvent;
        appendItemComment(nextEvent.mediaId, nextEvent.comment);
        void prependActivityFromComment(nextEvent);
      } catch {
        // Ignore malformed stream events; feed pagination/full comment loads can recover state.
      }
    });

    commentEvents.addEventListener('like', (event) => {
      try {
        const nextEvent = JSON.parse(event.data) as LikeEvent;
        updateItemLikeCount(nextEvent.mediaId, nextEvent.likeCount);
      } catch {
        // Ignore malformed stream events; feed pagination can recover state.
      }
    });

    commentEvents.addEventListener('comment-like', (event) => {
      try {
        const nextEvent = JSON.parse(event.data) as CommentLikeEvent;
        latestCommentLikeEvent = nextEvent;
        updateItemCommentLikeCount(nextEvent.mediaId, nextEvent.commentId, nextEvent.likeCount);
      } catch {
        // Ignore malformed stream events; feed pagination/full comment loads can recover state.
      }
    });

    commentEvents.addEventListener('feed-item-created', (event) => {
      try {
        const nextEvent = JSON.parse(event.data) as FeedItemCreatedEvent;
        handleFeedItemCreated(nextEvent);
      } catch {
        // Ignore malformed stream events; a manual refresh or feed reload can recover state.
      }
    });
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && activityModalOpen) {
      closeActivityModal();
      return;
    }
    if (event.key === 'Escape' && commentsPanelItemID) {
      closeComments();
      return;
    }
    if (event.key === 'Escape' && expandedItemID) {
      closeExpandedItem();
    }
    if (event.key === 'Escape' && activityBoardExpandedID) {
      closeActivityBoard();
    }
    if (event.key === 'Escape' && masterBoardExpanded) {
      toggleMasterBoard();
    }
  }

  function handleWindowDragEnter(event: DragEvent) {
    if (!hasDraggedFiles(event) || gameActive) return;
    event.preventDefault();
    pageDragActive = true;
    pageDragHasMultipleFiles = hasMultipleDraggedFiles(event);
  }

  function handleWindowDragOver(event: DragEvent) {
    if (!hasDraggedFiles(event) || gameActive) return;
    event.preventDefault();
    pageDragActive = true;
    pageDragHasMultipleFiles = hasMultipleDraggedFiles(event);
  }

  function handleWindowDragLeave(event: DragEvent) {
    if (event.relatedTarget) return;
    pageDragActive = false;
    pageDragHasMultipleFiles = false;
  }

  function handleWindowDrop(event: DragEvent) {
    if (!hasDraggedFiles(event) || gameActive) return;
    event.preventDefault();
    pageDragActive = false;
    pageDragHasMultipleFiles = false;
    const files = Array.from(event.dataTransfer?.files ?? []);
    if (files.length > 0) {
      void handleUploadFiles(files);
    }
  }

  function hasDraggedFiles(event: DragEvent) {
    return Array.from(event.dataTransfer?.types ?? []).includes('Files');
  }

  function hasMultipleDraggedFiles(event: DragEvent) {
    const items = event.dataTransfer?.items;
    if (items && items.length > 0) return items.length > 1;
    return (event.dataTransfer?.files.length ?? 0) > 1;
  }

  function formatFileSize(bytes: number) {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = bytes;
    let unitIndex = 0;
    while (value >= 1024 && unitIndex < units.length - 1) {
      value /= 1024;
      unitIndex += 1;
    }
    return `${Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1)} ${units[unitIndex]}`;
  }

  function updateBackgroundKeyboardFocus(event: PointerEvent) {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const backgroundFocused = !target.closest(
      'article, header, aside, .ui-panel, .drawing-board, .debug-overlay, button, input, textarea, select, a, [role="button"], [role="application"], [role="dialog"]'
    );

    window.dispatchEvent(
      new CustomEvent(backgroundKeyboardFocusEvent, {
        detail: { focused: backgroundFocused }
      })
    );

    if (backgroundFocused) {
      window.dispatchEvent(new CustomEvent(clearActiveVideoEvent));
    }
  }

  function activateGameMode() {
    gameActive = true;
    commentsPanelItemID = null;
    expandedItemID = null;
    activityBoardExpandedID = null;
    activityModalOpen = false;
    selectedActivityMedia = null;
    activeOverlayID = null;
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
  }

  function deactivateGameMode() {
    gameActive = false;
  }
</script>

<svelte:head>
  <title>Feed AI</title>
  <meta
    name="description"
    content={t.meta.description}
  />
</svelte:head>

<svelte:window
  onkeydown={handleWindowKeydown}
  ondragenter={handleWindowDragEnter}
  ondragover={handleWindowDragOver}
  ondragleave={handleWindowDragLeave}
  ondrop={handleWindowDrop}
/>

<main class="app-shell min-h-screen" class:app-shell-daylight={pageBackgroundMode === 'daylight'}>
  {#if backgroundParticlesEnabled}
    <BackgroundParticles mode={pageBackgroundMode} />
  {/if}
  {#if asteroidsEnabled}
    <AsteroidsShip username={commentUsername} />
  {/if}
  {#if !gameActive}
    {#if pageDragActive}
      <div class="pointer-events-none fixed inset-0 z-30 grid place-items-center bg-black/45 p-6 backdrop-blur-sm">
        <div class="ui-panel flex min-h-44 w-full max-w-md flex-col items-center justify-center gap-3 p-6 text-center">
          {#if uploadStatus === 'uploading'}
            <LoaderCircle class="animate-spin text-fg-primary" size={30} />
          {:else if pageDragHasMultipleFiles}
            <AlertCircle class="text-fg-danger" size={30} />
          {:else}
            <Upload class="text-fg-primary" size={30} />
          {/if}
          <p class="text-sm font-bold text-fg-primary">
            {#if uploadStatus === 'uploading'}
              {t.upload.alreadyUploading}
            {:else if pageDragHasMultipleFiles}
              {t.upload.oneFileOnly}
            {:else}
              {t.feed.dropFilesToUpload}
            {/if}
          </p>
        </div>
      </div>
    {/if}

    <div class="app-layout">
      <div class="left-rail">
        <FeedHeader
          {uploadStatus}
          {uploadMessage}
          {uploadProgress}
          {feedMode}
          {newFeedItemCount}
          onToggleFavoriteMode={toggleFavoriteMode}
          onRefreshFeed={refreshFeedFromTop}
          onUploadFiles={handleUploadFiles}
          onCreateBoard={handleCreateBoard}
        />
        <UserSidebar 
          bind:username 
          {pageBackgroundMode}
          {debugToolsEnabled}
          onExpandMasterBoard={toggleMasterBoard}
          onPageBackgroundModeChange={(mode) => (pageBackgroundMode = mode)}
        />
      </div>

      <section
        bind:this={listEl}
        class="virtual-feed mx-auto flex w-full max-w-2xl flex-col px-3 pb-5 pt-4 sm:px-4"
      >
        {#if !initialLoaded && loading}
          <div class="flex min-h-96 items-center justify-center">
            <LoaderCircle class="animate-spin text-fg-primary" size={34} />
          </div>
        {/if}

        {#if isEmpty && !isFavoriteMode}
          <EmptyFeedState onRetry={retry} onUploadFiles={handleUploadFiles} />
        {/if}

        {#if isEmpty && isFavoriteMode}
          <div class="ui-panel flex min-h-80 flex-col items-center justify-center gap-4 p-6 text-center">
            <div class="grid size-12 place-items-center rounded-full border border-border-glass-soft bg-action-bg text-fg-primary">
              <Star size={22} />
            </div>
            <div class="space-y-1">
              <h2 class="text-base font-bold text-fg-primary">{t.feed.noFavoritesTitle}</h2>
              <p class="max-w-xs text-sm font-semibold text-fg-muted">{t.feed.noFavoritesDescription}</p>
            </div>
            <button class="ui-button gap-2" type="button" onclick={showAllMedia}>{t.feed.showAllMedia}</button>
          </div>
        {/if}

        {#if topSpacer > 0}
          <div aria-hidden="true" style={`height: ${topSpacer}px`}></div>
        {/if}

        {#each visibleRows as row (row.item.id)}
          {@const item = row.item}
          <article
            data-media-id={item.id}
            class="ui-media-card mb-4 overflow-hidden"
            class:media-card-expanded={expandedItemID === item.id}
            use:measureCard={item.id}
            use:prepareAmbient={item.id}
          >
            <MediaCard
              {item}
              expanded={expandedItemID === item.id}
              favorite={favoriteIDSet.has(item.id)}
              ambientActive={
                cardBackgroundMode === 'ambient' &&
                (ambientReadyIDs[item.id] || expandedItemID === item.id || commentsPanelItemID === item.id)
              }
              overlayVisible={activeOverlayID === item.id}
              likePending={(pendingLikeCounts[item.id] ?? 0) > 0}
              username={commentUsername}
              {debugToolsEnabled}
              onReveal={revealCardOverlay}
              onKeep={keepCardOverlay}
              onHide={hideCardOverlay}
              onToggleFavorite={toggleFavorite}
              onToggleExpanded={toggleExpandedItem}
              onOpenComments={openComments}
              onLike={likeItem}
            />
            {#if commentsPanelItemID === item.id && !commentsPanelFullscreen}
              <CommentsPanel
                {item}
                username={commentUsername}
                commentEvent={latestCommentEvent}
                commentLikeEvent={latestCommentLikeEvent}
                onClose={closeComments}
                onCommentsChanged={updateItemComments}
                onCommentLikeChanged={updateItemCommentLikeCount}
              />
            {/if}
          </article>
        {/each}

        {#if bottomSpacer > 0}
          <div aria-hidden="true" style={`height: ${bottomSpacer}px`}></div>
        {/if}

        {#if error}
          <FeedError message={error} onRetry={retry} />
        {/if}

        <div bind:this={sentinel} class="flex min-h-20 items-center justify-center">
          {#if loading && initialLoaded}
            <LoaderCircle class="animate-spin text-fg-muted" size={26} />
          {:else if initialLoaded && !hasMore && items.length > 0}
            <p class="text-sm font-semibold text-fg-muted">{t.feed.end}</p>
          {/if}
        </div>
      </section>

      <SocialActivityPanel
        bind:mobileOpen={activityPanelOpen}
        items={activityItems}
        loading={activityLoading}
        error={activityError}
        onSelect={openActivityMedia}
      />
    </div>

    {#if commentsPanelFullscreen && commentsPanelItem}
      <div class="comments-panel-fullscreen">
        <CommentsPanel
          item={commentsPanelItem}
          username={commentUsername}
          commentEvent={latestCommentEvent}
          commentLikeEvent={latestCommentLikeEvent}
          onClose={closeComments}
          onCommentsChanged={updateItemComments}
          onCommentLikeChanged={updateItemCommentLikeCount}
        />
      </div>
    {/if}

    {#if activityModalOpen}
      <ActivityMediaModal
        item={selectedActivityMedia}
        loading={activityMediaLoading}
        error={activityMediaError}
        username={commentUsername}
        commentEvent={latestCommentEvent}
        commentLikeEvent={latestCommentLikeEvent}
        likePending={selectedActivityMedia ? (pendingLikeCounts[selectedActivityMedia.id] ?? 0) > 0 : false}
        onClose={closeActivityModal}
        onCommentsChanged={updateItemComments}
        onCommentLikeChanged={updateItemCommentLikeCount}
        onOpenBoardEdit={openActivityBoardFromMedia}
        {debugToolsEnabled}
        onLike={likeItem}
      />
    {/if}

    <div class="floating-action-stack" aria-label={t.feed.controls}>
      {#if scrollY > viewportHeight * 0.85}
        <button
          class="floating-action-button"
          type="button"
          aria-label={t.feed.scrollToTop}
          title={t.feed.scrollToTop}
          onclick={scrollFeedToTop}
        >
          <Rocket size={19} />
        </button>
      {/if}
      <button
        class="floating-action-button activity-floating-action"
        type="button"
        aria-label={t.activity.open}
        title={t.activity.open}
        onclick={() => (activityPanelOpen = true)}
      >
        <PanelRightOpen size={18} />
      </button>
      {#if debugToolsEnabled && debugCollapsed}
        <FeedDebugOverlay
          collapsed={debugCollapsed}
          loadedCount={items.length}
          mountedCount={visibleRows.length}
          {unloadedBefore}
          {unloadedAfter}
          {visibleStartIndex}
          {visibleEndIndex}
          {loading}
          {initialLoaded}
          {hasMore}
          {feedMode}
          {viewportStart}
          {viewportEnd}
          {viewportHeight}
          {scrollY}
          {listTop}
          {totalHeight}
          {loadedBottom}
          {topSpacer}
          {bottomSpacer}
          {measuredCount}
          {firstFeedIndex}
          {lastFeedIndex}
          {topFeedIndex}
          {bottomFeedIndex}
          {bottomSentinelTop}
          {preloadAheadPx}
          {overscanRows}
          {cardBackgroundMode}
          {backgroundParticlesEnabled}
          {asteroidsEnabled}
          {glassEffectsMode}
          onToggle={toggleDebugCollapsed}
          onCardBackgroundModeChange={(mode) => (cardBackgroundMode = mode)}
          onBackgroundParticlesEnabledChange={(enabled) => (backgroundParticlesEnabled = enabled)}
          onAsteroidsEnabledChange={(enabled) => (asteroidsEnabled = enabled)}
          onGlassEffectsModeChange={(mode) => (glassEffectsMode = mode)}
          onResetSwitches={resetDebugSwitches}
        />
      {/if}
    </div>
  {/if}

  {#if masterBoardExpanded}
    <div class="master-board-expanded-overlay">
      <DrawingBoard
        mediaId="master"
        expanded={true}
        username={commentUsername}
        {debugToolsEnabled}
        onClose={toggleMasterBoard}
      />
    </div>
  {/if}

  {#if activityBoardExpandedID}
    <div class="master-board-expanded-overlay">
      <DrawingBoard
        mediaId={activityBoardExpandedID}
        expanded={true}
        username={commentUsername}
        {debugToolsEnabled}
        onClose={closeActivityBoard}
      />
    </div>
  {/if}
</main>

{#if !gameActive && debugToolsEnabled && !debugCollapsed}
  <FeedDebugOverlay
    collapsed={debugCollapsed}
    loadedCount={items.length}
    mountedCount={visibleRows.length}
    {unloadedBefore}
    {unloadedAfter}
    {visibleStartIndex}
    {visibleEndIndex}
    {loading}
    {initialLoaded}
    {hasMore}
    {feedMode}
    {viewportStart}
    {viewportEnd}
    {viewportHeight}
    {scrollY}
    {listTop}
    {totalHeight}
    {loadedBottom}
    {topSpacer}
    {bottomSpacer}
    {measuredCount}
    {firstFeedIndex}
    {lastFeedIndex}
    {topFeedIndex}
    {bottomFeedIndex}
    {bottomSentinelTop}
    {preloadAheadPx}
    {overscanRows}
    {cardBackgroundMode}
    {backgroundParticlesEnabled}
    {asteroidsEnabled}
    {glassEffectsMode}
    onToggle={toggleDebugCollapsed}
    onCardBackgroundModeChange={(mode) => (cardBackgroundMode = mode)}
    onBackgroundParticlesEnabledChange={(enabled) => (backgroundParticlesEnabled = enabled)}
    onAsteroidsEnabledChange={(enabled) => (asteroidsEnabled = enabled)}
    onGlassEffectsModeChange={(mode) => (glassEffectsMode = mode)}
    onResetSwitches={resetDebugSwitches}
  />
{/if}

<style>
  .app-shell {
    --desktop-feed-section-width: 42rem;
    --desktop-left-rail-width: 18rem;
    --desktop-activity-rail-width: 19rem;
    --desktop-rail-gap: 0rem;
    background: transparent;
  }

  .app-shell-daylight {
    min-height: 100vh;
  }

  .app-shell-daylight::before {
    position: fixed;
    inset: 0;
    z-index: -2;
    background:
      repeating-linear-gradient(45deg, rgb(226 232 240 / 0.08) 0 1px, transparent 1px 96px),
      repeating-linear-gradient(-45deg, rgb(226 232 240 / 0.055) 0 1px, transparent 1px 96px),
      radial-gradient(circle at 18% 14%, rgb(100 116 139 / 0.48), transparent 30rem),
      radial-gradient(circle at 82% 18%, rgb(30 41 59 / 0.46), transparent 26rem),
      radial-gradient(circle at 70% 82%, rgb(148 163 184 / 0.26), transparent 30rem),
      linear-gradient(135deg, rgb(82 89 96), rgb(65 72 79) 48%, rgb(48 55 63));
    content: '';
    pointer-events: none;
  }

  .app-layout {
    display: grid;
    width: 100%;
    grid-template-columns:
      var(--desktop-left-rail-width) var(--desktop-feed-section-width)
      var(--desktop-activity-rail-width);
    column-gap: var(--desktop-rail-gap);
    align-items: start;
    justify-content: center;
  }

  .left-rail {
    position: sticky;
    top: 1rem;
    z-index: 13;
    display: flex;
    width: var(--desktop-left-rail-width);
    flex-direction: column;
    gap: 1rem;
    margin-top: 1rem;
  }

  @media (width < 1344px) {
    .app-layout {
      display: block;
    }

    .left-rail {
      position: relative;
      top: auto;
      width: min(100% - 1.5rem, 40rem);
      margin: 1rem auto 0;
    }
  }

  .master-board-expanded-overlay {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: #0f0f17;
  }

  .floating-action-stack {
    position: fixed;
    right: max(1rem, env(safe-area-inset-right));
    bottom: max(1rem, calc(env(safe-area-inset-bottom) + 1rem));
    z-index: 50;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.65rem;
  }

  .floating-action-button,
  .floating-action-stack :global(.ui-icon-button) {
    display: grid;
    width: 2.75rem;
    height: 2.75rem;
    flex: 0 0 2.75rem;
    place-items: center;
    border: 1px solid var(--color-border-glass);
    border-radius: 999px;
    background: var(--background-image-glass-strong);
    box-shadow: var(--shadow-control);
    color: var(--color-fg-primary);
    backdrop-filter: blur(24px) saturate(180%);
    -webkit-backdrop-filter: blur(24px) saturate(180%);
    transition:
      border-color 140ms ease,
      box-shadow 140ms ease,
      transform 140ms ease;
  }

  .floating-action-button:hover,
  .floating-action-stack :global(.ui-icon-button:hover) {
    border-color: var(--color-border-glass-hover);
    box-shadow: var(--shadow-control-hover);
    transform: translateY(-2px);
  }

  .floating-action-button:focus-visible,
  .floating-action-stack :global(.ui-icon-button:focus-visible) {
    outline: 2px solid rgb(255 255 255 / 0.82);
    outline-offset: 3px;
  }

  .activity-floating-action {
    display: none;
  }

  @media (width < 1344px) {
    .activity-floating-action {
      display: grid;
    }
  }
</style>
