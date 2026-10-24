<script lang="ts">
  import { onMount } from 'svelte'
  import type { SvelteSet } from 'svelte/reactivity'
  import { backend, type CandidateGroup, type CandidateItem, type CandidateView } from '../lib/api'
  import { bytes, middle } from '../lib/format'
  import { ageText, blockedReason, daysAgo, evidenceNames, evidenceText, riskLabel, ruleExplain, ruleName } from '../lib/labels'

  let { selection, onBack }: { selection: SvelteSet<string>; onBack: () => void } = $props()

  const PAGE = 500
  const icons: Record<string, string> = { pass: '✓', fail: '✗', unknown: '?', error: '!' }

  let view = $state<CandidateView | null>(null)
  let error = $state('')
  let activeId = $state('')
  let hideBlocked = $state(false)
  let ageDays = $state(0)
  let shown = $state(PAGE)
  let expandedPath = $state('')

  let refreshing = $state(false)
  let checkResult = $state('')

  function executablePaths(v: CandidateView | null): Set<string> {
    return new Set((v?.groups ?? []).flatMap((g) => g.items.filter((i) => i.executable).map((i) => i.path)))
  }

  // Loads the plan (cached for this scan unless refresh) and drops selected paths that
  // are no longer executable in it, so the selection never outlives its evidence.
  async function load(refresh: boolean) {
    error = ''
    refreshing = refresh
    try {
      const before = executablePaths(view)
      const v = await backend.BuildCandidates(refresh)
      const executable = executablePaths(v)
      let unselected = 0
      for (const path of [...selection]) {
        if (!executable.has(path)) {
          selection.delete(path)
          unselected++
        }
      }
      if (refresh) {
        const lost = [...before].filter((p) => !executable.has(p)).length
        const gained = [...executable].filter((p) => !before.has(p)).length
        const time = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
        checkResult =
          lost === 0 && gained === 0
            ? `${time} 已重新核对，没有变化`
            : `${time} 已重新核对：${gained} 项变为可清理，${lost} 项不再能清理${unselected ? `（已取消其中 ${unselected} 项的勾选）` : ''}`
      }
      view = v
      if (!v.groups.some((g) => g.ruleId === activeId) && v.groups.length > 0) selectGroup(v.groups[0])
    } catch (e) {
      error = String(e)
    } finally {
      refreshing = false
    }
  }

  onMount(() => load(false))

  function selectGroup(g: CandidateGroup) {
    activeId = g.ruleId
    ageDays = g.minAgeDays
    shown = PAGE
    expandedPath = ''
  }

  const groups = $derived(view?.groups ?? [])
  const active = $derived(groups.find((g) => g.ruleId === activeId) ?? null)
  const missingRules = $derived((view?.rules ?? []).filter((r) => !groups.some((g) => g.ruleId === r.ruleId)))
  const byPath = $derived.by(() => {
    const map = new Map<string, CandidateItem>()
    for (const g of groups) for (const item of g.items) map.set(item.path, item)
    return map
  })
  const selectedBytes = $derived.by(() => {
    let total = 0
    for (const path of selection) total += byPath.get(path)?.allocationSize ?? 0
    return total
  })
  const visibleItems = $derived(active ? (hideBlocked ? active.items.filter((i) => i.executable) : active.items) : [])
  const groupSelected = $derived(active ? active.items.filter((i) => selection.has(i.path)).length : 0)

  function selectAll(g: CandidateGroup) {
    for (const i of g.items) if (i.executable) selection.add(i.path)
  }
  function clearGroup(g: CandidateGroup) {
    for (const i of g.items) selection.delete(i.path)
  }
  function applyAge(g: CandidateGroup) {
    clearGroup(g)
    for (const i of g.items) if (i.executable && daysAgo(i.lastWriteTime) >= ageDays) selection.add(i.path)
  }
  function toggleItem(item: CandidateItem, on: boolean) {
    if (!item.executable) return
    if (on) selection.add(item.path)
    else selection.delete(item.path)
  }
  function toggleExpand(path: string) {
    expandedPath = expandedPath === path ? '' : path
  }
</script>

{#if error}
  <div class="center">
    <p>生成候选失败：{error}</p>
    <button onclick={onBack}>返回看清</button>
  </div>
{:else if !view}
  <div class="center">
    <div class="spinner" aria-hidden="true"></div>
    <p>正在按规则核对文件…（会检查文件是否被占用、是否有变化）</p>
  </div>
{:else}
  <div class="decide">
    <aside class="panel left">
      <h3>规则候选</h3>
      {#each groups as g}
        {@const risk = riskLabel(g.risk)}
        <button class="card" class:active={g.ruleId === activeId} onclick={() => selectGroup(g)}>
          <span class="row">
            <strong>{ruleName(g.ruleId, g.description)}</strong>
            <span class="badge {risk.tone}">{risk.label}</span>
          </span>
          <span class="muted small">可清理 {g.executable} / 共 {g.candidates} 个</span>
          <span class="size">{bytes(g.executableAllocation)}</span>
        </button>
      {/each}
      {#each missingRules as r}
        <div class="muted small none">{ruleName(r.ruleId, r.description)} · 未发现</div>
      {/each}

    </aside>

    <section class="panel right">
      {#if !active}
        <p class="muted empty">没有找到符合规则的文件。目前内置规则只覆盖用户临时文件、npm / pip / Go 缓存。</p>
      {:else}
        {@const g = active}
        {@const risk = riskLabel(g.risk)}
        <div class="head">
          <div class="row">
            <h2>{ruleName(g.ruleId, g.description)}</h2>
            <span class="badge {risk.tone}">{risk.label}</span>
          </div>
          <p class="muted">{ruleExplain(g.ruleId, g.minAgeDays, g.description)}</p>
        </div>
        <div class="toolbar">
          <button onclick={() => selectAll(g)}>全选可清理项</button>
          <span class="age">
            只选 <input type="number" min="0" bind:value={ageDays} /> 天以前修改的
            <button onclick={() => applyAge(g)}>应用</button>
          </span>
          <button onclick={() => clearGroup(g)}>清空本组</button>
          <label><input type="checkbox" bind:checked={hideBlocked} /> 隐藏不能清理的项</label>
          <span class="muted">已选本组 {groupSelected} 个</span>
        </div>
        <div class="list-head muted">
          <span>选择</span><span>名称</span><span class="num">占用</span><span>修改</span><span>状态</span>
        </div>
        <div class="list" role="table">
          {#each visibleItems.slice(0, shown) as item (item.path)}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <div class="item" class:open={expandedPath === item.path} role="row" tabindex="-1" onclick={() => toggleExpand(item.path)}>
              <div class="line">
                <input
                  type="checkbox"
                  aria-label="选择 {item.name}"
                  disabled={!item.executable}
                  checked={selection.has(item.path)}
                  onclick={(e) => e.stopPropagation()}
                  onchange={(e) => toggleItem(item, e.currentTarget.checked)}
                />
                <div class="name">
                  <span class="fname">{item.name}</span>
                  <span class="mono muted dir">{middle(item.directory, 60)}</span>
                </div>
                <span class="num">{bytes(item.allocationSize)}</span>
                <span title={item.lastWriteTime}>{ageText(item.lastWriteTime)}</span>
                <span>
                  {#if item.executable}
                    <span class="badge safe">可清理</span>
                  {:else}
                    <span class="warn small">{blockedReason(item.evidence, g.ruleId)}</span>
                  {/if}
                </span>
              </div>
              {#if expandedPath === item.path}
                <ul class="evidence">
                  {#each item.evidence as e}
                    <li class={e.status}>
                      <span class="icon">{icons[e.status] ?? '?'}</span>
                      <span class="ename">{evidenceNames[e.kind] ?? e.kind}</span>
                      <span>{evidenceText(e, g.ruleId)}</span>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          {/each}
          {#if visibleItems.length > shown}
            <div class="more"><button onclick={() => (shown += PAGE)}>再显示 {PAGE} 项</button></div>
          {/if}
        </div>
      {/if}
    </section>

    <footer class="bar">
      <strong>已选 {selection.size} 个文件，共 {bytes(selectedBytes)}</strong>
      <span class="muted small">移到隔离区不会立刻腾出空间；确认和执行会在下一步开放。</span>
      <button disabled={refreshing} onclick={() => load(true)} title="重新检查文件是否被占用、是否有变化">
        {refreshing ? '正在检查…' : '重新检查文件状态'}
      </button>
      {#if checkResult}<span class="small check">{checkResult}</span>{/if}
      <span class="next">
        <button class="primary" disabled>下一步：确认执行</button>
        <span class="muted small">执行功能在下一版本（G3）开放，目前只能挑选</span>
      </span>
    </footer>
  </div>
{/if}

<style>
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 12vh;
    gap: 18px;
  }
  .spinner {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: 3px solid var(--surface-2);
    border-top-color: var(--accent);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .decide {
    height: 100%;
    display: grid;
    grid-template-columns: 300px 1fr;
    grid-template-rows: minmax(0, 1fr) auto;
    gap: 12px;
    padding: 14px 20px 20px;
  }
  .panel {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    min-height: 0;
  }
  .left {
    padding: 8px 12px 12px;
    overflow: auto;
  }
  .right {
    display: flex;
    flex-direction: column;
  }
  h3 {
    margin: 12px 0 6px;
    font-size: 12px;
    font-weight: 600;
    color: var(--muted);
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  .small {
    font-size: 12px;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    text-align: left;
    margin-bottom: 6px;
    padding: 8px 10px;
  }
  .card.active {
    background: var(--accent-soft);
    border-color: var(--accent);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
    justify-content: space-between;
  }
  .size {
    font-variant-numeric: tabular-nums;
    font-weight: 600;
  }
  .none {
    opacity: 0.6;
    padding: 2px 4px;
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
  .warn {
    color: var(--warn);
  }
  .empty {
    padding: 24px;
  }
  .head {
    padding: 12px 16px 0;
  }
  .head p {
    margin: 4px 0 8px;
  }
  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px 14px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
  }
  .age input[type='number'] {
    width: 64px;
  }
  .list-head,
  .line {
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr) 80px 72px minmax(110px, 220px);
    align-items: center;
    gap: 8px;
    padding: 6px 16px;
  }
  .list-head {
    font-size: 12px;
    border-bottom: 1px solid var(--border);
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .item {
    border-bottom: 1px solid var(--surface-2);
    cursor: pointer;
  }
  .item:hover,
  .item.open {
    background: var(--surface-2);
  }
  .name {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .fname {
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dir {
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .evidence {
    list-style: none;
    margin: 0;
    padding: 4px 16px 10px 60px;
    font-size: 13px;
    user-select: text;
  }
  .evidence li {
    display: flex;
    gap: 8px;
  }
  .evidence li.fail,
  .evidence li.error {
    color: var(--warn);
  }
  .evidence li.unknown {
    color: var(--muted);
  }
  .icon {
    width: 14px;
    text-align: center;
    font-weight: 700;
  }
  .ename {
    min-width: 110px;
    font-weight: 500;
  }
  .more {
    padding: 10px;
    text-align: center;
  }
  .bar {
    grid-column: 1 / -1;
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 10px 16px;
  }
  .check {
    color: var(--accent);
  }
  .next {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 10px;
  }
</style>
