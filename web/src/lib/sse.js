import { api } from './api.js';
import { sseConnected } from './stores.js';

let es = null;
let reconnectTimer = null;
let reconnectDelay = 1000;
const listeners = new Set();

export function connectSSE() {
  if (es) {
    es.close();
  }

  const key = api.getApiKey();
  const url = `/api/events${key ? '?api_key=' + encodeURIComponent(key) : ''}`;
  console.log('[SSE] Connecting to', url);

  try {
    es = new EventSource(url);
  } catch (e) {
    console.error('[SSE] Failed to create EventSource', e);
    scheduleReconnect();
    return;
  }

  es.onopen = () => {
    console.log('[SSE] Connected');
    sseConnected.set(true);
    reconnectDelay = 1000;
  };

  es.onmessage = (event) => {
    if (event.data === 'ping') return;
    console.log('[SSE] Event:', event.data);
    try {
      const data = JSON.parse(event.data);
      listeners.forEach((fn) => fn(data));
    } catch (e) {
      console.warn('[SSE] Failed to parse event', e);
    }
  };

  es.onerror = (e) => {
    console.warn('[SSE] Error, reconnecting...', e);
    es.close();
    es = null;
    sseConnected.set(false);
    scheduleReconnect();
  };
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    reconnectDelay = Math.min(reconnectDelay * 2, 30000);
    connectSSE();
  }, reconnectDelay);
}

export function onSSEEvent(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export function disconnectSSE() {
  if (es) {
    es.close();
    es = null;
    sseConnected.set(false);
  }
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}
