<script lang="ts">
  import { Image } from 'lucide-svelte';

  let { loadedCount }: { loadedCount: number } = $props();
</script>

<header class="glass-nav sticky top-0 z-20">
  <div class="mx-auto flex h-16 w-full max-w-2xl items-center justify-between px-4">
    <div>
      <h1 class="text-xl font-semibold tracking-normal text-primary">Feed AI</h1>
      <p class="text-xs font-semibold text-muted">Local media stream</p>
    </div>
    <div class="glass-pill gap-2">
      <Image size={14} />
      {loadedCount} loaded
    </div>
  </div>
</header>
