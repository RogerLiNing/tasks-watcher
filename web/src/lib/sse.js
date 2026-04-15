import { sseConnected } from './stores.js';

const DEBUG = import.meta.env.VITE_DEBUG === 'true';

function debug(...args) {
  if (DEBUG) console.log('[SSE]', ...args);
}

let es = null;
let reconnectTimer = null;
let reconnectDelay = 1000;
const listeners = new Set();

export function connectSSE() {
  if (es) {
    es.close();
  }

  // Browser automatically sends HttpOnly session cookie
  const url = '/api/events';
  debug('Connecting to', url);

  try {
    es = new EventSource(url);
  } catch (e) {
    console.error('[SSE] Failed to create EventSource', e);
    scheduleReconnect();
    return;
  }

  es.onopen = () => {
    debug('Connected');
    sseConnected.set(true);
    reconnectDelay = 1000;
  };

  es.onmessage = (event) => {
    if (event.data === 'ping') return;
    debug('Event:', event.data);
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
