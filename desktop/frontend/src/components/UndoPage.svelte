<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type BatchSummary } from '../lib/api'
  import { bytes } from '../lib/format'

  let batches = $state<BatchSummary[]>([])
  let loading = $state(false)
  let error = $state('')
  let busy = $state('')
  let messages = $state<Record<string, string>>({})
  let confirming = $state('')
  let confirmTimer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    loading = true
    try {
      batches = await backend.ListQuarantine()
      error = ''
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
      messages[b.manifest] = `已撤回 ${restored} 项` + (conflicts ? `，${conflicts} 项因原位置已有同名文件而没有恢复` : '')
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
      messages[b.manifest] = `已永久清空，释放了 ${bytes(r.freedBytes)}`
    } catch (e) {
      messages[b.manifest] = clean(e)
    }
    busy = ''
    await load()
  }
</script>

<div class="page">
  <div class="top">
    <h2>撤回</h2>
    <span class="muted small">被移入隔离区的文件可以在这里恢复，或永久清空以释放空间。</span>
    <button class="refresh" disabled={loading} onclick={load}>{loading ? '刷新中…' : '刷新'}</button>
  </div>
  {#if error}<p class="warn">{error}</p>{/if}
  <div class="list">
    {#if batches.length === 0 && !loading && !error}
      <p class="muted empty">隔离区是空的。执行清理后，移入隔离区的文件会出现在这里，可以随时撤回。</p>
    {/if}
    {#each batches as b (b.manifest)}
      <section class="card">
        <div class="info">
          <strong>{when(b.createdAt)}</strong>
          <span class="muted small">隔离中 {b.quarantined} · 已撤回 {b.restored} · 已清空 {b.purged}</span>
          <span class="muted small mono ell" title={b.manifest}>{b.manifest}</span>
          {#if messages[b.manifest]}<span class="small msg">{messages[b.manifest]}</span>{/if}
        </div>
        <span class="size">{bytes(b.quarantinedBytes)}</span>
        <div class="buttons">
          <button disabled={b.quarantined === 0 || busy === b.manifest} onclick={() => undo(b)}>撤回这批</button>
          <button class:danger={confirming === b.manifest} disabled={b.quarantined === 0 || busy === b.manifest} onclick={() => purgeClick(b)}>
            {confirming === b.manifest ? '再点一次确认永久删除' : '永久清空'}
          </button>
        </div>
      </section>
    {/each}
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
    border-radius: var(--radius);
    padding: 10px 14px;
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
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .buttons {
    display: flex;
    gap: 8px;
  }
  button.danger {
    background: var(--warn-soft);
    color: var(--warn);
    border-color: var(--warn);
  }
</style>
