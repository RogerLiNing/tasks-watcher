<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from '../lib/i18n/index.js';
  import { api } from '../lib/api.js';
  import { tasks } from '../lib/stores.js';

  export let task;

  const dispatch = createEventDispatcher();

  let blockers = [];
  let dependents = [];
  let canStartResult = null;
  let loading = true;
  let addError = '';
  let newBlockerId = '';

  // All tasks available to add as blocker (exclude self, already-blocked, and those missing status)
  $: availableBlockers = $tasks.filter(t =>
    t.id !== task.id &&
    t.status != null &&
    t.status !== task.status &&
    !blockers.find(b => b.id === t.id)
  );

  async function loadDeps() {
    loading = true;
    addError = '';
    try {
      const [blockerRes, depRes, canRes] = await Promise.all([
        api.getBlockers(task.id),
        api.getDependents(task.id),
        api.canStart(task.id),
      ]);
      blockers = blockerRes.blockers || [];
      dependents = depRes.dependents || [];
      canStartResult = canRes;
    } catch (e) {
      addError = $t('common.errors.loadDeps');
    }
    loading = false;
  }

  async function addBlocker() {
    if (!newBlockerId) return;
    addError = '';
    try {
      await api.addBlocker(task.id, newBlockerId);
      newBlockerId = '';
      await loadDeps();
      dispatch('refresh');
    } catch (e) {
      try { addError = JSON.parse(e.message).error || e.message; }
      catch (_) { addError = e.message; }
    }
  }

  async function removeBlocker(blockerId) {
    addError = '';
    try {
      await api.removeBlocker(task.id, blockerId);
      await loadDeps();
      dispatch('refresh');
    } catch (e) {
      try { addError = JSON.parse(e.message).error || e.message; }
      catch (_) { addError = e.message; }
    }
  }

  function openTask(taskId) {
    dispatch('openTask', taskId);
  }

  // Load on mount
  $: if (task) loadDeps();
</script>

<div class="dep-panel">
  <div class="section-header">
    <span class="section-title">{$t('depPanel.title')}</span>
    {#if !loading}
      <span class="can-start-badge" class:blocked={!canStartResult?.can_start}>
        {canStartResult?.can_start ? $t('depPanel.canStart') : $t('depPanel.blocked')}
      </span>
    {/if}
  </div>

  {#if addError}
    <div class="error-msg">{addError}</div>
  {/if}

  {#if loading}
    <div class="loading">{$t('common.loading')}</div>
  {:else}
    <div class="dep-group">
      <p class="dep-label">{$t('depPanel.blockedBy')} ({blockers.length})</p>
      {#if blockers.length === 0}
        <p class="empty-hint">{$t('depPanel.noBlockers')}</p>
      {:else}
        {#each blockers as b (b.id)}
          <div class="dep-item">
            <span class="dep-status" data-status={b.status}></span>
            <button class="dep-title link-btn" on:click={() => openTask(b.id)}>{b.title}</button>
            <button class="remove-btn" on:click={() => removeBlocker(b.id)} title={$t('depPanel.removeBlocker')}>×</button>
          </div>
        {/each}
      {/if}
    </div>

    <div class="dep-group">
      <p class="dep-label">{$t('depPanel.blocking')} ({dependents.length})</p>
      {#if dependents.length === 0}
        <p class="empty-hint">{$t('depPanel.noDependents')}</p>
      {:else}
        {#each dependents as d (d.id)}
          <div class="dep-item">
            <span class="dep-status" data-status={d.status}></span>
            <button class="dep-title link-btn" on:click={() => openTask(d.id)}>{d.title}</button>
          </div>
        {/each}
      {/if}
    </div>

    {#if availableBlockers.length > 0}
      <div class="add-blocker">
        <select bind:value={newBlockerId} on:change={addBlocker}>
          <option value="">{$t('depPanel.addBlocker')}</option>
          {#each availableBlockers as t (t.id)}
            <option value={t.id}>[{t.status}] {t.title}</option>
          {/each}
        </select>
      </div>
    {/if}
  {/if}
</div>

<style>
  .dep-panel { padding: 0.5rem 0; }

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

  .can-start-badge {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 2px 8px;
    border-radius: 10px;
    background: #e8f5e9;
    color: #2e7d32;
  }

  .can-start-badge.blocked {
    background: #ffebee;
    color: #c62828;
  }

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

  .dep-group { margin-bottom: 0.75rem; }

  .dep-label {
    font-size: 0.7rem;
    font-weight: 600;
    color: #86868b;
    margin: 0 0 0.25rem 0;
    text-transform: uppercase;
  }

  .empty-hint {
    font-size: 0.8rem;
    color: #b0b0b5;
    margin: 0 0 0.5rem 0;
    font-style: italic;
  }

  .dep-item {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.25rem 0;
    font-size: 0.8rem;
  }

  .dep-status {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dep-status[data-status="pending"] { background: #86868b; }
  .dep-status[data-status="in_progress"] { background: #0071e3; }
  .dep-status[data-status="completed"] { background: #34c759; }
  .dep-status[data-status="failed"] { background: #ff3b30; }
  .dep-status[data-status="cancelled"] { background: #ff9500; }

  .dep-title {
    flex: 1;
    cursor: pointer;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #1d1d1f;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    text-align: left;
  }
  .dep-title:hover { color: #0071e3; }

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

  .add-blocker select {
    width: 100%;
    padding: 0.35rem 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    color: #1d1d1f;
    background: white;
    cursor: pointer;
    outline: none;
  }
  .add-blocker select:focus { border-color: #0071e3; }
</style>
