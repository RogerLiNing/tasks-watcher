<script>
  import { createEventDispatcher } from 'svelte';
  import { api } from '../lib/api.js';
  import { t } from '../lib/i18n/index.js';

  export let projects = [];

  const dispatch = createEventDispatcher();
  let title = '';
  let projectName = '';
  let priority = 'medium';

  async function createTask() {
    if (!title.trim()) return;
    const task = { title: title.trim(), priority };
    if (projectName) {
      task.project_name = projectName;
    }
    dispatch('create', task);
    title = '';
  }
</script>

<div class="quick-create">
  <input
    bind:value={title}
    placeholder={$t('quickCreate.placeholder')}
    on:keydown={(e) => e.key === 'Enter' && createTask()}
  />
  <select bind:value={projectName}>
    <option value="">{$t('quickCreate.projectSelect')}</option>
    {#each projects as p (p.id)}
      <option value={p.name}>{p.name}</option>
    {/each}
  </select>
  <select bind:value={priority}>
    <option value="low">{$t('quickCreate.low')}</option>
    <option value="medium">{$t('quickCreate.med')}</option>
    <option value="high">{$t('quickCreate.high')}</option>
    <option value="urgent">{$t('quickCreate.urgent')}</option>
  </select>
</div>

<style>
  .quick-create {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .quick-create input {
    flex: 1;
    padding: 0.5rem 0.75rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.9rem;
    outline: none;
    transition: border-color 0.15s, box-shadow 0.15s;
  }
  .quick-create input:focus {
    border-color: #0071e3;
    box-shadow: 0 0 0 3px rgba(0,113,227,0.15);
  }

  .quick-create select {
    padding: 0.5rem 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.85rem;
    background: white;
    outline: none;
    cursor: pointer;
  }
</style>
