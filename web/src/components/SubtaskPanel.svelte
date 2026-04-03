<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from '../lib/i18n/index.js';
  import { api } from '../lib/api.js';

  export let task;

  const dispatch = createEventDispatcher();

  // subtaskWithPos shape: { id, title, status, position, ...Task }
  let subtasks = [];
  let parent = null;
  let loading = true;
  let error = '';
  let showCreateForm = false;
  let newTitle = '';
  let newDesc = '';
  let creating = false;

  // Local parent mode (synced from parent task prop)
  $: parentMode = task.task_mode || '';

  async function loadSubtasks() {
    loading = true;
    error = '';
    try {
      const [subRes, parentRes] = await Promise.all([
        api.getSubtasks(task.id),
        api.getParent(task.id),
      ]);
      subtasks = subRes.subtasks || [];
      parent = parentRes.parent || null;
    } catch (e) {
      error = $t('common.errors.loadSubtasks');
    }
    loading = false;
  }

  async function createSubtask() {
    if (!newTitle.trim()) return;
    creating = true;
    error = '';
    try {
      await api.createSubtask(task.id, {
        title: newTitle.trim(),
        description: newDesc.trim(),
      });
      newTitle = '';
      newDesc = '';
      showCreateForm = false;
      await loadSubtasks();
      dispatch('refresh');
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
    creating = false;
  }

  async function removeSubtask(childId) {
    error = '';
    try {
      await api.removeSubtask(task.id, childId);
      await loadSubtasks();
      dispatch('refresh');
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
  }

  async function moveSubtask(childId, direction) {
    const idx = subtasks.findIndex(s => s.id === childId);
    if (idx < 0) return;
    const newPos = direction === 'up' ? idx - 1 : idx + 1;
    if (newPos < 0 || newPos >= subtasks.length) return;
    try {
      await api.reorderSubtask(task.id, childId, newPos);
      await loadSubtasks();
      dispatch('refresh');
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
  }

  function openTask(taskId) {
    dispatch('openTask', taskId);
  }

  $: if (task) loadSubtasks();
</script>

<div class="subtask-panel">
  <div class="section-header">
    <span class="section-title">{$t('subtaskPanel.title')}</span>
    {#if !loading}
      <button class="add-btn" on:click={() => showCreateForm = !showCreateForm}>
        {showCreateForm ? $t('subtaskPanel.cancelAdd') : $t('subtaskPanel.addSubtask')}
      </button>
    {/if}
  </div>

  {#if error}
    <div class="error-msg">{error}</div>
  {/if}

  {#if loading}
    <div class="loading">{$t('common.loading')}</div>
  {:else}
    {#if parent}
      <div class="parent-link">
        <span class="parent-label">Part of:</span>
        <button class="parent-title link-btn" on:click={() => openTask(parent.id)}>{parent.title}</button>
      </div>
    {/if}

    {#if parentMode === 'sequential' || parentMode === 'parallel'}
      <div class="mode-indicator" class:sequential={parentMode === 'sequential'} class:parallel={parentMode === 'parallel'}>
        {parentMode === 'sequential' ? $t('subtaskPanel.sequentialOrder') : $t('subtaskPanel.parallelAllRun')}
      </div>
    {/if}

    {#if showCreateForm}
      <div class="create-form">
        <input
          class="form-input"
          bind:value={newTitle}
          placeholder="Subtask title..."
          on:keydown={(e) => e.key === 'Enter' && createSubtask()}
        />
        <textarea
          class="form-textarea"
          bind:value={newDesc}
          placeholder="Description (optional)"
          rows="2"
        ></textarea>
        <div class="form-actions">
          <button class="create-submit" on:click={createSubtask} disabled={creating || !newTitle.trim()}>
            {creating ? $t('subtaskPanel.creating') : $t('subtaskPanel.create')}
          </button>
        </div>
      </div>
    {/if}

    <div class="subtask-list">
      <p class="subtask-label">({subtasks.length})</p>
      {#if subtasks.length === 0 && !showCreateForm}
        <p class="empty-hint">{$t('subtaskPanel.noSubtasks')}</p>
      {:else}
        {#each subtasks as s, i (s.id)}
          <div class="subtask-item">
            {#if parentMode === 'sequential'}
              <span class="pos-num">{i + 1}</span>
              <button
                class="reorder-btn"
                disabled={i === 0}
                on:click={() => moveSubtask(s.id, 'up')}
                title="Move up"
              >↑</button>
              <button
                class="reorder-btn"
                disabled={i === subtasks.length - 1}
                on:click={() => moveSubtask(s.id, 'down')}
                title="Move down"
              >↓</button>
            {/if}
            <span class="subtask-status" data-status={s.status}></span>
            <button class="subtask-title link-btn" on:click={() => openTask(s.id)}>{s.title}</button>
            <button class="remove-btn" on:click={() => removeSubtask(s.id)} title="Remove from subtasks">×</button>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .subtask-panel { padding: 0.5rem 0; }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }

  .section-title {
    font-size: 0.75rem;
    font-weight: 700;
    text-transform: uppercase;
    color: #86868b;
    letter-spacing: 0.05em;
  }

  .add-btn {
    background: none;
    border: 1px solid #d2d2d7;
    padding: 2px 8px;
    border-radius: 6px;
    font-size: 0.75rem;
    cursor: pointer;
    color: #0071e3;
    transition: all 0.15s;
  }
  .add-btn:hover { background: #f0f7ff; }

  .error-msg {
    background: #ffebee;
    color: #c62828;
    font-size: 0.75rem;
    padding: 0.4rem 0.6rem;
    border-radius: 6px;
    margin-bottom: 0.5rem;
  }

  .loading {
    font-size: 0.8rem;
    color: #86868b;
    padding: 0.5rem 0;
  }

  .parent-link {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.5rem;
    background: #f5f5f7;
    border-radius: 6px;
    margin-bottom: 0.5rem;
    font-size: 0.8rem;
  }

  .parent-label {
    color: #86868b;
    font-weight: 600;
    font-size: 0.7rem;
    text-transform: uppercase;
  }

  .parent-title {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: #0071e3;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }
  .parent-title:hover { text-decoration: underline; }

  .mode-indicator {
    font-size: 0.75rem;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    margin-bottom: 0.5rem;
  }
  .mode-indicator.sequential { background: #e8f0fe; color: #0071e3; }
  .mode-indicator.parallel { background: #e8fef0; color: #34c759; }

  .create-form {
    background: #f5f5f7;
    border-radius: 8px;
    padding: 0.75rem;
    margin-bottom: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .form-input, .form-textarea {
    width: 100%;
    padding: 0.4rem 0.6rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.85rem;
    outline: none;
    font-family: inherit;
    resize: none;
  }
  .form-input:focus, .form-textarea:focus { border-color: #0071e3; }

  .form-actions { display: flex; justify-content: flex-end; }

  .create-submit {
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.35rem 0.75rem;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .create-submit:disabled { opacity: 0.5; cursor: not-allowed; }
  .create-submit:hover:not(:disabled) { background: #0077ed; }

  .subtask-label {
    font-size: 0.7rem;
    color: #86868b;
    margin: 0 0 0.25rem 0;
    font-weight: 600;
  }

  .empty-hint {
    font-size: 0.8rem;
    color: #b0b0b5;
    margin: 0 0 0.5rem 0;
    font-style: italic;
  }

  .subtask-item {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.2rem 0;
    font-size: 0.8rem;
  }

  .pos-num {
    font-size: 0.65rem;
    font-weight: 700;
    color: #0071e3;
    background: #e8f0fe;
    border-radius: 4px;
    padding: 0 4px;
    min-width: 16px;
    text-align: center;
    flex-shrink: 0;
  }

  .reorder-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.65rem;
    color: #86868b;
    padding: 0 3px;
    line-height: 1;
    flex-shrink: 0;
  }
  .reorder-btn:hover:not(:disabled) { background: #f5f5f7; color: #0071e3; }
  .reorder-btn:disabled { opacity: 0.3; cursor: not-allowed; }

  .subtask-status {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .subtask-status[data-status="pending"] { background: #86868b; }
  .subtask-status[data-status="in_progress"] { background: #0071e3; }
  .subtask-status[data-status="completed"] { background: #34c759; }
  .subtask-status[data-status="failed"] { background: #ff3b30; }
  .subtask-status[data-status="cancelled"] { background: #ff9500; }

  .subtask-title {
    flex: 1;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #1d1d1f;
    text-align: left;
  }
  .subtask-title:hover { color: #0071e3; }

  .remove-btn {
    background: none;
    border: none;
    color: #86868b;
    cursor: pointer;
    font-size: 1rem;
    padding: 0 4px;
    border-radius: 4px;
    line-height: 1;
  }
  .remove-btn:hover { background: #f5f5f7; color: #ff3b30; }
</style>
