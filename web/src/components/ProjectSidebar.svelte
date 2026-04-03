<script>
  import { createEventDispatcher } from 'svelte';
  import { api } from '../lib/api.js';
  import { t } from '../lib/i18n/index.js';

  export let projects = [];
  export let selectedId = '';

  const dispatch = createEventDispatcher();
  let newProjectName = '';
  let newProjectDesc = '';
  let showNewProject = false;
  let expandedId = '';
  let editDesc = '';
  let editRepoPath = '';
  let editingDesc = false;

  async function createProject() {
    if (!newProjectName.trim()) return;
    try {
      const p = await api.createProject({
        name: newProjectName.trim(),
        description: newProjectDesc.trim(),
      });
      dispatch('createProject', p);
      newProjectName = '';
      newProjectDesc = '';
      showNewProject = false;
    } catch (e) {
      console.error('Failed to create project', e);
    }
  }

  function toggleExpand(project) {
    if (expandedId === project.id) {
      expandedId = '';
      editingDesc = false;
    } else {
      expandedId = project.id;
      editDesc = project.description || '';
      editRepoPath = project.repo_path || '';
      editingDesc = false;
    }
  }

  function startEditDesc() {
    editingDesc = true;
  }

  async function saveDesc(project) {
    if (editDesc === (project.description || '') && editRepoPath === (project.repo_path || '')) {
      editingDesc = false;
      return;
    }
    try {
      const updated = await api.updateProject(project.id, { description: editDesc, repo_path: editRepoPath });
      dispatch('updateProject', updated);
      editingDesc = false;
    } catch (e) {
      console.error('Failed to update project', e);
    }
  }

  function cancelEdit() {
    const proj = projects.find(p => p.id === expandedId);
    editDesc = proj ? (proj.description || '') : '';
    editRepoPath = proj ? (proj.repo_path || '') : '';
    editingDesc = false;
  }

  async function deleteProject(project) {
    if (!confirm($t('sidebar.deleteProjectConfirm', { name: project.name }))) return;
    dispatch('deleteProject', project);
  }
</script>

<aside class="sidebar">
  <div class="sidebar-header">
    <h2>{$t('sidebar.projects')}</h2>
    <button class="add-btn" on:click={() => showNewProject = !showNewProject}>+</button>
  </div>

  {#if showNewProject}
    <div class="new-project-form">
      <input
        bind:value={newProjectName}
        placeholder={$t('sidebar.projectNamePlaceholder')}
        on:keydown={(e) => e.key === 'Enter' && createProject()}
      />
      <textarea
        class="desc-input"
        bind:value={newProjectDesc}
        placeholder={$t('sidebar.projectDescPlaceholder')}
        rows="2"
      ></textarea>
      <button class="create-btn" on:click={createProject}>{$t('sidebar.add')}</button>
    </div>
  {/if}

  <nav class="project-list">
    <button
      class="project-item"
      class:active={!selectedId}
      on:click={() => dispatch('select', '')}
    >
      <span class="project-icon">📋</span>
      <span class="project-name">{$t('sidebar.allTasks')}</span>
    </button>

    {#each projects as project (project.id)}
      <div class="project-wrapper">
        <button
          class="project-item"
          class:active={selectedId === project.id}
          class:expanded={expandedId === project.id}
          on:click={() => dispatch('select', project.id)}
        >
          <button
            class="expand-btn"
            on:click|stopPropagation={() => toggleExpand(project)}
            title={$t('sidebar.editDesc')}
          >
            <span class="chevron" class:open={expandedId === project.id}>▶</span>
          </button>
          <span class="project-icon">📁</span>
          <span class="project-name">{project.name}</span>
        </button>

        {#if expandedId === project.id}
          <div class="project-detail">
            {#if editingDesc}
              <textarea
                class="desc-edit"
                bind:value={editDesc}
                placeholder={$t('sidebar.projectDescPlaceholder')}
                rows="3"
              ></textarea>
              <label class="repo-input-label">
                {$t('sidebar.repoPath')}
                <input
                  class="repo-input"
                  bind:value={editRepoPath}
                  placeholder={$t('sidebar.repoPathPlaceholder')}
                />
              </label>
              <div class="desc-actions">
                <button class="save-btn" on:click={() => saveDesc(project)}>{$t('sidebar.save')}</button>
                <button class="cancel-btn" on:click={cancelEdit}>{$t('taskModal.cancel')}</button>
              </div>
            {:else}
              {#if project.description}
                <p class="desc-text">{project.description}</p>
              {/if}
              {#if project.repo_path}
                <p class="repo-path">📁 {project.repo_path}</p>
              {/if}
              <div class="detail-actions">
                <button class="edit-desc-btn" on:click={startEditDesc}>
                  {$t('sidebar.editDesc')}
                </button>
                <button class="delete-project-btn" on:click={() => deleteProject(project)}>
                  {$t('sidebar.deleteProject')}
                </button>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </nav>
</aside>

<style>
  .sidebar {
    width: 220px;
    background: white;
    border-right: 1px solid #e5e5ea;
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    overflow-y: auto;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 1rem 0.5rem;
  }

  .sidebar-header h2 { font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: #86868b; }

  .add-btn {
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    width: 24px;
    height: 24px;
    cursor: pointer;
    font-size: 0.9rem;
    color: #6e6e73;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .add-btn:hover { background: #f5f5f7; }

  .new-project-form {
    padding: 0.5rem 1rem 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .new-project-form input,
  .desc-input {
    width: 100%;
    padding: 0.4rem 0.6rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.85rem;
    outline: none;
    resize: none;
    font-family: inherit;
  }
  .new-project-form input:focus,
  .desc-input:focus { border-color: #0071e3; }

  .create-btn {
    padding: 0.4rem 0.75rem;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 0.85rem;
    cursor: pointer;
  }

  .project-list { padding: 0.25rem 0.5rem; }

  .project-item {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    padding: 0.5rem 0.5rem;
    border: none;
    background: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 0.9rem;
    color: #1d1d1f;
    text-align: left;
    transition: background 0.15s;
  }
  .project-item:hover { background: #f5f5f7; }
  .project-item.active { background: #e8f0fe; color: #0071e3; font-weight: 600; }

  .project-icon { font-size: 0.85rem; flex-shrink: 0; }
  .project-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

  .expand-btn {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    font-size: 0.6rem;
    color: #86868b;
    width: 16px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .chevron {
    display: inline-block;
    transition: transform 0.15s;
  }
  .chevron.open { transform: rotate(90deg); }

  .project-detail {
    padding: 0.4rem 0.5rem 0.5rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .desc-text {
    font-size: 0.8rem;
    color: #6e6e73;
    white-space: pre-wrap;
    margin: 0 0 0.25rem;
    line-height: 1.4;
  }

  .repo-path {
    font-size: 0.75rem;
    color: #86868b;
    margin: 0 0 0.4rem;
    font-family: monospace;
    word-break: break-all;
  }

  .repo-input-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #6e6e73;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    margin-bottom: 0.4rem;
  }

  .repo-input {
    width: 100%;
    padding: 0.3rem 0.5rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    font-family: monospace;
    outline: none;
  }
  .repo-input:focus { border-color: #0071e3; }

  .desc-edit {
    width: 100%;
    padding: 0.4rem;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    resize: none;
    font-family: inherit;
    outline: none;
  }
  .desc-edit:focus { border-color: #0071e3; }

  .desc-actions { display: flex; gap: 0.4rem; }

  .save-btn {
    padding: 0.25rem 0.6rem;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
  }

  .cancel-btn {
    padding: 0.25rem 0.6rem;
    background: none;
    border: 1px solid #d2d2d7;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
    color: #6e6e73;
  }

  .detail-actions { display: flex; gap: 0.5rem; align-items: center; }

  .edit-desc-btn {
    background: none;
    border: none;
    color: #0071e3;
    font-size: 0.75rem;
    cursor: pointer;
    padding: 0;
    text-align: left;
  }
  .edit-desc-btn:hover { text-decoration: underline; }

  .delete-project-btn {
    background: none;
    border: none;
    color: #ff3b30;
    font-size: 0.75rem;
    cursor: pointer;
    padding: 0;
  }
  .delete-project-btn:hover { text-decoration: underline; }
</style>
