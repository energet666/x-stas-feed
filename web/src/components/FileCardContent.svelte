<script lang="ts">
  import { Download, ExternalLink, File, FileArchive, FileCode2, FileText, Table2 } from 'lucide-svelte';
  import { formatFileSize, formatMediaDate } from '../lib/date';
  import type { MediaItem } from '../lib/feed';

  let {
    item
  }: {
    item: MediaItem;
  } = $props();

  const extension = $derived(fileExtension(item.displayName || item.filename));
  const fileLabel = $derived(extension ? extension.toUpperCase() : 'FILE');
  const mimeLabel = $derived(item.mimeType || 'application/octet-stream');
  const visualKind = $derived(iconKind(mimeLabel, extension));
  const visibleName = $derived(displayFileName(item.displayName));
  const canOpenInBrowser = $derived(browserCanOpenInline(mimeLabel));

  function fileExtension(name: string) {
    const dot = name.lastIndexOf('.');
    if (dot < 0 || dot === name.length - 1) return '';
    return name.slice(dot + 1);
  }

  function displayFileName(name: string) {
    const maxRunes = 76;
    const runes = Array.from(name);
    if (runes.length <= maxRunes) return name;

    const dot = name.lastIndexOf('.');
    if (dot <= 0 || dot === name.length - 1) {
      return `${runes.slice(0, maxRunes - 1).join('')}...`;
    }

    const extensionWithDot = name.slice(dot);
    const extensionRunes = Array.from(extensionWithDot);
    const prefixLength = Math.max(16, maxRunes - extensionRunes.length - 3);
    return `${runes.slice(0, prefixLength).join('')}...${extensionWithDot}`;
  }

  function iconKind(mimeType: string, ext: string) {
    if (mimeType.startsWith('text/') || ['md', 'txt', 'rtf'].includes(ext)) return 'text';
    if (['csv', 'tsv', 'xls', 'xlsx', 'numbers'].includes(ext)) return 'table';
    if (['zip', 'gz', 'rar', '7z', 'tar'].includes(ext)) return 'archive';
    if (['css', 'go', 'html', 'js', 'json', 'svelte', 'ts', 'tsx', 'xml', 'yaml', 'yml'].includes(ext)) return 'code';
    return 'file';
  }

  function browserCanOpenInline(mimeType: string) {
    const type = mimeType.toLowerCase().split(';', 1)[0].trim();
    if (type === 'text/html' || type === 'image/svg+xml') return false;
    if (type.startsWith('text/')) return true;
    if (type.startsWith('audio/')) return true;
    return [
      'application/json',
      'application/pdf',
      'application/xml',
      'text/xml'
    ].includes(type);
  }
</script>

<div class="file-card-surface">
  <div class="file-card-content">
    <div class="file-card-icon-shell" aria-hidden="true">
      {#if visualKind === 'text'}
        <FileText size={52} strokeWidth={1.7} />
      {:else if visualKind === 'table'}
        <Table2 size={52} strokeWidth={1.7} />
      {:else if visualKind === 'archive'}
        <FileArchive size={52} strokeWidth={1.7} />
      {:else if visualKind === 'code'}
        <FileCode2 size={52} strokeWidth={1.7} />
      {:else}
        <File size={52} strokeWidth={1.7} />
      {/if}
    </div>

    <div class="file-card-details">
      <p class="file-card-kind">{fileLabel}</p>
      <h3 title={item.displayName}>{visibleName}</h3>
      <dl>
        <div>
          <dt>Size</dt>
          <dd>{formatFileSize(item.size)}</dd>
        </div>
        <div>
          <dt>Type</dt>
          <dd>{mimeLabel}</dd>
        </div>
        <div>
          <dt>Modified</dt>
          <dd>{formatMediaDate(item.modifiedAt)}</dd>
        </div>
      </dl>
    </div>

    <div class="file-card-actions">
      {#if canOpenInBrowser}
        <a class="file-card-action" href={item.url} target="_blank" rel="noreferrer" onclick={(event) => event.stopPropagation()}>
          <ExternalLink size={17} />
          <span>Open</span>
        </a>
      {/if}
      <a class="file-card-action" href={item.url} download={item.displayName} onclick={(event) => event.stopPropagation()}>
        <Download size={17} />
        <span>Download</span>
      </a>
    </div>
  </div>
</div>

<style>
  .file-card-surface {
    display: grid;
    height: 100%;
    min-height: 34rem;
    place-items: center;
    padding: clamp(1.25rem, 4vw, 3rem);
    background:
      linear-gradient(90deg, rgb(255 255 255 / 0.04) 1px, transparent 1px),
      linear-gradient(0deg, rgb(255 255 255 / 0.035) 1px, transparent 1px),
      linear-gradient(135deg, rgb(10 14 20), rgb(29 35 44) 52%, rgb(16 22 28));
    background-size: 34px 34px, 34px 34px, auto;
  }

  .file-card-content {
    display: grid;
    width: min(100%, 26rem);
    gap: 1rem;
    color: var(--color-text-primary);
  }

  .file-card-icon-shell {
    display: grid;
    width: 5.5rem;
    height: 5.5rem;
    place-items: center;
    border: 1px solid var(--color-border-glass);
    border-radius: 1.25rem;
    background: rgb(255 255 255 / 0.08);
    box-shadow: var(--shadow-overlay);
    color: rgb(134 239 172);
  }

  .file-card-details {
    display: grid;
    gap: 0.8rem;
  }

  .file-card-kind {
    width: fit-content;
    max-width: 100%;
    border: 1px solid rgb(134 239 172 / 0.28);
    border-radius: 999px;
    padding: 0.25rem 0.55rem;
    color: rgb(187 247 208);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0;
  }

  h3 {
    display: -webkit-box;
    max-height: 6.3em;
    overflow: hidden;
    overflow-wrap: anywhere;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 4;
    line-clamp: 4;
    font-size: clamp(1.35rem, 4vw, 2rem);
    font-weight: 850;
    line-height: 1.05;
    letter-spacing: 0;
  }

  dl {
    display: grid;
    gap: 0.6rem;
    margin: 0;
  }

  dl div {
    display: grid;
    gap: 0.15rem;
  }

  dt {
    color: var(--color-text-muted);
    font-size: 0.72rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--color-text-primary);
    font-size: 0.9rem;
    font-weight: 650;
  }

  .file-card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
  }

  .file-card-action {
    display: inline-flex;
    width: fit-content;
    max-width: 100%;
    align-items: center;
    justify-content: center;
    gap: 0.45rem;
    border: 1px solid var(--color-border-glass);
    border-radius: 999px;
    padding: 0.65rem 0.85rem;
    background: rgb(255 255 255 / 0.1);
    color: var(--color-text-primary);
    font-size: 0.9rem;
    font-weight: 800;
    text-decoration: none;
    transition:
      border-color 160ms ease,
      background 160ms ease,
      transform 160ms ease;
  }

  .file-card-action:hover {
    border-color: var(--color-border-glass-hover);
    background: rgb(255 255 255 / 0.16);
    transform: translateY(-1px);
  }
</style>
