<script lang="ts">
  import { Dice5, UserRound } from 'lucide-svelte';

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

  let { username = $bindable(storageFallbackName) }: { username: string } = $props();

  const displayUsername = $derived(username.trim() || storageFallbackName);

  function randomizeUsername() {
    const word = funnyWords[Math.floor(Math.random() * funnyWords.length)] ?? storageFallbackName;
    const suffix = Math.floor(Math.random() * 1000);
    username = `${word} ${suffix}`;
  }
</script>

<aside class="user-sidebar glass-panel" aria-label="Profile settings">
  <div class="flex items-center gap-3">
    <div class="user-sidebar-avatar" aria-hidden="true">
      <UserRound size={20} />
    </div>
    <div class="min-w-0">
      <p class="text-xs font-semibold uppercase text-subtle">Profile</p>
      <p class="truncate text-sm font-semibold text-primary">{displayUsername}</p>
    </div>
  </div>

  <div class="mt-4">
    <label class="mb-2 block text-xs font-semibold uppercase text-subtle" for="username-input">Nickname</label>
    <div class="flex gap-2">
      <input
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

<style>
  .user-sidebar {
    position: fixed;
    top: 6.25rem;
    left: max(1rem, calc((100vw - 78rem) / 2));
    z-index: 12;
    width: 17rem;
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

  @media (width < 1280px) {
    .user-sidebar {
      position: relative;
      top: auto;
      left: auto;
      width: min(100% - 1.5rem, 42rem);
      margin: 1rem auto 0;
    }
  }
</style>
