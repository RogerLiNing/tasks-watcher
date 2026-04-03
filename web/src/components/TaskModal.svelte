<script>
  import { createEventDispatcher } from 'svelte';
  import { t, locale } from '../lib/i18n/index.js';
  import DependencyPanel from './DependencyPanel.svelte';
  import SubtaskPanel from './SubtaskPanel.svelte';

  export let task;
  export let projects = [];

  const dispatch = createEventDispatcher();
  let editing = false;
  let editTitle = task.title;
  let editDesc = '';
  let editPriority = task.priority;
  let editProjectId = task.project_id;
  let editTaskMode = task.task_mode || '';
  let activeTab = 'details';

  // Extract description for current locale; fall back to raw string (legacy data)
  function getLocalizedDesc(desc, loc) {
    if (!desc) return '';
    if (typeof desc === 'object') {
      return desc[loc] || desc['en'] || desc['zh'] || desc['_raw'] || Object.values(desc)[0] || '';
    }
    return desc;
  }

  // Initialize editDesc when task prop changes
  $: {
    editDesc = getLocalizedDesc(task.description, $locale);
    editTitle = task.title;
    editPriority = task.priority;
    editProjectId = task.project_id;
    editTaskMode = task.task_mode || '';
  }

  const tabs = [
    { key: 'details', label: $t('taskModal.tabDetails') },
    { key: 'dependencies', label: $t('taskModal.tabDependencies') },
    { key: 'subtasks', label: $t('taskModal.tabSubtasks') },
  ];

  $: statuses = [
    { key: 'pending', color: '#86868b' },
    { key: 'in_progress', color: '#0071e3' },
    { key: 'completed', color: '#34c759' },
    { key: 'failed', color: '#ff3b30' },
    { key: 'cancelled', color: '#ff9500' },
  ];

  function statusColor(s) {
    return statuses.find(x => x.key === s)?.color || '#86868b';
  }

  async function saveEdit() {
    const updated = {
      ...task,
      title: editTitle,
      description: { [$locale]: editDesc },
      priority: editPriority,
      project_id: editProjectId,
      task_mode: editTaskMode,
    };
    dispatch('update', updated);
    editing = false;
  }

  function formatTime(ts) {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleString();
  }

  function openTaskInModal(taskId) {
    // Close current modal and dispatch openTask for the new task
    dispatch('openTask', taskId);
  }
</script>

<button class="modal-backdrop" on:click={() => dispatch('close')} aria-label="Close modal">
</button>
<div class="modal" role="dialog">
    <div class="modal-header">
      <span class="status-indicator" style="background:{statusColor(task.status)}"></span>
      <span class="status-label">{$t('columns.' + task.status)}</span>
      <div class="modal-actions">
        <button class="delete-btn" on:click={() => dispatch('delete', task.id)}>{$t('taskModal.delete')}</button>
        <button class="close-btn" on:click={() => dispatch('close')}>×</button>
      </div>
    </div>

    <div class="tab-bar">
      {#each tabs as tab (tab.key)}
        <button
          class="tab"
          class:active={activeTab === tab.key}
          on:click={() => activeTab = tab.key}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <div class="modal-body">
      {#if activeTab === 'details'}
        {#if editing}
          <input class="edit-title" bind:value={editTitle} placeholder={$t('taskModal.taskTitle')} />
          <textarea class="edit-desc" bind:value={editDesc} placeholder={$t('taskModal.description')} rows="4"></textarea>
          <div class="edit-row">
            <label>
              {$t('taskModal.priority')}:
              <select bind:value={editPriority}>
                <option value="low">{$t('quickCreate.low')}</option>
                <option value="medium">{$t('quickCreate.med')}</option>
                <option value="high">{$t('quickCreate.high')}</option>
                <option value="urgent">{$t('quickCreate.urgent')}</option>
              </select>
            </label>
            <label>
              {$t('taskModal.project')}:
              <select bind:value={editProjectId}>
                {#each projects as p (p.id)}
                  <option value={p.id}>{p.name}</option>
                {/each}
              </select>
            </label>
          </div>
          <div class="edit-row edit-mode-row">
            <span class="mode-label">Task mode:</span>
            <label class="mode-radio">
              <input type="radio" bind:group={editTaskMode} value="" />
              Default (parallel)
            </label>
            <label class="mode-radio">
              <input type="radio" bind:group={editTaskMode} value="sequential" />
              🔗 Sequential
            </label>
            <label class="mode-radio">
              <input type="radio" bind:group={editTaskMode} value="parallel" />
              ⚡ Parallel
            </label>
          </div>
          <div class="edit-actions">
            <button class="save-btn" on:click={saveEdit}>{$t('taskModal.save')}</button>
            <button class="cancel-btn" on:click={() => editing = false}>{$t('taskModal.cancel')}</button>
          </div>
        {:else}
          <button class="task-title link-btn" on:click={() => editing = true}>{task.title}</button>
          {#if task.description}
            <button class="task-desc link-btn" on:click={() => editing = true}>{getLocalizedDesc(task.description, $locale)}</button>
          {/if}
          <div class="meta-grid">
            <div class="meta-item">
              <span class="meta-label">{$t('taskModal.priority')}</span>
              <span class="meta-value priority-badge" data-priority={task.priority}>{task.priority}</span>
            </div>
            {#if task.assignee}
              <div class="meta-item">
                <span class="meta-label">{$t('taskModal.assignee')}</span>
                <span class="meta-value">{task.assignee}</span>
              </div>
            {/if}
            {#if task.source && task.source !== 'manual'}
              <div class="meta-item">
                <span class="meta-label">{$t('taskModal.source')}</span>
                <span class="meta-value source-badge" data-source={task.source}>{$t('sources.' + task.source) || task.source}</span>
              </div>
            {/if}
            {#if task.task_mode === 'sequential' || task.task_mode === 'parallel'}
              <div class="meta-item">
                <span class="meta-label">{$t('taskModal.mode')}</span>
                <span class="meta-value mode-badge" data-mode={task.task_mode}>{task.task_mode === 'sequential' ? '🔗 Sequential' : '⚡ Parallel'}</span>
              </div>
            {/if}
            <div class="meta-item">
              <span class="meta-label">{$t('taskModal.created')}</span>
              <span class="meta-value">{formatTime(task.created_at)}</span>
            </div>
            {#if task.completed_at}
              <div class="meta-item">
                <span class="meta-label">{$t('taskModal.completed')}</span>
                <span class="meta-value">{formatTime(task.completed_at)}</span>
              </div>
            {/if}
            {#if task.error_message}
              <div class="meta-item error-row">
                <span class="meta-label">{$t('taskModal.error')}</span>
                <span class="meta-value error">{task.error_message}</span>
              </div>
            {/if}
          </div>
          <button class="edit-btn" on:click={() => editing = true}>{$t('taskModal.edit')}</button>
        {/if}
      {:else if activeTab === 'dependencies'}
        <DependencyPanel
          {task}
          on:refresh
          on:openTask={(e) => openTaskInModal(e.detail)}
        />
      {:else if activeTab === 'subtasks'}
        <SubtaskPanel
          {task}
          on:refresh
          on:openTask={(e) => openTaskInModal(e.detail)}
        />
      {/if}
    </div>

    {#if !editing && activeTab === 'details'}
      <div class="modal-footer">
        {#each statuses as s (s.key)}
          {#if s.key !== task.status}
            <button
              class="status-btn"
              style="--c:{s.color}"
              on:click={() => dispatch('statusChange', { id: task.id, status: s.key, reason: '' })}
            >
              {$t('columns.' + s.key)}
            </button>
          {/if}
        {/each}
      </div>
    {/if}
  </div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.4);
    border: none;
    padding: 0;
    cursor: pointer;
    z-index: 99;
  }

  .modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: white;
    border-radius: 16px;
    width: 520px;
    max-width: 95vw;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 20px 60px rgba(0,0,0,0.2);
    z-index: 100;
  }

  .modal-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid #e5e5ea;
  }

  .status-indicator { width: 10px; height: 10px; border-radius: 50%; }
  .status-label { font-size: 0.8rem; font-weight: 600; color: #6e6e73; text-transform: capitalize; flex: 1; }

  .modal-actions { display: flex; gap: 0.5rem; align-items: center; }

  .delete-btn {
    background: none;
    border: none;
    color: #ff3b30;
    font-size: 0.8rem;
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    border-radius: 6px;
  }
  .delete-btn:hover { background: #fff0ee; }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: #86868b;
    padding: 0 0.5rem;
    line-height: 1;
  }

  .tab-bar {
    display: flex;
    border-bottom: 1px solid #e5e5ea;
    padding: 0 1.25rem;
  }

  .tab {
    background: none;
    border: none;
    padding: 0.6rem 1rem;
    font-size: 0.85rem;
    color: #86868b;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 0.15s;
    margin-bottom: -1px;
  }
  .tab.active {
    color: #0071e3;
    border-bottom-color: #0071e3;
    font-weight: 600;
  }
  .tab:not(.active):hover { color: #1d1d1f; }

  .modal-body { padding: 1.25rem; }

  .task-title {
    font-size: 1.1rem;
    font-weight: 600;
    color: #1d1d1f;
    cursor: pointer;
    margin-bottom: 0.5rem;
    background: none;
    border: none;
    padding: 0;
    text-align: left;
    width: 100%;
  }

  .task-desc {
    font-size: 0.9rem;
    color: #6e6e73;
    margin-bottom: 1rem;
    white-space: pre-wrap;
    background: none;
    border: none;
    padding: 0;
    text-align: left;
    width: 100%;
    cursor: pointer;
  }

  .meta-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }

  .meta-item { display: flex; flex-direction: column; gap: 0.2rem; }
  .meta-label { font-size: 0.7rem; font-weight: 600; text-transform: uppercase; color: #86868b; }
  .meta-value { font-size: 0.85rem; color: #1d1d1f; }
  .error-row .meta-value { color: #ff3b30; }

  .priority-badge[data-priority="urgent"] { color: #ff3b30; font-weight: 600; }
  .priority-badge[data-priority="high"] { color: #ff9500; }
  .priority-badge[data-priority="medium"] { color: #0071e3; }
  .priority-badge[data-priority="low"] { color: #34c759; }

  .source-badge[data-source="claude-code"] { color: #7c3aed; font-weight: 600; }
  .source-badge[data-source="cursor"] { color: #0891b2; font-weight: 600; }

  .mode-badge[data-mode="sequential"] { color: #0071e3; font-weight: 600; }
  .mode-badge[data-mode="parallel"] { color: #34c759; font-weight: 600; }

  .edit-mode-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
    margin-bottom: 0.5rem;
  }

  .mode-label {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    color: #86868b;
  }

  .mode-radio {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.8rem;
    color: #1d1d1f;
    cursor: pointer;
  }
  .mode-radio input { cursor: pointer; }

  .edit-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    padding: 0.4rem 1rem;
    font-size: 0.85rem;
    cursor: pointer;
    color: #1d1d1f;
  }
  .edit-btn:hover { background: #f5f5f7; }

  .edit-title {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 1rem;
    margin-bottom: 0.5rem;
    outline: none;
  }
  .edit-title:focus { border-color: #0071e3; }

  .edit-desc {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.9rem;
    resize: vertical;
    margin-bottom: 0.75rem;
    outline: none;
    font-family: inherit;
  }
  .edit-desc:focus { border-color: #0071e3; }

  .edit-row {
    display: flex;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }
  .edit-row label { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.8rem; color: #6e6e73; }
  .edit-row select { padding: 0.4rem; border: 1px solid #d2d2d7; border-radius: 6px; font-size: 0.85rem; }

  .edit-actions { display: flex; gap: 0.5rem; }
  .save-btn {
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 8px;
    padding: 0.5rem 1rem;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .cancel-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    padding: 0.5rem 1rem;
    font-size: 0.9rem;
    cursor: pointer;
  }

  .modal-footer {
    padding: 1rem 1.25rem;
    border-top: 1px solid #e5e5ea;
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .status-btn {
    padding: 0.4rem 1rem;
    border: 1px solid var(--c);
    background: none;
    border-radius: 20px;
    font-size: 0.8rem;
    cursor: pointer;
    color: var(--c);
    font-weight: 600;
    transition: all 0.15s;
  }
  .status-btn:hover {
    background: var(--c);
    color: white;
  }
</style>
