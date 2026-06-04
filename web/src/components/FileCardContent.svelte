<script lang="ts">
  import { Download, ExternalLink, File, FileArchive, FileCode2, FileText, Table2 } from 'lucide-svelte';
  import { formatFileSize, formatMediaDate } from '../lib/date';
  import type { MediaItem } from '../lib/feed';
  import { uiText as t } from '../lib/ui_text';

  let {
    item
  }: {
    item: MediaItem;
  } = $props();

  const extension = $derived(fileExtension(item.displayName || item.filename));
  const fileLabel = $derived(extension ? extension.toUpperCase() : t.files.genericLabel);
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
          <dt>{t.files.size}</dt>
          <dd>{formatFileSize(item.size)}</dd>
        </div>
        <div>
          <dt>{t.files.type}</dt>
          <dd>{mimeLabel}</dd>
        </div>
        <div>
          <dt>{t.files.modified}</dt>
          <dd>{formatMediaDate(item.modifiedAt)}</dd>
        </div>
      </dl>
    </div>

    <div class="file-card-actions">
      {#if canOpenInBrowser}
        <a class="file-card-action" href={item.url} target="_blank" rel="noreferrer" onclick={(event) => event.stopPropagation()}>
          <ExternalLink size={17} />
          <span>{t.files.open}</span>
        </a>
      {/if}
      <a class="file-card-action" href={item.url} download={item.displayName} onclick={(event) => event.stopPropagation()}>
        <Download size={17} />
        <span>{t.files.download}</span>
      </a>
    </div>
  </div>
</div>

<style>
  .file-card-surface {
    container: file-card-surface / size;
    display: grid;
    height: 100%;
    min-height: 0;
    place-items: center;
    padding: clamp(0.9rem, 3.5vw, 3rem);
    background: transparent;
  }

  .file-card-content {
    display: grid;
    width: min(100%, 26rem);
    gap: 1rem;
    color: var(--color-fg-primary);
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
    color: var(--color-fg-muted);
    font-size: 0.72rem;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--color-fg-primary);
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
    color: var(--color-fg-primary);
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

  @container file-card-surface (height < 34rem) {
    .file-card-content {
      width: min(100%, 28rem);
      gap: 0.72rem;
    }

    .file-card-icon-shell {
      width: 4.35rem;
      height: 4.35rem;
      border-radius: 1rem;
    }

    .file-card-icon-shell :global(svg) {
      width: 2.3rem;
      height: 2.3rem;
    }

    .file-card-details {
      gap: 0.55rem;
    }

    h3 {
      max-height: 3.45em;
      -webkit-line-clamp: 3;
      line-clamp: 3;
      font-size: clamp(1.12rem, 5cqw, 1.55rem);
      line-height: 1.15;
    }

    dl {
      gap: 0.4rem;
    }

    dt {
      font-size: 0.65rem;
    }

    dd {
      font-size: 0.82rem;
      line-height: 1.18;
    }

    .file-card-actions {
      gap: 0.45rem;
    }

    .file-card-action {
      padding: 0.55rem 0.72rem;
      font-size: 0.82rem;
    }
  }

  @container file-card-surface (height < 29rem) {
    .file-card-content {
      gap: 0.58rem;
    }

    .file-card-icon-shell {
      width: 3.6rem;
      height: 3.6rem;
    }

    .file-card-icon-shell :global(svg) {
      width: 2rem;
      height: 2rem;
    }

    .file-card-kind {
      padding: 0.2rem 0.48rem;
      font-size: 0.66rem;
    }

    h3 {
      max-height: 2.3em;
      -webkit-line-clamp: 2;
      line-clamp: 2;
      font-size: clamp(1rem, 4.5cqw, 1.28rem);
    }

    dl {
      gap: 0.32rem;
    }

    dl div {
      gap: 0.08rem;
    }

    .file-card-action {
      min-height: 2.15rem;
      padding: 0.45rem 0.62rem;
    }
  }
</style>
