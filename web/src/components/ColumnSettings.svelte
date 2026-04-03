<script>
  import { t } from '../lib/i18n/index.js';
  import { api } from '../lib/api.js';
  import { columns } from '../lib/stores.js';

  let cols = [];
  let loading = true;
  let error = '';
  let showCreate = false;
  let newLabel = '';
  let newColor = '#86868b';
  let creating = false;

  // Editing state: col id -> {label, color}
  let editing = {};

  const defaultColors = [
    '#86868b', '#0071e3', '#34c759', '#ff3b30', '#ff9500',
    '#af52de', '#5856d6', '#ff2d55', '#00c7be', '#64d2ff',
  ];

  async function load() {
    loading = true;
    error = '';
    try {
      const res = await api.listColumns();
      cols = res.columns || [];
      columns.set(cols);
    } catch (e) {
      error = $t('common.errors.loadColumns');
    }
    loading = false;
  }

  async function createCol() {
    if (!newLabel.trim()) return;
    creating = true;
    error = '';
    try {
      await api.createColumn({ label: newLabel.trim(), color: newColor });
      newLabel = '';
      newColor = '#86868b';
      showCreate = false;
      await load();
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
    creating = false;
  }

  function startEdit(col) {
    editing = { ...editing, [col.id]: { label: col.label, color: col.color } };
  }

  async function saveEdit(col) {
    const vals = editing[col.id];
    if (!vals || !vals.label.trim()) return;
    try {
      await api.updateColumn(col.id, { label: vals.label, color: vals.color });
      delete editing[col.id];
      editing = { ...editing };
      await load();
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
  }

  function cancelEdit(id) {
    delete editing[id];
    editing = { ...editing };
  }

  async function deleteCol(col) {
    if (!confirm($t('columnSettings.deleteConfirm', { name: col.label }))) return;
    try {
      await api.deleteColumn(col.id);
      await load();
    } catch (e) {
      try { error = JSON.parse(e.message).error || e.message; }
      catch (_) { error = e.message; }
    }
  }

  // Load on mount
  load();
</script>

<div class="cs-wrapper">
  <div class="settings-header">
    <h3>{$t('columnSettings.title')}</h3>
    <button class="add-btn" on:click={() => showCreate = !showCreate}>
      {showCreate ? $t('columnSettings.cancel') : $t('columnSettings.addColumn')}
    </button>
  </div>

  {#if error}
    <div class="error-msg">{error}</div>
  {/if}

  {#if showCreate}
    <div class="create-form">
      <div class="form-row">
        <label>
          {$t('columnSettings.label')}
          <input bind:value={newLabel} placeholder={$t('columnSettings.labelPlaceholder')} on:keydown={(e) => e.key === 'Enter' && createCol()} />
        </label>
        <label>
          {$t('columnSettings.color')}
          <div class="color-row">
            {#each defaultColors as c}
              <button
                class="color-swatch"
                class:selected={newColor === c}
                style="background:{c}"
                on:click={() => newColor = c}
                title={c}
              ></button>
            {/each}
          </div>
        </label>
      </div>
      <div class="form-actions">
        <button class="create-submit" on:click={createCol} disabled={creating || !newLabel.trim()}>
          {creating ? $t('columnSettings.creating') : $t('columnSettings.addColumn')}
        </button>
      </div>
    </div>
  {/if}

  {#if loading}
    <p class="loading">{$t('common.loading')}</p>
  {:else}
    <div class="col-list">
      {#each cols as col (col.id)}
        <div class="col-item">
          <div class="col-color" style="background:{col.color}"></div>
          {#if editing[col.id]}
            <div class="col-edit">
              <input bind:value={editing[col.id].label} on:keydown={(e) => e.key === 'Enter' && saveEdit(col)} />
              <div class="color-row">
                {#each defaultColors as c}
                  <button
                    class="color-swatch sm"
                    class:selected={editing[col.id].color === c}
                    style="background:{c}"
                    on:click={() => editing[col.id] = { ...editing[col.id], color: c }}
                  ></button>
                {/each}
              </div>
              <div class="edit-actions">
                <button class="save-btn" on:click={() => saveEdit(col)}>{$t('columnSettings.save')}</button>
                <button class="cancel-btn" on:click={() => cancelEdit(col.id)}>{$t('columnSettings.cancel')}</button>
              </div>
            </div>
          {:else}
            <div class="col-info">
              <span class="col-label">{col.label}</span>
              <span class="col-key">{col.key}</span>
            </div>
            <div class="col-actions">
              <button class="icon-btn" on:click={() => startEdit(col)} title={$t('columnSettings.editColumn')}>✏</button>
              <button class="icon-btn delete" on:click={() => deleteCol(col)} title={$t('columnSettings.deleteColumn')}>🗑</button>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .cs-wrapper { padding: 0 1.5rem 1.5rem; }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
  }
  .settings-header h3 { font-size: 1rem; font-weight: 600; margin: 0; }

  .add-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    padding: 0.3rem 0.75rem;
    font-size: 0.8rem;
    cursor: pointer;
    color: #0071e3;
    transition: all 0.15s;
  }
  .add-btn:hover { background: #f0f7ff; }

  .error-msg {
    background: #ffebee;
    color: #c62828;
    font-size: 0.8rem;
    padding: 0.4rem 0.6rem;
    border-radius: 6px;
    margin-bottom: 0.5rem;
  }

  .create-form {
    background: #f5f5f7;
    border-radius: 10px;
    padding: 0.75rem;
    margin-bottom: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .form-row { display: flex; flex-direction: column; gap: 0.5rem; }
  label { font-size: 0.75rem; font-weight: 600; color: #6e6e73; display: flex; flex-direction: column; gap: 0.2rem; }
  label input {
    padding: 0.4rem 0.6rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.85rem;
    outline: none;
  }
  label input:focus { border-color: #0071e3; }

  .color-row { display: flex; gap: 4px; flex-wrap: wrap; margin-top: 0.25rem; }
  .color-swatch {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: transform 0.1s;
  }
  .color-swatch.sm { width: 16px; height: 16px; }
  .color-swatch.selected { border-color: #1d1d1f; transform: scale(1.2); }
  .color-swatch:hover { transform: scale(1.15); }

  .form-actions { display: flex; justify-content: flex-end; }
  .create-submit {
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.4rem 0.75rem;
    font-size: 0.85rem;
    cursor: pointer;
  }
  .create-submit:disabled { opacity: 0.5; cursor: not-allowed; }
  .create-submit:hover:not(:disabled) { background: #0077ed; }

  .loading { font-size: 0.85rem; color: #86868b; padding: 1rem 0; }

  .col-list { display: flex; flex-direction: column; gap: 0.4rem; }

  .col-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem;
    border-radius: 8px;
    border: 1px solid #e5e5ea;
    background: white;
    transition: border-color 0.15s;
  }
  .col-item:hover { border-color: #d2d2d7; }

  .col-color {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .col-info {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }
  .col-label { font-size: 0.875rem; font-weight: 500; color: #1d1d1f; }
  .col-key { font-size: 0.7rem; color: #86868b; font-family: monospace; }

  .col-actions { display: flex; gap: 0.25rem; opacity: 0; transition: opacity 0.15s; }
  .col-item:hover .col-actions { opacity: 1; }

  .icon-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 0.85rem;
    padding: 2px 4px;
    border-radius: 4px;
  }
  .icon-btn:hover { background: #f5f5f7; }
  .icon-btn.delete:hover { background: #fff0ee; }

  .col-edit {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .col-edit input {
    padding: 0.3rem 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.85rem;
    outline: none;
  }
  .col-edit input:focus { border-color: #0071e3; }

  .edit-actions { display: flex; gap: 0.4rem; }
  .save-btn {
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    padding: 0.25rem 0.6rem;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .cancel-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    padding: 0.25rem 0.6rem;
    font-size: 0.75rem;
    cursor: pointer;
    color: #6e6e73;
  }
</style>
