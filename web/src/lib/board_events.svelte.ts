import type { BoardEvent, BoardImageEvent, StrokeEvent } from './feed';

class BoardEventManager {
  private es: EventSource | null = null;
  private listeners: Set<(event: BoardEvent) => void> = new Set();
  
  status = $state<'connecting' | 'connected' | 'error'>('connecting');

  subscribe(callback: (event: BoardEvent) => void) {
    this.listeners.add(callback);
    if (!this.es) {
      this.connect();
    }
    return () => {
      this.listeners.delete(callback);
      if (this.listeners.size === 0) {
        this.disconnect();
      }
    };
  }

  private connect() {
    if (this.es) return;
    
    this.es = new EventSource('/api/boards/events');
    this.status = 'connecting';

    this.es.addEventListener('stroke', (e) => {
      try {
        const data = JSON.parse(e.data) as Omit<StrokeEvent, 'type'>;
        this.listeners.forEach(l => l({ ...data, type: 'stroke' }));
      } catch (err) {
        console.error('Failed to parse board stroke event', err);
      }
    });

    this.es.addEventListener('image', (e) => {
      try {
        const data = JSON.parse(e.data) as Omit<BoardImageEvent, 'type'>;
        this.listeners.forEach(l => l({ ...data, type: 'image' }));
      } catch (err) {
        console.error('Failed to parse board image event', err);
      }
    });

    this.es.onopen = () => {
      this.status = 'connected';
    };

    this.es.onerror = () => {
      this.status = 'error';
    };
  }

  private disconnect() {
    this.es?.close();
    this.es = null;
    this.status = 'connecting';
  }
}

export const boardEvents = new BoardEventManager();
