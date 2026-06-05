<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { ChevronDown, ChevronUp, Dice5, Moon, Sun } from 'lucide-svelte';
  import { fallbackUsername, randomUsername } from '../lib/usernames';
  import { uiText as t } from '../lib/ui_text';
  import DrawingBoard from './DrawingBoard.svelte';

  const visitorPanelCollapsedStorageKey = 'feed-ai:visitor-panel-collapsed';

  let { 
    username = $bindable(fallbackUsername),
    pageBackgroundMode = 'cosmos',
    debugToolsEnabled = false,
    onExpandMasterBoard,
    onPageBackgroundModeChange
  }: { 
    username: string;
    pageBackgroundMode?: 'cosmos' | 'daylight';
    debugToolsEnabled?: boolean;
    onExpandMasterBoard: () => void;
    onPageBackgroundModeChange?: (mode: 'cosmos' | 'daylight') => void;
  } = $props();
  let usernameInput = $state<HTMLInputElement | undefined>(undefined);
  let visitorPanelCollapsed = $state(false);

  onMount(() => {
    let usernameFocusAllowedUntil = 0;
    let lastFocusedElement: HTMLElement | null = null;
    const focusCheckTimers: number[] = [];

    const allowUsernameFocus = () => {
      usernameFocusAllowedUntil = performance.now() + 1000;
    };
    const isUsernameFocusAllowed = () => {
      return performance.now() <= usernameFocusAllowedUntil;
    };
    visitorPanelCollapsed = readStoredVisitorPanelCollapsed();

    const blurRestoredFocus = () => {
      if (!isUsernameFocusAllowed() && document.activeElement === usernameInput) {
        usernameInput.blur();
        if (lastFocusedElement?.isConnected) {
          lastFocusedElement.focus({ preventScroll: true });
        }
      }
    };
    const scheduleFocusCheck = (delay: number) => {
      focusCheckTimers.push(window.setTimeout(blurRestoredFocus, delay));
    };
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      if (target === usernameInput || target.closest('label[for="username-input"]')) {
        allowUsernameFocus();
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Tab') {
        allowUsernameFocus();
      }
    };
    const handleFocusIn = (event: FocusEvent) => {
      if (event.target === usernameInput) {
        scheduleFocusCheck(0);
      } else if (event.target instanceof HTMLElement) {
        lastFocusedElement = event.target;
      }
    };

    window.addEventListener('pointerdown', handlePointerDown, { capture: true });
    window.addEventListener('keydown', handleKeyDown, { capture: true });
    document.addEventListener('focusin', handleFocusIn, { capture: true });

    void tick().then(() => {
      if (document.activeElement === usernameInput) {
        usernameInput.blur();
      }
      scheduleFocusCheck(0);
      scheduleFocusCheck(50);
      scheduleFocusCheck(250);
      scheduleFocusCheck(750);
    });

    return () => {
      window.removeEventListener('pointerdown', handlePointerDown, { capture: true });
      window.removeEventListener('keydown', handleKeyDown, { capture: true });
      document.removeEventListener('focusin', handleFocusIn, { capture: true });
      for (const timer of focusCheckTimers) {
        window.clearTimeout(timer);
      }
    };
  });

  function randomizeUsername() {
    username = randomUsername();
  }

  function toggleVisitorPanel() {
    visitorPanelCollapsed = !visitorPanelCollapsed;
    persistVisitorPanelCollapsed(visitorPanelCollapsed);
  }

  function readStoredVisitorPanelCollapsed() {
    try {
      return window.localStorage.getItem(visitorPanelCollapsedStorageKey) === 'true';
    } catch {
      return false;
    }
  }

  function persistVisitorPanelCollapsed(collapsed: boolean) {
    try {
      window.localStorage.setItem(visitorPanelCollapsedStorageKey, String(collapsed));
    } catch {
      // Ignore storage failures; the in-memory collapsed state still applies.
    }
  }

  function selectPageBackground(mode: 'cosmos' | 'daylight') {
    onPageBackgroundModeChange?.(mode);
  }
</script>

<aside class="user-sidebar ui-panel ui-panel-side" aria-label={t.profile.settings}>
  <button
    class="visitor-panel-toggle"
    type="button"
    aria-expanded={!visitorPanelCollapsed}
    aria-controls="visitor-panel-body"
    aria-label={visitorPanelCollapsed ? t.profile.expandVisitorPanel : t.profile.collapseVisitorPanel}
    onclick={toggleVisitorPanel}
  >
    <span>{t.profile.visitor}</span>
    {#if visitorPanelCollapsed}
      <ChevronDown size={18} />
    {:else}
      <ChevronUp size={18} />
    {/if}
  </button>

  {#if !visitorPanelCollapsed}
    <div id="visitor-panel-body" class="user-sidebar-body">
      <label class="mb-2 block text-xs font-semibold text-fg-subtle" for="username-input">Тебя зовут:</label>
      <div class="flex gap-2">
        <input
          bind:this={usernameInput}
          id="username-input"
          class="username-input"
          type="text"
          maxlength="40"
          autocomplete="nickname"
          bind:value={username}
          placeholder={fallbackUsername}
        />
        <span class="nickname-random-tooltip">
          <button
            class="ui-icon-button"
            type="button"
            aria-label={t.profile.randomNickname}
            aria-describedby="random-nickname-tooltip"
            onclick={randomizeUsername}
          >
            <Dice5 size={18} />
          </button>
          <span id="random-nickname-tooltip" class="nickname-random-tooltip-content" role="tooltip">
            {t.profile.randomNicknameTooltip}
          </span>
        </span>
      </div>

      <div class="background-field">
        <span class="background-label">{t.profile.background}</span>
        <div class="background-segmented" role="group" aria-label={t.profile.background}>
          <button
            class:background-segment-active={pageBackgroundMode === 'cosmos'}
            type="button"
            aria-pressed={pageBackgroundMode === 'cosmos'}
            onclick={() => selectPageBackground('cosmos')}
          >
            <Moon size={15} />
            <span>{t.profile.backgroundCosmos}</span>
          </button>
          <button
            class:background-segment-active={pageBackgroundMode === 'daylight'}
            type="button"
            aria-pressed={pageBackgroundMode === 'daylight'}
            onclick={() => selectPageBackground('daylight')}
          >
            <Sun size={15} />
            <span>{t.profile.backgroundDaylight}</span>
          </button>
        </div>
      </div>
    </div>
  {/if}

</aside>

<button
  class="master-board-preview-container"
  type="button"
  aria-label={t.board.openMaster}
  title={t.board.openMaster}
  onclick={onExpandMasterBoard}
>
  <DrawingBoard mediaId="master" expanded={false} {username} previewFill={true} {debugToolsEnabled} />
</button>

<style>
  .user-sidebar {
    width: 100%;
    overflow: hidden;
  }

  .visitor-panel-toggle {
    display: flex;
    width: 100%;
    min-height: 3.25rem;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.85rem 1rem;
    color: var(--color-fg-secondary);
    font-size: 0.875rem;
    font-weight: 600;
    text-align: left;
    transition:
      background 140ms ease,
      color 140ms ease;
  }

  .visitor-panel-toggle:hover {
    background: var(--color-action-hover);
    color: var(--color-fg-primary);
  }

  .visitor-panel-toggle :global(svg) {
    flex-shrink: 0;
  }

  .user-sidebar-body {
    border-top: 1px solid var(--color-border-glass-soft);
    padding: 0.85rem 1rem 1rem;
  }

  .username-input {
    min-width: 0;
    width: 100%;
    height: 2.25rem;
    border: 1px solid var(--color-border-glass-soft);
    border-radius: var(--radius-control);
    background: var(--color-action-bg);
    padding: 0 0.85rem;
    color: var(--color-fg-primary);
    font-size: 0.875rem;
    font-weight: 700;
    outline: none;
  }

  .username-input::placeholder {
    color: var(--color-fg-subtle);
  }

  .username-input:focus {
    border-color: var(--color-border-glass-hover);
  }

  .nickname-random-tooltip {
    position: relative;
    display: inline-grid;
    flex-shrink: 0;
    place-items: center;
  }

  .nickname-random-tooltip-content {
    position: absolute;
    top: calc(100% + 0.5rem);
    right: 0;
    z-index: 3;
    width: max-content;
    max-width: min(13rem, calc(100vw - 2rem));
    padding: 0.45rem 0.6rem;
    border: 1px solid var(--color-border-glass-soft);
    border-radius: 0.625rem;
    background: rgb(8 9 12 / 0.94);
    color: var(--color-fg-secondary);
    font-size: 0.72rem;
    font-weight: 700;
    line-height: 1.25;
    opacity: 0;
    pointer-events: none;
    text-align: left;
    transform: translateY(-0.2rem);
    transition:
      opacity 140ms ease,
      transform 140ms ease;
  }

  .nickname-random-tooltip:hover .nickname-random-tooltip-content,
  .nickname-random-tooltip:focus-within .nickname-random-tooltip-content {
    opacity: 1;
    transform: translateY(0);
  }

  .background-field {
    margin-top: 0.85rem;
  }

  .background-label {
    display: block;
    margin-bottom: 0.45rem;
    color: var(--color-fg-subtle);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .background-segmented {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    overflow: hidden;
    border: 1px solid var(--color-border-glass-soft);
    border-radius: var(--radius-control);
    background: var(--color-action-bg);
  }

  .background-segmented button {
    display: inline-flex;
    min-width: 0;
    min-height: 2.15rem;
    align-items: center;
    justify-content: center;
    gap: 0.4rem;
    padding: 0 0.65rem;
    color: var(--color-fg-muted);
    font-size: 0.75rem;
    font-weight: 700;
    transition:
      background 140ms ease,
      color 140ms ease;
  }

  .background-segmented button:hover,
  .background-segment-active {
    background: var(--color-action-hover-strong);
    color: var(--color-fg-primary);
  }

  .background-segmented :global(svg) {
    flex-shrink: 0;
  }

  .master-board-preview-container {
    position: relative;
    display: block;
    width: 100%;
    aspect-ratio: 3 / 2;
    border-radius: var(--radius-panel);
    border: 0;
    overflow: hidden;
    cursor: pointer;
    padding: 0;
    transition: all 0.2s ease;
    background: transparent;
  }

  .master-board-preview-container:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }

  .master-board-preview-container :global(.drawing-preview) {
    height: 100%;
  }

</style>
