<script lang="ts">
  import { onMount } from 'svelte';
  import { AlertCircle, Bug, ChevronDown, ChevronUp, Image, LoaderCircle, RefreshCw, Video } from 'lucide-svelte';
  import FeedVideoPlayer from './FeedVideoPlayer.svelte';

  type MediaItem = {
    id: string;
    filename: string;
    type: 'image' | 'video';
    url: string;
    mimeType: string;
    size: number;
    modifiedAt: string;
  };

  type FeedPage = {
    items: MediaItem[];
    nextCursor?: string;
  };

  const pageSize = 24;
  const estimatedCardHeight = 760;
  const itemGap = 16;
  const overscan = 1600;

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
  let overlayHideTimer: ReturnType<typeof setTimeout> | undefined = undefined;

  const hasMore = $derived(!initialLoaded || nextCursor !== undefined);
  const isEmpty = $derived(initialLoaded && items.length === 0 && !error);
  const viewportStart = $derived(Math.max(0, scrollY - listTop));
  const viewportEnd = $derived(viewportStart + viewportHeight);
  const rows = $derived.by(() => {
    let top = 0;
    return items.map((item) => {
      const height = (measuredHeights[item.id] ?? estimatedCardHeight) + itemGap;
      const row = { item, top, height };
      top += height;
      return row;
    });
  });
  const totalHeight = $derived(rows.reduce((total, row) => total + row.height, 0));
  const visibleRows = $derived(
    rows.filter((row) => row.top + row.height >= viewportStart - overscan && row.top <= viewportEnd + overscan)
  );
  const visibleStartIndex = $derived(
    visibleRows.length > 0 ? items.findIndex((item) => item.id === visibleRows[0].item.id) : -1
  );
  const visibleEndIndex = $derived(visibleStartIndex >= 0 ? visibleStartIndex + visibleRows.length - 1 : -1);
  const topSpacer = $derived(visibleRows[0]?.top ?? 0);
  const bottomSpacer = $derived.by(() => {
    const last = visibleRows.at(-1);
    if (!last) return 0;
    return Math.max(0, totalHeight - last.top - last.height);
  });
  const unloadedBefore = $derived(Math.max(0, visibleStartIndex));
  const unloadedAfter = $derived(Math.max(0, items.length - visibleEndIndex - 1));

  onMount(() => {
    updateViewport();
    window.addEventListener('scroll', updateViewport, { passive: true });
    window.addEventListener('resize', updateViewport);
    void loadPage();

    return () => {
      clearTimeout(overlayHideTimer);
      window.removeEventListener('scroll', updateViewport);
      window.removeEventListener('resize', updateViewport);
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

  async function loadPage() {
    if (loading || (initialLoaded && !nextCursor)) return;

    loading = true;
    error = null;

    try {
      const params = new URLSearchParams({ limit: String(pageSize) });
      if (nextCursor) params.set('cursor', nextCursor);

      const response = await fetch(`/api/feed?${params.toString()}`);
      if (!response.ok) {
        throw new Error(`Feed request failed with ${response.status}`);
      }

      const page = (await response.json()) as FeedPage;
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
    scrollY = window.scrollY;
    viewportHeight = window.innerHeight;
    if (listEl) {
      listTop = listEl.getBoundingClientRect().top + window.scrollY;
    }
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

  function rowTopForID(id: string) {
    return rows.find((row) => row.item.id === id)?.top;
  }

  function revealCardOverlay(id: string) {
    activeOverlayID = id;
    clearTimeout(overlayHideTimer);
    overlayHideTimer = setTimeout(() => {
      if (activeOverlayID === id) {
        activeOverlayID = null;
      }
    }, 1800);
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

  function formatSize(size: number) {
    if (size < 1024 * 1024) return `${Math.max(1, Math.round(size / 1024))} KB`;
    return `${(size / 1024 / 1024).toFixed(1)} MB`;
  }

  function formatDate(value: string) {
    return new Intl.DateTimeFormat(undefined, {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(value));
  }
</script>

<svelte:head>
  <title>Feed AI</title>
  <meta
    name="description"
    content="An infinite local media feed for photos and videos."
  />
</svelte:head>

<main class="app-shell min-h-screen">
  <header class="glass-nav sticky top-0 z-20">
    <div class="mx-auto flex h-16 w-full max-w-2xl items-center justify-between px-4">
      <div>
        <h1 class="text-xl font-semibold tracking-normal text-white">Feed AI</h1>
        <p class="text-xs font-semibold text-white/65">Local media stream</p>
      </div>
      <div class="glass-pill gap-2">
        <Image size={14} />
        {items.length} loaded
      </div>
    </div>
  </header>

  <section bind:this={listEl} class="virtual-feed mx-auto flex w-full max-w-2xl flex-col px-3 py-5 sm:px-4">
    {#if !initialLoaded && loading}
      <div class="flex min-h-96 items-center justify-center">
        <LoaderCircle class="animate-spin text-primary" size={34} />
      </div>
    {/if}

    {#if isEmpty}
      <div class="glass-empty flex min-h-96 flex-col items-center justify-center p-8 text-center">
        <Image class="mb-4 text-white/70" size={42} />
        <h2 class="text-lg font-semibold text-white">No media yet</h2>
        <p class="mt-2 max-w-sm text-sm font-medium text-white/65">
          Add photos or videos to <span class="font-mono">test-content</span> and refresh the feed.
        </p>
        <button class="glass-button mt-5 gap-2" type="button" onclick={retry}>
          <RefreshCw size={16} />
          Refresh
        </button>
      </div>
    {/if}

    {#if topSpacer > 0}
      <div aria-hidden="true" style={`height: ${topSpacer}px`}></div>
    {/if}

    {#each visibleRows as row (row.item.id)}
      {@const item = row.item}
      <article
        class="glass-card mb-4 overflow-hidden"
        use:measureCard={item.id}
      >
        <div
          class="media-frame bg-black"
          role="presentation"
          onpointermove={() => revealCardOverlay(item.id)}
          onpointerenter={() => revealCardOverlay(item.id)}
          onmousemove={() => revealCardOverlay(item.id)}
          onmouseenter={() => revealCardOverlay(item.id)}
          ontouchstart={() => revealCardOverlay(item.id)}
          onpointerdown={() => revealCardOverlay(item.id)}
          onclick={() => revealCardOverlay(item.id)}
          onfocusin={() => keepCardOverlay(item.id)}
          onmouseleave={() => hideCardOverlay(item.id)}
        >
          <div class="card-overlay" class:card-overlay-visible={activeOverlayID === item.id}>
            <div class="min-w-0">
              <h2 class="truncate text-sm font-semibold text-white">{item.filename}</h2>
              <p class="text-xs font-semibold text-white/62">{formatDate(item.modifiedAt)}</p>
            </div>
            <div class="glass-pill shrink-0 gap-1">
              {#if item.type === 'video'}
                <Video size={13} />
              {:else}
                <Image size={13} />
              {/if}
              {formatSize(item.size)}
            </div>
          </div>

          {#if item.type === 'video'}
            <FeedVideoPlayer src={item.url} title={item.filename} />
          {:else}
            <img
              class="h-full w-full bg-black object-contain"
              src={item.url}
              alt={item.filename}
              loading="lazy"
              decoding="async"
            />
          {/if}
        </div>
      </article>
    {/each}

    {#if bottomSpacer > 0}
      <div aria-hidden="true" style={`height: ${bottomSpacer}px`}></div>
    {/if}

    {#if error}
      <div class="glass-alert items-start">
        <AlertCircle class="mt-0.5 shrink-0" size={20} />
        <div>
          <h2 class="font-semibold">Could not load the feed</h2>
          <p class="text-sm opacity-85">{error}</p>
          <button class="glass-button mt-3 gap-2" type="button" onclick={retry}>
            <RefreshCw size={16} />
            Try again
          </button>
        </div>
      </div>
    {/if}

    <div bind:this={sentinel} class="flex min-h-20 items-center justify-center">
      {#if loading && initialLoaded}
        <span class="loading loading-spinner loading-md text-white/70"></span>
      {:else if initialLoaded && !hasMore && items.length > 0}
        <p class="text-sm font-semibold text-white/65">End of feed</p>
      {/if}
    </div>
  </section>
</main>

<aside class="debug-overlay">
  <button
    class="debug-toggle"
    type="button"
    aria-label={debugCollapsed ? 'Expand debug overlay' : 'Collapse debug overlay'}
    onclick={() => (debugCollapsed = !debugCollapsed)}
  >
    <span class="inline-flex items-center gap-2">
      <Bug size={14} />
      Feed debug
    </span>
    {#if debugCollapsed}
      <ChevronUp size={14} />
    {:else}
      <ChevronDown size={14} />
    {/if}
  </button>

  {#if !debugCollapsed}
    <dl class="debug-grid">
      <div>
        <dt>Loaded</dt>
        <dd>{items.length}</dd>
      </div>
      <div>
        <dt>Mounted</dt>
        <dd>{visibleRows.length}</dd>
      </div>
      <div>
        <dt>Unloaded</dt>
        <dd>{unloadedBefore} / {unloadedAfter}</dd>
      </div>
      <div>
        <dt>Window</dt>
        <dd>{visibleStartIndex >= 0 ? `${visibleStartIndex}-${visibleEndIndex}` : '-'}</dd>
      </div>
      <div>
        <dt>Cursor</dt>
        <dd>{nextCursor ?? 'end'}</dd>
      </div>
      <div>
        <dt>Loading</dt>
        <dd>{loading ? 'yes' : 'no'}</dd>
      </div>
      <div>
        <dt>Viewport</dt>
        <dd>{Math.round(viewportStart)}-{Math.round(viewportEnd)}</dd>
      </div>
      <div>
        <dt>Total height</dt>
        <dd>{Math.round(totalHeight)}</dd>
      </div>
      <div>
        <dt>Spacers</dt>
        <dd>{Math.round(topSpacer)} / {Math.round(bottomSpacer)}</dd>
      </div>
      <div>
        <dt>Measured</dt>
        <dd>{Object.keys(measuredHeights).length}</dd>
      </div>
    </dl>
  {/if}
</aside>
