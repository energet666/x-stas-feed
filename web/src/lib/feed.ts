export type Comment = {
  id: string;
  author: string;
  text: string;
  createdAt: string;
};

export type CommentEvent = {
  mediaId: string;
  comment: Comment;
};

export type LikeEvent = {
  mediaId: string;
  likeCount: number;
};

export type ShipState = {
  id: string;
  name: string;
  x: number;
  y: number;
  angle: number;
  thrusting: boolean;
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

export type ShipSnapshot = {
  ships: ShipState[];
  events?: ShipEvent[];
};

export type MediaItem = {
  id: string;
  filename: string;
  displayName: string;
  type: 'image' | 'video';
  url: string;
  mimeType: string;
  size: number;
  modifiedAt: string;
  comments: Comment[];
  commentCount: number;
  likeCount: number;
};

export type FeedPage = {
  items: MediaItem[];
  nextCursor?: string;
};

export type UploadResult = {
  items: MediaItem[];
  errors?: { filename: string; error: string }[];
};

export type UploadProgress = {
  loaded: number;
  total: number;
  percent: number;
};

export async function fetchFeedPage({ cursor, limit }: { cursor?: string; limit: number }) {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set('cursor', cursor);

  const response = await fetch(`/api/feed?${params.toString()}`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Feed request failed with ${response.status}`);
  }

  return (await response.json()) as FeedPage;
}

export async function fetchComments(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`);
  if (!response.ok) {
    const message = await responseErrorMessage(response);
    throw new Error(message ?? `Comments request failed with ${response.status}`);
  }

  return (await response.json()) as { comments: Comment[] };
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

export function uploadMedia(files: File[], onProgress?: (progress: UploadProgress) => void) {
  return new Promise<UploadResult>((resolve, reject) => {
    const form = new FormData();
    for (const file of files) {
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
