<script lang="ts">
  import { SvelteSet } from 'svelte/reactivity'
  import type { NodeView, ScanSummary } from '../lib/api'
  import { bytes, count, gapReasons, seconds } from '../lib/format'
  import TreeRow from './TreeRow.svelte'
  import Treemap from './Treemap.svelte'

  let { summary, onRescan, onHome }: { summary: ScanSummary; onRescan: () => void; onHome: () => void } = $props()

  let selected = $state(summary.root)
  const expanded = new SvelteSet<string>([summary.root])
  let showGaps = $state(false)

  const rootNode = $derived<NodeView>({
    path: summary.root,
    name: summary.root,
    kind: 'directory',
    logicalSize: summary.logicalSize,
    allocationSize: summary.allocationSize,
    files: summary.files,
    directories: summary.directories,
    incomplete: !summary.complete,
    hasChildren: summary.entries > 0,
    others: 0,
  })

  // Paths from the scan root down to path, so selecting in the treemap opens the tree.
  function ancestors(path: string): string[] {
    const root = summary.root
    if (!path.startsWith(root)) return []
    const rest = path.slice(root.length).split('\\').filter(Boolean)
    const result = [root]
    let current = root
    for (const part of rest) {
      current = current.endsWith('\\') ? current + part : `${current}\\${part}`
      result.push(current)
    }
    return result
  }

  function select(path: string) {
    selected = path
    for (const ancestor of ancestors(path).slice(0, -1)) expanded.add(ancestor)
  }

  function toggle(path: string) {
    if (expanded.has(path)) expanded.delete(path)
    else expanded.add(path)
  }

  const crumbs = $derived(ancestors(selected))
</script>

<div class="overview">
  <section class="summary">
    <div class="figure">
      <span class="muted">{summary.root} 实际占用</span>
      <strong>{bytes(summary.allocationSize)}</strong>
    </div>
    <div class="figure"><span class="muted">文件</span><strong>{count(summary.files)}</strong></div>
    <div class="figure"><span class="muted">文件夹</span><strong>{count(summary.directories)}</strong></div>
    <div class="figure"><span class="muted">用时</span><strong>{seconds(summary.seconds)}</strong></div>
    {#if summary.gapCount > 0}
      <button class="gaps" onclick={() => (showGaps = !showGaps)}>
        {count(summary.gapCount)} 处读不到 {showGaps ? '▴' : '▾'}
      </button>
    {/if}
    <div class="actions">
      <button onclick={onRescan}>重新扫描</button>
      <button onclick={onHome}>换个位置</button>
    </div>
  </section>

  {#if showGaps}
    <section class="gap-list">
      <p class="muted">
        下列位置没有读到内容，它们的大小没有计入上面的总数，所在文件夹会标记“不完整”。以管理员身份运行可以读到其中一部分。
      </p>
      <ul>
        {#each summary.gaps as gap}
          <li><span class="mono">{gap.path}</span><span class="muted">{gapReasons[gap.code] ?? gap.code}</span></li>
        {/each}
      </ul>
      {#if summary.gapCount > summary.gaps.length}
        <p class="muted">只显示前 {summary.gaps.length} 处。</p>
      {/if}
    </section>
  {/if}

  <div class="split">
    <section class="tree" aria-label="目录树">
      <div class="tree-head muted"><span>名称</span><span>占用</span><span>占比</span></div>
      <div class="rows">
        <TreeRow node={rootNode} depth={0} parentSize={summary.allocationSize} {selected} {expanded} onSelect={select} onToggle={toggle} />
      </div>
    </section>
    <section class="map">
      <div class="crumbs">
        {#each crumbs as crumb, i}
          {#if i > 0}<span class="muted">›</span>{/if}
          <button class="crumb" class:current={crumb === selected} onclick={() => select(crumb)}>
            {i === 0 ? crumb : crumb.slice(crumb.lastIndexOf('\\') + 1)}
          </button>
        {/each}
      </div>
      <Treemap path={selected} onSelect={select} />
    </section>
  </div>
</div>

<style>
  .overview {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 14px 20px 20px;
    gap: 12px;
  }
  .summary {
    display: flex;
    align-items: center;
    gap: 32px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 12px 18px;
  }
  .figure {
    display: flex;
    flex-direction: column;
  }
  .figure span {
    font-size: 12px;
  }
  .figure strong {
    font-size: 20px;
    font-variant-numeric: tabular-nums;
  }
  .gaps {
    background: var(--warn-soft);
    color: var(--warn);
    border-color: transparent;
  }
  .actions {
    margin-left: auto;
    display: flex;
    gap: 8px;
  }
  .gap-list {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 4px 18px 8px;
    max-height: 30vh;
    overflow: auto;
    user-select: text;
  }
  .gap-list ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .gap-list li {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    padding: 3px 0;
    border-bottom: 1px solid var(--surface-2);
  }
  .split {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(360px, 2fr) 3fr;
    gap: 12px;
  }
  .tree,
  .map {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .tree-head {
    display: grid;
    grid-template-columns: 1fr 80px 120px;
    padding: 8px 12px;
    font-size: 12px;
    border-bottom: 1px solid var(--border);
  }
  .rows {
    overflow: auto;
    flex: 1;
    padding: 4px 0;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    border-bottom: 1px solid var(--border);
    overflow: hidden;
    white-space: nowrap;
  }
  .crumb {
    border: none;
    background: transparent;
    padding: 2px 6px;
  }
  .crumb.current {
    font-weight: 600;
    color: var(--accent);
  }
</style>
