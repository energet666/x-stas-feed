<script lang="ts">
  import { Image, RefreshCw } from 'lucide-svelte';

  let { onRetry }: { onRetry: () => void } = $props();
</script>

<div class="glass-empty flex min-h-96 flex-col items-center justify-center p-8 text-center">
  <Image class="mb-4 text-white/70" size={42} />
  <h2 class="text-lg font-semibold text-white">No media yet</h2>
  <p class="mt-2 max-w-sm text-sm font-medium text-white/65">
    Add photos or videos to <span class="font-mono">test-content</span> and refresh the feed.
  </p>
  <button class="glass-button mt-5 gap-2" type="button" onclick={onRetry}>
    <RefreshCw size={16} />
    Refresh
  </button>
</div>

<style>
  .glass-empty {
    position: relative;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 28px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow:
      0 1px 0 rgb(255 255 255 / 0.18) inset,
      0 28px 70px rgb(0 0 0 / 0.18),
      0 8px 24px rgb(0 0 0 / 0.12);
    color: white;
    backdrop-filter: blur(30px) saturate(170%);
    -webkit-backdrop-filter: blur(30px) saturate(170%);
  }

  .glass-empty::before {
    position: absolute;
    inset: 0;
    pointer-events: none;
    border-radius: inherit;
    background: linear-gradient(135deg, rgb(255 255 255 / 0.16), rgb(255 255 255 / 0) 42%);
    content: '';
  }

  .glass-empty > * {
    position: relative;
  }

  .glass-button {
    display: inline-flex;
    min-height: 2rem;
    align-items: center;
    justify-content: center;
    padding: 0.45rem 0.85rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.16) inset, 0 10px 26px rgb(0 0 0 / 0.14);
    color: rgb(255 255 255 / 0.86);
    font-size: 0.75rem;
    font-weight: 700;
    line-height: 1;
    backdrop-filter: blur(28px) saturate(210%);
    -webkit-backdrop-filter: blur(28px) saturate(210%);
    transition:
      border-color 140ms ease,
      box-shadow 140ms ease,
      transform 140ms ease;
  }

  .glass-button:hover {
    border-color: rgb(255 255 255 / 0.3);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.22) inset, 0 14px 34px rgb(0 0 0 / 0.2);
    transform: translateY(-1px);
  }

  @supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .glass-empty,
    .glass-button {
      background: rgb(17 19 24 / 0.96);
    }
  }
</style>
