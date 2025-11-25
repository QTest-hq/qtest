'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import {
  getWebSocketClient,
  WebSocketClient,
  WSMessage,
  WSMessageType,
  WSJobPayload,
} from '@/lib/websocket';

export interface UseRealtimeUpdatesOptions {
  organizationId?: string;
  autoConnect?: boolean;
  debug?: boolean;
}

export interface RealtimeState {
  isConnected: boolean;
  isConnecting: boolean;
  error: Error | null;
}

/**
 * Hook for managing WebSocket connection and real-time updates
 */
export function useRealtimeUpdates(options: UseRealtimeUpdatesOptions = {}) {
  const { organizationId, autoConnect = true, debug = false } = options;

  const [state, setState] = useState<RealtimeState>({
    isConnected: false,
    isConnecting: false,
    error: null,
  });

  const wsRef = useRef<WebSocketClient | null>(null);
  const mountedRef = useRef(true);

  // Get or create WebSocket client
  useEffect(() => {
    wsRef.current = getWebSocketClient({ debug });
    return () => {
      mountedRef.current = false;
    };
  }, [debug]);

  // Connect to WebSocket
  const connect = useCallback(async () => {
    if (!wsRef.current) return;

    if (mountedRef.current) {
      setState((prev) => ({ ...prev, isConnecting: true, error: null }));
    }

    try {
      await wsRef.current.connect(organizationId);
      if (mountedRef.current) {
        setState({ isConnected: true, isConnecting: false, error: null });
      }
    } catch (error) {
      if (mountedRef.current) {
        setState({
          isConnected: false,
          isConnecting: false,
          error: error instanceof Error ? error : new Error('Connection failed'),
        });
      }
    }
  }, [organizationId]);

  // Disconnect from WebSocket
  const disconnect = useCallback(() => {
    if (!wsRef.current) return;
    wsRef.current.disconnect();
    if (mountedRef.current) {
      setState({ isConnected: false, isConnecting: false, error: null });
    }
  }, []);

  // Auto-connect on mount if enabled
  useEffect(() => {
    if (autoConnect && organizationId) {
      connect();
    }

    return () => {
      // Don't disconnect on unmount - let other components use the connection
      // disconnect();
    };
  }, [autoConnect, organizationId, connect]);

  // Update connection state periodically
  useEffect(() => {
    const interval = setInterval(() => {
      if (wsRef.current && mountedRef.current) {
        const wsState = wsRef.current.getState();
        setState((prev) => ({
          ...prev,
          isConnected: wsState === 'connected',
          isConnecting: wsState === 'connecting',
        }));
      }
    }, 1000);

    return () => clearInterval(interval);
  }, []);

  return {
    ...state,
    connect,
    disconnect,
    ws: wsRef.current,
  };
}

/**
 * Hook for subscribing to job updates
 */
export function useJobUpdates(
  onUpdate?: (job: WSJobPayload) => void,
  options: { jobId?: string; repositoryId?: string } = {}
) {
  const { jobId, repositoryId } = options;
  const [latestUpdate, setLatestUpdate] = useState<WSJobPayload | null>(null);
  const wsRef = useRef<WebSocketClient | null>(null);

  useEffect(() => {
    wsRef.current = getWebSocketClient();
    const ws = wsRef.current;

    // Subscribe to relevant channels
    const channels: string[] = [];
    if (jobId) {
      channels.push(`jobs:${jobId}`);
    }
    if (repositoryId) {
      channels.push(`repos:${repositoryId}:jobs`);
    }

    const subscribeToChannels = async () => {
      for (const channel of channels) {
        try {
          await ws.subscribe(channel);
        } catch (error) {
          console.error('Failed to subscribe to channel:', channel, error);
        }
      }
    };

    if (ws.isConnected()) {
      subscribeToChannels();
    }

    // Handle job updates
    const handleJobUpdate = (message: WSMessage<WSJobPayload>) => {
      if (message.payload) {
        setLatestUpdate(message.payload);
        onUpdate?.(message.payload);
      }
    };

    const handleJobCreated = (message: WSMessage<WSJobPayload>) => {
      if (message.payload) {
        setLatestUpdate(message.payload);
        onUpdate?.(message.payload);
      }
    };

    const handleJobCompleted = (message: WSMessage<WSJobPayload>) => {
      if (message.payload) {
        setLatestUpdate(message.payload);
        onUpdate?.(message.payload);
      }
    };

    const handleJobFailed = (message: WSMessage<WSJobPayload>) => {
      if (message.payload) {
        setLatestUpdate(message.payload);
        onUpdate?.(message.payload);
      }
    };

    const unsubUpdate = ws.on<WSJobPayload>('job.update', handleJobUpdate);
    const unsubCreated = ws.on<WSJobPayload>('job.created', handleJobCreated);
    const unsubCompleted = ws.on<WSJobPayload>('job.completed', handleJobCompleted);
    const unsubFailed = ws.on<WSJobPayload>('job.failed', handleJobFailed);

    return () => {
      unsubUpdate();
      unsubCreated();
      unsubCompleted();
      unsubFailed();

      // Unsubscribe from channels
      channels.forEach((channel) => {
        ws.unsubscribe(channel);
      });
    };
  }, [jobId, repositoryId, onUpdate]);

  return latestUpdate;
}

/**
 * Hook for subscribing to specific message types
 */
export function useWebSocketMessage<T = unknown>(
  messageType: WSMessageType,
  handler: (message: WSMessage<T>) => void
) {
  useEffect(() => {
    const ws = getWebSocketClient();
    const unsub = ws.on<T>(messageType, handler);
    return unsub;
  }, [messageType, handler]);
}

/**
 * Hook for subscribing to a channel
 */
export function useChannel(channel: string | null) {
  const [isSubscribed, setIsSubscribed] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!channel) {
      setIsSubscribed(false);
      return;
    }

    const ws = getWebSocketClient();

    const subscribe = async () => {
      try {
        await ws.subscribe(channel);
        setIsSubscribed(true);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err : new Error('Subscription failed'));
        setIsSubscribed(false);
      }
    };

    if (ws.isConnected()) {
      subscribe();
    }

    return () => {
      if (channel) {
        ws.unsubscribe(channel);
        setIsSubscribed(false);
      }
    };
  }, [channel]);

  return { isSubscribed, error };
}
