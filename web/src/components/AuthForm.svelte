<script>
  import { t } from '../lib/i18n/index.js';
  import { api, currentUser, isAuthenticated } from '../lib/api.js';

  let mode = 'login'; // 'login' | 'register'
  let username = '';
  let password = '';
  let confirmPassword = '';
  let error = '';
  let loading = false;

  function switchMode() {
    mode = mode === 'login' ? 'register' : 'login';
    error = '';
    password = '';
    confirmPassword = '';
  }

  async function handleSubmit() {
    error = '';

    if (!username.trim() || !password) {
      error = $t('auth.errors.fillAll');
      return;
    }

    if (mode === 'register') {
      if (password !== confirmPassword) {
        error = $t('auth.errors.passwordMismatch');
        return;
      }
      if (password.length < 8) {
        error = $t('auth.errors.passwordTooShort');
        return;
      }
    }

    loading = true;
    try {
      let res;
      if (mode === 'login') {
        res = await api.login(username.trim(), password);
      } else {
        res = await api.register(username.trim(), password);
      }
      currentUser.set(res.user);
      isAuthenticated.set(true);
    } catch (e) {
      error = e.message;
    }
    loading = false;
  }
</script>

<div class="auth-screen">
  <div class="auth-card">
    <div class="auth-header">
      <h1>{$t('app.title')}</h1>
      <p>{mode === 'login' ? $t('auth.loginHint') : $t('auth.registerHint')}</p>
    </div>

    <form on:submit|preventDefault={handleSubmit}>
      <div class="field">
        <label for="auth-username">{$t('auth.username')}</label>
        <input
          id="auth-username"
          type="text"
          bind:value={username}
          placeholder={$t('auth.usernamePlaceholder')}
          autocomplete={mode === 'register' ? 'off' : 'username'}
          disabled={loading}
        />
      </div>

      <div class="field">
        <label for="auth-password">{$t('auth.password')}</label>
        <input
          id="auth-password"
          type="password"
          bind:value={password}
          placeholder={$t('auth.passwordPlaceholder')}
          autocomplete={mode === 'login' ? 'current-password' : 'new-password'}
          disabled={loading}
        />
      </div>

      {#if mode === 'register'}
        <div class="field">
          <label for="auth-confirm">{$t('auth.confirmPassword')}</label>
          <input
            id="auth-confirm"
            type="password"
            bind:value={confirmPassword}
            placeholder={$t('auth.confirmPasswordPlaceholder')}
            autocomplete="new-password"
            disabled={loading}
          />
        </div>
      {/if}

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <button type="submit" class="submit-btn" disabled={loading}>
        {#if loading}
          <span class="spinner-sm"></span>
        {/if}
        {mode === 'login' ? $t('auth.login') : $t('auth.register')}
      </button>
    </form>

    <div class="switch-mode">
      {#if mode === 'login'}
        <span>{$t('auth.noAccount')}</span>
        <button type="button" on:click={switchMode}>{$t('auth.register')}</button>
      {:else}
        <span>{$t('auth.hasAccount')}</span>
        <button type="button" on:click={switchMode}>{$t('auth.login')}</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .auth-screen {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100vh;
    background: linear-gradient(135deg, #f0f2f5 0%, #e8eaf0 100%);
  }

  .auth-card {
    background: white;
    border-radius: 16px;
    padding: 2.5rem;
    width: 400px;
    max-width: 95vw;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
  }

  .auth-header {
    text-align: center;
    margin-bottom: 2rem;
  }

  .auth-header h1 {
    font-size: 1.5rem;
    font-weight: 700;
    color: #1d1d1f;
    margin: 0 0 0.5rem;
  }

  .auth-header p {
    color: #6e6e73;
    font-size: 0.875rem;
    margin: 0;
  }

  .field {
    margin-bottom: 1rem;
  }

  .field label {
    display: block;
    font-size: 0.8rem;
    font-weight: 600;
    color: #6e6e73;
    margin-bottom: 0.35rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  .field input {
    width: 100%;
    padding: 0.65rem 0.85rem;
    border: 1px solid #d2d2d7;
    border-radius: 8px;
    font-size: 0.95rem;
    outline: none;
    transition: border-color 0.15s;
    box-sizing: border-box;
  }

  .field input:focus {
    border-color: #0071e3;
    box-shadow: 0 0 0 3px rgba(0, 113, 227, 0.15);
  }

  .field input:disabled {
    background: #f5f5f7;
    cursor: not-allowed;
  }

  .error {
    background: #fff0ee;
    color: #ff3b30;
    border-radius: 8px;
    padding: 0.5rem 0.75rem;
    font-size: 0.85rem;
    margin-bottom: 1rem;
    text-align: center;
  }

  .submit-btn {
    width: 100%;
    padding: 0.7rem;
    background: #0071e3;
    color: white;
    border: none;
    border-radius: 8px;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    transition: background 0.15s;
    margin-top: 0.5rem;
  }

  .submit-btn:hover:not(:disabled) {
    background: #0077ed;
  }

  .submit-btn:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .spinner-sm {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .switch-mode {
    text-align: center;
    margin-top: 1.25rem;
    font-size: 0.85rem;
    color: #6e6e73;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.4rem;
  }

  .switch-mode button {
    background: none;
    border: none;
    color: #0071e3;
    font-size: 0.85rem;
    cursor: pointer;
    font-weight: 600;
    padding: 0;
  }

  .switch-mode button:hover {
    text-decoration: underline;
  }
</style>
