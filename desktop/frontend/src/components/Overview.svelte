<script lang="ts">
  import { onMount } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import { backend, type NodeView, type ScanSummary } from '../lib/api'
  import { bytes, count, gapReasons, seconds } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import TreeRow from './TreeRow.svelte'
  import Treemap from './Treemap.svelte'

  let { summary, onRescan, onHome, onDecide, onInvestigate }: { summary: ScanSummary; onRescan: () => void; onHome: () => void; onDecide?: () => void; onInvestigate?: (path: string) => void } = $props()

  let selected = $state(summary.root)
  const expanded = new SvelteSet<string>([summary.root])
  let showGaps = $state(false)
  let elevated = $state(false)
  onMount(async () => (elevated = await backend.Elevated()))

  // Used space the scan could not list: reserved storage, NTFS metadata such as $MFT,
  // shadow copies. Compared with the hard-link-deduplicated total. Shown only when it
  // is a noticeable share of the drive.
  const unlisted = $derived(summary.volumeUsed > summary.uniqueAllocation ? summary.volumeUsed - summary.uniqueAllocation : 0)
  const showUnlisted = $derived(summary.volumeUsed > 0 && unlisted > summary.volumeUsed * 0.01)

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
    {#if summary.volumeUsed > 0}
      <div class="figure"><span class="muted">{t('{root} 盘上已用', { root: summary.root })}</span><strong>{bytes(summary.volumeUsed)}</strong></div>
    {/if}
    {#if summary.volumeUsed > 0}
      <div class="figure" title={t('同一个文件的多个硬链接只算一次；下方目录树按路径计算，所以可能更大')}>
        <span class="muted">{t('扫描列出')}</span><strong>{bytes(summary.uniqueAllocation)}</strong>
      </div>
    {:else}
      <div class="figure"><span class="muted">{t('{root} 实际占用', { root: summary.root })}</span><strong>{bytes(summary.allocationSize)}</strong></div>
    {/if}
    <div class="figure"><span class="muted">{t('文件')}</span><strong>{count(summary.files)}</strong></div>
    <div class="figure"><span class="muted">{t('文件夹')}</span><strong>{count(summary.directories)}</strong></div>
    <div class="figure"><span class="muted">{t('用时')}</span><strong>{seconds(summary.seconds)}</strong></div>
    {#if summary.gapCount > 0}
      <button class="gaps" onclick={() => (showGaps = !showGaps)}>
        {t('{n} 处读不到', { n: count(summary.gapCount) })} {showGaps ? '▴' : '▾'}
      </button>
    {/if}
    <div class="actions">
      <button onclick={onRescan}>{t('重新扫描')}</button>
      <button onclick={onHome}>{t('换个位置')}</button>
      {#if onDecide}<button class="primary" onclick={onDecide}>{t('去挑选要清理的 →')}</button>{/if}
    </div>
  </section>

  {#if showUnlisted}
    <p class="unlisted muted">
      {t('盘上还有约 {size} 已用空间没有出现在扫描结果里，通常是系统保留存储、NTFS 元数据（如 $MFT）和卷影副本。按文件夹读取看不到这些，它们也不是普通文件，不能在这里清理。', { size: bytes(unlisted) })}
    </p>
  {/if}

  {#if showGaps}
    <section class="gap-list">
      <p class="muted">
        {t('下列位置没有读到内容，它们的大小没有计入上面的总数，所在文件夹会标记“不完整”。')}{elevated
          ? t('当前已是管理员身份，这些位置可能正被系统独占或在扫描时发生了变化。')
          : t('以管理员身份运行可以读到其中大部分。')}
      </p>
      <ul>
        {#each summary.gaps as gap}
          <li><span class="mono">{gap.path}</span><span class="muted">{t(gapReasons[gap.code] ?? gap.code)}</span></li>
        {/each}
      </ul>
      {#if summary.gapCount > summary.gaps.length}
        <p class="muted">{t('只显示前 {n} 处。', { n: summary.gaps.length })}</p>
      {/if}
    </section>
  {/if}

  <div class="split">
    <section class="tree" aria-label={t('目录树')}>
      <div class="tree-head muted"><span>{t('名称')}</span><span>{t('占用')}</span><span>{t('占比')}</span></div>
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
        {#if onInvestigate}<button class="ask" onclick={() => onInvestigate(selected)}>{t('让 AI 深挖这里')}</button>{/if}
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
    box-shadow: var(--shadow);
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
  .unlisted {
    margin: -4px 4px 0;
    font-size: 12px;
  }
  .gap-list {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
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
    box-shadow: var(--shadow);
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
  .ask {
    margin-left: auto;
    flex: none;
    padding: 2px 10px;
    font-size: 12px;
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
