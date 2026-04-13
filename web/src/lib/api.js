const API_BASE = '/api';

async function request(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  if (res.status === 401) {
    // Clear session and redirect to login
    currentUser.set(null);
    isAuthenticated.set(false);
    throw new Error('UNAUTHORIZED');
  }
  if (!res.ok) {
    const contentType = res.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      const err = await res.json();
      throw new Error(err.error || res.statusText);
    }
    throw new Error(await res.text());
  }
  if (res.status === 204) return null;
  return res.json();
}

import { currentUser, isAuthenticated } from './stores.js';

export { currentUser, isAuthenticated };

export const api = {
  // Health
  health: () => request('GET', '/health'),

  // Auth
  register: (username, password) =>
    request('POST', '/auth/register', { username, password }),
  login: (username, password) =>
    request('POST', '/auth/login', { username, password }),
  logout: () => request('POST', '/auth/logout'),
  me: () => request('GET', '/auth/me'),

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
  createTask: (data) => request('POST', '/tasks', {
    ...data,
    description: data.description ? JSON.parse(JSON.stringify(data.description)) : undefined,
  }),
  getTask: (id) => request('GET', `/tasks/${id}`),
  updateTask: (id, data) => request('PUT', `/tasks/${id}`, {
    ...data,
    description: data.description ? JSON.parse(JSON.stringify(data.description)) : undefined,
  }),
  updateTaskStatus: (id, status, reason) => request('PATCH', `/tasks/${id}/status`, { status, reason }),
  deleteTask: (id) => request('DELETE', `/tasks/${id}`),
  heartbeat: (id) => request('POST', `/tasks/${id}/heartbeat`),

  // Dependencies
  getBlockers: (taskId) => request('GET', `/tasks/${taskId}/dependencies`),
  addBlocker: (taskId, blockerId) => request('POST', `/tasks/${taskId}/dependencies`, { blocker_id: blockerId }),
  removeBlocker: (taskId, blockerId) => request('DELETE', `/tasks/${taskId}/dependencies/${blockerId}`),
  getDependents: (taskId) => request('GET', `/tasks/${taskId}/dependents`),
  canStart: (taskId) => request('GET', `/tasks/${taskId}/can-start`),

  // Subtasks
  getSubtasks: (parentId) => request('GET', `/tasks/${parentId}/subtasks`),
  createSubtask: (parentId, data) => request('POST', `/tasks/${parentId}/subtasks`, data),
  removeSubtask: (parentId, childId) => request('DELETE', `/tasks/${parentId}/subtasks/${childId}`),
  reorderSubtask: (parentId, childId, position) => request('PATCH', `/tasks/${parentId}/subtasks/${childId}/position`, { position }),
  getParent: (taskId) => request('GET', `/tasks/${taskId}/parent`),

  // Notifications
  listNotifications: () => request('GET', '/notifications'),
  markAllRead: () => request('POST', '/notifications/read'),
  clearNotifications: () => request('DELETE', '/notifications'),

  // Agents
  listAgents: () => request('GET', '/agents'),

  // Export
  export: () => request('GET', '/export'),

  // Columns
  listColumns: () => request('GET', '/columns'),
  createColumn: (data) => request('POST', '/columns', data),
  updateColumn: (id, data) => request('PUT', `/columns/${id}`, data),
  deleteColumn: (id) => request('DELETE', `/columns/${id}`),

  // Notification configs
  listNotificationConfigs: () => request('GET', '/notifications/configs'),
  upsertNotificationConfig: (data) => request('POST', '/notifications/configs', data),

  // Comments
  getComments: (taskId) => request('GET', `/tasks/${taskId}/comments`),
  createComment: (taskId, data) => request('POST', `/tasks/${taskId}/comments`, data),
  updateComment: (taskId, commentId, data) => request('PUT', `/tasks/${taskId}/comments/${commentId}`, data),
  deleteComment: (taskId, commentId) => request('DELETE', `/tasks/${taskId}/comments/${commentId}`),
};
