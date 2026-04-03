<script>
  import { createEventDispatcher } from 'svelte';
  import { t } from '../lib/i18n/index.js';

  export let notifications = [];
  const dispatch = createEventDispatcher();

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

  function iconForType(type) {
    switch (type) {
      case 'task.started': return '▶️';
      case 'task.completed': return '✅';
      case 'task.failed': return '❌';
      case 'task.cancelled': return '○';
      case 'task.created': return '🆕';
      case 'task.updated': return '✏️';
      default: return '📌';
    }
  }
</script>

<button class="panel-backdrop" on:click={() => dispatch('close')} aria-label={$t('notifications.close')}>
</button>
<aside class="panel">
    <div class="panel-header">
      <h2>{$t('notifications.title')}</h2>
      <div class="panel-actions">
        {#if notifications.length > 0}
          <button class="mark-read-btn" on:click={() => dispatch('markAllRead')}>{$t('notifications.markAllRead')}</button>
        {/if}
        <button class="close-btn" on:click={() => dispatch('close')}>×</button>
      </div>
    </div>

    <div class="notif-list">
      {#if notifications.length === 0}
        <div class="empty">{$t('notifications.empty')}</div>
      {:else}
        {#each notifications as n (n.id)}
          <div class="notif-item" class:unread={!n.read}>
            <span class="notif-icon">{iconForType(n.type)}</span>
            <div class="notif-content">
              <p class="notif-msg">{n.message}</p>
              <span class="notif-time">{formatTime(n.created_at)}</span>
            </div>
          </div>
        {/each}
      {/if}
    </div>
  </aside>


<style>
  .panel-backdrop {
    position: fixed;
    inset: 0;
    z-index: 199;
    border: none;
    padding: 0;
    background: transparent;
    cursor: pointer;
  }

  .panel {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: 360px;
    background: white;
    box-shadow: -4px 0 20px rgba(0,0,0,0.1);
    display: flex;
    flex-direction: column;
    z-index: 200;
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid #e5e5ea;
    flex-shrink: 0;
  }

  .panel-header h2 { font-size: 1rem; font-weight: 600; }

  .panel-actions { display: flex; align-items: center; gap: 0.5rem; }

  .mark-read-btn {
    background: none;
    border: none;
    color: #0071e3;
    font-size: 0.8rem;
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    border-radius: 6px;
  }
  .mark-read-btn:hover { background: #e8f0fe; }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: #86868b;
    padding: 0 0.5rem;
    line-height: 1;
  }

  .notif-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
  }

  .empty {
    text-align: center;
    color: #86868b;
    padding: 3rem 1rem;
    font-size: 0.9rem;
  }

  .notif-item {
    display: flex;
    gap: 0.75rem;
    padding: 0.75rem;
    border-radius: 10px;
    transition: background 0.15s;
  }
  .notif-item:hover { background: #f5f5f7; }
  .notif-item.unread { background: #f0f7ff; }
  .notif-item.unread:hover { background: #e8f0fe; }

  .notif-icon { font-size: 1.1rem; flex-shrink: 0; padding-top: 2px; }

  .notif-content { flex: 1; min-width: 0; }

  .notif-msg { font-size: 0.875rem; color: #1d1d1f; margin-bottom: 0.2rem; }

  .notif-time { font-size: 0.75rem; color: #86868b; }
</style>
