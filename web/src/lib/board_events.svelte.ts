import type { Stroke } from './feed';

export interface StrokeEvent {
  mediaId: string;
  stroke: Stroke;
}

class BoardEventManager {
  private es: EventSource | null = null;
  private listeners: Set<(event: StrokeEvent) => void> = new Set();
  
  status = $state<'connecting' | 'connected' | 'error'>('connecting');

  subscribe(callback: (event: StrokeEvent) => void) {
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
        const data = JSON.parse(e.data) as StrokeEvent;
        this.listeners.forEach(l => l(data));
      } catch (err) {
        console.error('Failed to parse board stroke event', err);
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
