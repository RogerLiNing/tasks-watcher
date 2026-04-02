<script>
  import { createEventDispatcher } from 'svelte';
  import TaskCard from './TaskCard.svelte';
  import { t } from '../lib/i18n/index.js';

  export let tasksByStatus = {};

  const dispatch = createEventDispatcher();

  const columns = [
    { key: 'pending', color: '#86868b' },
    { key: 'in_progress', color: '#0071e3' },
    { key: 'completed', color: '#34c759' },
    { key: 'failed', color: '#ff3b30' },
    { key: 'cancelled', color: '#ff9500' },
  ];

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
  {#each columns as col (col.key)}
    <div
      class="column"
      on:drop={(e) => handleDrop(e, col.key)}
      on:dragover={handleDragOver}
      on:dragleave={handleDragLeave}
    >
      <div class="column-header">
        <span class="column-dot" style="background:{col.color}"></span>
        <span class="column-label">{$t('columns.' + col.key)}</span>
        <span class="column-count">{tasksByStatus[col.key]?.length || 0}</span>
      </div>
      <div class="column-cards">
        {#each (tasksByStatus[col.key] || []) as task (task.id)}
          <div
            draggable="true"
            on:dragstart={(e) => handleDragStart(e, task)}
            on:click={() => dispatch('openTask', task)}
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

  .column.drag-over { background: #e8f0fe; }

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

  .empty-col {
    color: #86868b;
    font-size: 0.8rem;
    text-align: center;
    padding: 2rem 1rem;
    border: 2px dashed #d2d2d7;
    border-radius: 8px;
  }
</style>
