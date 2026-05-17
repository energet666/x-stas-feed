<script lang="ts">
  import { tick } from 'svelte';
  import { AlertCircle, Check, CheckCircle2, LoaderCircle, Pencil, RefreshCw, Star, Upload, X } from 'lucide-svelte';

  type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';
  type FeedMode = 'all' | 'favorites';

  let {
    uploadStatus,
    uploadMessage,
    uploadProgress,
    feedMode,
    newFeedItemCount,
    onToggleFavoriteMode,
    onRefreshFeed,
    onUploadFiles,
    onCreateBoard
  }: {
    uploadStatus: UploadStatus;
    uploadMessage: string;
    uploadProgress: number | null;
    feedMode: FeedMode;
    newFeedItemCount: number;
    onToggleFavoriteMode: () => void;
    onRefreshFeed: () => void;
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

  function openBoardForm() {
    boardError = '';
    boardFormOpen = true;
    focusBoardNameInput();
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
      boardCreating = false;
      focusBoardNameInput();
    } finally {
      boardCreating = false;
    }
  }

  function focusBoardNameInput() {
    void tick().then(() => {
      if (!boardFormOpen || !boardNameInputEl) return;
      boardNameInputEl.focus({ preventScroll: true });
      boardNameInputEl.select();
    });
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
      <h1 class="truncate text-base font-bold tracking-normal text-primary">Feed+AI</h1>
    </div>
  </div>
  <div class="feed-toolbar-actions">
      {#if newFeedItemCount > 0 && feedMode === 'all'}
        <button
          class="glass-button feed-refresh-button gap-2"
          type="button"
          aria-label="Show new feed items"
          title="Show new feed items"
          onclick={onRefreshFeed}
        >
          <RefreshCw size={15} />
          <span class="min-w-0 flex-1 truncate text-left">
            {newFeedItemCount === 1 ? '1 new item' : `${newFeedItemCount} new items`}
          </span>
        </button>
      {/if}
      <button
        class="glass-button favorites-mode-button gap-2"
        class:favorites-mode-button-active={feedMode === 'favorites'}
        type="button"
        aria-label={feedMode === 'favorites' ? 'Show all media' : 'Show favorites'}
        title={feedMode === 'favorites' ? 'Show all media' : 'Show favorites'}
        onclick={onToggleFavoriteMode}
      >
        <Star size={15} fill={feedMode === 'favorites' ? 'currentColor' : 'none'} />
        <span class="min-w-0 flex-1 truncate text-left">{feedMode === 'favorites' ? 'All' : 'Favorites'}</span>
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
        <span class="min-w-0 flex-1 truncate text-left">
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
        <span class="min-w-0 flex-1 truncate text-left">Board</span>
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
    grid-template-columns: minmax(0, 1fr);
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .feed-toolbar-actions :global(.glass-button) {
    width: 100%;
    min-width: 0;
    justify-content: flex-start;
  }

  .feed-toolbar-actions :global(.glass-button > svg) {
    flex-shrink: 0;
  }

  .upload-drop-in {
    max-width: 100%;
  }

  .feed-refresh-button {
    position: relative;
    isolation: isolate;
    overflow: hidden;
    border-color: transparent;
    background:
      linear-gradient(rgb(8 14 18 / 0.88), rgb(8 14 18 / 0.88)) padding-box,
      linear-gradient(115deg, #22d3ee, #a78bfa, #f472b6, #facc15, #34d399, #22d3ee) border-box;
    background-size: 100% 100%, 260% 260%;
    color: white;
    animation: feed-refresh-border 2.8s linear infinite;
  }

  .feed-refresh-button::before {
    content: '';
    position: absolute;
    inset: 1px;
    z-index: -1;
    border-radius: inherit;
    background: linear-gradient(115deg, rgb(34 211 238 / 0.18), rgb(244 114 182 / 0.2), rgb(250 204 21 / 0.14));
  }

  .feed-refresh-button:hover {
    transform: translateY(-1px);
  }

  @keyframes feed-refresh-border {
    from {
      background-position: 0 0, 0% 50%;
    }

    to {
      background-position: 0 0, 260% 50%;
    }
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
