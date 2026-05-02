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
