<script lang="ts">
  import { hierarchy, treemap, treemapSquarify, type HierarchyRectangularNode } from 'd3-hierarchy'
  import { backend, type NodeTree } from '../lib/api'
  import { bytes, percent } from '../lib/format'

  let { path, onSelect }: { path: string; onSelect: (path: string) => void } = $props()

  const depth = 3
  const limit = 30
  const header = 17
  const palette = ['#3a9a84', '#5b7fc7', '#d08a3c', '#9b6bc8', '#c75b6f', '#6e9a3d', '#4aa3c7', '#a7814e']

  let data = $state<NodeTree | null>(null)
  let failed = $state('')
  let width = $state(0)
  let height = $state(0)
  let hover = $state<{ node: NodeTree; x: number; y: number } | null>(null)

  $effect(() => {
    const requested = path
    failed = ''
    backend.Subtree(requested, depth, limit).then(
      (tree) => requested === path && (data = tree),
      (e) => (failed = String(e)),
    )
  })

  type Cell = HierarchyRectangularNode<NodeTree>

  const cells = $derived.by<Cell[]>(() => {
    if (!data || width < 10 || height < 10) return []
    const root = hierarchy(data, (d) => (d.children.length ? d.children : undefined))
      .sum((d) => (d.children.length ? 0 : d.allocationSize))
      .sort((a, b) => (b.value ?? 0) - (a.value ?? 0))
    const laidOut = treemap<NodeTree>()
      .tile(treemapSquarify)
      .size([width, height])
      .paddingInner(2)
      .paddingOuter((d) => (d.depth === 0 ? 0 : 2))
      .paddingTop((d) => (d.depth === 0 ? 0 : header))
      .round(true)(root)
    return laidOut.descendants().filter((d) => d.depth > 0 && d.x1 - d.x0 >= 1 && d.y1 - d.y0 >= 1)
  })

  function topIndex(cell: Cell): number {
    let node: Cell = cell
    while (node.depth > 1 && node.parent) node = node.parent
    return node.parent ? node.parent.children!.indexOf(node) : 0
  }

  function fill(cell: Cell): string {
    if (cell.data.kind === 'others') return 'var(--surface-2)'
    const color = palette[topIndex(cell) % palette.length]
    if (cell.children) return `color-mix(in srgb, ${color} ${14 + cell.depth * 10}%, var(--surface))`
    return `color-mix(in srgb, ${color} ${100 - (cell.depth - 1) * 12}%, #000)`
  }

  function clickable(cell: Cell): boolean {
    return cell.data.kind === 'directory' && cell.data.path !== ''
  }

  function move(event: MouseEvent, node: NodeTree) {
    const box = (event.currentTarget as Element).closest('.treemap')!.getBoundingClientRect()
    hover = { node, x: event.clientX - box.left, y: event.clientY - box.top }
  }
</script>

<div class="treemap" bind:clientWidth={width} bind:clientHeight={height} role="presentation" onmouseleave={() => (hover = null)}>
  {#if failed}
    <p class="muted empty">无法显示：{failed}</p>
  {:else if data && data.allocationSize === 0}
    <p class="muted empty">这个位置没有占用空间的文件。</p>
  {:else}
    <svg {width} {height} role="img" aria-label="空间占用图">
      <defs>
        <pattern id="hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <line x1="0" y1="0" x2="0" y2="6" stroke="var(--warn)" stroke-width="1.5" opacity="0.45" />
        </pattern>
      </defs>
      {#each cells as cell (cell.data.path + '|' + cell.data.name + '|' + cell.depth)}
        {@const w = cell.x1 - cell.x0}
        {@const h = cell.y1 - cell.y0}
        <g
          transform="translate({cell.x0},{cell.y0})"
          class:clickable={clickable(cell)}
          role="button"
          tabindex="-1"
          onclick={(e) => (e.stopPropagation(), clickable(cell) && onSelect(cell.data.path))}
          onkeydown={() => {}}
          onmousemove={(e) => move(e, cell.data)}
        >
          <rect width={w} height={h} rx="2" fill={fill(cell)} stroke="var(--surface)" stroke-width="1" />
          {#if cell.data.incomplete || cell.data.kind === 'others'}
            <rect width={w} height={h} rx="2" fill="url(#hatch)" />
          {/if}
          {#if cell.children && w > 40}
            <text x="5" y="12.5" class="group">{cell.data.name} · {bytes(cell.data.allocationSize)}</text>
          {:else if !cell.children && w > 56 && h > 30}
            <text x="6" y="16" class="leaf">{cell.data.name}</text>
            <text x="6" y="30" class="leaf small">{bytes(cell.data.allocationSize)}</text>
          {/if}
        </g>
      {/each}
    </svg>
  {/if}

  {#if hover}
    <div class="tip" style:left="{Math.min(hover.x + 14, width - 260)}px" style:top="{Math.min(hover.y + 14, height - 90)}px">
      <strong>{hover.node.name}</strong>
      <div>{bytes(hover.node.allocationSize)} · 占当前位置 {percent(hover.node.allocationSize, data?.allocationSize ?? 0)}</div>
      {#if hover.node.kind === 'directory'}<div class="muted">{hover.node.files.toLocaleString('zh-CN')} 个文件 · 单击进入</div>{/if}
      {#if hover.node.incomplete}<div class="warn">其中有读不到的位置，大小是下限</div>{/if}
    </div>
  {/if}
</div>

<style>
  .treemap {
    position: relative;
    flex: 1;
    min-height: 0;
    margin: 8px;
    overflow: hidden;
  }
  svg {
    display: block;
  }
  g.clickable {
    cursor: pointer;
  }
  g.clickable:hover > rect:first-child {
    filter: brightness(1.08);
  }
  text {
    pointer-events: none;
    font-size: 12px;
  }
  .group {
    fill: var(--text);
    font-weight: 600;
  }
  .leaf {
    fill: #fff;
  }
  .small {
    font-size: 11px;
    opacity: 0.85;
  }
  .tip {
    position: absolute;
    width: 250px;
    pointer-events: none;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 10px;
    box-shadow: 0 4px 14px rgb(0 0 0 / 0.12);
    font-size: 12px;
    word-break: break-all;
  }
  .warn {
    color: var(--warn);
  }
  .empty {
    padding: 24px;
  }
</style>
