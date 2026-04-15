<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { t } from '../lib/i18n/index.js';
  import { api, currentUser } from '../lib/api.js';

  export let task;

  const dispatch = createEventDispatcher();

  let comments = [];
  let loading = true;
  let error = '';

  // New comment form
  let newContent = '';
  let posting = false;

  // Editing state
  let editingId = '';
  let editContent = '';
  let saving = false;

  onMount(() => {
    loadComments();
  });

  async function loadComments() {
    loading = true;
    error = '';
    try {
      const res = await api.getComments(task.id);
      comments = res.comments || [];
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }

  async function submitComment() {
    if (!newContent.trim() || posting) return;
    posting = true;
    error = '';
    try {
      const created = await api.createComment(task.id, { content: newContent.trim() });
      comments = [...comments, created];
      newContent = '';
      dispatch('refresh');
    } catch (e) {
      error = e.message;
    }
    posting = false;
  }

  function startEdit(comment) {
    editingId = comment.id;
    editContent = comment.content;
  }

  function cancelEdit() {
    editingId = '';
    editContent = '';
  }

  async function saveEdit(comment) {
    if (!editContent.trim() || saving) return;
    saving = true;
    error = '';
    try {
      const updated = await api.updateComment(task.id, comment.id, {
        content: editContent.trim(),
      });
      comments = comments.map(c => c.id === updated.id ? updated : c);
      cancelEdit();
      dispatch('refresh');
    } catch (e) {
      error = e.message;
    }
    saving = false;
  }

  async function deleteComment(comment) {
    if (!confirm($t('commentPanel.deleteConfirm'))) return;
    try {
      await api.deleteComment(task.id, comment.id);
      comments = comments.filter(c => c.id !== comment.id);
      dispatch('refresh');
    } catch (e) {
      error = e.message;
    }
  }

  function formatTime(ts) {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    const now = new Date();
    const diff = Math.floor((now - d) / 1000);
    if (diff < 60) return $t('time.justNow');
    if (diff < 3600) return $t('time.minutesAgo').replace('{n}', Math.floor(diff / 60));
    if (diff < 86400) return $t('time.hoursAgo').replace('{n}', Math.floor(diff / 3600));
    return d.toLocaleDateString();
  }

  function isOwnComment(comment) {
    return $currentUser && comment.author === $currentUser.id;
  }
</script>

<div class="comment-panel">
  {#if loading}
    <div class="loading">{$t('common.loading')}</div>
  {:else if error}
    <div class="error">{error}</div>
  {:else}
    {#if comments.length === 0}
      <div class="empty">{$t('commentPanel.empty')}</div>
    {:else}
      <div class="comment-list">
        {#each comments as comment (comment.id)}
          <div class="comment">
            <div class="comment-header">
              <span class="comment-author">{comment.author_username || comment.author || '—'}</span>
              <span class="comment-time">{formatTime(comment.created_at)}</span>
              {#if isOwnComment(comment)}
                <button class="icon-btn-sm" on:click={() => startEdit(comment)} title={$t('commentPanel.edit')}>✏</button>
                <button class="icon-btn-sm danger" on:click={() => deleteComment(comment)} title={$t('commentPanel.delete')}>🗑</button>
              {/if}
            </div>
            {#if editingId === comment.id}
              <div class="edit-form">
                <textarea
                  class="edit-textarea"
                  bind:value={editContent}
                  placeholder={$t('commentPanel.editPlaceholder')}
                  rows="3"
                ></textarea>
                <div class="edit-actions">
                  <button class="save-btn" on:click={() => saveEdit(comment)} disabled={saving}>
                    {saving ? $t('commentPanel.saving') : $t('taskModal.save')}
                  </button>
                  <button class="cancel-btn-sm" on:click={cancelEdit}>{$t('commentPanel.cancel')}</button>
                </div>
              </div>
            {:else}
              <p class="comment-content">{comment.content}</p>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <div class="add-form">
      <textarea
        class="comment-textarea"
        bind:value={newContent}
        placeholder={$t('commentPanel.placeholder')}
        rows="3"
        on:keydown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            submitComment();
          }
        }}
      ></textarea>
      <button class="post-btn" on:click={submitComment} disabled={posting || !newContent.trim()}>
        {posting ? $t('commentPanel.saving') : $t('commentPanel.add')}
      </button>
    </div>
  {/if}
</div>

<style>
  .comment-panel {
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .loading, .error, .empty {
    text-align: center;
    color: #86868b;
    font-size: 0.85rem;
    padding: 2rem 0;
  }

  .error { color: #ff3b30; }

  .comment-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }

  .comment {
    background: #f5f5f7;
    border-radius: 10px;
    padding: 0.75rem;
  }

  .comment-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.4rem;
  }

  .comment-author {
    font-weight: 600;
    font-size: 0.8rem;
    color: #1d1d1f;
  }

  .comment-time {
    font-size: 0.75rem;
    color: #86868b;
    flex: 1;
  }

  .comment-content {
    margin: 0;
    font-size: 0.875rem;
    color: #1d1d1f;
    white-space: pre-wrap;
    line-height: 1.5;
  }

  .icon-btn-sm {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.75rem;
    padding: 2px 4px;
    border-radius: 4px;
    color: #86868b;
    transition: color 0.15s;
  }

  .icon-btn-sm:hover { color: #1d1d1f; background: #e5e5ea; }
  .icon-btn-sm.danger:hover { color: #ff3b30; }

  .edit-form { display: flex; flex-direction: column; gap: 0.5rem; }

  .edit-textarea {
    width: 100%;
    padding: 0.4rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.875rem;
    resize: vertical;
    outline: none;
    font-family: inherit;
  }

  .edit-textarea:focus { border-color: #0071e3; }

  .edit-actions { display: flex; gap: 0.4rem; }

  .save-btn {
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.3rem 0.75rem;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .save-btn:disabled { opacity: 0.6; cursor: default; }

  .cancel-btn-sm {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    padding: 0.3rem 0.75rem;
    font-size: 0.8rem;
    cursor: pointer;
    color: #6e6e73;
  }

  .add-form {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    border-top: 1px solid #e5e5ea;
    padding-top: 0.75rem;
  }

  .comment-textarea {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.875rem;
    resize: vertical;
    outline: none;
    font-family: inherit;
    line-height: 1.5;
  }

  .comment-textarea:focus { border-color: #0071e3; }

  .post-btn {
    align-self: flex-end;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 8px;
    padding: 0.4rem 1rem;
    font-size: 0.85rem;
    cursor: pointer;
    font-weight: 600;
  }

  .post-btn:disabled { opacity: 0.5; cursor: default; }
  .post-btn:hover:not(:disabled) { background: #0077ed; }
</style>
