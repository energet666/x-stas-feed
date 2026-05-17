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
  boardId: string;
  mediaId?: string;
  boardName: string;
  strokeCount: number;
  authors: string[];
  lastAuthor: string;
  updatedAt: string;
};

export type ActivityItem = CommentActivityItem | BoardActivityItem;

export type ShipState = {
  id: string;
  name: string;
  x: number;
  y: number;
  angle: number;
  thrusting: boolean;
  active?: boolean;
  bullets?: ShipBullet[];
  asteroid?: ShipAsteroid;
  updatedAt?: string;
};

export type ShipBullet = {
  x: number;
  y: number;
};

export type ShipAsteroid = {
  id: number;
  x: number;
  y: number;
  radius: number;
  angle: number;
  path: string;
};

export type ShipEvent = {
  type: 'asteroid-destroyed';
  ownerId?: string;
  asteroidId?: number;
  x?: number;
  y?: number;
};

export type ShipScore = {
  name: string;
  score: number;
  createdAt: string;
};

export type ShipSnapshot = {
  ships: ShipState[];
  events?: ShipEvent[];
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
  boardId?: string;
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
    throw new Error(message ?? `Feed item request failed with ${response.status}`);
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
    throw new Error(message ?? `Activity request failed with ${response.status}`);
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
    const error = new Error(message ?? `Media item request failed with ${response.status}`);
    (error as Error & { status?: number }).status = response.status;
    throw error;
  }

  return normalizeMediaItem((await response.json()) as MediaItem);
}

export async function fetchComments(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Comments request failed with ${response.status}`);
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
    throw new Error(message ?? `Comment request failed with ${response.status}`);
  }

  return (await response.json()) as Comment;
}

export async function createLike(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/likes`, {
    method: 'POST'
  });

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Like request failed with ${response.status}`);
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
    throw new Error(message ?? `Comment like request failed with ${response.status}`);
  }

  return (await response.json()) as { likeCount: number };
}

export async function fetchShipScores() {
  const response = await fetch('/api/ships/scores');
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Score request failed with ${response.status}`);
  }

  const data = (await response.json()) as { scores: ShipScore[] };
  return Array.isArray(data.scores) ? data.scores : [];
}

export async function createShipScore(name: string, score: number) {
  const response = await fetch('/api/ships/scores', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, score })
  });

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Score submission failed with ${response.status}`);
  }

  const data = (await response.json()) as { scores: ShipScore[] };
  return Array.isArray(data.scores) ? data.scores : [];
}

export function uploadMedia(files: File[], onProgress?: (progress: UploadProgress) => void) {
  return new Promise<UploadResult>((resolve, reject) => {
    const form = new FormData();
    for (const file of files) {
      form.append('modifiedAt', String(file.lastModified));
      form.append('files', file);
    }

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
          : uploadErrorsMessage(body as UploadResult | null) ?? `Upload failed with ${request.status}`;
      reject(new Error(message));
    };

    request.onerror = () => reject(new Error('Upload failed'));
    request.onabort = () => reject(new Error('Upload was cancelled'));
    request.send(form);
  });
}


export function commentEventsURL() {
  return '/api/comments/events';
}

export function shipSocketURL() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${protocol}//${window.location.host}/api/ships/socket`;
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
  return `${firstError.filename || 'File'}: ${firstError.error}`;
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
  name: string;
  strokeCount: number;
  createdAt: string;
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
  boardId: string;
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
    throw new Error(message ?? `Board creation failed with ${response.status}`);
  }

  return (await response.json()) as BoardInfo;
}

export async function fetchBoard(boardId: string) {
  const response = await fetch(`/api/boards/${encodeURIComponent(boardId)}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Board request failed with ${response.status}`);
  }

  const data = (await response.json()) as BoardData;
  data.strokes = Array.isArray(data.strokes) ? data.strokes : [];
  return data;
}

export async function fetchBoards() {
  const response = await fetch('/api/boards');
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Boards request failed with ${response.status}`);
  }

  const data = (await response.json()) as { boards: BoardInfo[] };
  return Array.isArray(data.boards) ? data.boards : [];
}

export async function createStroke(
  boardId: string,
  tool: string,
  points: number[][],
  color: string,
  size: number,
  author: string
) {
  const response = await fetch(
    `/api/boards/${encodeURIComponent(boardId)}/strokes`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tool, points, color, size, author })
    }
  );

  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Stroke creation failed with ${response.status}`);
  }
}
