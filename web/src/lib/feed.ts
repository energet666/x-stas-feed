export type Comment = {
  id: string;
  text: string;
  createdAt: string;
};

export type CommentEvent = {
  mediaId: string;
  comment: Comment;
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
    throw new Error(`Feed request failed with ${response.status}`);
  }

  return (await response.json()) as FeedPage;
}

export async function fetchComments(mediaId: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`);
  if (!response.ok) {
    throw new Error(`Comments request failed with ${response.status}`);
  }

  return (await response.json()) as { comments: Comment[] };
}

export async function createComment(mediaId: string, text: string) {
  const response = await fetch(`/api/media/${encodeURIComponent(mediaId)}/comments`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text })
  });

  if (!response.ok) {
    const message = await response
      .json()
      .then((body) => (typeof body.error === 'string' ? body.error : undefined))
      .catch(() => undefined);
    throw new Error(message ?? `Comment request failed with ${response.status}`);
  }

  return (await response.json()) as Comment;
}

export function commentEventsURL() {
  return '/api/comments/events';
}
