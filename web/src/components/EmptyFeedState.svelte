<script lang="ts">
  import { Image, RefreshCw, Upload } from 'lucide-svelte';

  let {
    onRetry,
    onUploadFiles
  }: {
    onRetry: () => void;
    onUploadFiles: (files: File[]) => void;
  } = $props();

  let inputEl = $state<HTMLInputElement | undefined>(undefined);
  function openFilePicker() {
    inputEl?.click();
  }

  function handleInputChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    if (files.length > 0) {
      onUploadFiles(files);
    }
    input.value = '';
  }
</script>

<div class="ui-panel flex min-h-96 flex-col items-center justify-center p-8 text-center">
  <Image class="mb-4 text-muted" size={42} />
  <h2 class="text-lg font-semibold text-primary">No media yet</h2>
  <p class="mt-2 max-w-sm text-sm font-medium text-muted">
    Upload files here, or add them to <span class="font-mono">test-content</span>.
  </p>
  <div class="mt-5 flex flex-wrap justify-center gap-2">
    <button class="ui-button gap-2" type="button" onclick={openFilePicker}>
      <Upload size={16} />
      Upload
    </button>
    <button class="ui-button gap-2" type="button" onclick={onRetry}>
      <RefreshCw size={16} />
      Refresh
    </button>
  </div>
  <input bind:this={inputEl} class="sr-only" type="file" multiple onchange={handleInputChange} />
</div>
