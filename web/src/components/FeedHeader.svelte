<script lang="ts">
  import { Image } from 'lucide-svelte';

  let { loadedCount }: { loadedCount: number } = $props();
</script>

<header class="glass-nav sticky top-0 z-20">
  <div class="mx-auto flex h-16 w-full max-w-2xl items-center justify-between px-4">
    <div>
      <h1 class="text-xl font-semibold tracking-normal text-white">Feed AI</h1>
      <p class="text-xs font-semibold text-white/65">Local media stream</p>
    </div>
    <div class="glass-pill gap-2">
      <Image size={14} />
      {loadedCount} loaded
    </div>
  </div>
</header>

<style>
  .glass-nav {
    border-bottom: 1px solid rgb(255 255 255 / 0.14);
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.3)),
      rgb(0 0 0 / 0.2);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.16) inset, 0 18px 48px rgb(0 0 0 / 0.18);
    backdrop-filter: blur(30px) saturate(170%);
    -webkit-backdrop-filter: blur(30px) saturate(170%);
  }

  .glass-pill {
    display: inline-flex;
    min-height: 2rem;
    align-items: center;
    justify-content: center;
    padding: 0.35rem 0.65rem;
    border: 1px solid rgb(255 255 255 / 0.18);
    border-radius: 999px;
    background:
      linear-gradient(180deg, rgb(20 20 24 / 0.54), rgb(10 10 14 / 0.34)),
      rgb(0 0 0 / 0.24);
    box-shadow: 0 1px 0 rgb(255 255 255 / 0.16) inset, 0 10px 26px rgb(0 0 0 / 0.14);
    color: rgb(255 255 255 / 0.72);
    font-size: 0.75rem;
    font-weight: 700;
    line-height: 1;
    backdrop-filter: blur(28px) saturate(210%);
    -webkit-backdrop-filter: blur(28px) saturate(210%);
  }

  @supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
    .glass-nav,
    .glass-pill {
      background: rgb(17 19 24 / 0.96);
    }
  }
</style>
