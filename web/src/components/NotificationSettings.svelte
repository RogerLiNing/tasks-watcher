<script>
  import { t } from '../lib/i18n/index.js';
  import { api } from '../lib/api.js';

  let configs = {};
  let loading = true;
  let saving = false;
  let saved = false;

  // Email form
  let emailEnabled = false;
  let smtpHost = '';
  let smtpPort = 587;
  let smtpUsername = '';
  let smtpPassword = '';
  let fromAddress = '';
  let toAddressesStr = '';

  // macOS
  let macosEnabled = true;

  async function loadConfigs() {
    try {
      const data = await fetch('/api/notifications/configs', {
        headers: { 'Authorization': 'Bearer ' + localStorage.getItem('tasks_watcher_api_key') }
      }).then(r => r.json());

      configs = (data.configs || []).reduce((acc, c) => {
        acc[c.type] = c;
        return acc;
      }, {});

      if (configs.macos) {
        macosEnabled = configs.macos.enabled;
      }
      if (configs.email) {
        emailEnabled = configs.email.enabled;
        const ec = configs.email.config || {};
        smtpHost = ec.smtp_host || '';
        smtpPort = ec.smtp_port || 587;
        smtpUsername = ec.smtp_username || '';
        smtpPassword = ec.smtp_password || '';
        fromAddress = ec.from_address || '';
        toAddressesStr = (ec.to_addresses || []).join(', ');
      }
    } catch (e) {
      console.error('Failed to load configs', e);
    } finally {
      loading = false;
    }
  }

  async function saveMacos() {
    saving = true;
    try {
      await fetch('/api/notifications/configs', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + localStorage.getItem('tasks_watcher_api_key')
        },
        body: JSON.stringify({ type: 'macos', enabled: macosEnabled, config: {} })
      });
      showSaved();
    } catch (e) {
      console.error('Failed to save macOS config', e);
    } finally {
      saving = false;
    }
  }

  async function saveEmail() {
    saving = true;
    const toAddresses = toAddressesStr.split(',').map(s => s.trim()).filter(Boolean);
    try {
      await fetch('/api/notifications/configs', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + localStorage.getItem('tasks_watcher_api_key')
        },
        body: JSON.stringify({
          type: 'email',
          enabled: emailEnabled,
          config: {
            smtp_host: smtpHost,
            smtp_port: smtpPort,
            smtp_username: smtpUsername,
            smtp_password: smtpPassword,
            from_address: fromAddress,
            to_addresses: toAddresses,
          }
        })
      });
      showSaved();
    } catch (e) {
      console.error('Failed to save email config', e);
    } finally {
      saving = false;
    }
  }

  function showSaved() {
    saved = true;
    setTimeout(() => saved = false, 2000);
  }

  loadConfigs();
</script>

<div class="ns-content">
    <div class="panel-header">
      <h2>{$t('notificationSettings.title')}</h2>
      <div class="header-actions">
        {#if saved}
          <span class="saved-badge">{$t('notificationSettings.saved')}</span>
        {/if}
      </div>
    </div>

    {#if loading}
      <div class="loading">Loading...</div>
    {:else}
      <div class="panel-body">

        <!-- macOS Notifications -->
        <section class="notif-section">
          <div class="section-header">
            <h3>{$t('notificationSettings.macosSection') || 'macOS Notification Center'}</h3>
            <label class="toggle-label">
              <input type="checkbox" bind:checked={macosEnabled} />
              <span>{macosEnabled ? $t('common.on') : $t('common.off')}</span>
            </label>
          </div>
          <p class="section-desc">{$t('notificationSettings.macosDesc') || 'Show macOS system notifications when tasks start, complete, or fail.'}</p>
          <button class="save-btn" on:click={saveMacos} disabled={saving}>
            {saving ? $t('common.saving') : $t('notificationSettings.saveBtn')}
          </button>
        </section>

        <hr />

        <!-- Email Notifications -->
        <section class="notif-section">
          <div class="section-header">
            <h3>{$t('notificationSettings.emailSection')}</h3>
            <label class="toggle-label">
              <input type="checkbox" bind:checked={emailEnabled} />
              <span>{emailEnabled ? $t('common.on') : $t('common.off')}</span>
            </label>
          </div>

          <div class="form-grid">
            <div class="form-row">
              <label for="smtp-host">{$t('notificationSettings.smtpHost')}</label>
              <input id="smtp-host" type="text" bind:value={smtpHost} placeholder="smtp.gmail.com" />
            </div>
            <div class="form-row form-row--sm">
              <label for="smtp-port">{$t('notificationSettings.port')}</label>
              <input id="smtp-port" type="number" bind:value={smtpPort} placeholder="587" />
            </div>
            <div class="form-row">
              <label for="smtp-user">{$t('notificationSettings.smtpUser')}</label>
              <input id="smtp-user" type="text" bind:value={smtpUsername} placeholder="your@email.com" />
            </div>
            <div class="form-row">
              <label for="smtp-pass">{$t('notificationSettings.smtpPass')}</label>
              <input id="smtp-pass" type="password" bind:value={smtpPassword} placeholder="App password (not login password)" />
            </div>
            <div class="form-row">
              <label for="from-addr">{$t('notificationSettings.fromAddr')}</label>
              <input id="from-addr" type="email" bind:value={fromAddress} placeholder="tasks@example.com" />
            </div>
            <div class="form-row form-row--full">
              <label for="to-addrs">{$t('notificationSettings.toAddrs')} <span class="hint">{$t('notificationSettings.toAddrsHint')}</span></label>
              <input id="to-addrs" type="text" bind:value={toAddressesStr} placeholder="user1@example.com, user2@example.com" />
            </div>
          </div>

          <p class="section-tip">{@html $t('notificationSettings.tipGmail')}</p>

          <button class="save-btn" on:click={saveEmail} disabled={saving}>
            {saving ? $t('common.saving') : $t('notificationSettings.saveEmailBtn')}
          </button>
        </section>

      </div>
    {/if}
</div>

<style>
  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1.25rem 1.5rem 1rem;
  }

  .panel-header h2 { font-size: 1rem; font-weight: 600; margin: 0; }

  .header-actions { display: flex; align-items: center; gap: 0.75rem; }

  .saved-badge {
    background: #34c759;
    color: white;
    font-size: 0.75rem;
    padding: 2px 10px;
    border-radius: 12px;
    font-weight: 600;
  }

  .panel-body { padding: 0 1.5rem 1.5rem; }

  .loading {
    text-align: center;
    color: #86868b;
    padding: 3rem 1.5rem;
  }

  hr {
    border: none;
    border-top: 1px solid #e5e5ea;
    margin: 1.5rem 0;
  }

  .notif-section { display: flex; flex-direction: column; gap: 1rem; }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section-header h3 { font-size: 1rem; font-weight: 600; }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
    font-size: 0.85rem;
    color: #6e6e73;
  }

  .toggle-label input[type="checkbox"] {
    width: 18px;
    height: 18px;
    accent-color: #0071e3;
  }

  .section-desc { font-size: 0.85rem; color: #6e6e73; margin: 0; }

  .section-tip {
    font-size: 0.8rem;
    color: #6e6e73;
    background: #f5f5f7;
    padding: 0.75rem;
    border-radius: 8px;
    margin: 0;
  }

  .section-tip :global(a) { color: #0071e3; }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  .form-row {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .form-row--full { grid-column: 1 / -1; }
  .form-row--sm { }

  .form-row label {
    font-size: 0.8rem;
    font-weight: 600;
    color: #6e6e73;
  }

  .form-row .hint { font-weight: 400; color: #86868b; }

  .form-row input {
    padding: 0.5rem 0.75rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.9rem;
    outline: none;
    width: 100%;
  }

  .form-row input:focus { border-color: #0071e3; }

  .save-btn {
    align-self: flex-start;
    padding: 0.5rem 1.5rem;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 0.9rem;
    cursor: pointer;
    font-weight: 600;
  }

  .save-btn:hover { background: #0077ed; }
  .save-btn:disabled { opacity: 0.6; cursor: not-allowed; }
</style>
