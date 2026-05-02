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
    onToggle
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
    onToggle: () => void;
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

<style>
  .debug-overlay {
    position: fixed;
    right: 0.75rem;
    bottom: 0.75rem;
    z-index: 50;
    width: min(22rem, calc(100vw - 1.5rem));
    overflow: hidden;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 24px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.18) inset, 0 22px 54px rgb(0 0 0 / 0.22);
    color: white;
    font-size: 0.75rem;
    line-height: 1.2;
    backdrop-filter: blur(34px) saturate(210%);
    -webkit-backdrop-filter: blur(34px) saturate(210%);
  }

  .debug-toggle {
    display: flex;
    min-height: 2.25rem;
    width: 100%;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.5rem 0.75rem;
    font-weight: 700;
    text-align: left;
  }

  .debug-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1px;
    border-top: 1px solid rgb(255 255 255 / 0.14);
    background: rgb(255 255 255 / 0.08);
  }

  .debug-grid > div {
    min-width: 0;
    padding: 0.5rem 0.75rem;
    background: rgb(255 255 255 / 0.05);
  }

  .debug-grid dt {
    color: rgb(255 255 255 / 0.54);
    font-size: 0.68rem;
    font-weight: 600;
    text-transform: uppercase;
  }

  .debug-grid dd {
    margin: 0.15rem 0 0;
    overflow-wrap: anywhere;
    color: rgb(255 255 255 / 0.88);
    font-variant-numeric: tabular-nums;
    font-weight: 700;
  }

  @supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .debug-overlay {
      background: rgb(17 19 24 / 0.96);
    }
  }
</style>
