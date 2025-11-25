/**
 * WebSocket client for real-time updates
 */

export type WSMessageType =
  | 'job.update'
  | 'job.created'
  | 'job.completed'
  | 'job.failed'
  | 'test.update'
  | 'coverage.update'
  | 'error'
  | 'ping'
  | 'pong'
  | 'subscribe'
  | 'unsubscribe'
  | 'subscribed';

export interface WSMessage<T = unknown> {
  type: WSMessageType;
  payload?: T;
  timestamp: string;
  request_id?: string;
}

export interface WSJobPayload {
  job_id: string;
  repository_id?: string;
  type: string;
  status: string;
  progress?: number;
  message?: string;
  error?: string;
  started_at?: string;
  completed_at?: string;
  result?: unknown;
  organization_id: string;
}

export interface WSSubscription {
  channel: string;
}

export type WSEventHandler<T = unknown> = (message: WSMessage<T>) => void;

interface PendingSubscription {
  channel: string;
  resolve: () => void;
  reject: (error: Error) => void;
  requestId: string;
}

export interface WebSocketClientOptions {
  url?: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  pingInterval?: number;
  debug?: boolean;
}

export class WebSocketClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectInterval: number;
  private maxReconnectAttempts: number;
  private pingInterval: number;
  private debug: boolean;

  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private pingTimer: NodeJS.Timeout | null = null;

  private eventHandlers: Map<WSMessageType, Set<WSEventHandler>> = new Map();
  private subscriptions: Set<string> = new Set();
  private pendingSubscriptions: Map<string, PendingSubscription> = new Map();

  private isConnecting = false;
  private shouldReconnect = true;

  constructor(options: WebSocketClientOptions = {}) {
    const baseUrl = options.url || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    // Convert http(s) to ws(s)
    this.url = baseUrl.replace(/^http/, 'ws') + '/api/v1/ws';
    this.reconnectInterval = options.reconnectInterval || 3000;
    this.maxReconnectAttempts = options.maxReconnectAttempts || 10;
    this.pingInterval = options.pingInterval || 30000;
    this.debug = options.debug || false;
  }

  /**
   * Connect to the WebSocket server
   */
  connect(organizationId?: string): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      if (this.isConnecting) {
        // Wait for existing connection attempt
        const checkConnection = setInterval(() => {
          if (this.ws?.readyState === WebSocket.OPEN) {
            clearInterval(checkConnection);
            resolve();
          }
        }, 100);
        return;
      }

      this.isConnecting = true;
      this.shouldReconnect = true;

      let url = this.url;
      if (organizationId) {
        url += `?organization_id=${organizationId}`;
      }

      this.log('Connecting to WebSocket:', url);

      try {
        this.ws = new WebSocket(url);
      } catch (error) {
        this.isConnecting = false;
        reject(error);
        return;
      }

      this.ws.onopen = () => {
        this.log('WebSocket connected');
        this.isConnecting = false;
        this.reconnectAttempts = 0;
        this.startPingInterval();

        // Resubscribe to channels
        this.resubscribe();

        resolve();
      };

      this.ws.onclose = (event) => {
        this.log('WebSocket closed:', event.code, event.reason);
        this.isConnecting = false;
        this.stopPingInterval();

        if (this.shouldReconnect) {
          this.scheduleReconnect(organizationId);
        }
      };

      this.ws.onerror = (error) => {
        this.log('WebSocket error:', error);
        if (this.isConnecting) {
          this.isConnecting = false;
          reject(new Error('WebSocket connection failed'));
        }
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data);
      };
    });
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect(): void {
    this.shouldReconnect = false;
    this.stopPingInterval();

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = null;
    }

    this.subscriptions.clear();
    this.pendingSubscriptions.clear();
  }

  /**
   * Subscribe to a channel
   */
  subscribe(channel: string): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.subscriptions.has(channel)) {
        resolve();
        return;
      }

      const requestId = this.generateRequestId();

      this.pendingSubscriptions.set(requestId, {
        channel,
        resolve,
        reject,
        requestId,
      });

      const message: WSMessage<WSSubscription> = {
        type: 'subscribe',
        payload: { channel },
        timestamp: new Date().toISOString(),
        request_id: requestId,
      };

      this.send(message);

      // Timeout after 10 seconds
      setTimeout(() => {
        const pending = this.pendingSubscriptions.get(requestId);
        if (pending) {
          this.pendingSubscriptions.delete(requestId);
          reject(new Error('Subscription timeout'));
        }
      }, 10000);
    });
  }

  /**
   * Unsubscribe from a channel
   */
  unsubscribe(channel: string): void {
    if (!this.subscriptions.has(channel)) {
      return;
    }

    const message: WSMessage<WSSubscription> = {
      type: 'unsubscribe',
      payload: { channel },
      timestamp: new Date().toISOString(),
    };

    this.send(message);
    this.subscriptions.delete(channel);
  }

  /**
   * Add an event handler for a message type
   */
  on<T = unknown>(type: WSMessageType, handler: WSEventHandler<T>): () => void {
    if (!this.eventHandlers.has(type)) {
      this.eventHandlers.set(type, new Set());
    }

    this.eventHandlers.get(type)!.add(handler as WSEventHandler);

    // Return unsubscribe function
    return () => {
      this.eventHandlers.get(type)?.delete(handler as WSEventHandler);
    };
  }

  /**
   * Remove an event handler
   */
  off<T = unknown>(type: WSMessageType, handler: WSEventHandler<T>): void {
    this.eventHandlers.get(type)?.delete(handler as WSEventHandler);
  }

  /**
   * Check if connected
   */
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  /**
   * Get connection state
   */
  getState(): 'connecting' | 'connected' | 'disconnected' {
    if (this.isConnecting) return 'connecting';
    if (this.ws?.readyState === WebSocket.OPEN) return 'connected';
    return 'disconnected';
  }

  private send(message: WSMessage): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      this.log('Cannot send message, WebSocket not connected');
      return;
    }

    this.ws.send(JSON.stringify(message));
  }

  private handleMessage(data: string): void {
    try {
      const message: WSMessage = JSON.parse(data);
      this.log('Received message:', message.type);

      // Handle subscription confirmations
      if (message.type === 'subscribed' && message.request_id) {
        const pending = this.pendingSubscriptions.get(message.request_id);
        if (pending) {
          this.subscriptions.add(pending.channel);
          this.pendingSubscriptions.delete(message.request_id);
          pending.resolve();
        }
      }

      // Handle pong
      if (message.type === 'pong') {
        this.log('Received pong');
        return;
      }

      // Notify event handlers
      const handlers = this.eventHandlers.get(message.type);
      if (handlers) {
        handlers.forEach((handler) => {
          try {
            handler(message);
          } catch (error) {
            console.error('Error in WebSocket event handler:', error);
          }
        });
      }

      // Also notify wildcard handlers (if we add them later)
    } catch (error) {
      this.log('Failed to parse WebSocket message:', error);
    }
  }

  private scheduleReconnect(organizationId?: string): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.log('Max reconnect attempts reached');
      return;
    }

    const delay = Math.min(
      this.reconnectInterval * Math.pow(2, this.reconnectAttempts),
      30000 // Max 30 seconds
    );

    this.log(`Scheduling reconnect in ${delay}ms (attempt ${this.reconnectAttempts + 1})`);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectAttempts++;
      this.connect(organizationId).catch((error) => {
        this.log('Reconnect failed:', error);
      });
    }, delay);
  }

  private resubscribe(): void {
    // Resubscribe to all channels after reconnection
    const channels = Array.from(this.subscriptions);
    this.subscriptions.clear();

    channels.forEach((channel) => {
      this.subscribe(channel).catch((error) => {
        this.log('Failed to resubscribe to channel:', channel, error);
      });
    });
  }

  private startPingInterval(): void {
    this.stopPingInterval();

    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        const message: WSMessage = {
          type: 'ping',
          timestamp: new Date().toISOString(),
          request_id: this.generateRequestId(),
        };
        this.send(message);
      }
    }, this.pingInterval);
  }

  private stopPingInterval(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  private generateRequestId(): string {
    return `req_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }

  private log(...args: unknown[]): void {
    if (this.debug) {
      console.log('[WebSocket]', ...args);
    }
  }
}

// Singleton instance
let wsClient: WebSocketClient | null = null;

/**
 * Get or create the WebSocket client singleton
 */
export function getWebSocketClient(options?: WebSocketClientOptions): WebSocketClient {
  if (!wsClient) {
    wsClient = new WebSocketClient(options);
  }
  return wsClient;
}

/**
 * Reset the WebSocket client (useful for testing)
 */
export function resetWebSocketClient(): void {
  if (wsClient) {
    wsClient.disconnect();
    wsClient = null;
  }
}
