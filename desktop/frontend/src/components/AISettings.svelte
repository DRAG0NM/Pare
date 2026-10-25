<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type AISettingsView } from '../lib/api'
  import { appearance, setTheme, type Theme } from '../lib/theme.svelte'
  import { i18n, locales, setLocale, t, type Locale } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'

  const themes: { id: Theme; label: string }[] = [
    { id: 'system', label: '跟随系统' },
    { id: 'light', label: '浅色' },
    { id: 'dark', label: '深色' },
  ]

  let { onClose, intro = '' }: { onClose: () => void; intro?: string } = $props()

  let loaded = $state<AISettingsView | null>(null)
  let provider = $state('')
  let baseUrl = $state('')
  let model = $state('')
  let apiKey = $state('')
  let clearKey = $state(false)
  let testing = $state(false)
  let result = $state<{ ok: boolean; text: string } | null>(null)
  let error = $state('')
  let exporting = $state(false)
  let exported = $state('')
  let exportError = $state('')

  async function exportLog() {
    exporting = true
    exportError = ''
    try {
      const path = await backend.ExportLog()
      if (path) exported = path
    } catch (e) {
      exportError = String(e).replace(/^Error:\s*/, '')
    } finally {
      exporting = false
    }
  }

  onMount(async () => {
    try {
      const s = await backend.GetAISettings()
      loaded = s
      provider = s.provider
      baseUrl = s.baseUrl
      model = s.model
    } catch (e) {
      error = String(e)
    }
  })

  const hasKey = $derived(!!loaded?.hasKey && !clearKey)

  function changeProvider() {
    const p = loaded?.providers.find((x) => x.id === provider)
    if (!p || p.id === 'custom') return
    baseUrl = p.baseUrl
    model = p.defaultModel
  }

  async function save() {
    await backend.SaveAISettings({ provider, baseUrl: baseUrl.trim(), model: model.trim(), apiKey: apiKey.trim(), clearKey })
    if (loaded) loaded.hasKey = clearKey ? !!apiKey.trim() : loaded.hasKey || !!apiKey.trim()
    apiKey = ''
    clearKey = false
  }

  async function saveAndClose() {
    error = ''
    try {
      await save()
      onClose()
    } catch (e) {
      error = String(e)
    }
  }

  async function test() {
    testing = true
    result = null
    error = ''
    try {
      await save()
      const reply = await backend.TestAI()
      result = { ok: true, text: reply }
    } catch (e) {
      result = { ok: false, text: String(e).replace(/^Error:\s*/, '') }
    } finally {
      testing = false
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose()
  }
</script>

<svelte:window onkeydown={onKey} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" onclick={(e) => e.target === e.currentTarget && onClose()}>
  <div class="modal" role="dialog" aria-modal="true" aria-label={t('设置')}>
    <h2>{t('设置')}</h2>
    <div class="appearance">
      <span>{t('外观')}</span>
      <div class="seg" role="radiogroup" aria-label={t('外观')}>
        {#each themes as th}
          <button role="radio" aria-checked={appearance.theme === th.id} class:on={appearance.theme === th.id} onclick={() => setTheme(th.id)}>{t(th.label)}</button>
        {/each}
      </div>
    </div>
    <div class="appearance">
      <span>{t('语言')}{i18n.locale === 'en' ? '' : ' · Language'}</span>
      <select class="lang" value={i18n.locale} onchange={(e) => setLocale(e.currentTarget.value as Locale)} aria-label="Language">
        {#each locales as l}<option value={l.id}>{l.name}</option>{/each}
      </select>
    </div>
    <div class="feedback">
      <span>{t('反馈问题')}</span>
      <div class="row">
        <button onclick={exportLog} disabled={exporting}>{exporting ? t('导出中…') : t('导出日志…')}</button>
        <button onclick={() => backend.OpenIssuePage()}>{t('去 GitHub 提交问题')}</button>
      </div>
    </div>
    <p class="muted small fb-note">
      {#if exported}{@const parts = t('已保存到 {path}，提交问题时把它作为附件上传即可。', { path: '\u0000' }).split('\u0000')}{parts[0]}<span class="mono">{exported}</span>{parts[1]}{:else}{t('日志包含系统版本、扫描和清理的路径与结果，不包含 API Key。')}{/if}
    </p>
    {#if exportError}<p class="bad small">{bt(exportError)}</p>{/if}
    <h3>{t('AI 服务')}</h3>
    {#if intro}<p class="intro"><span class="led"></span><span>{intro}</span></p>{/if}
    {#if !loaded}
      <p class="muted">{bt(error) || t('正在读取…')}</p>
    {:else}
      <label>
        <span>{t('服务商')}</span>
        <select bind:value={provider} onchange={changeProvider}>
          {#each loaded.providers as p}<option value={p.id}>{bt(p.name)}</option>{/each}
        </select>
      </label>
      <label>
        <span>{t('接口地址')}</span>
        <input type="text" bind:value={baseUrl} placeholder="https://…" />
      </label>
      <label>
        <span>{t('模型')} <span class="muted small">{t('模型名可修改')}</span></span>
        <input type="text" bind:value={model} />
      </label>
      <label>
        <span>API Key</span>
        <input type="password" autocomplete="off" bind:value={apiKey} placeholder={hasKey ? t('已保存，留空则不修改') : t('粘贴 API Key')} />
      </label>
      {#if loaded.hasKey && !clearKey}
        <button class="link" onclick={() => (clearKey = true)}>{t('清除已保存的 Key')}</button>
      {:else if clearKey}
        <span class="small warn">{t('保存后将清除已保存的 Key')} <button class="link" onclick={() => (clearKey = false)}>{t('撤销')}</button></span>
      {/if}

      {#if result}
        <p class="result" class:ok={result.ok} class:bad={!result.ok}>{result.ok ? '✓' : '✗'} {result.ok ? result.text : bt(result.text)}</p>
      {/if}
      {#if error}<p class="bad small">{bt(error)}</p>{/if}

      <div class="buttons">
        <button onclick={test} disabled={testing}>{testing ? t('测试中…') : t('测试连接')}</button>
        <span class="spacer"></span>
        <button onclick={onClose}>{t('取消')}</button>
        <button class="primary" onclick={saveAndClose} disabled={testing}>{t('保存')}</button>
      </div>
      <p class="note muted small">
        {t('API Key 用 Windows 加密保存在本机，只有当前 Windows 用户能解密。调查时会把目录结构、文件名和部分文件内容发送给所选服务商；读取密钥、凭据等敏感文件的内容前会先征得你同意。')}
      </p>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.4);
    backdrop-filter: blur(6px);
    animation: fade-in 0.15s;
  }
  .modal {
    width: min(480px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    overflow: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 18px;
    padding: 16px 20px;
    box-shadow: var(--shadow-lg);
    animation: pop-in 0.22s var(--ease);
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  h2 {
    margin: 0 0 4px;
    font-size: 17px;
  }
  h3 {
    margin: 6px 0 0;
    padding-top: 10px;
    border-top: 1px solid var(--border);
    font-size: 14px;
  }
  .appearance {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
  }
  .feedback {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
  }
  .feedback .row {
    display: flex;
    gap: 6px;
    font-weight: 400;
  }
  .feedback button {
    padding: 3px 12px;
    font-size: 12px;
  }
  .fb-note {
    margin: -4px 0 0;
    word-break: break-all;
  }
  .seg {
    display: inline-flex;
    gap: 4px;
    font-weight: 400;
  }
  .seg button {
    padding: 3px 12px;
    font-size: 12px;
    background: transparent;
    border-color: var(--border-strong);
    color: var(--muted);
  }
  .seg button.on {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  label > span {
    font-size: 12px;
    font-weight: 600;
  }
  select {
    font: inherit;
    font-size: 13px;
    color: inherit;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 0 10px;
    height: 34px;
    box-sizing: border-box;
    user-select: text;
  }
  .small {
    font-size: 12px;
    font-weight: 400;
  }
  .link {
    align-self: flex-start;
    border: none;
    background: transparent;
    padding: 0;
    color: var(--accent);
    font-size: 12px;
    text-decoration: underline;
  }
  .warn {
    color: var(--warn);
  }
  .result {
    margin: 0;
  }
  .ok {
    color: var(--text);
  }
  .bad {
    color: var(--warn);
    margin: 0;
  }
  .buttons {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
  .spacer {
    flex: 1;
  }
  .note {
    margin: 0;
  }
  .intro {
    margin: 0;
    padding: 8px 12px;
    background: var(--surface-2);
    border-radius: 10px;
    display: flex;
    gap: 10px;
    align-items: flex-start;
  }
  .intro .led {
    flex: none;
    margin-top: 6px;
  }
</style>
