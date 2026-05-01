<script lang="ts">
  import { onMount } from 'svelte';
  import { AlertCircle, Image, LoaderCircle, RefreshCw, Video } from 'lucide-svelte';

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

  const pageSize = 6;

  let items = $state<MediaItem[]>([]);
  let nextCursor = $state<string | undefined>(undefined);
  let loading = $state(false);
  let initialLoaded = $state(false);
  let error = $state<string | null>(null);
  let sentinel = $state<HTMLDivElement | undefined>(undefined);

  const hasMore = $derived(!initialLoaded || nextCursor !== undefined);
  const isEmpty = $derived(initialLoaded && items.length === 0 && !error);

  onMount(() => {
    void loadPage();
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

<main class="min-h-screen">
  <header class="sticky top-0 z-20 border-b border-base-300/80 bg-base-100/90 backdrop-blur">
    <div class="mx-auto flex h-16 w-full max-w-2xl items-center justify-between px-4">
      <div>
        <h1 class="text-xl font-semibold tracking-normal">Feed AI</h1>
        <p class="text-xs text-base-content/60">Local media stream</p>
      </div>
      <div class="badge badge-neutral gap-2">
        <Image size={14} />
        {items.length}
      </div>
    </div>
  </header>

  <section class="mx-auto flex w-full max-w-2xl flex-col gap-4 px-3 py-5 sm:px-4">
    {#if !initialLoaded && loading}
      <div class="flex min-h-96 items-center justify-center">
        <LoaderCircle class="animate-spin text-primary" size={34} />
      </div>
    {/if}

    {#if isEmpty}
      <div class="flex min-h-96 flex-col items-center justify-center rounded-box border border-dashed border-base-300 bg-base-100 p-8 text-center">
        <Image class="mb-4 text-base-content/50" size={42} />
        <h2 class="text-lg font-semibold">No media yet</h2>
        <p class="mt-2 max-w-sm text-sm text-base-content/60">
          Add photos or videos to <span class="font-mono">test-content</span> and refresh the feed.
        </p>
        <button class="btn btn-primary btn-sm mt-5 gap-2" type="button" onclick={retry}>
          <RefreshCw size={16} />
          Refresh
        </button>
      </div>
    {/if}

    {#each items as item (item.id)}
      <article class="overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm">
        <div class="flex items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0">
            <h2 class="truncate text-sm font-semibold">{item.filename}</h2>
            <p class="text-xs text-base-content/55">{formatDate(item.modifiedAt)}</p>
          </div>
          <div class="badge badge-outline shrink-0 gap-1">
            {#if item.type === 'video'}
              <Video size={13} />
            {:else}
              <Image size={13} />
            {/if}
            {formatSize(item.size)}
          </div>
        </div>

        <div class="bg-black">
          {#if item.type === 'video'}
            <video
              class="max-h-[76vh] w-full bg-black object-contain"
              controls
              playsinline
              preload="metadata"
              src={item.url}
              title={item.filename}
            >
              <track kind="captions" />
            </video>
          {:else}
            <img
              class="max-h-[76vh] w-full bg-black object-contain"
              src={item.url}
              alt={item.filename}
              loading="lazy"
              decoding="async"
            />
          {/if}
        </div>
      </article>
    {/each}

    {#if error}
      <div class="alert alert-error items-start">
        <AlertCircle class="mt-0.5 shrink-0" size={20} />
        <div>
          <h2 class="font-semibold">Could not load the feed</h2>
          <p class="text-sm opacity-85">{error}</p>
          <button class="btn btn-sm mt-3 gap-2" type="button" onclick={retry}>
            <RefreshCw size={16} />
            Try again
          </button>
        </div>
      </div>
    {/if}

    <div bind:this={sentinel} class="flex min-h-20 items-center justify-center">
      {#if loading && initialLoaded}
        <span class="loading loading-spinner loading-md text-primary"></span>
      {:else if initialLoaded && !hasMore && items.length > 0}
        <p class="text-sm text-base-content/50">End of feed</p>
      {/if}
    </div>
  </section>
</main>
