<script lang="ts">
  import { AlertCircle, CheckCircle2, LoaderCircle, Upload } from 'lucide-svelte';

  type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';

  let {
    uploadStatus,
    uploadMessage,
    uploadProgress,
    onUploadFiles
  }: {
    uploadStatus: UploadStatus;
    uploadMessage: string;
    uploadProgress: number | null;
    onUploadFiles: (files: File[]) => void;
  } = $props();

  let inputEl = $state<HTMLInputElement | undefined>(undefined);
  let dragActive = $state(false);

  const accept = '.avif,.gif,.jpeg,.jpg,.png,.webp,.m4v,.mov,.mp4,.ogg,.ogv,.webm,image/*,video/*';

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

  function handleDragEnter(event: DragEvent) {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    dragActive = true;
  }

  function handleDragOver(event: DragEvent) {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    dragActive = true;
  }

  function handleDragLeave(event: DragEvent) {
    if (event.currentTarget === event.target) {
      dragActive = false;
    }
  }

  function handleDrop(event: DragEvent) {
    if (!hasDraggedFiles(event)) return;
    event.preventDefault();
    dragActive = false;
    const files = Array.from(event.dataTransfer?.files ?? []);
    if (files.length > 0) {
      onUploadFiles(files);
    }
  }

  function hasDraggedFiles(event: DragEvent) {
    return Array.from(event.dataTransfer?.types ?? []).includes('Files');
  }
</script>

<header class="glass-nav sticky top-0 z-20">
  <div class="mx-auto flex min-h-16 w-full max-w-2xl items-center justify-between gap-3 px-4 py-2">
    <div>
      <h1 class="text-xl font-semibold tracking-normal text-primary">Feed AI</h1>
      <p class="text-xs font-semibold text-muted">Local media stream</p>
    </div>
    <div class="flex min-w-0 items-center gap-2">
      <button
        class="glass-button upload-drop-in gap-2"
        class:upload-drop-in-active={dragActive}
        class:upload-drop-in-error={uploadStatus === 'error'}
        type="button"
        aria-label="Upload photos or videos"
        title="Upload photos or videos"
        disabled={uploadStatus === 'uploading'}
        onclick={openFilePicker}
        ondragenter={handleDragEnter}
        ondragover={handleDragOver}
        ondragleave={handleDragLeave}
        ondrop={handleDrop}
      >
        {#if uploadStatus === 'uploading'}
          <LoaderCircle class="animate-spin" size={15} />
        {:else if uploadStatus === 'success'}
          <CheckCircle2 size={15} />
        {:else if uploadStatus === 'error'}
          <AlertCircle size={15} />
        {:else}
          <Upload size={15} />
        {/if}
        <span class="hidden max-w-40 truncate sm:inline">
          {#if uploadStatus === 'uploading' && uploadProgress !== null}
            {uploadProgress}%
          {:else}
            {uploadMessage}
          {/if}
        </span>
      </button>
      <input
        bind:this={inputEl}
        class="sr-only"
        type="file"
        multiple
        {accept}
        onchange={handleInputChange}
      />
    </div>
  </div>
</header>

<style>
  .upload-drop-in {
    max-width: min(12rem, 46vw);
  }

  .upload-drop-in-active {
    border-color: var(--color-glass-border-hover);
    background: var(--background-image-glass-strong);
    transform: translateY(-1px);
  }

  .upload-drop-in-error {
    color: var(--color-danger);
  }
</style>
