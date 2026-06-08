import { uiText } from './ui_text';

export const maxUploadBytes = 5 * 1024 ** 3;

export type Comment = {
  id: string;
  author: string;
  text: string;
  createdAt: string;
  likeCount: number;
};

export type CommentEvent = {
  mediaId: string;
  comment: Comment;
};

export type LikeEvent = {
  mediaId: string;
  likeCount: number;
};

export type CommentLikeEvent = {
  mediaId: string;
  commentId: string;
  likeCount: number;
};

export type CommentActivityItem = {
  type: 'comment';
  mediaId: string;
  mediaDisplayName: string;
  mediaType: MediaKind;
  comment: Comment;
};

export type BoardActivityItem = {
  type: 'board';
  mediaId: string;
  boardName: string;
  strokeCount: number;
  authors: string[];
  lastAuthor: string;
  updatedAt: string;
};

export type ActivityItem = CommentActivityItem | BoardActivityItem;

export type ShipInput = {
  left: boolean;
  right: boolean;
  thrust: boolean;
};

export type ShipCommand = {
  type: 'input' | 'shoot' | 'restart' | 'finish' | 'leave' | 'name' | 'heartbeat';
  seq: number;
  input?: ShipInput;
  name?: string;
  sentAtMs?: number;
};

export type ShipState = {
  id: string;
  name: string;
  state: 'spectator' | 'active' | 'inactive' | 'away';
  x: number;
  y: number;
  vx: number;
  vy: number;
  angle: number;
  thrusting: boolean;
  active: boolean;
  score: number;
  kills: number;
  ackSeq: number;
  pingEcho?: number;
};

export type ShipBullet = {
  id: number;
  ownerId: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
};

export type ShipAsteroid = {
  id: number;
  ownerId: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
  radius: number;
  angle: number;
  spin: number;
  path: string;
};

export type ShipEvent = {
  id: number;
  type: 'asteroid-destroyed' | 'ship-kill' | 'ship-crash' | 'round-finished';
  ownerId?: string;
  shooterId?: string;
  shooterName?: string;
  victimId?: string;
  victimName?: string;
  x?: number;
  y?: number;
  saved?: boolean;
};

export type ShipScore = {
  name: string;
  score: number;
  createdAt: string;
};

export type ShipSnapshot = {
  type: 'snapshot';
  tick: number;
  mode: 'idle' | 'solo' | 'multiplayer';
  status: 'idle' | 'playing' | 'finished';
  remainingMs: number;
  players: ShipState[];
  bullets?: ShipBullet[];
  asteroids?: ShipAsteroid[];
  events?: ShipEvent[];
};

export type ShipWelcome = {
  type: 'welcome';
  playerId: string;
  resumeToken: string;
  arena: { width: number; height: number };
  snapshot: ShipSnapshot;
};

export type MediaKind = 'image' | 'video' | 'audio' | 'file' | 'board';

export type AudioTags = {
  title?: string;
  artist?: string;
  album?: string;
  albumArtist?: string;
  date?: string;
  genre?: string;
  track?: string;
};

export type MediaItem = {
  id: string;
  filename: string;
  displayName: string;
  type: MediaKind;
  url: string;
  mimeType: string;
  size: number;
  modifiedAt: string;
  comments: Comment[];
  commentCount: number;
  likeCount: number;
  durationSeconds?: number;
  audioTags?: AudioTags;
  coverUrl?: string;
};

export type IndexedFeedItem = {
  index: number;
  firstIndex: number;
  lastIndex: number;
  item: MediaItem;
};

export type FeedItemCreatedEvent = IndexedFeedItem;

export type UploadResult = {
  items: MediaItem[];
  errors?: { filename: string; error: string }[];
};

export type UploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

export async function fetchFeedItem(index: number) {
  const params = new URLSearchParams({ index: String(index) });

  const response = await fetch(`/api/feed?${params.toString()}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.feedItemRequest(response.status));
  }

  const result = (await response.json()) as IndexedFeedItem;
  return {
    ...result,
    item: normalizeMediaItem(result.item)
  };
}

export async function fetchActivity({ limit }: { limit: number }) {
  const params = new URLSearchParams({ limit: String(limit) });
  const response = await fetch(`/api/activity?${params.toString()}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.activityRequest(response.status));
  }

  const data = (await response.json()) as { items: Omit<CommentActivityItem, 'type'>[] };
  return {
    items: Array.isArray(data.items)
      ? data.items.map((item) => ({ ...item, type: 'comment' as const }))
      : []
  };
}

export async function fetchMediaItem(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    const error = new Error(message ?? uiText.errors.mediaItemRequest(response.status));
    (error as Error & { status?: number }).status = response.status;
    throw error;
  }

  return normalizeMediaItem((await response.json()) as MediaItem);
}

export async function fetchComments(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.commentsRequest(response.status));
  }

  const body = (await response.json()) as { comments: Comment[] | null };
  return { comments: Array.isArray(body.comments) ? body.comments : [] };
}

export async function createComment(mediaId: string, text: string, author: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text, author })
  });

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.commentRequest(response.status));
  }

  return (await response.json()) as Comment;
}

export async function createLike(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/likes`, {
    method: 'POST'
  });

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.likeRequest(response.status));
  }

  return (await response.json()) as { likeCount: number };
}

export async function createCommentLike(mediaId: string, commentId: string) {
  const response = await fetch(
    `/api/media/${encodeURIComponent(mediaId)}/comments/${encodeURIComponent(commentId)}/likes`,
    {
      method: 'POST'
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.commentLikeRequest(response.status));
  }

  return (await response.json()) as { likeCount: number };
}

export async function fetchShipScores() {
  const response = await fetch('/api/ships/scores');
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.scoreRequest(response.status));
  }

  const data = (await response.json()) as { scores: ShipScore[] };
  return Array.isArray(data.scores) ? data.scores : [];
}

export function uploadMedia(file: File, onProgress?: (progress: UploadProgress) => void) {
  return new Promise<UploadResult>((resolve, reject) => {
    const form = new FormData();
    form.append('modifiedAt', String(file.lastModified));
    form.append('files', file);

    const request = new XMLHttpRequest();
    request.open('POST', '/api/uploads');
    request.responseType = 'json';

    request.upload.onprogress = (event) => {
      if (!event.lengthComputable) return;
      onProgress?.({
        loaded: event.loaded,
        total: event.total,
        percent: Math.round((event.loaded / event.total) * 100)
      });
    };

    request.onload = () => {
      const body = request.response as UploadResult | { error?: string } | null;
      if (request.status >= 200 && request.status < 300) {
        resolve((body ?? { items: [] }) as UploadResult);
        return;
      }

      const message =
        body && 'error' in body && typeof body.error === 'string'
          ? body.error
          : uploadErrorsMessage(body as UploadResult | null) ?? uiText.upload.failedWithStatus(request.status);
      reject(new Error(message));
    };

    request.onerror = () => reject(new Error(uiText.upload.failed));
    request.onabort = () => reject(new Error(uiText.upload.cancelled));
    request.send(form);
  });
}


export function commentEventsURL() {
  return '/api/comments/events';
}

export function shipSocketURL(resumeToken: string, name: string) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const params = new URLSearchParams();
  if (resumeToken) params.set('resumeToken', resumeToken);
  if (name.trim()) params.set('name', name.trim());
  return `${protocol}//${window.location.host}/api/ships/socket?${params}`;
}

export function mediaPosterURL(mediaId: string, seconds: number) {
  const time = Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds * 2) / 2 : 0;
  return `/api/media/${encodeURIComponent(mediaId)}/poster?time=${time.toFixed(1)}`;
}

async function responseErrorMessage(response: Response) {
  return response
    .json()
    .then((body) => (typeof body.error === 'string' ? body.error : undefined))
    .catch(() => undefined);
}

function uploadErrorsMessage(body: UploadResult | null) {
  const firstError = body?.errors?.[0];
  if (!firstError) return undefined;
  return `${firstError.filename || uiText.common.file}: ${firstError.error}`;
}

function normalizeMediaItem(item: MediaItem) {
  return {
    ...item,
    comments: Array.isArray(item.comments) ? item.comments : []
  };
}

// --- Drawing Board API ---

export type BoardInfo = {
  id: string;
  mediaId?: string;
  filename?: string;
  name: string;
  background?: BoardBackground;
  canvas?: BoardCanvas;
  strokeCount: number;
  createdAt: string;
};

export type BoardBackground = {
  type: 'default' | 'image';
  filename?: string;
  mimeType?: string;
  url?: string;
};

export type BoardCanvas = {
  width: number;
  height: number;
};

export type Stroke = {
  id: string;
  tool: string;
  points: number[][];
  color: string;
  size: number;
  author: string;
  createdAt: string;
};

export type StrokeEvent = {
  mediaId: string;
  stroke: Stroke;
};

export type BoardData = {
  board: BoardInfo;
  strokes: Stroke[];
};

export async function createBoard(name: string) {
  const response = await fetch('/api/boards', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name })
  });

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.boardCreation(response.status));
  }

  return (await response.json()) as BoardInfo;
}

export async function fetchBoard(mediaId: string) {
  const response = await fetch(`/api/boards/${encodeURIComponent(mediaId)}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.boardRequest(response.status));
  }

  const data = (await response.json()) as BoardData;
  data.strokes = Array.isArray(data.strokes) ? data.strokes : [];
  return data;
}

export async function fetchBoards() {
  const response = await fetch('/api/boards');
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.boardsRequest(response.status));
  }

  const data = (await response.json()) as { boards: BoardInfo[] };
  return Array.isArray(data.boards) ? data.boards : [];
}

export async function createStroke(
  mediaId: string,
  tool: string,
  points: number[][],
  color: string,
  size: number,
  author: string
) {
  const response = await fetch(
    `/api/boards/${encodeURIComponent(mediaId)}/strokes`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tool, points, color, size, author })
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.strokeCreation(response.status));
  }
}
