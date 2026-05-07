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
  type: 'image' | 'video';
  url: string;
  mimeType: string;
  size: number;
  modifiedAt: string;
  comments: Comment[];
  commentCount: number;
};

export type FeedPage = {
  items: MediaItem[];
  nextCursor?: string;
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
