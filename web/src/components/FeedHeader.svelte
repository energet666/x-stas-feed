<script lang="ts">
  import { AlertCircle, CheckCircle2, LoaderCircle, Star, Upload } from 'lucide-svelte';

  type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';
  type FeedMode = 'all' | 'favorites';

  let {
    uploadStatus,
    uploadMessage,
    uploadProgress,
    feedMode,
    favoriteCount,
    onToggleFavoriteMode,
    onUploadFiles
  }: {
    uploadStatus: UploadStatus;
    uploadMessage: string;
    uploadProgress: number | null;
    feedMode: FeedMode;
    favoriteCount: number;
    onToggleFavoriteMode: () => void;
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
        class="glass-button favorites-mode-button gap-2"
        class:favorites-mode-button-active={feedMode === 'favorites'}
        type="button"
        aria-label={feedMode === 'favorites' ? 'Show all media' : 'Show favorites'}
        title={feedMode === 'favorites' ? 'Show all media' : 'Show favorites'}
        onclick={onToggleFavoriteMode}
      >
        <Star size={15} fill={feedMode === 'favorites' ? 'currentColor' : 'none'} />
        <span class="hidden sm:inline">{feedMode === 'favorites' ? 'All' : 'Favorites'}</span>
        {#if favoriteCount > 0}
          <span class="favorites-count">{favoriteCount}</span>
        {/if}
      </button>
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

  .favorites-mode-button {
    max-width: min(10rem, 42vw);
  }

  .favorites-mode-button-active {
    border-color: rgb(250 204 21 / 0.45);
    color: rgb(253 224 71);
  }

  .favorites-count {
    min-width: 1.15rem;
    padding: 0.15rem 0.35rem;
    border-radius: var(--radius-control);
    background: rgb(255 255 255 / 0.12);
    color: currentColor;
    font-size: 0.68rem;
    line-height: 1;
  }
</style>
