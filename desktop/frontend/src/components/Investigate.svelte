<script lang="ts">
  import { tick } from 'svelte'
  import { Tween } from 'svelte/motion'
  import { cubicOut } from 'svelte/easing'
  import { SvelteSet } from 'svelte/reactivity'
  import type { Action, Finding, InvestigationDepth } from '../lib/api'
  import { bytes, middle } from '../lib/format'
  import { openConfirm } from '../lib/execution.svelte'
  import { choiceOf, inv, startInvestigation, stopInvestigation } from '../lib/investigation.svelte'
  import { t } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'

  let { root, onOpenSettings, selection }: { root: string; onOpenSettings: () => void; selection: SvelteSet<string> } = $props()

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

  const depthOptions: { id: InvestigationDepth; label: string; hint: string }[] = [
    { id: 'standard', label: '标准', hint: '约 3–5 分钟，找出主要问题' },
    { id: 'deep', label: '深入', hint: '约 6–8 分钟，覆盖更多小问题' },
    { id: 'thorough', label: '彻底', hint: '10 分钟以上，逐个目录排查' },
  ]
  const depthHint = $derived(depthOptions.find((d) => d.id === inv.depth)?.hint ?? '')

  let selectedId = $state('')
  // Collapsed by default so findings keep the room on small windows; the header still
  // shows the latest step.
  let logOpen = $state(false)
  const latest = $derived.by(() => {
    const e = inv.events.at(-1)
    if (!e) return ''
    if (e.kind === 'tool') return t('调用 {tool}', { tool: e.tool ?? '' })
    if (e.kind === 'thinking') return t('第 {step} 步：思考中…', { step: e.step ?? 0 })
    if (e.kind === 'finding') return t('发现：{title}', { title: e.finding?.title ?? '' })
    if (e.kind === 'result') return t('{tool} 返回结果', { tool: e.tool ?? '' })
    return bt((e.text ?? '').split('\n')[0])
  })
  let logEl = $state<HTMLElement>()
  const expanded = new SvelteSet<number>()

  // Handled findings sink to the bottom of the list (stable within each group).
  const sorted = $derived([...inv.findings.filter((f) => !inv.handled[f.id]), ...inv.findings.filter((f) => inv.handled[f.id])])
  const selected = $derived(inv.findings.find((f) => f.id === selectedId) ?? sorted[0] ?? null)
  const canContinue = $derived(inv.finished && !inv.running && inv.findings.length > 0)
  const notConfigured = $derived(inv.error.includes('尚未配置'))

  const chosenActions = $derived.by(() => {
    const list: Action[] = []
    for (const f of inv.findings) {
      const a = f.actions[choiceOf(f)]
      if (a) list.push(a)
    }
    return list
  })
  const canNext = $derived(chosenActions.length > 0 || selection.size > 0)
  const chosenBytes = $derived(chosenActions.reduce((sum, a) => sum + a.bytes, 0))
  const shownBytes = Tween.of(() => chosenBytes, { duration: 400, easing: cubicOut })
  // One key per sentence; the styled size span is spliced in at the placeholder.
  // Where the user is in the flow: 1 set up and start, 2 review findings, 3 confirm.
  const flowStep = $derived(inv.running || inv.findings.length === 0 ? 1 : canNext ? 3 : 2)
  const flowSteps = [
    { title: '设定范围和深度，开始深挖', hint: '范围留空就是整个扫描位置' },
    { title: '逐条查看，选好处理方案', hint: '低风险、可撤回的问题已预选方案；其余默认「暂不处理」，需要时再自己选' },
    { title: '确认执行', hint: '执行前会列出每一项，再确认一次' },
  ]
  const totalParts = $derived(t('已选 {n} 个方案，预计可回收 {size}', { n: chosenActions.length, size: '\u0000' }).split('\u0000'))

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
  <ol class="flow" aria-label={t('步骤')}>
    {#each flowSteps as s, i}
      <li class:on={flowStep === i + 1} class:done={flowStep > i + 1}>
        <span class="num">{flowStep > i + 1 ? '✓' : '0' + (i + 1)}</span>
        {#if flowStep === i + 1}<span class="led"></span>{/if}
        <span class="ftext"><strong>{t(s.title)}</strong><span class="muted">{t(s.hint)}</span></span>
      </li>
    {/each}
  </ol>
  <section class="panel top">
    <div class="inputs">
      <input type="text" class="scope mono" bind:value={inv.scope} placeholder={root} disabled={inv.running} aria-label={t('深挖范围')} />
      <input type="text" class="note" bind:value={inv.note} placeholder={t('补充说明（可选），例如：重点看 WSL、别动游戏目录')} disabled={inv.running} aria-label={t('补充说明')} />
      {#if inv.running}
        <button onclick={stopInvestigation}>{t('停止')}</button>
      {:else}
        {#if canContinue}
          <button title={t('在已有结果的基础上再查一轮，不重复已经报告过的问题')} onclick={() => startInvestigation(true)}>{t('继续深挖')}</button>
        {/if}
        <button class="primary" class:pulse={inv.findings.length === 0 && !inv.error} onclick={() => startInvestigation(false)}>{t('开始深挖')}</button>
      {/if}
    </div>
    <div class="depth small">
      <div class="seg" role="radiogroup" aria-label={t('深挖程度')}>
        {#each depthOptions as d}
          <button role="radio" aria-checked={inv.depth === d.id} class:on={inv.depth === d.id} disabled={inv.running} onclick={() => (inv.depth = d.id)}>{t(d.label)}</button>
        {/each}
      </div>
      <span class="muted">{t('{hint}（按 DeepSeek 计费，通常不到 1 元）', { hint: depthHint ? t(depthHint) : '' })}</span>
    </div>
    <div class="status small">
      {#if inv.running}
        <span class="spinner" aria-hidden="true"></span><span>{inv.continuing ? t('继续深挖 · 第 {step} 步 · 已用 {tokens} tokens', { step: inv.step, tokens: inv.tokens }) : t('第 {step} 步 · 已用 {tokens} tokens', { step: inv.step, tokens: inv.tokens })}</span>
      {:else if inv.error}
        <span class="warn">{bt(inv.error)}</span>
      {:else if inv.finished}
        <span>{t('深挖完成，发现 {n} 个问题', { n: inv.findings.length })}</span>
      {:else}
        <span class="muted">{t('范围留空则深挖整个扫描位置。AI 会先看目录结构，再逐个深入。')}</span>
      {/if}
    </div>
    {#if notConfigured}
      <div class="banner">
        <span>{t('还没有配置 AI 服务商。')}</span>
        <button onclick={onOpenSettings}>{t('打开设置')}</button>
      </div>
    {/if}
  </section>

  <div class="main">
    <section class="panel list">
      {#if inv.summary && inv.finished}
        <p class="summary small">{inv.summary}</p>
      {/if}
      {#if inv.findings.length === 0}
        <p class="muted empty">{t('还没有发现。开始深挖后，AI 每确认一个问题就会出现在这里。')}</p>
      {/if}
      {#each sorted as f (f.id)}
        <button class="card" class:active={f.id === selected?.id} class:muted-card={!!inv.handled[f.id]} onclick={() => pick(f)}>
          <strong class="title">{f.title}</strong>
          <span class="row">
            {#if inv.handled[f.id]}<span class="badge handled">{inv.handled[f.id] === 'done' ? t('已处理') : t('部分处理')}</span>{/if}
            <span class="size">{bytes(f.bytes)}</span>
            <span class="badge {riskTone[f.risk]}">{t(riskText[f.risk])}</span>
            <span class="muted small">{t('把握 {level}', { level: t(confText[f.confidence]) })}</span>
          </span>
        </button>
      {/each}
    </section>

    <section class="panel detail">
      {#if !selected}
        <p class="muted empty">{t('选中左侧的问题查看详情和处理方案。')}</p>
      {:else}
        {@const f = selected}
        <div class="scroll">
          <div class="row head">
            <h2>{f.title}</h2>
            <span class="badge {riskTone[f.risk]}">{t(riskText[f.risk])}</span>
            <span class="muted small">{t('把握 {level}', { level: t(confText[f.confidence]) })}</span>
          </div>
          <p class="text">{f.summary}</p>
          <h4>{t('原因')}</h4>
          <p class="text">{f.cause}</p>
          <h4>{t('会不会再长出来')}</h4>
          <p class="text">{f.regrowth}</p>
          <h4>{t('证据')}</h4>
          <ul class="text">
            {#each f.evidence as e}<li>{e}</li>{/each}
          </ul>
          <h4>{t('涉及路径')}</h4>
          <ul class="paths">
            {#each f.paths as p}<li class="mono" title={p}>{middle(p, 90)}</li>{/each}
          </ul>
          <h4>{t('处理方案')}</h4>
          {#each f.actions as a, i}
            <label class="action" class:on={choiceOf(f) === i}>
              <input type="radio" name="action-{f.id}" checked={choiceOf(f) === i} onchange={() => (inv.chosen[f.id] = i)} />
              <div class="abody">
                <div class="row">
                  <span class="kind" class:warn={a.kind === 'command'}>{t(kindText[a.kind])}{a.kind === 'template' && a.template ? ` · ${a.template}` : ''}</span>
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
                    {#each paramText(a.params) as pt}<li class="mono" title={pt}>{middle(pt, 80)}</li>{/each}
                  </ul>
                {/if}
                {#if a.command}<pre class="mono code">{a.command}</pre>{/if}
              </div>
            </label>
          {/each}
          <label class="action skip" class:on={choiceOf(f) === -1}>
            <input type="radio" name="action-{f.id}" checked={choiceOf(f) === -1} onchange={() => (inv.chosen[f.id] = -1)} />
            <div class="abody"><strong>{t('暂不处理')}</strong><span class="muted small">{t('这一项先不动，也不会计入下面的统计。')}</span></div>
          </label>
        </div>
      {/if}
    </section>
  </div>

  <section class="panel log">
    <button class="log-head" onclick={() => (logOpen = !logOpen)}>
      <strong>{t('深挖过程')}</strong>
      {#if !logOpen && latest}<span class="muted small latest">{latest}</span>{/if}
      <span class="muted small">{t('{n} 条', { n: inv.events.length })} {logOpen ? '▾ ' + t('收起') : '▸ ' + t('展开')}</span>
    </button>
    {#if logOpen}
      <div class="log-body" bind:this={logEl}>
        {#if inv.events.length === 0}<p class="muted small">{t('暂无记录。')}</p>{/if}
        {#each inv.events as e, i}
          {#if e.kind === 'thinking'}
            <div class="ev muted">{t('第 {step} 步：思考中…', { step: e.step ?? 0 })}</div>
          {:else if e.kind === 'tool'}
            <div class="ev mono">{t('调用 {tool}', { tool: e.tool ?? '' })} <span class="muted" title={e.args}>{middle(e.args ?? '', 90)}</span></div>
          {:else if e.kind === 'result'}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
            <div class="ev result muted" onclick={() => toggle(i)} title={t('点击展开 / 收起')}>
              {#if expanded.has(i)}<pre>{e.text}</pre>{:else}<span class="one">{(e.text ?? '').split('\n')[0]}</span>{/if}
            </div>
          {:else if e.kind === 'message'}
            <div class="ev msg">{bt(e.text ?? '')}</div>
          {:else if e.kind === 'error'}
            <div class="ev warn">{bt(e.text ?? '')}</div>
          {:else if e.kind === 'finding' && e.finding}
            <div class="ev found">{t('发现：{title}', { title: e.finding.title })}</div>
          {/if}
        {/each}
      </div>
    {/if}
  </section>

  <footer class="bar">
    <strong>{totalParts[0]}<span class="total">{bytes(shownBytes.current)}</span>{totalParts[1] ?? ''}</strong>
    {#if selection.size > 0}<span class="muted small">{t('另有 {n} 个常规缓存文件已在「常规清理」页选中', { n: selection.size })}</span>{/if}
    <button class="primary next" class:pulse={inv.finished && canNext} disabled={!canNext} onclick={() => openConfirm(selection)}>{t('下一步：确认执行')}</button>
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
  .flow {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .flow li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--surface);
    min-width: 0;
    transition: border-color 0.2s, background-color 0.2s;
  }
  .flow li.on {
    border-color: var(--text);
  }
  .flow li .led {
    order: 3;
    margin-left: auto;
  }
  .num {
    flex: none;
    font-family: var(--font-label);
    font-size: 13px;
    letter-spacing: 0.04em;
    color: var(--muted);
  }
  .flow li.on .num,
  .flow li.done .num {
    color: var(--text);
  }
  .ftext {
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-size: 12px;
    line-height: 1.4;
  }
  .ftext > * {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .flow li:not(.on) strong {
    font-weight: 500;
  }
  /* The button for the next action breathes gently until it is used. */
  .pulse {
    position: relative;
  }
  .pulse::after {
    content: '';
    position: absolute;
    top: 3px;
    right: 3px;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--signal);
    animation: blink 1.4s steps(2, jump-none) infinite;
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
    border-radius: 10px;
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
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--signal);
    animation: blink 1s steps(2, jump-none) infinite;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 12px;
    border: 1px solid var(--warn);
    color: var(--warn);
    padding: 6px 10px;
    border-radius: 10px;
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
    background: var(--surface-2);
    border-radius: 10px;
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
    padding: 9px 11px;
    border-radius: 12px;
    animation: fade-up 0.3s var(--ease) both;
  }
  .card.muted-card {
    opacity: 0.55;
  }
  .badge.handled {
    color: var(--muted);
    border-color: var(--border);
  }
  .depth {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }
  .seg {
    display: inline-flex;
    gap: 2px;
    padding: 2px;
    border: 1px solid var(--border-strong);
    border-radius: 999px;
  }
  .seg button {
    padding: 2px 12px;
    font-size: 12px;
    border: none;
    background: transparent;
    border-radius: 999px;
  }
  .seg button.on {
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
  .card.active,
  .card.active:hover {
    background: var(--accent-soft);
    border-color: var(--text);
    box-shadow: inset 2px 0 0 var(--text);
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
    border-radius: 999px;
    white-space: nowrap;
    background: transparent;
    border: 1px solid var(--border-strong);
    color: var(--muted);
  }
  .badge.manual,
  .badge.blocked {
    border-color: var(--warn);
    color: var(--warn);
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
    margin: 14px 0 4px;
    font-family: var(--font-label);
    font-size: 11px;
    font-weight: 400;
    letter-spacing: 0.08em;
    text-transform: uppercase;
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
    border-radius: 12px;
    cursor: pointer;
  }
  .action {
    transition: border-color 0.15s, background-color 0.15s;
  }
  .action:hover {
    border-color: var(--border-strong);
  }
  .action.on {
    border-color: var(--text);
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
    font-family: var(--font-label);
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .kind.warn {
    color: var(--warn);
  }
  .code {
    margin: 4px 0 0;
    padding: 6px 8px;
    background: var(--bg);
    border: 1px dashed var(--border-strong);
    border-radius: 10px;
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
  .ev {
    animation: fade-in 0.2s;
  }
  .ev.found {
    color: var(--text);
    font-weight: 600;
  }
  .ev.found::before {
    content: '● ';
    color: var(--signal);
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
  .total {
    color: var(--text);
    font-family: var(--font-dot);
    font-weight: 800;
    font-size: 22px;
    letter-spacing: 0.02em;
    font-variant-numeric: tabular-nums;
  }
  .next {
    margin-left: auto;
  }
</style>
