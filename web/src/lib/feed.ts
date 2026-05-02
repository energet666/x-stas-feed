export type MediaItem = {
  id: string;
  filename: string;
  type: 'image' | 'video';
  url: string;
  mimeType: string;
  size: number;
  modifiedAt: string;
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
