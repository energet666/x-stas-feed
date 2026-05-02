<script lang="ts">
  import { AlertCircle, RefreshCw } from 'lucide-svelte';

  let {
    message,
    onRetry
  }: {
    message: string;
    onRetry: () => void;
  } = $props();
</script>

<div class="glass-alert items-start">
  <AlertCircle class="mt-0.5 shrink-0" size={20} />
  <div>
    <h2 class="font-semibold">Could not load the feed</h2>
    <p class="text-sm opacity-85">{message}</p>
    <button class="glass-button mt-3 gap-2" type="button" onclick={onRetry}>
      <RefreshCw size={16} />
      Try again
    </button>
  </div>
</div>

<style>
  .glass-alert {
    position: relative;
    display: flex;
    gap: 0.75rem;
    padding: 1rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 28px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow:
      0 1px 0 rgb(255 255 255 / 0.18) inset,
      0 28px 70px rgb(0 0 0 / 0.18),
      0 8px 24px rgb(0 0 0 / 0.12);
    color: rgb(254 202 202);
    backdrop-filter: blur(30px) saturate(170%);
    -webkit-backdrop-filter: blur(30px) saturate(170%);
  }

  .glass-alert::before {
    position: absolute;
    inset: 0;
    pointer-events: none;
    border-radius: inherit;
    background: linear-gradient(135deg, rgb(255 255 255 / 0.16), rgb(255 255 255 / 0) 42%);
    content: '';
  }

  .glass-alert > * {
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
    .glass-alert,
    .glass-button {
      background: rgb(17 19 24 / 0.96);
    }
  }
</style>
