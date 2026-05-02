export const AVAILABLE_SPEEDS = [1, 1.25, 1.5, 2];
export const FEED_VIDEO_PLAY_EVENT = 'feed-video-play';
export const FEED_VIDEO_VOLUME_EVENT = 'feed-video-volume';
export const LONG_PRESS_DELAY_MS = 200;
export const SEEK_FEEDBACK_ACCUMULATION_MS = 600;
export const TOUCHPAD_SEEK_SENSITIVITY = 0.05;

const videoVolumeKey = 'feed-ai:video-volume';
const videoMutedKey = 'feed-ai:video-muted';

export type SafariVideoElement = HTMLVideoElement & {
  webkitSupportsPresentationMode?: (mode: 'picture-in-picture') => boolean;
  webkitSetPresentationMode?: (mode: 'inline' | 'picture-in-picture') => void;
  webkitPresentationMode?: 'inline' | 'picture-in-picture' | 'fullscreen';
};

export function supportsPictureInPicture(video?: HTMLVideoElement) {
  const safariVideo = video as SafariVideoElement | undefined;
  return Boolean(
    document.pictureInPictureEnabled ||
      safariVideo?.webkitSupportsPresentationMode?.('picture-in-picture')
  );
}

export function canSetVolume(element: HTMLVideoElement) {
  const currentVolume = element.volume;
  const testVolume = currentVolume === 1 ? 0.95 : 1;

  try {
    element.volume = testVolume;
    const supported = Math.abs(element.volume - testVolume) < 0.001;
    element.volume = currentVolume;
    return supported;
  } catch {
    return false;
  }
}

export function clampTime(nextTime: number, maxTime: number) {
  return Math.max(0, Math.min(maxTime, nextTime));
}

export function clampVolume(value: number) {
  if (!Number.isFinite(value)) return 1;
  return Math.max(0, Math.min(1, value));
}

export function formatVideoTime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
  const minutes = Math.floor(seconds / 60);
  const rest = Math.floor(seconds % 60);
  return `${minutes}:${String(rest).padStart(2, '0')}`;
}

export function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return tagName === 'input' || tagName === 'textarea' || tagName === 'select' || target.isContentEditable;
}

export function readStoredVolume() {
  const storedVolumeValue = storageGet(videoVolumeKey);
  const storedVolume = Number(storedVolumeValue);
  const storedMuted = storageGet(videoMutedKey);

  return {
    volume: storedVolumeValue !== null && Number.isFinite(storedVolume) ? clampVolume(storedVolume) : 0.5,
    muted: storedMuted === 'true'
  };
}

export function saveStoredVolume(volume: number, muted: boolean) {
  storageSet(videoVolumeKey, String(clampVolume(volume)));
  storageSet(videoMutedKey, String(muted));
}

export function progressStorageKey(mediaId: string) {
  return `feed-ai:video-progress:${mediaId}`;
}

export function readStoredProgress(mediaId: string) {
  const value = Number(storageGet(progressStorageKey(mediaId)));
  return Number.isFinite(value) ? value : 0;
}

export function saveStoredProgress(mediaId: string, time: number, duration: number) {
  if (!Number.isFinite(time) || time <= 0.5 || time >= duration - 1) {
    clearStoredProgress(mediaId);
    return;
  }

  storageSet(progressStorageKey(mediaId), time.toFixed(2));
}

export function clearStoredProgress(mediaId: string) {
  storageRemove(progressStorageKey(mediaId));
}

function storageGet(key: string) {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function storageSet(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Ignore storage failures; playback should keep working.
  }
}

function storageRemove(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Ignore storage failures; playback should keep working.
  }
}
