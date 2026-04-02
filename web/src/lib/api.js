const API_BASE = '/api';

let apiKey = localStorage.getItem('tasks_watcher_api_key');

export function setApiKey(key) {
  apiKey = key;
  localStorage.setItem('tasks_watcher_api_key', key);
}

export function getApiKey() {
  return apiKey;
}

function headers() {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
  };
}

async function request(method, path, body) {
  const opts = { method, headers: headers() };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  if (res.status === 401) {
    throw new Error('UNAUTHORIZED');
  }
  if (!res.ok) {
    const err = await res.text();
    throw new Error(err);
  }
  if (res.status === 204) return null;
  return res.json();
}

export const api = {
  // Health
  health: () => request('GET', '/health'),

  // Projects
  listProjects: () => request('GET', '/projects'),
  createProject: (data) => request('POST', '/projects', data),
  updateProject: (id, data) => request('PUT', `/projects/${id}`, data),
  deleteProject: (id) => request('DELETE', `/projects/${id}`),

  // Tasks
  listTasks: (filters = {}) => {
    const params = new URLSearchParams(filters).toString();
    return request('GET', `/tasks${params ? '?' + params : ''}`);
  },
  createTask: (data) => request('POST', '/tasks', data),
  getTask: (id) => request('GET', `/tasks/${id}`),
  updateTask: (id, data) => request('PUT', `/tasks/${id}`, data),
  updateTaskStatus: (id, status, reason) => request('PATCH', `/tasks/${id}/status`, { status, reason }),
  deleteTask: (id) => request('DELETE', `/tasks/${id}`),
  heartbeat: (id) => request('POST', `/tasks/${id}/heartbeat`),

  // Notifications
  listNotifications: () => request('GET', '/notifications'),
  markAllRead: () => request('POST', '/notifications/read'),
  clearNotifications: () => request('DELETE', '/notifications'),

  // Agents
  listAgents: () => request('GET', '/agents'),

  // Export
  export: () => request('GET', '/export'),
};
