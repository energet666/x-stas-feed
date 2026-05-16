<script lang="ts">
  import { tick } from 'svelte';
  import { AlertCircle, Check, CheckCircle2, LoaderCircle, Pencil, Star, Upload, X } from 'lucide-svelte';

  type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';
  type FeedMode = 'all' | 'favorites';

  let {
    uploadStatus,
    uploadMessage,
    uploadProgress,
    feedMode,
    onToggleFavoriteMode,
    onUploadFiles,
    onCreateBoard
  }: {
    uploadStatus: UploadStatus;
    uploadMessage: string;
    uploadProgress: number | null;
    feedMode: FeedMode;
    onToggleFavoriteMode: () => void;
    onUploadFiles: (files: File[]) => void;
    onCreateBoard: (name: string) => Promise<void>;
  } = $props();

  let inputEl = $state<HTMLInputElement | undefined>(undefined);
  let boardNameInputEl = $state<HTMLInputElement | undefined>(undefined);
  let dragActive = $state(false);
  let boardFormOpen = $state(false);
  let boardName = $state('');
  let boardCreating = $state(false);
  let boardError = $state('');

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

  async function openBoardForm() {
    boardError = '';
    boardFormOpen = true;
    await tick();
    boardNameInputEl?.focus();
    boardNameInputEl?.select();
  }

  function closeBoardForm() {
    if (boardCreating) return;
    boardFormOpen = false;
    boardName = '';
    boardError = '';
  }

  async function submitBoardForm() {
    if (boardCreating) return;
    boardCreating = true;
    boardError = '';
    try {
      await onCreateBoard(boardName.trim());
      boardFormOpen = false;
      boardName = '';
    } catch (err) {
      boardError = err instanceof Error ? err.message : 'Board creation failed';
      await tick();
      boardNameInputEl?.focus();
    } finally {
      boardCreating = false;
    }
  }

  function handleBoardNameKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeBoardForm();
    }
  }
</script>

<header class="feed-toolbar glass-panel side-glass-panel" aria-label="Feed controls">
  <div class="feed-toolbar-brand">
    <div class="min-w-0">
      <h1 class="truncate text-base font-bold tracking-normal text-primary">Feed AI</h1>
      <p class="text-xs font-semibold text-muted">Local media stream</p>
    </div>
  </div>
  <div class="feed-toolbar-actions">
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
      </button>
      <button
        class="glass-button upload-drop-in gap-2"
        class:upload-drop-in-active={dragActive}
        class:upload-drop-in-error={uploadStatus === 'error'}
        type="button"
        aria-label="Upload files"
        title="Upload files"
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
        onchange={handleInputChange}
      />
      <button
        class="glass-button board-create-button gap-2"
        type="button"
        aria-label="Create drawing board"
        title="Create drawing board"
        onclick={openBoardForm}
      >
        <Pencil size={15} />
        <span class="hidden sm:inline">Board</span>
      </button>
  </div>
  {#if boardFormOpen}
    <form class="board-name-form" onsubmit={submitBoardForm}>
      <label class="sr-only" for="board-name-input">Board name</label>
      <input
        bind:this={boardNameInputEl}
        bind:value={boardName}
        id="board-name-input"
        class="board-name-input"
        maxlength="80"
        placeholder="Board name"
        disabled={boardCreating}
        onkeydown={handleBoardNameKeydown}
      />
      <button
        class="glass-button board-name-action"
        type="submit"
        aria-label="Create board"
        title="Create board"
        disabled={boardCreating}
      >
        {#if boardCreating}
          <LoaderCircle class="animate-spin" size={15} />
        {:else}
          <Check size={15} />
        {/if}
      </button>
      <button
        class="glass-button board-name-action"
        type="button"
        aria-label="Cancel board creation"
        title="Cancel"
        disabled={boardCreating}
        onclick={closeBoardForm}
      >
        <X size={15} />
      </button>
      {#if boardError}
        <p class="board-name-error">{boardError}</p>
      {/if}
    </form>
  {/if}
</header>

<style>
  .feed-toolbar {
    width: 100%;
    overflow: hidden;
    padding: 0.85rem;
  }

  .feed-toolbar-brand {
    display: flex;
    min-width: 0;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .feed-toolbar-actions {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .feed-toolbar-actions :global(.glass-button) {
    min-width: 0;
  }

  .upload-drop-in {
    max-width: 100%;
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
    max-width: 100%;
  }

  .favorites-mode-button-active {
    border-color: rgb(250 204 21 / 0.45);
    color: rgb(253 224 71);
  }

  .board-create-button {
    max-width: 100%;
  }

  .board-create-button:hover {
    border-color: rgb(168 85 247 / 0.45);
    color: rgb(192 132 252);
  }

  .board-name-form {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 2.35rem 2.35rem;
    gap: 0.45rem;
    margin-top: 0.65rem;
  }

  .board-name-input {
    min-width: 0;
    border-radius: 0.85rem;
    border: 1px solid var(--color-glass-border-soft);
    background: rgb(0 0 0 / 0.22);
    padding: 0.55rem 0.7rem;
    color: var(--color-text-primary);
    font-size: 0.82rem;
    font-weight: 700;
    outline: none;
  }

  .board-name-input::placeholder {
    color: rgb(255 255 255 / 0.28);
  }

  .board-name-input:focus {
    border-color: var(--color-glass-border-hover);
    background: rgb(0 0 0 / 0.3);
  }

  .board-name-action {
    min-width: 0;
    width: 2.35rem;
    padding-inline: 0;
  }

  .board-name-error {
    grid-column: 1 / -1;
    color: var(--color-danger);
    font-size: 0.72rem;
    font-weight: 700;
    line-height: 1.2;
  }

</style>
