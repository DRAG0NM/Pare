<script lang="ts">
  import { hierarchy, treemap, treemapSquarify, type HierarchyRectangularNode } from 'd3-hierarchy'
  import { backend, type NodeTree } from '../lib/api'
  import { bytes, percent } from '../lib/format'
  import { numberLocale, t } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'

  let { path, onSelect }: { path: string; onSelect: (path: string) => void } = $props()

  const depth = 3
  const limit = 30
  const header = 17

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

  // Grayscale only: tone varies by top-level group and depth, within a calm 10%-34% range.
  function fill(cell: Cell): string {
    if (cell.data.kind === 'others') return 'var(--surface-2)'
    if (cell.children) return 'transparent'
    const tones = [14, 26, 18, 30, 22, 34, 12, 28]
    const p = Math.max(10, Math.min(34, tones[topIndex(cell) % tones.length] - (cell.depth - 1) * 2))
    return `color-mix(in srgb, var(--text) ${p}%, var(--surface))`
  }

  function clickable(cell: Cell): boolean {
    return cell.data.kind === 'directory' && cell.data.path !== ''
  }

  // Label fitting: measure with an offscreen canvas using the CSS font of each label class.
  let box: HTMLDivElement | undefined
  let ctx: CanvasRenderingContext2D | null | undefined
  let fonts: Record<string, { font: string; spacing: number; upper: boolean }> | undefined
  const widths = new Map<string, number>()

  function setup(): boolean {
    if (fonts) return !!ctx
    if (!box) return false
    ctx = document.createElement('canvas').getContext('2d')
    const cs = getComputedStyle(box)
    const label = cs.getPropertyValue('--font-label').trim() || cs.fontFamily
    fonts = {
      group: { font: `11px ${label}`, spacing: 11 * 0.04, upper: true },
      leaf: { font: `12px ${cs.fontFamily}`, spacing: 0, upper: false },
      small: { font: `11px ${cs.fontFamily}`, spacing: 0, upper: false },
    }
    return !!ctx
  }

  function measure(cls: string, text: string): number {
    const key = cls + '|' + text
    let v = widths.get(key)
    if (v === undefined) {
      const f = fonts![cls]
      ctx!.font = f.font
      v = ctx!.measureText(f.upper ? text.toUpperCase() : text).width + f.spacing * [...text].length
      widths.set(key, v)
    }
    return v
  }

  // Returns text if it fits, else the longest prefix + "…" that fits, else '' (needs at least 2 chars + "…").
  function fit(cls: string, text: string, max: number): string {
    if (!setup()) return text
    if (measure(cls, text) <= max) return text
    const chars = [...text]
    let lo = 0
    let hi = chars.length - 1
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1
      if (measure(cls, chars.slice(0, mid).join('').trimEnd() + '…') <= max) lo = mid
      else hi = mid - 1
    }
    return lo >= 2 ? chars.slice(0, lo).join('').trimEnd() + '…' : ''
  }

  function displayName(node: NodeTree): string {
    return node.kind === 'others' ? bt(node.name) : node.name
  }

  function groupLabel(node: NodeTree, w: number): string {
    const max = w - 10
    const name = displayName(node)
    const full = `${name} · ${bytes(node.allocationSize)}`
    if (!setup() || measure('group', full) <= max) return full
    return fit('group', name, max)
  }

  function leafLabel(node: NodeTree, w: number): string {
    return fit('leaf', displayName(node), w - 12)
  }

  function sizeLabel(node: NodeTree, w: number): string {
    const s = bytes(node.allocationSize)
    return !setup() || measure('small', s) <= w - 12 ? s : ''
  }

  function move(event: MouseEvent, node: NodeTree) {
    const box = (event.currentTarget as Element).closest('.treemap')!.getBoundingClientRect()
    hover = { node, x: event.clientX - box.left, y: event.clientY - box.top }
  }
</script>

<div class="treemap" bind:this={box} bind:clientWidth={width} bind:clientHeight={height} role="presentation" onmouseleave={() => (hover = null)}>
  {#if failed}
    <p class="muted empty">{t('无法显示：{error}', { error: failed })}</p>
  {:else if data && data.allocationSize === 0}
    <p class="muted empty">{t('这个位置没有占用空间的文件。')}</p>
  {:else}
    <svg {width} {height} role="img" aria-label={t('空间占用图')}>
      <defs>
        <pattern id="hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <line x1="0" y1="0" x2="0" y2="6" stroke="var(--signal)" stroke-width="1.5" opacity="0.35" />
        </pattern>
        <pattern id="dots" width="5" height="5" patternUnits="userSpaceOnUse">
          <circle cx="2.5" cy="2.5" r="0.7" fill="var(--text)" opacity="0.14" />
        </pattern>
      </defs>
      {#each cells as cell, i (cell.data.path + '|' + cell.data.name + '|' + cell.depth)}
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
          <rect class="base" width={w} height={h} rx="2" fill={fill(cell)} stroke={cell.children ? 'var(--border-strong)' : 'var(--surface)'} stroke-width="1" />
          {#if !cell.children && cell.data.kind !== 'others'}
            <rect width={w} height={h} rx="2" fill="url(#dots)" pointer-events="none" />
          {/if}
          {#if cell.data.incomplete || cell.data.kind === 'others'}
            <rect width={w} height={h} rx="2" fill="url(#hatch)" />
          {/if}
          <clipPath id="clip-{i}"><rect width={w} height={h} /></clipPath>
          <g clip-path="url(#clip-{i})">
            {#if cell.children && w > 40}
              {@const label = groupLabel(cell.data, w)}
              {#if label}<text x="5" y="12.5" class="group">{label}</text>{/if}
            {:else if !cell.children && w > 56 && h > 30}
              {@const name = leafLabel(cell.data, w)}
              {@const size = sizeLabel(cell.data, w)}
              {#if name}<text x="6" y="16" class="leaf">{name}</text>{/if}
              {#if size}<text x="6" y="30" class="leaf small">{size}</text>{/if}
            {/if}
          </g>
        </g>
      {/each}
    </svg>
  {/if}

  {#if hover}
    <div class="tip" style:left="{Math.min(hover.x + 14, width - 260)}px" style:top="{Math.min(hover.y + 14, height - 90)}px">
      <strong>{hover.node.kind === 'others' ? bt(hover.node.name) : hover.node.name}</strong>
      <div>{t('{size} · 占当前位置 {pct}', { size: bytes(hover.node.allocationSize), pct: percent(hover.node.allocationSize, data?.allocationSize ?? 0) })}</div>
      {#if hover.node.kind === 'directory'}<div class="muted">{t('{n} 个文件 · 单击进入', { n: hover.node.files.toLocaleString(numberLocale()) })}</div>{/if}
      {#if hover.node.incomplete}<div class="warn">{t('其中有读不到的位置，大小是下限')}</div>{/if}
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
  g.clickable:hover > rect.base {
    stroke: var(--signal);
    stroke-width: 1.5px;
  }
  text {
    pointer-events: none;
    font-size: 12px;
  }
  .group {
    fill: var(--text);
    font-family: var(--font-label);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .leaf {
    fill: var(--text);
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
    border-radius: 10px;
    padding: 8px 10px;
    box-shadow: var(--shadow-lg);
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
