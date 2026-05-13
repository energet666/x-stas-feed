<script lang="ts">
  type EmojiMartSelection = {
    native?: string;
  };

  type EmojiMartPickerConstructor = new (props: Record<string, unknown>) => unknown;

  let {
    onSelect
  }: {
    onSelect: (emoji: string) => void;
  } = $props();

  let panelEl = $state<HTMLDivElement | undefined>(undefined);

  $effect(() => {
    if (!panelEl) return;

    let cancelled = false;
    const panel = panelEl;
    panel.replaceChildren();
    void mountEmojiPicker(panel, () => cancelled);

    return () => {
      cancelled = true;
      panel.replaceChildren();
    };
  });

  async function mountEmojiPicker(panel: HTMLDivElement, isCancelled: () => boolean) {
    const [{ default: emojiData }, { default: ruI18n }, { Picker }] = await Promise.all([
      import('@emoji-mart/data'),
      import('@emoji-mart/data/i18n/ru.json'),
      import('emoji-mart') as Promise<{ Picker: EmojiMartPickerConstructor }>
    ]);

    if (isCancelled()) return;

    const picker = new Picker({
      data: emojiData,
      i18n: ruI18n,
      locale: 'ru',
      theme: 'dark',
      set: 'native',
      dynamicWidth: true,
      categories: ['frequent', 'people', 'nature', 'foods', 'activity', 'objects', 'symbols'],
      emojiSize: 18,
      emojiButtonSize: 30,
      maxFrequentRows: 1,
      navPosition: 'bottom',
      previewPosition: 'none',
      skinTonePosition: 'none',
      onEmojiSelect: (emoji: EmojiMartSelection) => {
        if (emoji.native) {
          onSelect(emoji.native);
        }
      }
    }) as unknown as HTMLElement;

    panel.replaceChildren(picker);
    patchEmojiPickerLayout(picker);
  }

  function patchEmojiPickerLayout(picker: HTMLElement, attempt = 0) {
    const shadowRoot = picker.shadowRoot;
    if (!shadowRoot) {
      if (attempt < 10) {
        requestAnimationFrame(() => patchEmojiPickerLayout(picker, attempt + 1));
      }
      return;
    }

    if (shadowRoot.getElementById('feed-ai-emoji-picker-layout')) return;

    const style = document.createElement('style');
    style.id = 'feed-ai-emoji-picker-layout';
    style.textContent = `
      :host {
        width: 100% !important;
        height: 100% !important;
        min-height: 0 !important;
      }

      #root {
        height: 100%;
        min-height: 0;
        overflow: hidden;
      }

      .search {
        flex: 0 0 auto;
      }

      .scroll {
        flex: 1 1 auto;
        min-height: 0;
        overscroll-behavior: contain;
      }

      #nav {
        flex: 0 0 auto;
        padding-top: 9px;
        padding-bottom: 9px;
      }
    `;
    shadowRoot.appendChild(style);
  }
</script>

<div bind:this={panelEl} class="emoji-panel" aria-label="Emoji panel"></div>

<style>
  .emoji-panel {
    position: absolute;
    right: 0;
    bottom: calc(100% + 0.55rem);
    z-index: 5;
    width: min(18.75rem, calc(100vw - 2rem));
    height: min(20rem, 44vh);
    min-height: 16rem;
    max-width: calc(100vw - 2rem);
    border: 1px solid var(--color-glass-border-soft);
    border-radius: 1rem;
    background: color-mix(in srgb, var(--color-glass-fallback) 94%, transparent);
    box-shadow: 0 18px 45px rgb(0 0 0 / 0.22);
    overflow: hidden;
    overscroll-behavior: contain;
    backdrop-filter: blur(18px);
  }

  .emoji-panel :global(em-emoji-picker) {
    display: block;
    width: 100%;
    height: 100%;
    min-height: 0;
    overscroll-behavior: contain;
    --border-radius: 1rem;
    --font-size: 13px;
    --category-icon-size: 15px;
    --rgb-background: 17, 19, 24;
    --rgb-input: 35, 37, 42;
    --rgb-accent: 244, 63, 94;
  }
</style>
