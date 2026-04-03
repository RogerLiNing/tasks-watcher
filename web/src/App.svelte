<script>
  import { onMount, onDestroy } from 'svelte';
  import { api, setApiKey, getApiKey } from './lib/api.js';
  import { connectSSE, disconnectSSE, onSSEEvent } from './lib/sse.js';
  import {
    projects, tasks, notifications, unreadCount,
    selectedProjectId, selectedSource, showNotifications,
    addTaskToStore, updateTaskInStore, removeTaskFromStore,
    addProjectToStore, updateProjectInStore, removeProjectFromStore,
    filteredTasksByStatus, sseConnected
  } from './lib/stores.js';
  import { locale, t, locales } from './lib/i18n/index.js';
  import ProjectSidebar from './components/ProjectSidebar.svelte';
  import NotificationSettings from './components/NotificationSettings.svelte';
  import ColumnSettings from './components/ColumnSettings.svelte';
  import KanbanBoard from './components/KanbanBoard.svelte';
  import TaskModal from './components/TaskModal.svelte';
  import NotificationsPanel from './components/NotificationsPanel.svelte';
  import QuickCreate from './components/QuickCreate.svelte';
  import { columns } from './lib/stores.js';

  let selectedTask = null;
  let loading = true;
  let authError = false;
  let showSettings = false;
  let settingsTab = 'notifications'; // 'notifications' | 'columns'
  let toastMessage = '';
  let toastVisible = false;
  let toastTimer;

  async function loadData() {
    try {
      const [projData, taskData, notifData] = await Promise.all([
        api.listProjects(),
        api.listTasks(),
        api.listNotifications(),
      ]);
      projects.set(projData.projects || []);
      tasks.set(taskData.tasks || []);
      notifications.set(notifData.notifications || []);
      unreadCount.set(notifData.unread_count || 0);
      loading = false;
      authError = false;
    } catch (e) {
      if (e.message === 'UNAUTHORIZED') {
        authError = true;
        loading = false;
      } else {
        console.error('Failed to load data', e);
        loading = false;
      }
    }
    // Load columns non-blocking — old servers may not have this endpoint
    try {
      const colData = await api.listColumns();
      columns.set(colData.columns || []);
    } catch (_) {
      // Columns not supported by this server version; use defaults
    }
  }

  async function handleApiKeySubmit() {
    const input = document.getElementById('api-key-input');
    if (!input) return;
    const key = input.value.trim();
    if (!key) return;
    setApiKey(key);
    authError = false;
    await loadData();
    connectSSE();
  }

  onMount(async () => {
    const storedKey = getApiKey();
    let key = storedKey;

    // Auto-fetch API key if not in localStorage
    if (!key) {
      try {
        const res = await fetch('/api/key');
        if (res.ok) {
          const data = await res.json();
          key = data.api_key;
          setApiKey(key);
        }
      } catch (_) {
        // Server may not be reachable, fall through to setup screen
      }
    }

    if (!key) {
      loading = false;
      return;
    }
    await loadData();
    if (!authError) {
      connectSSE();
    }

    // Handle SSE events
    const unsub = onSSEEvent((event) => {
      switch (event.type) {
        case 'task.created':
          addTaskToStore(event.payload);
          break;
        case 'task.started':
        case 'task.completed':
        case 'task.failed':
        case 'task.cancelled':
        case 'task.updated':
          updateTaskInStore(event.payload);
          break;
        case 'task.deleted':
          removeTaskFromStore(event.payload.id);
          break;
        case 'task.dependency.added':
        case 'task.dependency.removed':
        case 'task.subtask.added':
        case 'task.subtask.removed':
        case 'task.subtask.reordered':
          // Refresh the affected task if it's the selected one
          if (selectedTask && event.payload && (event.payload.task_id || event.payload.parent_id)) {
            const taskId = event.payload.task_id || event.payload.parent_id;
            if (selectedTask.id === taskId) {
              api.getTask(taskId).then(updated => {
                if (updated) {
                  updateTaskInStore(updated);
                  selectedTask = updated;
                }
              }).catch(() => {});
            }
          }
          break;
        case 'task.subtask.status_changed':
          updateTaskInStore(event.payload.parent);
          if (selectedTask && selectedTask.id === event.payload.parent.id) {
            selectedTask = event.payload.parent;
          }
          break;
        case 'project.created':
          addProjectToStore(event.payload);
          break;
        case 'project.updated':
          updateProjectInStore(event.payload);
          break;
        case 'project.deleted':
          removeProjectFromStore(event.payload.id);
          break;
        case 'column.created':
          columns.update(cols => {
            if (cols.find(c => c.id === event.payload.id)) return cols;
            return [...cols, event.payload];
          });
          break;
        case 'column.updated':
          columns.update(cols => cols.map(c => c.id === event.payload.id ? event.payload : c));
          break;
        case 'column.deleted':
          columns.update(cols => cols.filter(c => c.id !== event.payload.id));
          break;
      }
    });

    return () => {
      unsub();
      disconnectSSE();
    };
  });

  function openTask(task) {
    selectedTask = task;
  }

  function closeTask() {
    selectedTask = null;
  }

  async function handleTaskUpdate(task) {
    try {
      const updated = await api.updateTask(task.id, task);
      updateTaskInStore(updated);
      selectedTask = updated;
    } catch (e) {
      console.error('Failed to update task', e);
    }
  }

  async function handleStatusChange(taskId, status, reason) {
    try {
      const updated = await api.updateTaskStatus(taskId, status, reason);
      updateTaskInStore(updated);
      if (selectedTask && selectedTask.id === taskId) {
        selectedTask = updated;
      }
    } catch (e) {
      let msg = 'Failed to update status';
      try {
        const errData = JSON.parse(e.message);
        if (errData.error === 'task is blocked') {
          msg = 'Task is blocked';
          if (errData.blockers?.length) msg += ': ' + errData.blockers.join(', ');
          else if (errData.child_titles?.length) msg += ' (has non-terminal subtasks): ' + errData.child_titles.join(', ');
        } else {
          msg = errData.error || msg;
        }
      } catch (_) {}
      showToast(msg);
    }
  }

  function showToast(msg) {
    clearTimeout(toastTimer);
    toastMessage = msg;
    toastVisible = true;
    toastTimer = setTimeout(() => { toastVisible = false; }, 4000);
  }

  async function handleTaskCreate(task) {
    try {
      const created = await api.createTask(task);
      addTaskToStore(created);
    } catch (e) {
      console.error('Failed to create task', e);
    }
  }

  async function handleTaskDelete(taskId) {
    try {
      await api.deleteTask(taskId);
      removeTaskFromStore(taskId);
      selectedTask = null;
    } catch (e) {
      console.error('Failed to delete task', e);
    }
  }
</script>

{#if loading}
  <div class="loading-screen">
    <div class="spinner"></div>
    <p>{$t('app.loading')}</p>
  </div>
{:else if authError}
  <div class="setup-screen">
    <div class="setup-card">
      <h1>{$t('app.connectTitle')}</h1>
      <p>{$t('app.connectHint')}</p>
      <input
        id="api-key-input"
        type="password"
        placeholder={$t('app.apiKeyPlaceholder')}
        on:keydown={(e) => e.key === 'Enter' && handleApiKeySubmit()}
      />
      <button on:click={handleApiKeySubmit}>{$t('app.connectBtn')}</button>
    </div>
  </div>
{:else}
  <div class="app-layout">
    <header class="topbar">
      <div class="topbar-left">
        <h1 class="logo">{$t('app.title')}</h1>
      </div>
      <div class="topbar-center">
        <QuickCreate on:create={(e) => handleTaskCreate(e.detail)} />
      </div>
      <div class="topbar-right">
        <span class="sse-indicator" class:connected={$sseConnected} title={$sseConnected ? $t('sse.connected') : $t('sse.disconnected')}></span>
        <div class="lang-switch">
          {#each locales as l}
            <button class="lang-btn" class:active={$locale === l.code} on:click={() => locale.set(l.code)}>{l.label}</button>
          {/each}
        </div>
        <button class="icon-btn" on:click={() => showNotifications.update(v => !v)}>
          🔔
          {#if $unreadCount > 0}
            <span class="badge">{$unreadCount}</span>
          {/if}
        </button>
        <button class="icon-btn settings-btn" on:click={() => showSettings = true} title="Settings">⚙</button>
        <select class="source-filter" bind:value={$selectedSource}>
          <option value="">{$t('sources.all')}</option>
          <option value="claude-code">{$t('sources.claude-code')}</option>
          <option value="cursor">{$t('sources.cursor')}</option>
          <option value="manual">{$t('sources.manual')}</option>
        </select>
        <select class="project-filter" bind:value={$selectedProjectId}>
          <option value="">{$t('projects.allProjects') || 'All Projects'}</option>
          {#each $projects as p (p.id)}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </div>
    </header>

    <div class="main-layout">
      <ProjectSidebar
        projects={$projects}
        selectedId={$selectedProjectId}
        on:select={(e) => selectedProjectId.set(e.detail)}
        on:createProject={async (e) => {
          const p = await api.createProject(e.detail);
          addProjectToStore(p);
        }}
        on:updateProject={(e) => updateProjectInStore(e.detail)}
        on:deleteProject={async (e) => {
          const p = e.detail;
          try {
            await api.deleteProject(p.id);
            removeProjectFromStore(p.id);
            if ($selectedProjectId === p.id) selectedProjectId.set('');
          } catch (e) {
            showToast('Failed to delete project: ' + e.message);
          }
        }}
      />
      <main class="content">
        <KanbanBoard
          tasksByStatus={$filteredTasksByStatus}
          on:openTask={(e) => openTask(e.detail)}
          on:statusChange={(e) => handleStatusChange(e.detail.id, e.detail.status, e.detail.reason)}
        />
      </main>
    </div>
  </div>

  {#if selectedTask}
    <TaskModal
      task={selectedTask}
      projects={$projects}
      on:close={closeTask}
      on:update={(e) => handleTaskUpdate(e.detail)}
      on:statusChange={(e) => handleStatusChange(e.detail.id, e.detail.status, e.detail.reason)}
      on:delete={(e) => handleTaskDelete(e.detail)}
      on:openTask={async (e) => {
        const taskId = typeof e.detail === 'string' ? e.detail : e.detail.id;
        const t = await api.getTask(taskId);
        if (t) openTask(t);
      }}
    />
  {/if}

  {#if $showNotifications}
    <NotificationsPanel
      notifications={$notifications}
      on:close={() => showNotifications.set(false)}
      on:markAllRead={async () => {
        await api.markAllRead();
        unreadCount.set(0);
        notifications.update(ns => ns.map(n => ({ ...n, read: true })));
      }}
    />
  {/if}

  {#if showSettings}
    <button class="settings-overlay" on:click={() => showSettings = false} aria-label="Close settings">
    </button>
    <div class="settings-panel" role="dialog">
      <div class="panel-header">
        <h2>Settings</h2>
        <button class="close-btn" on:click={() => showSettings = false}>×</button>
      </div>
      <div class="settings-tabs">
        <button class="tab-btn" class:active={settingsTab === 'notifications'} on:click={() => settingsTab = 'notifications'}>
          Notifications
        </button>
        <button class="tab-btn" class:active={settingsTab === 'columns'} on:click={() => settingsTab = 'columns'}>
          Kanban Columns
        </button>
      </div>
      <div class="settings-body">
        {#if settingsTab === 'notifications'}
          <NotificationSettings />
        {:else}
          <ColumnSettings />
        {/if}
      </div>
    </div>
  {/if}

  {#if toastVisible}
    <div class="toast">{toastMessage}</div>
  {/if}
{/if}

<style>
  :global(*) { box-sizing: border-box; }
  :global(body) { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f0f2f5; color: #1d1d1f; }

  .settings-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    border: none;
    padding: 0;
    cursor: pointer;
    z-index: 299;
  }

  .settings-panel {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: white;
    border-radius: 16px;
    width: 580px;
    max-width: 95vw;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 20px 60px rgba(0,0,0,0.2);
    display: flex;
    flex-direction: column;
    z-index: 300;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid #e5e5ea;
    flex-shrink: 0;
  }

  .panel-header h2 { font-size: 1.1rem; font-weight: 600; margin: 0; }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: #86868b;
    padding: 0 0.5rem;
    line-height: 1;
  }

  .settings-tabs {
    display: flex;
    border-bottom: 1px solid #e5e5ea;
    padding: 0 1.5rem;
    flex-shrink: 0;
  }

  .tab-btn {
    padding: 0.75rem 1rem;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    cursor: pointer;
    font-size: 0.9rem;
    font-weight: 500;
    color: #6e6e73;
    transition: all 0.15s;
    margin-bottom: -1px;
  }

  .tab-btn:hover { color: #1d1d1f; }
  .tab-btn.active { color: #0071e3; border-bottom-color: #0071e3; font-weight: 600; }

  .settings-body {
    flex: 1;
    overflow-y: auto;
  }

  .loading-screen, .setup-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    gap: 1rem;
  }

  .spinner {
    width: 40px;
    height: 40px;
    border: 3px solid #e5e5e5;
    border-top: 3px solid #0071e3;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .setup-card {
    background: white;
    border-radius: 16px;
    padding: 2.5rem;
    max-width: 420px;
    width: 90%;
    box-shadow: 0 4px 24px rgba(0,0,0,0.1);
    text-align: center;
  }

  .setup-card h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
  .setup-card p { color: #6e6e73; font-size: 0.9rem; margin-bottom: 1rem; }

  .setup-card input {
    width: 100%;
    padding: 0.75rem 1rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.95rem;
    margin-bottom: 1rem;
    outline: none;
  }
  .setup-card input:focus { border-color: #0071e3; box-shadow: 0 0 0 3px rgba(0,113,227,0.15); }

  .setup-card button {
    width: 100%;
    padding: 0.75rem;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 1rem;
    cursor: pointer;
    font-weight: 600;
  }
  .setup-card button:hover { background: #0077ed; }

  .app-layout { display: flex; flex-direction: column; height: 100vh; }

  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
    background: white;
    border-bottom: 1px solid #e5e5ea;
    gap: 1rem;
    height: 60px;
    flex-shrink: 0;
  }

  .topbar-left, .topbar-right { min-width: 200px; }
  .topbar-right { display: flex; justify-content: flex-end; }
  .topbar-center { flex: 1; max-width: 500px; }

  .logo { font-size: 1.1rem; font-weight: 700; color: #1d1d1f; }

  .icon-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 1.3rem;
    padding: 0.5rem;
    border-radius: 8px;
    position: relative;
    transition: background 0.15s;
  }
  .icon-btn:hover { background: #f5f5f7; }

  .sse-indicator {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #ff3b30;
    margin-right: 0.5rem;
    vertical-align: middle;
    transition: background 0.3s;
  }
  .settings-btn { font-size: 1.1rem; }

  .lang-switch {
    display: flex;
    gap: 2px;
    margin-right: 0.5rem;
  }

  .lang-btn {
    background: none;
    border: 1px solid #d2d2d7;
    padding: 2px 8px;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
    color: #6e6e73;
    transition: all 0.15s;
  }
  .lang-btn.active {
    background: #0071e3;
    color: white;
    border-color: #0071e3;
  }
  .lang-btn:not(.active):hover { background: #f5f5f7; }

  .source-filter {
    padding: 4px 8px;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    color: #1d1d1f;
    background: white;
    cursor: pointer;
    outline: none;
  }
  .source-filter:focus { border-color: #0071e3; }
  .project-filter {
    padding: 4px 8px;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    background: white;
    cursor: pointer;
    outline: none;
  }
  .project-filter:focus { border-color: #0071e3; }

  .badge {
    position: absolute;
    top: 2px;
    right: 2px;
    background: #ff3b30;
    color: white;
    font-size: 0.65rem;
    font-weight: 700;
    border-radius: 10px;
    min-width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 4px;
  }

  .main-layout { display: flex; flex: 1; overflow: hidden; }

  .content { flex: 1; overflow: auto; padding: 1.5rem; }

  .toast {
    position: fixed;
    bottom: 2rem;
    left: 50%;
    transform: translateX(-50%);
    background: #1d1d1f;
    color: white;
    padding: 0.75rem 1.5rem;
    border-radius: 10px;
    font-size: 0.875rem;
    z-index: 1000;
    box-shadow: 0 4px 20px rgba(0,0,0,0.2);
    animation: toastIn 0.2s ease;
  }

  @keyframes toastIn {
    from { opacity: 0; transform: translateX(-50%) translateY(10px); }
    to { opacity: 1; transform: translateX(-50%) translateY(0); }
  }
</style>
