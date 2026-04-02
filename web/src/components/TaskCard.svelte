<script>
  import { t } from '../lib/i18n/index.js';

  const priorityColors = {
    low: '#34c759',
    medium: '#0071e3',
    high: '#ff9500',
    urgent: '#ff3b30',
  };

  const sourceIcons = {
    'claude-code': '🤖',
    'cursor': '📎',
    'manual': '👤',
  };

  const sourceColors = {
    'claude-code': '#7c3aed',
    'cursor': '#0891b2',
    'manual': '#86868b',
  };

  export let task;

  function formatTime(ts) {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    const now = new Date();
    const diff = now - d;
    if (diff < 60000) return $t('time.justNow');
    if (diff < 3600000) return $t('time.minutesAgo', { n: Math.floor(diff / 60000) });
    if (diff < 86400000) return $t('time.hoursAgo', { n: Math.floor(diff / 3600000) });
    return d.toLocaleDateString();
  }
</script>

<div class="task-card" class:urgent={task.priority === 'urgent'}>
  <div class="card-header">
    <span class="priority-dot" style="background:{priorityColors[task.priority] || '#86868b'}"></span>
    {#if task.assignee}
      <span class="assignee">{task.assignee}</span>
    {/if}
    {#if task.source && task.source !== 'manual'}
      <span class="source-badge" style="background:{sourceColors[task.source] || '#86868b'}">
        {sourceIcons[task.source] || '📌'} {task.source}
      </span>
    {/if}
  </div>
  <p class="task-title">{task.title}</p>
  {#if task.description}
    <p class="task-desc">{task.description}</p>
  {/if}
  <div class="card-footer">
    <span class="time">{formatTime(task.updated_at)}</span>
    {#if task.error_message}
      <span class="error-badge" title={task.error_message}>⚠</span>
    {/if}
  </div>
</div>

<style>
  .task-card {
    background: white;
    border-radius: 10px;
    padding: 0.75rem;
    cursor: pointer;
    box-shadow: 0 1px 3px rgba(0,0,0,0.08);
    transition: box-shadow 0.15s, transform 0.1s;
    border-left: 3px solid transparent;
    user-select: none;
  }

  .task-card:hover {
    box-shadow: 0 4px 12px rgba(0,0,0,0.12);
    transform: translateY(-1px);
  }

  .task-card.urgent { border-left-color: #ff3b30; }

  .card-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.4rem;
  }

  .priority-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .assignee {
    font-size: 0.7rem;
    color: #86868b;
    background: #f5f5f7;
    padding: 1px 5px;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 80px;
  }

  .source-badge {
    font-size: 0.65rem;
    color: white;
    padding: 1px 5px;
    border-radius: 4px;
    font-weight: 600;
    white-space: nowrap;
  }

  .task-title {
    font-size: 0.875rem;
    font-weight: 500;
    color: #1d1d1f;
    line-height: 1.4;
    margin-bottom: 0.25rem;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .task-desc {
    font-size: 0.75rem;
    color: #86868b;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 1;
    -webkit-box-orient: vertical;
    margin-bottom: 0.25rem;
  }

  .card-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 0.4rem;
  }

  .time { font-size: 0.7rem; color: #86868b; }
  .error-badge { font-size: 0.8rem; cursor: help; }
</style>
