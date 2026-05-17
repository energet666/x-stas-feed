<script lang="ts">
  import { Bug, ChevronDown } from 'lucide-svelte';

  let {
    collapsed,
    loadedCount,
    mountedCount,
    unloadedBefore,
    unloadedAfter,
    visibleStartIndex,
    visibleEndIndex,
    loading,
    initialLoaded,
    hasMore,
    feedMode,
    viewportStart,
    viewportEnd,
    viewportHeight,
    scrollY,
    listTop,
    totalHeight,
    loadedBottom,
    topSpacer,
    bottomSpacer,
    measuredCount,
    firstFeedIndex,
    lastFeedIndex,
    topFeedIndex,
    bottomFeedIndex,
    bottomSentinelTop,
    preloadAheadPx,
    overscanRows,
    cardBackgroundMode,
    onToggle,
    onCardBackgroundModeChange
  }: {
    collapsed: boolean;
    loadedCount: number;
    mountedCount: number;
    unloadedBefore: number;
    unloadedAfter: number;
    visibleStartIndex: number;
    visibleEndIndex: number;
    loading: boolean;
    initialLoaded: boolean;
    hasMore: boolean;
    feedMode: 'all' | 'favorites';
    viewportStart: number;
    viewportEnd: number;
    viewportHeight: number;
    scrollY: number;
    listTop: number;
    totalHeight: number;
    loadedBottom: number;
    topSpacer: number;
    bottomSpacer: number;
    measuredCount: number;
    firstFeedIndex?: number;
    lastFeedIndex?: number;
    topFeedIndex?: number;
    bottomFeedIndex?: number;
    bottomSentinelTop?: number;
    preloadAheadPx: number;
    overscanRows: number;
    cardBackgroundMode: 'simple' | 'ambient';
    onToggle: () => void;
    onCardBackgroundModeChange: (mode: 'simple' | 'ambient') => void;
  } = $props();

  const formatPx = (value: number | undefined) => (value === undefined ? '-' : `${Math.round(value)}px`);
  const formatNumber = (value: number | undefined) => (value === undefined ? '-' : String(value));
  const formatBool = (value: boolean) => (value ? 'yes' : 'no');
</script>

<aside class="debug-overlay side-glass-panel" class:debug-overlay-collapsed={collapsed}>
  <button
    class="debug-toggle"
    class:debug-toggle-collapsed={collapsed}
    type="button"
    aria-label={collapsed ? 'Expand debug overlay' : 'Collapse debug overlay'}
    title={collapsed ? 'Expand debug overlay' : 'Collapse debug overlay'}
    onclick={onToggle}
  >
    {#if collapsed}
      <Bug size={17} />
    {:else}
      <span class="inline-flex items-center gap-2">
        <Bug size={14} />
        Feed debug
      </span>
      <ChevronDown size={14} />
    {/if}
  </button>

  {#if !collapsed}
    <div class="debug-control-row">
      <span>Card bg</span>
      <div class="debug-segmented" role="group" aria-label="Card background mode">
        <button
          class:debug-segment-active={cardBackgroundMode === 'simple'}
          type="button"
          onclick={() => onCardBackgroundModeChange('simple')}
        >
          Simple
        </button>
        <button
          class:debug-segment-active={cardBackgroundMode === 'ambient'}
          type="button"
          onclick={() => onCardBackgroundModeChange('ambient')}
        >
          Ambient
        </button>
      </div>
    </div>

    <div class="debug-section-title">Window</div>
    <dl class="debug-grid">
      <div>
        <dt>Mode</dt>
        <dd>{feedMode}</dd>
      </div>
      <div>
        <dt>Ready</dt>
        <dd>{formatBool(initialLoaded)}</dd>
      </div>
      <div>
        <dt>Items</dt>
        <dd>{loadedCount}</dd>
      </div>
      <div>
        <dt>Mounted</dt>
        <dd>{mountedCount}</dd>
      </div>
      <div>
        <dt>Visible rows</dt>
        <dd>{visibleStartIndex >= 0 ? `${visibleStartIndex}-${visibleEndIndex}` : '-'}</dd>
      </div>
      <div>
        <dt>Hidden rows</dt>
        <dd>{unloadedBefore} / {unloadedAfter}</dd>
      </div>
      <div>
        <dt>Measured</dt>
        <dd>{measuredCount}</dd>
      </div>
      <div>
        <dt>Overscan</dt>
        <dd>{overscanRows}</dd>
      </div>
    </dl>

    <div class="debug-section-title">Feed indexes</div>
    <dl class="debug-grid">
      <div>
        <dt>Bounds</dt>
        <dd>{formatNumber(firstFeedIndex)}-{formatNumber(lastFeedIndex)}</dd>
      </div>
      <div>
        <dt>Loaded span</dt>
        <dd>{formatNumber(topFeedIndex)}-{formatNumber(bottomFeedIndex)}</dd>
      </div>
      <div>
        <dt>Load older</dt>
        <dd>{formatBool(hasMore)} / {formatBool(loading)}</dd>
      </div>
      <div>
        <dt>Preload</dt>
        <dd>{preloadAheadPx}px</dd>
      </div>
    </dl>

    <div class="debug-section-title">Geometry</div>
    <dl class="debug-grid">
      <div>
        <dt>Viewport</dt>
        <dd>{Math.round(viewportStart)}-{Math.round(viewportEnd)}</dd>
      </div>
      <div>
        <dt>Window h</dt>
        <dd>{Math.round(viewportHeight)}px</dd>
      </div>
      <div>
        <dt>Scroll / list</dt>
        <dd>{Math.round(scrollY)} / {Math.round(listTop)}</dd>
      </div>
      <div>
        <dt>Items height</dt>
        <dd>{Math.round(totalHeight)}px</dd>
      </div>
      <div>
        <dt>Loaded bottom</dt>
        <dd>{Math.round(loadedBottom)}px</dd>
      </div>
      <div>
        <dt>Spacers</dt>
        <dd>{Math.round(topSpacer)} / {Math.round(bottomSpacer)}</dd>
      </div>
      <div>
        <dt>Sentinel</dt>
        <dd>{formatPx(bottomSentinelTop)}</dd>
      </div>
    </dl>
  {/if}
</aside>
