<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type BatchSummary, type HistoryEntry } from '../lib/api'
  import { bytes } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'

  let batches = $state<BatchSummary[]>([])
  let loading = $state(false)
  let error = $state('')
  let busy = $state('')
  let messages = $state<Record<string, string>>({})
  let confirming = $state('')
  let confirmTimer: ReturnType<typeof setTimeout> | undefined
  let history = $state<HistoryEntry[]>([])
  let historyOpen = $state(false)
  let entryOpen = $state<Record<number, boolean>>({})

  async function load() {
    loading = true
    try {
      batches = await backend.ListQuarantine()
      error = ''
      history = (await backend.ListHistory()) ?? []
    } catch (e) {
      error = clean(e)
    } finally {
      loading = false
    }
  }
  onMount(() => {
    load()
    return () => clearTimeout(confirmTimer)
  })

  const clean = (e: unknown) => String(e).replace(/^Error:\s*/, '')
  const statusText = (s: string) => (s === 'partial' ? t('部分完成') : s === 'skipped' ? t('未执行') : s === 'failed' ? t('失败') : s)
  const pad = (n: number) => String(n).padStart(2, '0')
  function when(iso: string) {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return iso
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }

  async function undo(b: BatchSummary) {
    busy = b.manifest
    try {
      const ex = await backend.UndoBatch(b.manifest)
      const restored = ex.items.filter((i) => i.state === 'restored').length
      const conflicts = ex.items.filter((i) => i.state === 'conflict').length
      messages[b.manifest] = t('已撤回 {n} 项', { n: restored }) + (conflicts ? t('，{n} 项因原位置已有同名文件而没有恢复', { n: conflicts }) : '')
    } catch (e) {
      messages[b.manifest] = clean(e)
    }
    busy = ''
    await load()
  }

  async function purgeClick(b: BatchSummary) {
    if (confirming !== b.manifest) {
      confirming = b.manifest
      clearTimeout(confirmTimer)
      confirmTimer = setTimeout(() => (confirming = ''), 5000)
      return
    }
    clearTimeout(confirmTimer)
    confirming = ''
    busy = b.manifest
    try {
      const r = await backend.PurgeBatch(b.manifest)
      messages[b.manifest] = t('已永久清空，释放了 {size}', { size: bytes(r.freedBytes) })
    } catch (e) {
      messages[b.manifest] = clean(e)
    }
    busy = ''
    await load()
  }
</script>

<div class="page">
  <div class="top">
    <h2>{t('隔离区')}</h2>
    <span class="muted small">{t('被移入隔离区的文件可以在这里恢复，或永久清空以释放空间。')}</span>
    <button class="refresh" disabled={loading} onclick={load}>{loading ? t('刷新中…') : t('刷新')}</button>
  </div>
  {#if error}<p class="warn">{bt(error)}</p>{/if}
  <div class="list">
    {#if batches.length === 0 && !loading && !error}
      <p class="muted empty">{t('隔离区是空的。执行清理后，移入隔离区的文件会出现在这里，可以随时撤回。')}</p>
    {/if}
    {#each batches as b (b.manifest)}
      <section class="card">
        <div class="info">
          <strong>{when(b.createdAt)}</strong>
          <span class="muted small">{t('隔离中 {a} · 已撤回 {b} · 已清空 {c}', { a: b.quarantined, b: b.restored, c: b.purged })}</span>
          <span class="muted small mono ell" title={b.manifest}>{b.manifest}</span>
          {#if messages[b.manifest]}<span class="small msg">{bt(messages[b.manifest])}</span>{/if}
        </div>
        <span class="size dot">{bytes(b.quarantinedBytes)}</span>
        <div class="buttons">
          <button disabled={b.quarantined === 0 || busy === b.manifest} onclick={() => undo(b)}>{t('撤回这批')}</button>
          <button class="purge" class:danger={confirming === b.manifest} disabled={b.quarantined === 0 || busy === b.manifest} onclick={() => purgeClick(b)}>
            {confirming === b.manifest ? t('再点一次确认永久删除') : t('永久清空')}
          </button>
        </div>
      </section>
    {/each}

    {#if history.length}
      <div class="history">
        <button class="quiet" onclick={() => (historyOpen = !historyOpen)}>{t('清理记录（{n}）', { n: history.length })} {historyOpen ? '▾' : '▸'}</button>
        {#if historyOpen}
          <ul>
            {#each history as h, i}
              <li>
                {#if h.kind === 'purge'}
                  <span>{t('{time} · 清空隔离区 {n} 项 · 释放 {size}', { time: when(h.time), n: h.items, size: bytes(Math.max(0, h.freedBytes)) })}</span>
                {:else}
                  <button class="quiet line" onclick={() => (entryOpen[i] = !entryOpen[i])}>
                    {t('{time} · 清理 {n} 项', { time: when(h.time), n: h.items })}{h.freedBytes > 0 ? ' · ' + t('释放 {size}', { size: bytes(h.freedBytes) }) : ''}{h.quarantinedBytes > 0 ? ' · ' + t('隔离 {size}', { size: bytes(h.quarantinedBytes) }) : ''}
                  </button>
                  {#if entryOpen[i]}
                    <ul class="actions">
                      {#each h.actions as a}
                        <li>{a.findingTitle ? `${a.findingTitle} — ` : ''}{a.label}{a.bytes > 0 ? ` · ${bytes(a.bytes)}` : ''}{a.status === 'done' ? '' : ` · ${statusText(a.status)}`}</li>
                      {/each}
                    </ul>
                  {/if}
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .page {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 20px 16px;
  }
  .top {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  .refresh {
    margin-left: auto;
  }
  .small {
    font-size: 12px;
  }
  .warn {
    color: var(--warn);
    margin: 0;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .empty {
    padding: 24px 8px;
    margin: 0;
  }
  .card {
    display: flex;
    align-items: center;
    gap: 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 12px 16px;
  }
  .info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .ell {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .msg {
    color: var(--accent);
  }
  .size {
    font-size: 20px;
    line-height: 1;
    white-space: nowrap;
  }
  .buttons {
    display: flex;
    gap: 8px;
  }
  /* The record list stays deliberately plain: it is for looking back, not a feature. */
  .history {
    margin-top: 8px;
    font-size: 12px;
    color: var(--muted);
  }
  .history ul {
    list-style: none;
    margin: 4px 0 0;
    padding: 0;
  }
  .history li {
    padding: 5px 0;
    border-top: 1px dashed var(--border-strong);
  }
  .history li:first-child {
    border-top: none;
  }
  .history .actions {
    padding-left: 16px;
  }
  .history .actions li {
    padding: 2px 0;
    border-top: none;
  }
  .quiet {
    border: none;
    background: transparent;
    padding: 0;
    color: var(--muted);
    font-size: 12px;
    text-align: left;
  }
  .quiet:hover:not(:disabled) {
    background: transparent;
    color: var(--text);
  }
  button.purge {
    background: transparent;
    color: var(--warn);
    border-color: var(--warn);
  }
  button.purge:hover:not(:disabled) {
    background: var(--warn-soft);
    color: var(--warn);
    border-color: var(--warn);
  }
  button.danger {
    background: var(--warn-soft);
  }
</style>
