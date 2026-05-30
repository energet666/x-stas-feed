<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { Dice5 } from 'lucide-svelte';
  import { fallbackUsername, randomUsername } from '../lib/usernames';
  import DrawingBoard from './DrawingBoard.svelte';

  let { 
    username = $bindable(fallbackUsername),
    onExpandMasterBoard
  }: { 
    username: string;
    onExpandMasterBoard: () => void;
  } = $props();
  let usernameInput = $state<HTMLInputElement | undefined>(undefined);

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
</script>

<aside class="user-sidebar ui-panel ui-panel-side" aria-label="Profile settings">
  <div class="user-sidebar-body">
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
      <button class="ui-icon-button" type="button" aria-label="Generate random nickname" onclick={randomizeUsername}>
        <Dice5 size={18} />
      </button>
    </div>
  </div>

</aside>

<button
  class="master-board-preview-container"
  type="button"
  aria-label="Open master drawing board"
  title="Open master drawing board"
  onclick={onExpandMasterBoard}
>
  <DrawingBoard boardId="master" expanded={false} {username} previewFill={true} />
</button>

<style>
  .user-sidebar {
    width: 100%;
    overflow: hidden;
  }

  .user-sidebar-body {
    padding: 1rem;
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
    border-radius: var(--radius-overlay);
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
