<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { ChevronDown, ChevronUp, Dice5 } from 'lucide-svelte';
  import { fallbackUsername, randomUsername } from '../lib/usernames';
  import { uiText as t } from '../lib/ui_text';
  import DrawingBoard from './DrawingBoard.svelte';

  const visitorPanelCollapsedStorageKey = 'feed-ai:visitor-panel-collapsed';

  let { 
    username = $bindable(fallbackUsername),
    debugToolsEnabled = false,
    onExpandMasterBoard
  }: { 
    username: string;
    debugToolsEnabled?: boolean;
    onExpandMasterBoard: () => void;
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
      <label class="mb-2 block text-xs font-semibold text-subtle" for="username-input">Тебя зовут:</label>
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
        <button class="ui-icon-button" type="button" aria-label={t.profile.randomNickname} onclick={randomizeUsername}>
          <Dice5 size={18} />
        </button>
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
    color: var(--color-text-secondary);
    font-size: 0.875rem;
    font-weight: 800;
    text-align: left;
    transition:
      background 140ms ease,
      color 140ms ease;
  }

  .visitor-panel-toggle:hover {
    background: var(--color-action-hover);
    color: var(--color-text-primary);
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
    color: var(--color-text-primary);
    font-size: 0.875rem;
    font-weight: 700;
    outline: none;
  }

  .username-input::placeholder {
    color: var(--color-text-subtle);
  }

  .username-input:focus {
    border-color: var(--color-border-glass-hover);
  }

  .master-board-preview-container {
    position: relative;
    display: block;
    width: 100%;
    aspect-ratio: 3 / 2;
    border-radius: var(--radius-panel);
    border: 1px solid var(--color-border-glass-soft);
    overflow: hidden;
    cursor: pointer;
    padding: 0;
    transition: all 0.2s ease;
    background: #0f0f17;
  }

  .master-board-preview-container:hover {
    border-color: var(--color-border-glass-hover);
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }

  .master-board-preview-container :global(.drawing-preview) {
    height: 100%;
  }

</style>
