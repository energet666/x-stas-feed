<script lang="ts">
  import { flushSync, onMount } from 'svelte';
  import { LoaderCircle } from 'lucide-svelte';
  import AsteroidsShip from './components/AsteroidsShip.svelte';
  import BackgroundParticles from './components/BackgroundParticles.svelte';
  import CommentsPanel from './components/CommentsPanel.svelte';
  import EmptyFeedState from './components/EmptyFeedState.svelte';
  import FeedDebugOverlay from './components/FeedDebugOverlay.svelte';
  import FeedError from './components/FeedError.svelte';
  import FeedHeader from './components/FeedHeader.svelte';
  import MediaCard from './components/MediaCard.svelte';
  import UserSidebar from './components/UserSidebar.svelte';
  import { commentEventsURL, fetchFeedPage, type Comment, type CommentEvent, type MediaItem } from './lib/feed';

  const pageSize = 6;
  const estimatedCardHeight = 760;
  const itemGap = 16;
  const overscanRows = 2;
  const usernameStorageKey = 'feed-ai:comment-username';
  const cardBackgroundModeStorageKey = 'feed-ai:card-background-mode';
  const clearActiveVideoEvent = 'feed-ai:video-clear-active';
  const gameStartedEvent = 'feed-ai:game-started';

  type CardBackgroundMode = 'simple' | 'ambient';

  type FeedRow = {
    item: MediaItem;
    top: number;
    height: number;
  };

  let items = $state<MediaItem[]>([]);
  let nextCursor = $state<string | undefined>(undefined);
  let loading = $state(false);
  let initialLoaded = $state(false);
  let error = $state<string | null>(null);
  let sentinel = $state<HTMLDivElement | undefined>(undefined);
  let listEl = $state<HTMLElement | undefined>(undefined);
  let scrollY = $state(0);
  let viewportHeight = $state(0);
  let listTop = $state(0);
  let measuredHeights = $state<Record<string, number>>({});
  let debugCollapsed = $state(false);
  let activeOverlayID = $state<string | null>(null);
  let expandedItemID = $state<string | null>(null);
  let commentsPanelItemID = $state<string | null>(null);
  let latestCommentEvent = $state<CommentEvent | null>(null);
  let username = $state('Guest');
  let usernameStorageReady = $state(false);
  let cardBackgroundMode = $state<CardBackgroundMode>('ambient');
  let cardBackgroundModeStorageReady = $state(false);
  let gameActive = $state(false);
  let ambientReadyIDs = $state<Record<string, boolean>>({});
  let overlayHideTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  let viewportFrameID: number | undefined = undefined;
  let commentEvents: EventSource | undefined = undefined;

  const hasMore = $derived(!initialLoaded || nextCursor !== undefined);
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
  const visibleRange = $derived.by(() => {
    if (rows.length === 0) {
      return { start: -1, end: -1 };
    }

    const firstVisible = rows.findIndex((row) => row.top + row.height >= viewportStart);
    if (firstVisible === -1) {
      const lastIndex = rows.length - 1;
      return {
        start: Math.max(0, lastIndex - overscanRows),
        end: lastIndex
      };
    }

    let lastVisible = firstVisible;
    while (lastVisible + 1 < rows.length && rows[lastVisible + 1].top <= viewportEnd) {
      lastVisible += 1;
    }

    return {
      start: Math.max(0, firstVisible - overscanRows),
      end: Math.min(rows.length - 1, lastVisible + overscanRows)
    };
  });
  const visibleRows = $derived(
    visibleRange.start >= 0 ? rows.slice(visibleRange.start, visibleRange.end + 1) : []
  );
  const visibleStartIndex = $derived(visibleRange.start);
  const visibleEndIndex = $derived(visibleRange.end);
  const topSpacer = $derived(visibleRows[0]?.top ?? 0);
  const bottomSpacer = $derived.by(() => {
    const last = visibleRows.at(-1);
    if (!last) return 0;
    return Math.max(0, totalHeight - last.top - last.height);
  });
  const unloadedBefore = $derived(Math.max(0, visibleStartIndex));
  const unloadedAfter = $derived(Math.max(0, items.length - visibleEndIndex - 1));
  const measuredCount = $derived(Object.keys(measuredHeights).length);
  const commentUsername = $derived(username.trim() || 'Guest');

  onMount(() => {
    document.documentElement.classList.toggle('safari-browser', isSafariBrowser());
    debugCollapsed = readStoredDebugCollapsed();
    username = readStoredUsername();
    usernameStorageReady = true;
    cardBackgroundMode = readStoredCardBackgroundMode();
    cardBackgroundModeStorageReady = true;
    updateViewport();
    window.addEventListener('scroll', scheduleViewportUpdate, { passive: true });
    window.addEventListener('resize', scheduleViewportUpdate);
    window.addEventListener(gameStartedEvent, activateGameMode);
    subscribeToCommentEvents();
    void loadPage();

    return () => {
      document.documentElement.classList.remove('safari-browser');
      clearTimeout(overlayHideTimer);
      if (viewportFrameID !== undefined) {
        cancelAnimationFrame(viewportFrameID);
      }
      commentEvents?.close();
      window.removeEventListener('scroll', scheduleViewportUpdate);
      window.removeEventListener('resize', scheduleViewportUpdate);
      window.removeEventListener(gameStartedEvent, activateGameMode);
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
      { rootMargin: '800px 0px 800px 0px' }
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  });

  $effect(() => {
    const previousOverflow = document.body.style.overflow;
    if (expandedItemID) {
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
  });

  async function loadPage() {
    if (loading || (initialLoaded && !nextCursor)) return;

    loading = true;
    error = null;

    try {
      const page = await fetchFeedPage({ cursor: nextCursor, limit: pageSize });
      items = [...items, ...page.items];
      nextCursor = page.nextCursor;
      initialLoaded = true;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Unable to load feed';
      initialLoaded = true;
    } finally {
      loading = false;
    }
  }

  function retry() {
    void loadPage();
  }

  function updateViewport() {
    viewportFrameID = undefined;
    scrollY = window.scrollY;
    viewportHeight = window.innerHeight;
    if (listEl) {
      listTop = listEl.getBoundingClientRect().top + window.scrollY;
    }
  }

  function scheduleViewportUpdate() {
    if (viewportFrameID !== undefined) return;
    viewportFrameID = requestAnimationFrame(updateViewport);
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
        if (delta !== 0 && rowTop !== undefined && rowTop + previousHeight < viewportStart) {
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
    scheduleCardOverlayHide(id);
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

  function readStoredUsername() {
    try {
      const storedUsername = window.localStorage.getItem(usernameStorageKey)?.trim();
      return storedUsername || 'Guest';
    } catch {
      return 'Guest';
    }
  }

  function readStoredCardBackgroundMode(): CardBackgroundMode {
    try {
      return window.localStorage.getItem(cardBackgroundModeStorageKey) === 'simple' ? 'simple' : 'ambient';
    } catch {
      return 'ambient';
    }
  }

  function toggleExpandedItem(id: string) {
    expandedItemID = expandedItemID === id ? null : id;
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
  }

  function subscribeToCommentEvents() {
    commentEvents?.close();
    commentEvents = new EventSource(commentEventsURL());

    commentEvents.addEventListener('comment', (event) => {
      try {
        const nextEvent = JSON.parse(event.data) as CommentEvent;
        latestCommentEvent = nextEvent;
        appendItemComment(nextEvent.mediaId, nextEvent.comment);
      } catch {
        // Ignore malformed stream events; feed pagination/full comment loads can recover state.
      }
    });
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && commentsPanelItemID) {
      closeComments();
      return;
    }
    if (event.key === 'Escape' && expandedItemID) {
      closeExpandedItem();
    }
  }

  function clearActiveVideoFromPageBackground(event: PointerEvent) {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    if (target.closest('article, header, aside, .debug-overlay, button, input, textarea, select, a, [role="button"]')) {
      return;
    }

    window.dispatchEvent(new CustomEvent(clearActiveVideoEvent));
  }

  function activateGameMode() {
    gameActive = true;
    commentsPanelItemID = null;
    expandedItemID = null;
    activeOverlayID = null;
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
  }
</script>

<svelte:head>
  <title>Feed AI</title>
  <meta
    name="description"
    content="An infinite local media feed for photos and videos."
  />
</svelte:head>

<svelte:window onkeydown={handleWindowKeydown} />

<main class="app-shell min-h-screen" onpointerdown={clearActiveVideoFromPageBackground}>
  <BackgroundParticles />
  <AsteroidsShip username={commentUsername} />
  {#if !gameActive}
    <FeedHeader loadedCount={items.length} />
    <UserSidebar bind:username />

    <section bind:this={listEl} class="virtual-feed mx-auto flex w-full max-w-2xl flex-col px-3 py-5 sm:px-4">
      {#if !initialLoaded && loading}
        <div class="flex min-h-96 items-center justify-center">
          <LoaderCircle class="animate-spin text-primary" size={34} />
        </div>
      {/if}

      {#if isEmpty}
        <EmptyFeedState onRetry={retry} />
      {/if}

      {#if topSpacer > 0}
        <div aria-hidden="true" style={`height: ${topSpacer}px`}></div>
      {/if}

      {#each visibleRows as row (row.item.id)}
        {@const item = row.item}
        <article
          class="glass-card mb-4 overflow-hidden"
          class:media-card-expanded={expandedItemID === item.id}
          use:measureCard={item.id}
          use:prepareAmbient={item.id}
        >
          <MediaCard
            {item}
            expanded={expandedItemID === item.id}
            ambientActive={
              cardBackgroundMode === 'ambient' &&
              (ambientReadyIDs[item.id] || expandedItemID === item.id || commentsPanelItemID === item.id)
            }
            overlayVisible={activeOverlayID === item.id}
            onReveal={revealCardOverlay}
            onKeep={keepCardOverlay}
            onHide={hideCardOverlay}
            onToggleExpanded={toggleExpandedItem}
            onOpenComments={openComments}
          />
          {#if commentsPanelItemID === item.id}
            <CommentsPanel
              {item}
              username={commentUsername}
              commentEvent={latestCommentEvent}
              onClose={closeComments}
              onCommentsChanged={updateItemComments}
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
          <LoaderCircle class="animate-spin text-muted" size={26} />
        {:else if initialLoaded && !hasMore && items.length > 0}
          <p class="text-sm font-semibold text-muted">End of feed</p>
        {/if}
      </div>
    </section>
  {/if}
</main>

{#if !gameActive}
  <FeedDebugOverlay
    collapsed={debugCollapsed}
    loadedCount={items.length}
    mountedCount={visibleRows.length}
    {unloadedBefore}
    {unloadedAfter}
    {visibleStartIndex}
    {visibleEndIndex}
    {nextCursor}
    {loading}
    {viewportStart}
    {viewportEnd}
    {totalHeight}
    {topSpacer}
    {bottomSpacer}
    {measuredCount}
    {cardBackgroundMode}
    onToggle={toggleDebugCollapsed}
    onCardBackgroundModeChange={(mode) => (cardBackgroundMode = mode)}
  />
{/if}
