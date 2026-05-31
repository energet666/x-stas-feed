import { uiText } from './ui_text';

export function formatMediaDate(value: string) {
  return new Intl.DateTimeFormat('ru-RU', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value));
}

export function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes < 0) return uiText.files.unknownSize;
  return new Intl.NumberFormat('ru-RU', {
    maximumFractionDigits: bytes < 1024 * 1024 ? 0 : 1,
    style: 'unit',
    unit: sizeUnit(bytes),
    unitDisplay: 'short'
  }).format(sizeValue(bytes));
}

function sizeUnit(bytes: number) {
  if (bytes >= 1024 * 1024 * 1024) return 'gigabyte';
  if (bytes >= 1024 * 1024) return 'megabyte';
  if (bytes >= 1024) return 'kilobyte';
  return 'byte';
}

function sizeValue(bytes: number) {
  if (bytes >= 1024 * 1024 * 1024) return bytes / (1024 * 1024 * 1024);
  if (bytes >= 1024 * 1024) return bytes / (1024 * 1024);
  if (bytes >= 1024) return bytes / 1024;
  return bytes;
}
