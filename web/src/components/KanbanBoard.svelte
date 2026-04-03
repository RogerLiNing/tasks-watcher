<script>
  import { createEventDispatcher } from 'svelte';
  import TaskCard from './TaskCard.svelte';
  import { columns } from '../lib/stores.js';
  import { t } from '../lib/i18n/index.js';

  export let tasksByStatus = {};

  const dispatch = createEventDispatcher();

  $: defaultColumns = [
    { key: 'pending', label: $t('columns.pending'), color: '#86868b' },
    { key: 'in_progress', label: $t('columns.in_progress'), color: '#0071e3' },
    { key: 'completed', label: $t('columns.completed'), color: '#34c759' },
    { key: 'failed', label: $t('columns.failed'), color: '#ff3b30' },
    { key: 'cancelled', label: $t('columns.cancelled'), color: '#ff9500' },
  ];

  $: boardColumns = $columns.length > 0 ? $columns : defaultColumns;

  function handleDragStart(e, task) {
    e.dataTransfer.setData('taskId', task.id);
    e.dataTransfer.setData('fromStatus', task.status);
  }

  function handleDrop(e, targetStatus) {
    e.preventDefault();
    const taskId = e.dataTransfer.getData('taskId');
    const fromStatus = e.dataTransfer.getData('fromStatus');
    if (fromStatus === targetStatus) return;
    dispatch('statusChange', { id: taskId, status: targetStatus, reason: '' });
  }

  function handleDragOver(e) {
    e.preventDefault();
    e.currentTarget.classList.add('drag-over');
  }

  function handleDragLeave(e) {
    e.currentTarget.classList.remove('drag-over');
  }
</script>

<div class="kanban-board">
  {#each boardColumns as col (col.key || col.id)}
    <div
      class="column"
      role="group"
      aria-label={col.label}
      on:drop={(e) => handleDrop(e, col.key)}
      on:dragover={handleDragOver}
      on:dragleave={handleDragLeave}
    >
      <div class="column-header">
        <span class="column-dot" style="background:{col.color}"></span>
        <span class="column-label">{col.label}</span>
        <span class="column-count">{tasksByStatus[col.key]?.length || 0}</span>
      </div>
      <div class="column-cards">
        {#each (tasksByStatus[col.key] || []) as task (task.id)}
          <div
            class="task-drag-card"
            draggable="true"
            role="button"
            tabindex="0"
            on:dragstart={(e) => handleDragStart(e, task)}
            on:click={() => dispatch('openTask', task)}
            on:keydown={(e) => e.key === 'Enter' && dispatch('openTask', task)}
          >
            <TaskCard {task} />
          </div>
        {/each}
        {#if !tasksByStatus[col.key]?.length}
          <div class="empty-col">{$t('kanban.dropHere')}</div>
        {/if}
      </div>
    </div>
  {/each}
</div>

<style>
  .kanban-board {
    display: flex;
    gap: 1rem;
    height: 100%;
    min-height: 0;
  }

  .column {
    flex: 1;
    min-width: 180px;
    background: #f5f5f7;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    transition: background 0.15s;
  }

  .column-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid #e5e5ea;
  }

  .column-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .column-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: #1d1d1f;
    flex: 1;
  }

  .column-count {
    background: #e5e5ea;
    color: #6e6e73;
    font-size: 0.75rem;
    font-weight: 600;
    padding: 1px 7px;
    border-radius: 10px;
  }

  .column-cards {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .task-drag-card {
    cursor: pointer;
    outline: none;
  }
  .task-drag-card:focus-visible {
    outline: 2px solid #0071e3;
    border-radius: 8px;
  }

  .empty-col {
    color: #86868b;
    font-size: 0.8rem;
    text-align: center;
    padding: 2rem 1rem;
    border: 2px dashed #d2d2d7;
    border-radius: 8px;
  }
</style>
