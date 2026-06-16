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
  mediaDisplayName: string;
  boardName: string;
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
  controlScore: number;
  ackSeq: number;
  pingEcho?: number;
  shield: number;
  rapidFire: boolean;
  tripleShot: boolean;
  overdrive: boolean;
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

export type ShipPowerUp = {
  id: number;
  kind: 'shield' | 'triple-shot' | 'rapid-fire' | 'overdrive' | 'nova';
  x: number;
  y: number;
  expiresAt: number;
};

export type ShipControlZone = {
  x: number;
  y: number;
  radius: number;
  state: 'empty' | 'controlled' | 'contested';
  ownerId?: string;
  targetScore: number;
};

export type ShipEvent = {
  id: number;
  type:
    | 'asteroid-destroyed'
    | 'ship-kill'
    | 'ship-crash'
    | 'round-finished'
    | 'power-up-collected'
    | 'shield-hit';
  ownerId?: string;
  shooterId?: string;
  shooterName?: string;
  victimId?: string;
  victimName?: string;
  winnerId?: string;
  winnerName?: string;
  x?: number;
  y?: number;
  saved?: boolean;
  powerUpKind?: ShipPowerUp['kind'];
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
  winnerId?: string;
  winnerName?: string;
  players: ShipState[];
  bullets?: ShipBullet[];
  asteroids?: ShipAsteroid[];
  powerUps?: ShipPowerUp[];
  controlZone?: ShipControlZone;
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

  const data = (await response.json()) as {
    items: Array<CommentActivityItem | Omit<BoardActivityItem, 'boardName'>>;
  };
  const items: ActivityItem[] = [];
  if (Array.isArray(data.items)) {
    for (const item of data.items) {
      if (item.type === 'board') {
        items.push({
          type: 'board',
          mediaId: item.mediaId,
          mediaDisplayName: item.mediaDisplayName,
          boardName: item.mediaDisplayName,
          updatedAt: item.updatedAt
        });
      } else if ('comment' in item) {
        items.push({ ...item, type: 'comment' });
      }
    }
  }
  return { items };
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
  opacity: number;
  author: string;
  createdAt: string;
};

export type StrokeInput = Pick<Stroke, 'tool' | 'points' | 'color' | 'size' | 'opacity' | 'author'>;

export type BoardImageInput = Pick<BoardImage, 'assetId' | 'x' | 'y' | 'width' | 'height' | 'rotation' | 'flipX' | 'author'>;

export type BoardOperationInput =
  | { type: 'stroke'; stroke: StrokeInput }
  | { type: 'image'; image: BoardImageInput };

export type BoardImage = {
  id: string;
  assetId: string;
  url: string;
  mimeType: string;
  x: number;
  y: number;
  width: number;
  height: number;
  rotation: number;
  flipX: boolean;
  author: string;
  createdAt: string;
};

export type BoardAsset = {
  id: string;
  url: string;
  mimeType: string;
  usageCount: number;
  createdAt: string;
};

export type BoardOperation =
  | { type: 'stroke'; stroke: Stroke }
  | { type: 'image'; image: BoardImage };

export type StrokeEvent = {
  type: 'stroke';
  mediaId: string;
  stroke: Stroke;
};

export type BoardImageEvent = {
  type: 'image';
  mediaId: string;
  image: BoardImage;
};

export type BoardEvent = StrokeEvent | BoardImageEvent;

export type BoardData = {
  board: BoardInfo;
  strokes: Stroke[];
  operations: BoardOperation[];
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
  data.operations = Array.isArray(data.operations)
    ? data.operations
    : data.strokes.map((stroke) => ({ type: 'stroke' as const, stroke }));
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
  opacity: number,
  author: string
) {
  const response = await fetch(
    `/api/boards/${encodeURIComponent(mediaId)}/strokes`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tool, points, color, size, opacity, author })
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.strokeCreation(response.status));
  }
}

export async function createStrokes(mediaId: string, strokes: StrokeInput[]) {
  const response = await fetch(
    `/api/boards/${encodeURIComponent(mediaId)}/strokes/batch`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ strokes })
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.strokesCreation(response.status));
  }

  const data = (await response.json()) as { strokes?: Stroke[] };
  return Array.isArray(data.strokes) ? data.strokes : [];
}

export async function createBoardOperations(mediaId: string, operations: BoardOperationInput[]) {
  const response = await fetch(
    `/api/boards/${encodeURIComponent(mediaId)}/operations/batch`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operations })
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? uiText.errors.boardOperationsCreation(response.status));
  }

  const data = (await response.json()) as { operations?: BoardOperation[] };
  return Array.isArray(data.operations) ? data.operations : [];
}

export async function fetchBoardAssets() {
  const response = await fetch('/api/board-assets');
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Не удалось загрузить ассеты (${response.status})`);
  }
  const data = (await response.json()) as { assets?: BoardAsset[] };
  return Array.isArray(data.assets) ? data.assets : [];
}

export async function createBoardAsset(file: File) {
  const form = new FormData();
  form.append('file', file);
  const response = await fetch('/api/board-assets', {
    method: 'POST',
    body: form
  });
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Не удалось сохранить ассет (${response.status})`);
  }
  return (await response.json()) as BoardAsset;
}

export async function createBoardImageFromAsset(
  mediaId: string,
  assetId: string,
  placement: Pick<BoardImage, 'x' | 'y' | 'width' | 'height' | 'rotation' | 'flipX'>,
  author: string
) {
  const response = await fetch(`/api/boards/${encodeURIComponent(mediaId)}/images`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ assetId, ...placement, author })
  });
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Не удалось переиспользовать ассет (${response.status})`);
  }
}
