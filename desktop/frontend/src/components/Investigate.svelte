<script lang="ts">
  import { tick } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import type { Action, Finding } from '../lib/api'
  import { bytes, middle } from '../lib/format'
  import { inv, startInvestigation, stopInvestigation } from '../lib/investigation.svelte'

  let { root, onOpenSettings }: { root: string; onOpenSettings: () => void } = $props()

  const riskText = { safe: '纯垃圾', rebuildable: '可重建缓存', careful: '需谨慎', system: '系统组件' }
  const riskTone = { safe: 'safe', rebuildable: 'check', careful: 'manual', system: 'blocked' }
  const confText = { high: '高', medium: '中', low: '低' }
  const kindText: Record<Action['kind'], string> = {
    quarantine: '移入隔离区（可撤回）',
    template: '受支持动作',
    command: '命令（非模板，需你自行判断）',
    config: '修改配置',
    manual: '需手动操作',
  }

  let selectedId = $state('')
  // Collapsed by default so findings keep the room on small windows; the header still
  // shows the latest step.
  let logOpen = $state(false)
  const latest = $derived.by(() => {
    const e = inv.events.at(-1)
    if (!e) return ''
    if (e.kind === 'tool') return `调用 ${e.tool}`
    if (e.kind === 'thinking') return `第 ${e.step} 步：思考中…`
    if (e.kind === 'finding') return `发现：${e.finding?.title ?? ''}`
    if (e.kind === 'result') return `${e.tool} 返回结果`
    return (e.text ?? '').split('\n')[0]
  })
  let logEl = $state<HTMLElement>()
  const expanded = new SvelteSet<number>()

  const selected = $derived(inv.findings.find((f) => f.id === selectedId) ?? inv.findings[0] ?? null)
  const notConfigured = $derived(inv.error.includes('尚未配置'))

  const chosenActions = $derived.by(() => {
    const list: Action[] = []
    for (const f of inv.findings) {
      const a = f.actions[inv.chosen[f.id] ?? 0]
      if (a) list.push(a)
    }
    return list
  })
  const chosenBytes = $derived(chosenActions.reduce((sum, a) => sum + a.bytes, 0))

  $effect(() => {
    inv.events.length
    if (!inv.running) return
    tick().then(() => logEl && (logEl.scrollTop = logEl.scrollHeight))
  })

  function paramText(params: Record<string, string>) {
    return Object.entries(params).map(([k, v]) => `${k}=${v}`)
  }
  function toggle(i: number) {
    if (expanded.has(i)) expanded.delete(i)
    else expanded.add(i)
  }
  function pick(f: Finding) {
    selectedId = f.id
  }
</script>

<div class="inv">
  <section class="panel top">
    <div class="inputs">
      <input type="text" class="scope mono" bind:value={inv.scope} placeholder={root} disabled={inv.running} aria-label="调查范围" />
      <input type="text" class="note" bind:value={inv.note} placeholder="补充说明（可选），例如：重点看 WSL、别动游戏目录" disabled={inv.running} aria-label="补充说明" />
      {#if inv.running}
        <button onclick={stopInvestigation}>停止</button>
      {:else}
        <button class="primary" onclick={startInvestigation}>开始调查</button>
      {/if}
    </div>
    <div class="status small">
      {#if inv.running}
        <span class="spinner" aria-hidden="true"></span><span>第 {inv.step} 步 · 已用 {inv.tokens} tokens</span>
      {:else if inv.error}
        <span class="warn">{inv.error}</span>
      {:else if inv.finished}
        <span>调查完成，发现 {inv.findings.length} 个问题</span>
      {:else}
        <span class="muted">范围留空则调查整个扫描位置。AI 会先看目录结构，再逐个深入。</span>
      {/if}
    </div>
    {#if notConfigured}
      <div class="banner">
        <span>还没有配置 AI 服务商。</span>
        <button onclick={onOpenSettings}>打开设置</button>
      </div>
    {/if}
  </section>

  <div class="main">
    <section class="panel list">
      {#if inv.summary && inv.finished}
        <p class="summary small">{inv.summary}</p>
      {/if}
      {#if inv.findings.length === 0}
        <p class="muted empty">还没有发现。开始调查后，AI 每确认一个问题就会出现在这里。</p>
      {/if}
      {#each inv.findings as f (f.id)}
        <button class="card" class:active={f.id === selected?.id} onclick={() => pick(f)}>
          <strong class="title">{f.title}</strong>
          <span class="row">
            <span class="size">{bytes(f.bytes)}</span>
            <span class="badge {riskTone[f.risk]}">{riskText[f.risk]}</span>
            <span class="muted small">把握 {confText[f.confidence]}</span>
          </span>
        </button>
      {/each}
    </section>

    <section class="panel detail">
      {#if !selected}
        <p class="muted empty">选中左侧的问题查看详情和处理方案。</p>
      {:else}
        {@const f = selected}
        <div class="scroll">
          <div class="row head">
            <h2>{f.title}</h2>
            <span class="badge {riskTone[f.risk]}">{riskText[f.risk]}</span>
            <span class="muted small">把握 {confText[f.confidence]}</span>
          </div>
          <p class="text">{f.summary}</p>
          <h4>原因</h4>
          <p class="text">{f.cause}</p>
          <h4>会不会再长出来</h4>
          <p class="text">{f.regrowth}</p>
          <h4>证据</h4>
          <ul class="text">
            {#each f.evidence as e}<li>{e}</li>{/each}
          </ul>
          <h4>涉及路径</h4>
          <ul class="paths">
            {#each f.paths as p}<li class="mono" title={p}>{middle(p, 90)}</li>{/each}
          </ul>
          <h4>处理方案</h4>
          {#each f.actions as a, i}
            <label class="action" class:on={(inv.chosen[f.id] ?? 0) === i}>
              <input type="radio" name="action-{f.id}" checked={(inv.chosen[f.id] ?? 0) === i} onchange={() => (inv.chosen[f.id] = i)} />
              <div class="abody">
                <div class="row">
                  <span class="kind" class:warn={a.kind === 'command'}>{kindText[a.kind]}{a.kind === 'template' && a.template ? ` · ${a.template}` : ''}</span>
                  {#if a.bytes > 0}<span class="size">{bytes(a.bytes)}</span>{/if}
                </div>
                <strong>{a.label}</strong>
                <span class="muted small">{a.explain}</span>
                {#if a.paths?.length}
                  <ul class="paths">
                    {#each a.paths as p}<li class="mono" title={p}>{middle(p, 80)}</li>{/each}
                  </ul>
                {/if}
                {#if a.params && Object.keys(a.params).length}
                  <ul class="paths">
                    {#each paramText(a.params) as t}<li class="mono" title={t}>{middle(t, 80)}</li>{/each}
                  </ul>
                {/if}
                {#if a.command}<pre class="mono code">{a.command}</pre>{/if}
              </div>
            </label>
          {/each}
        </div>
      {/if}
    </section>
  </div>

  <section class="panel log">
    <button class="log-head" onclick={() => (logOpen = !logOpen)}>
      <strong>调查过程</strong>
      {#if !logOpen && latest}<span class="muted small latest">{latest}</span>{/if}
      <span class="muted small">{inv.events.length} 条 {logOpen ? '▾ 收起' : '▸ 展开'}</span>
    </button>
    {#if logOpen}
      <div class="log-body" bind:this={logEl}>
        {#if inv.events.length === 0}<p class="muted small">暂无记录。</p>{/if}
        {#each inv.events as e, i}
          {#if e.kind === 'thinking'}
            <div class="ev muted">第 {e.step} 步：思考中…</div>
          {:else if e.kind === 'tool'}
            <div class="ev mono">调用 {e.tool} <span class="muted" title={e.args}>{middle(e.args ?? '', 90)}</span></div>
          {:else if e.kind === 'result'}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="ev result muted" onclick={() => toggle(i)} title="点击展开 / 收起">
              {#if expanded.has(i)}<pre>{e.text}</pre>{:else}<span class="one">{(e.text ?? '').split('\n')[0]}</span>{/if}
            </div>
          {:else if e.kind === 'message'}
            <div class="ev msg">{e.text}</div>
          {:else if e.kind === 'error'}
            <div class="ev warn">{e.text}</div>
          {:else if e.kind === 'finding' && e.finding}
            <div class="ev found">发现：{e.finding.title}</div>
          {/if}
        {/each}
      </div>
    {/if}
  </section>

  <footer class="bar">
    <strong>已选 {chosenActions.length} 个方案，预计可回收 {bytes(chosenBytes)}</strong>
    <span class="muted small">执行功能在下一版本（G3）开放，目前只能查看和选择方案</span>
    <button class="primary next" disabled>下一步：确认执行</button>
  </footer>
</div>

<style>
  .inv {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 20px 16px;
  }
  .panel {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    min-height: 0;
  }
  .small {
    font-size: 12px;
  }
  .warn {
    color: var(--warn);
  }
  .top {
    padding: 10px 14px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .inputs {
    display: flex;
    gap: 8px;
  }
  .inputs input {
    font: inherit;
    color: inherit;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 8px;
    min-width: 0;
    user-select: text;
  }
  .scope {
    flex: 2;
  }
  .note {
    flex: 3;
  }
  .status {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 18px;
  }
  .spinner {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    border: 2px solid var(--surface-2);
    border-top-color: var(--accent);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 12px;
    background: var(--warn-soft);
    color: var(--warn);
    padding: 6px 10px;
    border-radius: 6px;
  }
  .main {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(200px, 300px) minmax(0, 1fr);
    gap: 10px;
  }
  .list {
    overflow: auto;
    padding: 8px;
  }
  .summary {
    margin: 0 0 8px;
    padding: 8px 10px;
    background: var(--accent-soft);
    border-radius: 6px;
    user-select: text;
  }
  .empty {
    padding: 16px;
    margin: 0;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 100%;
    text-align: left;
    margin-bottom: 6px;
    padding: 8px 10px;
  }
  .card.active {
    background: var(--accent-soft);
    border-color: var(--accent);
  }
  .title {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .size {
    font-variant-numeric: tabular-nums;
    font-weight: 600;
  }
  .badge {
    font-size: 11px;
    padding: 1px 8px;
    border-radius: 10px;
    white-space: nowrap;
  }
  .badge.safe {
    background: color-mix(in srgb, #2e9e6a 18%, var(--surface));
    color: #2e9e6a;
  }
  .badge.check {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .badge.manual {
    background: var(--warn-soft);
    color: var(--warn);
  }
  .badge.blocked {
    background: var(--surface-2);
    color: var(--muted);
  }
  .detail {
    display: flex;
    flex-direction: column;
  }
  .scroll {
    overflow: auto;
    padding: 12px 16px;
    user-select: text;
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  h4 {
    margin: 12px 0 4px;
    font-size: 12px;
    color: var(--muted);
  }
  .text {
    margin: 0;
  }
  ul.text {
    padding-left: 20px;
  }
  .paths {
    list-style: none;
    margin: 2px 0 0;
    padding: 0;
  }
  .paths li {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .action {
    display: flex;
    gap: 10px;
    padding: 8px 10px;
    margin-bottom: 6px;
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
  }
  .action.on {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .abody {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }
  .kind {
    font-size: 12px;
    color: var(--muted);
  }
  .kind.warn {
    color: var(--warn);
  }
  .code {
    margin: 4px 0 0;
    padding: 6px 8px;
    background: var(--surface-2);
    border-radius: 4px;
    overflow: auto;
    user-select: text;
  }
  .log {
    display: flex;
    flex-direction: column;
  }
  .latest {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }
  .log-head {
    display: flex;
    gap: 12px;
    justify-content: space-between;
    border: none;
    background: transparent;
    padding: 6px 14px;
  }
  .log-body {
    max-height: 120px;
    overflow: auto;
    padding: 0 14px 8px;
    user-select: text;
  }
  .ev {
    font-size: 12px;
    padding: 1px 0;
  }
  .ev.result {
    cursor: pointer;
    padding-left: 12px;
  }
  .ev.result .one {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ev pre {
    margin: 0;
    white-space: pre-wrap;
    font: inherit;
  }
  .ev.msg {
    padding: 4px 0;
    font-size: 13px;
  }
  .ev.found {
    color: var(--accent);
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 8px 16px;
  }
  .next {
    margin-left: auto;
  }
</style>
