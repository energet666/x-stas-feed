<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { Dice5, UserRound } from 'lucide-svelte';
  import DrawingBoard from './DrawingBoard.svelte';

  const storageFallbackName = 'Guest';
  const funnyWords = [
    'Бодрый Кабачок',
    'Сонный Пельмень',
    'Хитрый Вареник',
    'Ламповый Сырник',
    'Космический Бублик',
    'Шустрый Компот',
    'Тихий Самовар',
    'Веселый Укроп',
    'Серьезный Батон',
    'Мятный Блинчик',
    'Пушистый Квас',
    'Грозный Сухарик',
    'Нежный Чебурек',
    'Важный Пончик',
    'Секретный Огурчик',
    'Сахарный Кексик',
    'Пиксельный Пряник',
    'Турбо Ряженка',
    'Уютный Лапоть',
    'Блестящий Крендель'
  ];

  let { 
    username = $bindable(storageFallbackName),
    onExpandMasterBoard
  }: { 
    username: string;
    onExpandMasterBoard: () => void;
  } = $props();
  let usernameInput = $state<HTMLInputElement | undefined>(undefined);

  const displayUsername = $derived(username.trim() || storageFallbackName);

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
    const word = funnyWords[Math.floor(Math.random() * funnyWords.length)] ?? storageFallbackName;
    const suffix = Math.floor(Math.random() * 1000);
    username = `${word} ${suffix}`;
  }
</script>

<aside class="user-sidebar glass-panel" aria-label="Profile settings">
  <header class="user-sidebar-header">
    <div class="user-sidebar-avatar" aria-hidden="true">
      <UserRound size={20} />
    </div>
    <div class="min-w-0">
      <p class="text-xs font-semibold uppercase text-subtle">Profile</p>
      <h2 class="truncate text-base font-bold text-primary">{displayUsername}</h2>
    </div>
  </header>

  <div class="user-sidebar-body">
    <label class="mb-2 block text-xs font-semibold uppercase text-subtle" for="username-input">Nickname</label>
    <div class="flex gap-2">
      <input
        bind:this={usernameInput}
        id="username-input"
        class="username-input"
        type="text"
        maxlength="40"
        autocomplete="nickname"
        bind:value={username}
        placeholder={storageFallbackName}
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

  .user-sidebar-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    border-bottom: 1px solid var(--color-glass-border-soft);
    padding: 1rem;
  }

  .user-sidebar-body {
    padding: 1rem;
  }

  .user-sidebar-avatar {
    display: grid;
    width: 2.5rem;
    height: 2.5rem;
    flex-shrink: 0;
    place-items: center;
    border: 1px solid var(--color-glass-border-soft);
    border-radius: var(--radius-control);
    background: var(--color-button-bg);
    color: var(--color-secondary);
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
