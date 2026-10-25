<script lang="ts">
  import { backend, type NodeView } from '../lib/api'
  import { bytes, percent } from '../lib/format'
  import { t } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'
  import TreeRow from './TreeRow.svelte'

  type Props = {
    node: NodeView
    depth: number
    parentSize: number
    selected: string
    expanded: Set<string>
    onSelect: (path: string) => void
    onToggle: (path: string) => void
  }
  let { node, depth, parentSize, selected, expanded, onSelect, onToggle }: Props = $props()

  const childLimit = 300
  let children = $state<NodeView[] | null>(null)
  let failed = $state('')
  const isOpen = $derived(node.hasChildren && expanded.has(node.path))
  const isDirectory = $derived(node.kind === 'directory')

  $effect(() => {
    if (isOpen && children === null) {
      backend.Children(node.path, childLimit).then(
        (loaded) => (children = loaded),
        (e) => (failed = String(e)),
      )
    }
  })
</script>

<div
  class="row"
  class:selected={selected === node.path}
  class:dim={!isDirectory}
  style:padding-left="{8 + depth * 16}px"
  role="treeitem"
  aria-selected={selected === node.path}
  aria-expanded={node.hasChildren ? isOpen : undefined}
  tabindex="-1"
  onclick={() => isDirectory && onSelect(node.path)}
  ondblclick={() => node.hasChildren && onToggle(node.path)}
  onkeydown={(e) => e.key === 'Enter' && node.hasChildren && onToggle(node.path)}
>
  <span class="name">
    {#if node.hasChildren}
      <button class="toggle" aria-label={isOpen ? t('收起') : t('展开')} onclick={(e) => (e.stopPropagation(), onToggle(node.path))}>
        {isOpen ? '▾' : '▸'}
      </button>
    {:else}
      <span class="toggle"></span>
    {/if}
    <span class="icon" aria-hidden="true">{node.kind === 'directory' ? '📁' : node.kind === 'reparse' ? '🔗' : node.kind === 'others' ? '…' : '📄'}</span>
    <span class="label" title={node.path || node.name}>{node.kind === 'others' ? bt(node.name) : node.name}</span>
    {#if node.incomplete}<span class="badge" title={t('其中有读不到的位置，大小是下限')}>{t('不完整')}</span>{/if}
  </span>
  <span class="size">{bytes(node.allocationSize)}</span>
  <span class="share">
    <span class="bar"><span style:width={percent(node.allocationSize, parentSize)}></span></span>
    <span class="pct muted">{percent(node.allocationSize, parentSize)}</span>
  </span>
</div>

{#if isOpen}
  {#if failed}
    <div class="row muted" style:padding-left="{24 + depth * 16}px">{t('读取失败：{error}', { error: failed })}</div>
  {:else if children === null}
    <div class="row muted" style:padding-left="{24 + depth * 16}px">{t('载入中…')}</div>
  {:else}
    {#each children as child (child.path || child.name)}
      <TreeRow node={child} depth={depth + 1} parentSize={node.allocationSize} {selected} {expanded} {onSelect} {onToggle} />
    {/each}
  {/if}
{/if}

<style>
  .row {
    display: grid;
    grid-template-columns: 1fr 80px 120px;
    align-items: center;
    padding: 2px 12px 2px 8px;
    min-height: 26px;
    cursor: default;
  }
  .row:hover {
    background: var(--surface-2);
  }
  .row.selected {
    background: var(--accent-soft);
  }
  .name {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .toggle {
    width: 18px;
    flex: none;
    border: none;
    background: transparent;
    padding: 0;
    color: var(--muted);
  }
  .icon {
    flex: none;
    font-size: 12px;
  }
  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dim .label {
    color: var(--muted);
  }
  .badge {
    flex: none;
    font-size: 11px;
    color: var(--warn);
    background: var(--warn-soft);
    border-radius: 4px;
    padding: 0 5px;
  }
  .size {
    text-align: right;
    font-variant-numeric: tabular-nums;
    padding-right: 10px;
  }
  .share {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .bar {
    flex: 1;
    height: 6px;
    background: var(--surface-2);
    border-radius: 3px;
    overflow: hidden;
  }
  .bar span {
    display: block;
    height: 100%;
    background: var(--bar);
  }
  .pct {
    width: 40px;
    text-align: right;
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }
</style>
