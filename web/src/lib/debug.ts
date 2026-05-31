export const debugToolsStorageKey = 'feed-ai:debug-tools';

export function readDebugToolsEnabled() {
  try {
    const value = window.localStorage.getItem(debugToolsStorageKey)?.trim().toLowerCase();
    return value === 'true' || value === '1' || value === 'yes' || value === 'on';
  } catch {
    return false;
  }
}
