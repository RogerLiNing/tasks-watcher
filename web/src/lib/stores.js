import { writable, derived } from 'svelte/store';

export const projects = writable([]);
export const tasks = writable([]);
export const notifications = writable([]);
export const unreadCount = writable(0);
export const selectedProjectId = writable('');
export const selectedSource = writable(''); // filter by source: '', 'claude-code', 'cursor', 'manual'
export const showNotifications = writable(false);
export const sseConnected = writable(false);

// Derived: tasks grouped by status
export const tasksByStatus = derived(tasks, ($tasks) => {
  const groups = {
    pending: [],
    in_progress: [],
    completed: [],
    failed: [],
    cancelled: [],
  };
  for (const t of $tasks) {
    if (groups[t.status]) {
      groups[t.status].push(t);
    }
  }
  return groups;
});

// Derived: filtered tasks (by project and source)
export const filteredTasks = derived([tasks, selectedProjectId, selectedSource], ([$tasks, $pid, $src]) => {
  let result = $tasks;
  if ($pid) result = result.filter((t) => t.project_id === $pid);
  if ($src) result = result.filter((t) => t.source === $src);
  return result;
});

// Derived: filtered tasks by status (with project filter)
export const filteredTasksByStatus = derived(filteredTasks, ($tasks) => {
  const groups = {
    pending: [],
    in_progress: [],
    completed: [],
    failed: [],
    cancelled: [],
  };
  for (const t of $tasks) {
    if (groups[t.status]) {
      groups[t.status].push(t);
    }
  }
  return groups;
});

export function updateTaskInStore(task) {
  tasks.update((list) => {
    const idx = list.findIndex((t) => t.id === task.id);
    if (idx >= 0) {
      return [...list.slice(0, idx), task, ...list.slice(idx + 1)];
    }
    return [task, ...list];
  });
}

export function addTaskToStore(task) {
  tasks.update((list) => {
    if (list.find((t) => t.id === task.id)) return list;
    return [task, ...list];
  });
}

export function removeTaskFromStore(id) {
  tasks.update((list) => list.filter((t) => t.id !== id));
}

export function updateProjectInStore(project) {
  projects.update((list) => {
    const idx = list.findIndex((p) => p.id === project.id);
    if (idx >= 0) {
      return [...list.slice(0, idx), project, ...list.slice(idx + 1)];
    }
    return [project, ...list];
  });
}

export function addProjectToStore(project) {
  projects.update((list) => {
    if (list.find((p) => p.id === project.id)) return list;
    return [...list, project];
  });
}

export function removeProjectFromStore(id) {
  projects.update((list) => list.filter((p) => p.id !== id));
}
