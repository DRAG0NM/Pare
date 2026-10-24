<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type AISettingsView } from '../lib/api'

  let { onClose }: { onClose: () => void } = $props()

  let loaded = $state<AISettingsView | null>(null)
  let provider = $state('')
  let baseUrl = $state('')
  let model = $state('')
  let apiKey = $state('')
  let clearKey = $state(false)
  let testing = $state(false)
  let result = $state<{ ok: boolean; text: string } | null>(null)
  let error = $state('')

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
  <div class="modal" role="dialog" aria-modal="true" aria-label="AI 设置">
    <h2>AI 设置</h2>
    {#if !loaded}
      <p class="muted">{error || '正在读取…'}</p>
    {:else}
      <label>
        <span>服务商</span>
        <select bind:value={provider} onchange={changeProvider}>
          {#each loaded.providers as p}<option value={p.id}>{p.name}</option>{/each}
        </select>
      </label>
      <label>
        <span>接口地址</span>
        <input type="text" bind:value={baseUrl} placeholder="https://…" />
      </label>
      <label>
        <span>模型 <span class="muted small">模型名可修改</span></span>
        <input type="text" bind:value={model} />
      </label>
      <label>
        <span>API Key</span>
        <input type="password" autocomplete="off" bind:value={apiKey} placeholder={hasKey ? '已保存，留空则不修改' : '粘贴 API Key'} />
      </label>
      {#if loaded.hasKey && !clearKey}
        <button class="link" onclick={() => (clearKey = true)}>清除已保存的 Key</button>
      {:else if clearKey}
        <span class="small warn">保存后将清除已保存的 Key <button class="link" onclick={() => (clearKey = false)}>撤销</button></span>
      {/if}

      {#if result}
        <p class="result" class:ok={result.ok} class:bad={!result.ok}>{result.ok ? '✓' : '✗'} {result.text}</p>
      {/if}
      {#if error}<p class="bad small">{error}</p>{/if}

      <div class="buttons">
        <button onclick={test} disabled={testing}>{testing ? '测试中…' : '测试连接'}</button>
        <span class="spacer"></span>
        <button onclick={onClose}>取消</button>
        <button class="primary" onclick={saveAndClose} disabled={testing}>保存</button>
      </div>
      <p class="note muted small">
        API Key 用 Windows 加密保存在本机，只有当前 Windows 用户能解密。调查时会把目录结构、文件名和部分文件内容发送给所选服务商；读取密钥、凭据等敏感文件的内容前会先征得你同意。
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
  }
  .modal {
    width: min(480px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    overflow: auto;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  h2 {
    margin: 0 0 4px;
    font-size: 17px;
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
  input,
  select {
    font: inherit;
    color: inherit;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 6px 8px;
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
    color: #2e9e6a;
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
</style>
