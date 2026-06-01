<script lang="ts">
  import { Bug, ChevronDown } from 'lucide-svelte';
  import { uiText as t } from '../lib/ui_text';

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
    backgroundParticlesEnabled,
    asteroidsEnabled,
    glassEffectsMode,
    onToggle,
    onCardBackgroundModeChange,
    onBackgroundParticlesEnabledChange,
    onAsteroidsEnabledChange,
    onGlassEffectsModeChange,
    onResetSwitches
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
    backgroundParticlesEnabled: boolean;
    asteroidsEnabled: boolean;
    glassEffectsMode: 'off' | 'full';
    onToggle: () => void;
    onCardBackgroundModeChange: (mode: 'simple' | 'ambient') => void;
    onBackgroundParticlesEnabledChange: (enabled: boolean) => void;
    onAsteroidsEnabledChange: (enabled: boolean) => void;
    onGlassEffectsModeChange: (mode: 'off' | 'full') => void;
    onResetSwitches: () => void;
  } = $props();

  const formatPx = (value: number | undefined) => (value === undefined ? '-' : `${Math.round(value)}px`);
  const formatNumber = (value: number | undefined) => (value === undefined ? '-' : String(value));
  const formatBool = (value: boolean) => (value ? t.debug.yes : t.debug.no);
</script>

{#if collapsed}
  <button
    class="ui-icon-button"
    type="button"
    aria-label={t.debug.expand}
    title={t.debug.expand}
    onclick={onToggle}
  >
    <Bug size={17} />
  </button>
{:else}
  <aside class="debug-overlay ui-panel-side">
    <button
      class="debug-toggle"
      type="button"
      aria-label={t.debug.collapse}
      title={t.debug.collapse}
      onclick={onToggle}
    >
      <span class="inline-flex items-center gap-2">
        <Bug size={14} />
        {t.debug.title}
      </span>
      <ChevronDown size={14} />
    </button>

    <div class="debug-scroll-body">
      <div class="debug-control-row">
        <span>{t.debug.cardBg}</span>
        <div class="debug-segmented" role="group" aria-label={t.debug.cardBackgroundMode}>
          <button
            class:debug-segment-active={cardBackgroundMode === 'simple'}
            type="button"
            onclick={() => onCardBackgroundModeChange('simple')}
          >
            {t.debug.simple}
          </button>
          <button
            class:debug-segment-active={cardBackgroundMode === 'ambient'}
            type="button"
            onclick={() => onCardBackgroundModeChange('ambient')}
          >
            {t.debug.ambient}
          </button>
        </div>
      </div>

      <div class="debug-control-row">
        <span>{t.debug.particles}</span>
        <div class="debug-segmented" role="group" aria-label={t.debug.particles}>
          <button
            class:debug-segment-active={!backgroundParticlesEnabled}
            type="button"
            onclick={() => onBackgroundParticlesEnabledChange(false)}
          >
            {t.debug.off}
          </button>
          <button
            class:debug-segment-active={backgroundParticlesEnabled}
            type="button"
            onclick={() => onBackgroundParticlesEnabledChange(true)}
          >
            {t.debug.on}
          </button>
        </div>
      </div>

      <div class="debug-control-row">
        <span>{t.debug.asteroids}</span>
        <div class="debug-segmented" role="group" aria-label={t.debug.asteroids}>
          <button
            class:debug-segment-active={!asteroidsEnabled}
            type="button"
            onclick={() => onAsteroidsEnabledChange(false)}
          >
            {t.debug.off}
          </button>
          <button
            class:debug-segment-active={asteroidsEnabled}
            type="button"
            onclick={() => onAsteroidsEnabledChange(true)}
          >
            {t.debug.on}
          </button>
        </div>
      </div>

      <div class="debug-control-row">
        <span>{t.debug.glass}</span>
        <div class="debug-segmented" role="group" aria-label={t.debug.glass}>
          <button
            class:debug-segment-active={glassEffectsMode === 'off'}
            type="button"
            onclick={() => onGlassEffectsModeChange('off')}
          >
            {t.debug.off}
          </button>
          <button
            class:debug-segment-active={glassEffectsMode === 'full'}
            type="button"
            onclick={() => onGlassEffectsModeChange('full')}
          >
            {t.debug.on}
          </button>
        </div>
      </div>

      <button class="debug-reset-button" type="button" onclick={onResetSwitches}>{t.debug.resetDefaults}</button>

      <div class="debug-section-title">{t.debug.window}</div>
      <dl class="debug-grid">
      <div>
        <dt>{t.debug.mode}</dt>
        <dd>{feedMode}</dd>
      </div>
      <div>
        <dt>{t.debug.ready}</dt>
        <dd>{formatBool(initialLoaded)}</dd>
      </div>
      <div>
        <dt>{t.debug.items}</dt>
        <dd>{loadedCount}</dd>
      </div>
      <div>
        <dt>{t.debug.mounted}</dt>
        <dd>{mountedCount}</dd>
      </div>
      <div>
        <dt>{t.debug.visibleRows}</dt>
        <dd>{visibleStartIndex >= 0 ? `${visibleStartIndex}-${visibleEndIndex}` : '-'}</dd>
      </div>
      <div>
        <dt>{t.debug.hiddenRows}</dt>
        <dd>{unloadedBefore} / {unloadedAfter}</dd>
      </div>
      <div>
        <dt>{t.debug.measured}</dt>
        <dd>{measuredCount}</dd>
      </div>
      <div>
        <dt>{t.debug.overscan}</dt>
        <dd>{overscanRows}</dd>
      </div>
      </dl>

      <div class="debug-section-title">{t.debug.feedIndexes}</div>
      <dl class="debug-grid">
      <div>
        <dt>{t.debug.bounds}</dt>
        <dd>{formatNumber(firstFeedIndex)}-{formatNumber(lastFeedIndex)}</dd>
      </div>
      <div>
        <dt>{t.debug.loadedSpan}</dt>
        <dd>{formatNumber(topFeedIndex)}-{formatNumber(bottomFeedIndex)}</dd>
      </div>
      <div>
        <dt>{t.debug.loadOlder}</dt>
        <dd>{formatBool(hasMore)} / {formatBool(loading)}</dd>
      </div>
      <div>
        <dt>{t.debug.preload}</dt>
        <dd>{preloadAheadPx}px</dd>
      </div>
      </dl>

      <div class="debug-section-title">{t.debug.geometry}</div>
      <dl class="debug-grid">
      <div>
        <dt>{t.debug.viewport}</dt>
        <dd>{Math.round(viewportStart)}-{Math.round(viewportEnd)}</dd>
      </div>
      <div>
        <dt>{t.debug.windowHeight}</dt>
        <dd>{Math.round(viewportHeight)}px</dd>
      </div>
      <div>
        <dt>{t.debug.scrollList}</dt>
        <dd>{Math.round(scrollY)} / {Math.round(listTop)}</dd>
      </div>
      <div>
        <dt>{t.debug.itemsHeight}</dt>
        <dd>{Math.round(totalHeight)}px</dd>
      </div>
      <div>
        <dt>{t.debug.loadedBottom}</dt>
        <dd>{Math.round(loadedBottom)}px</dd>
      </div>
      <div>
        <dt>{t.debug.spacers}</dt>
        <dd>{Math.round(topSpacer)} / {Math.round(bottomSpacer)}</dd>
      </div>
      <div>
        <dt>{t.debug.sentinel}</dt>
        <dd>{formatPx(bottomSentinelTop)}</dd>
      </div>
      </dl>
    </div>
  </aside>
{/if}
