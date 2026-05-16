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
    let userInitiatedFocus = false;
    let startupTimer: number | undefined = undefined;
    const focusCheckTimers: number[] = [];

    const markUserInitiatedFocus = () => {
      userInitiatedFocus = true;
    };
    const blurRestoredFocus = () => {
      if (!userInitiatedFocus && document.activeElement === usernameInput) {
        usernameInput.blur();
      }
    };
    const scheduleFocusCheck = (delay: number) => {
      focusCheckTimers.push(window.setTimeout(blurRestoredFocus, delay));
    };
    const handleFocusIn = (event: FocusEvent) => {
      if (event.target === usernameInput) {
        scheduleFocusCheck(0);
      }
    };

    window.addEventListener('pointerdown', markUserInitiatedFocus, { capture: true });
    window.addEventListener('keydown', markUserInitiatedFocus, { capture: true });
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

    startupTimer = window.setTimeout(() => {
      document.removeEventListener('focusin', handleFocusIn, { capture: true });
    }, 1500);

    return () => {
      window.removeEventListener('pointerdown', markUserInitiatedFocus, { capture: true });
      window.removeEventListener('keydown', markUserInitiatedFocus, { capture: true });
      document.removeEventListener('focusin', handleFocusIn, { capture: true });
      if (startupTimer !== undefined) {
        window.clearTimeout(startupTimer);
      }
      for (const timer of focusCheckTimers) {
        window.clearTimeout(timer);
      }
    };
  });

  function randomizeUsername() {
    username = randomUsername();
  }
</script>

<aside class="user-sidebar glass-panel side-glass-panel" aria-label="Profile settings">
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
      <button class="glass-icon-button" type="button" aria-label="Generate random nickname" onclick={randomizeUsername}>
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
  <DrawingBoard boardId="master" expanded={false} {username} />
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
    border: 1px solid var(--color-glass-border-soft);
    border-radius: var(--radius-control);
    background: var(--color-button-bg);
    padding: 0 0.85rem;
    color: var(--color-primary);
    font-size: 0.875rem;
    font-weight: 700;
    outline: none;
  }

  .username-input::placeholder {
    color: var(--color-subtle);
  }

  .username-input:focus {
    border-color: var(--color-glass-border-hover);
  }

  .master-board-preview-container {
    position: relative;
    display: block;
    width: 100%;
    aspect-ratio: 3 / 2;
    border-radius: var(--radius-overlay);
    border: 1px solid var(--color-glass-border-soft);
    overflow: hidden;
    cursor: pointer;
    padding: 0;
    transition: all 0.2s ease;
    background: #0f0f17;
  }

  .master-board-preview-container:hover {
    border-color: var(--color-glass-border-hover);
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }

  .master-board-preview-container :global(.drawing-preview) {
    height: 100%;
  }

</style>
