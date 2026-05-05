<script lang="ts">
  import { Bug, ChevronDown, ChevronUp } from 'lucide-svelte';

  let {
    collapsed,
    loadedCount,
    mountedCount,
    unloadedBefore,
    unloadedAfter,
    visibleStartIndex,
    visibleEndIndex,
    nextCursor,
    loading,
    viewportStart,
    viewportEnd,
    totalHeight,
    topSpacer,
    bottomSpacer,
    measuredCount,
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
    nextCursor?: string;
    loading: boolean;
    viewportStart: number;
    viewportEnd: number;
    totalHeight: number;
    topSpacer: number;
    bottomSpacer: number;
    measuredCount: number;
    cardBackgroundMode: 'simple' | 'ambient';
    onToggle: () => void;
    onCardBackgroundModeChange: (mode: 'simple' | 'ambient') => void;
  } = $props();
</script>

<aside class="debug-overlay">
  <button
    class="debug-toggle"
    type="button"
    aria-label={collapsed ? 'Expand debug overlay' : 'Collapse debug overlay'}
    onclick={onToggle}
  >
    <span class="inline-flex items-center gap-2">
      <Bug size={14} />
      Feed debug
    </span>
    {#if collapsed}
      <ChevronUp size={14} />
    {:else}
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

    <dl class="debug-grid">
      <div>
        <dt>Loaded</dt>
        <dd>{loadedCount}</dd>
      </div>
      <div>
        <dt>Mounted</dt>
        <dd>{mountedCount}</dd>
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
        <dd>{measuredCount}</dd>
      </div>
    </dl>
  {/if}
</aside>
