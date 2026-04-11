<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from '../lib/i18n/index.js';

  export let tasks = [];
  export let projects = [];

  const dispatch = createEventDispatcher();

  // Sort state
  let sortKey = 'created_at';
  let sortDir = 'desc'; // 'asc' | 'desc'

  const priorityOrder = { urgent: 0, high: 1, medium: 2, low: 3 };
  const statusOrder = { in_progress: 0, pending: 1, failed: 2, cancelled: 3, completed: 4 };

  function toggleSort(key) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = 'desc';
    }
  }

  function getSortValue(task, key) {
    switch (key) {
      case 'title': return task.title || '';
      case 'status': return statusOrder[task.status] ?? 99;
      case 'priority': return priorityOrder[task.priority] ?? 99;
      case 'assignee': return task.assignee || '';
      case 'source': return task.source || '';
      case 'project_id': return getProjectName(task.project_id) || '';
      case 'created_at': return task.created_at || 0;
      case 'updated_at': return task.updated_at || 0;
      default: return '';
    }
  }

  $: sortedTasks = [...tasks].sort((a, b) => {
    const av = getSortValue(a, sortKey);
    const bv = getSortValue(b, sortKey);
    let cmp = 0;
    if (typeof av === 'number') {
      cmp = av - bv;
    } else {
      cmp = String(av).localeCompare(String(bv));
    }
    return sortDir === 'asc' ? cmp : -cmp;
  });

  function getProjectName(projectId) {
    const p = projects.find(p => p.id === projectId);
    return p ? p.name : '';
  }

  function formatTime(ts) {
    if (!ts) return '—';
    const d = new Date(ts * 1000);
    const now = Date.now();
    const diff = Math.floor((now - d.getTime()) / 1000);
    if (diff < 60) return $t('time.justNow');
    if (diff < 3600) return $t('time.minutesAgo').replace('{n}', Math.floor(diff / 60));
    if (diff < 86400) return $t('time.hoursAgo').replace('{n}', Math.floor(diff / 3600));
    return d.toLocaleDateString();
  }

  const statusColors = {
    pending: { bg: '#f5f5f7', text: '#86868b' },
    in_progress: { bg: '#e8f0fe', text: '#0071e3' },
    completed: { bg: '#e8f5e9', text: '#34c759' },
    failed: { bg: '#fde8e8', text: '#ff3b30' },
    cancelled: { bg: '#fff3e0', text: '#ff9500' },
  };

  const priorityColors = {
    low: { bg: '#e8f5e9', text: '#34c759' },
    medium: { bg: '#e8f0fe', text: '#0071e3' },
    high: { bg: '#fff3e0', text: '#ff9500' },
    urgent: { bg: '#fde8e8', text: '#ff3b30' },
  };

  const sourceIcons = {
    'claude-code': '🤖',
    'cursor': '📎',
    'manual': '👤',
  };

  function handleStatusChange(taskId, status) {
    dispatch('statusChange', { id: taskId, status, reason: '' });
  }

  const columns = [
    { key: 'title', labelKey: 'table.title' },
    { key: 'status', labelKey: 'table.status' },
    { key: 'priority', labelKey: 'table.priority' },
    { key: 'assignee', labelKey: 'table.assignee' },
    { key: 'source', labelKey: 'table.source' },
    { key: 'project_id', labelKey: 'table.project' },
    { key: 'created_at', labelKey: 'table.created' },
    { key: 'updated_at', labelKey: 'table.updated' },
  ];
</script>

<div class="table-view">
  {#if tasks.length === 0}
    <div class="empty-state">{$t('table.empty')}</div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            {#each columns as col}
              <th
                class="sortable"
                class:sorted={sortKey === col.key}
                on:click={() => toggleSort(col.key)}
              >
                <span>{$t(col.labelKey)}</span>
                <span class="sort-icon">
                  {#if sortKey === col.key}
                    {sortDir === 'asc' ? '↑' : '↓'}
                  {:else}
                    <span class="sort-neutral">⇅</span>
                  {/if}
                </span>
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each sortedTasks as task (task.id)}
            <tr on:click={() => dispatch('openTask', task)} class="task-row">
              <td class="title-cell">
                <span class="title-text">{task.title}</span>
                {#if task.task_mode === 'sequential'}
                  <span class="mode-badge seq" title={$t('taskCard.modeTooltipSequential')}>🔗</span>
                {:else if task.task_mode === 'parallel'}
                  <span class="mode-badge par" title={$t('taskCard.modeTooltipParallel')}>⚡</span>
                {/if}
              </td>
              <td>
                <select
                  class="status-badge"
                  style="background:{statusColors[task.status]?.bg || '#f5f5f7'}; color:{statusColors[task.status]?.text || '#86868b'}"
                  value={task.status}
                  on:click|stopPropagation
                  on:change={(e) => handleStatusChange(task.id, e.target.value)}
                >
                  <option value="pending">{$t('columns.pending')}</option>
                  <option value="in_progress">{$t('columns.in_progress')}</option>
                  <option value="completed">{$t('columns.completed')}</option>
                  <option value="failed">{$t('columns.failed')}</option>
                  <option value="cancelled">{$t('columns.cancelled')}</option>
                </select>
              </td>
              <td>
                <span
                  class="priority-badge"
                  style="background:{priorityColors[task.priority]?.bg || '#f5f5f7'}; color:{priorityColors[task.priority]?.text || '#86868b'}"
                >
                  {$t(`table.priority${capitalize(task.priority)}`)}
                </span>
              </td>
              <td class="assignee-cell">
                {task.assignee || '—'}
              </td>
              <td>
                {#if task.source && sourceIcons[task.source]}
                  <span title={task.source}>{sourceIcons[task.source]}</span>
                {:else}
                  {task.source || '—'}
                {/if}
              </td>
              <td class="project-cell">{getProjectName(task.project_id) || '—'}</td>
              <td class="time-cell">{formatTime(task.created_at)}</td>
              <td class="time-cell">{formatTime(task.updated_at)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<script context="module">
  function capitalize(s) {
    if (!s) return '';
    return s.charAt(0).toUpperCase() + s.slice(1);
  }
</script>

<style>
  .table-view {
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .table-wrapper {
    overflow: auto;
    flex: 1;
    border-radius: 12px;
    background: white;
    box-shadow: 0 1px 4px rgba(0,0,0,0.08);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
  }

  thead {
    position: sticky;
    top: 0;
    z-index: 1;
    background: #f5f5f7;
  }

  th {
    padding: 0.6rem 0.75rem;
    text-align: left;
    font-weight: 600;
    color: #6e6e73;
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid #e5e5ea;
    white-space: nowrap;
    user-select: none;
  }

  th.sortable {
    cursor: pointer;
  }

  th.sortable:hover {
    color: #1d1d1f;
  }

  th.sorted {
    color: #0071e3;
  }

  .sort-icon {
    margin-left: 0.25rem;
    font-size: 0.75rem;
  }

  .sort-neutral {
    opacity: 0.3;
  }

  tbody tr {
    border-bottom: 1px solid #f0f0f5;
    cursor: pointer;
    transition: background 0.1s;
  }

  tbody tr:last-child {
    border-bottom: none;
  }

  tbody tr:hover {
    background: #f5f5f7;
  }

  td {
    padding: 0.6rem 0.75rem;
    color: #1d1d1f;
    vertical-align: middle;
  }

  .title-cell {
    max-width: 280px;
  }

  .title-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: inline-block;
    max-width: 240px;
    vertical-align: middle;
  }

  .mode-badge {
    font-size: 0.7rem;
    margin-left: 0.3rem;
    vertical-align: middle;
  }

  .status-badge {
    padding: 2px 8px;
    border-radius: 10px;
    border: none;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    outline: none;
    font-family: inherit;
    transition: opacity 0.15s;
  }

  .status-badge:hover {
    opacity: 0.8;
  }

  .priority-badge {
    padding: 2px 8px;
    border-radius: 10px;
    font-size: 0.75rem;
    font-weight: 600;
    white-space: nowrap;
  }

  .assignee-cell {
    color: #6e6e73;
    white-space: nowrap;
  }

  .project-cell {
    color: #6e6e73;
    white-space: nowrap;
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .time-cell {
    color: #86868b;
    font-size: 0.8rem;
    white-space: nowrap;
  }

  .empty-state {
    text-align: center;
    padding: 3rem;
    color: #86868b;
    font-size: 0.9rem;
  }
</style>
