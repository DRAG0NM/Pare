<script lang="ts">
  import type { ActionResult, PreparedAction } from '../lib/api'
  import { close, exec, execute, GONE_REASON, isGone } from '../lib/execution.svelte'
  import { bytes, middle } from '../lib/format'
  import { execText } from '../lib/labels'

  const MAX_ITEMS = 50
  const kindBadge: Record<PreparedAction['kind'], { text: string; tone: string }> = {
    quarantine: { text: '移入隔离区 · 可撤回', tone: 'accent' },
    template: { text: '受支持动作 · 无法撤回', tone: 'plain' },
    command: { text: 'AI 写的命令 · 无法撤回', tone: 'warn' },
    config: { text: '修改配置 · 需手动', tone: 'plain' },
    manual: { text: '需手动操作', tone: 'plain' },
  }
  const statusIcon: Record<ActionResult['status'], string> = { running: '', done: '✓', partial: '◐', skipped: '–', failed: '✗' }

  let readCmd = $state<Record<string, boolean>>({})
  let irreversibleAck = $state(false)
  let goneOpen = $state<Record<string, boolean>>({})

  const actions = $derived(exec.preview?.actions ?? [])
  const accepted = $derived(actions.filter((a) => a.runnable && exec.accepted[a.key]))
  const irreversible = $derived(accepted.filter((a) => a.kind !== 'quarantine').length)
  const total = $derived(accepted.reduce((s, a) => s + a.bytes, 0))
  const canGo = $derived(accepted.length > 0 && (irreversible === 0 || irreversibleAck))

  function setAccepted(a: PreparedAction, on: boolean) {
    exec.accepted[a.key] = on
  }
  function setRead(a: PreparedAction, on: boolean) {
    readCmd[a.key] = on
    if (!on) exec.accepted[a.key] = false
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && exec.phase === 'confirm') close()
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="backdrop">
  <div class="modal" role="dialog" aria-modal="true" aria-label="确认执行">
    {#if exec.phase === 'preparing'}
      <div class="center"><span class="spinner" aria-hidden="true"></span><span>正在核对每一项的最新状态…</span></div>
    {:else if exec.phase === 'confirm'}
      <h2>{exec.mode === 'prevent' ? '确认执行：防止再长出来' : '确认执行'}</h2>
      {#if exec.error}<p class="warn">{exec.error}</p>{/if}
      {#if actions.length === 0 && !exec.error}<p class="muted">没有可执行的项目。</p>{/if}
      <div class="list">
        {#each actions as a (a.key)}
          {@const badge = kindBadge[a.kind]}
          {@const gone = isGone(a)}
          {@const liveItems = a.items.filter((it) => it.reason !== GONE_REASON)}
          {@const goneItems = a.items.filter((it) => it.reason === GONE_REASON)}
          <section class="card" class:off={!a.runnable} class:gone>
            <div class="head">
              <input type="checkbox" aria-label="执行这一项" checked={!!exec.accepted[a.key]} disabled={!a.runnable || (a.kind === 'command' && !readCmd[a.key])} onchange={(e) => setAccepted(a, e.currentTarget.checked)} />
              <div class="titles">
                <span class="muted small ell">{a.findingTitle}</span>
                <strong>{a.label}</strong>
              </div>
              <span class="badge {badge.tone}">{badge.text}</span>
              {#if a.needsAdmin}<span class="badge warn">需要管理员权限</span>{/if}
              {#if a.bytes > 0}<span class="size">{bytes(a.bytes)}</span>{/if}
            </div>
            {#if gone}<p class="muted small">已不存在，可能已经清理过</p>{/if}
            {#if a.error}<p class="warn small">{a.error}</p>{/if}
            {#if a.kind === 'config' || a.kind === 'manual'}
              <p class="small instr">{a.explain}</p>
              <p class="muted small">这一项需要你自己完成，Pare 不会替你改动。</p>
            {:else}
              {#if a.kind === 'quarantine' && a.items.length}
                <ul class="items">
                  {#each liveItems.slice(0, MAX_ITEMS) as it}
                    <li class="mono" class:bad={!it.executable} title={it.path}>
                      <span class="mark">{it.executable ? '✓' : '✗'}</span>
                      <span class="p">{middle(it.path, 80)}</span>
                      {#if it.executable}<span class="muted">{bytes(it.bytes)}</span>{:else}<span class="warn">{execText(it.reason)}</span>{/if}
                    </li>
                  {/each}
                </ul>
                {#if liveItems.length > MAX_ITEMS}<p class="muted small">…另有 {liveItems.length - MAX_ITEMS} 项</p>{/if}
                {#if goneItems.length}
                  <button class="gone-row small" onclick={() => (goneOpen[a.key] = !goneOpen[a.key])}>
                    已不存在（可能已清理）{goneItems.length} 项 {goneOpen[a.key] ? '▾' : '▸'}
                  </button>
                  {#if goneOpen[a.key]}
                    <ul class="items muted">
                      {#each goneItems.slice(0, MAX_ITEMS) as it}
                        <li class="mono" title={it.path}><span class="p">{middle(it.path, 80)}</span></li>
                      {/each}
                    </ul>
                    {#if goneItems.length > MAX_ITEMS}<p class="muted small">…另有 {goneItems.length - MAX_ITEMS} 项</p>{/if}
                  {/if}
                {/if}
              {/if}
              {#if a.steps.length}<pre class="mono code">{a.steps.join('\n')}</pre>{/if}
              {#if a.warnings.length}
                <ul class="warns small">{#each a.warnings as w}<li>{w}</li>{/each}</ul>
              {/if}
              {#if a.kind === 'command' && a.runnable}
                <label class="inline small">
                  <input type="checkbox" checked={!!readCmd[a.key]} onchange={(e) => setRead(a, e.currentTarget.checked)} />
                  我已逐字读懂这条命令
                </label>
              {/if}
            {/if}
          </section>
        {/each}
      </div>
      <footer>
        <div class="summary">
          <strong>将执行 {accepted.length} 项{exec.mode === 'clean' ? `，预计回收 ${bytes(total)}` : ''}</strong>
          {#if irreversible > 0}
            <label class="inline small warn"><input type="checkbox" bind:checked={irreversibleAck} />我知道其中 {irreversible} 项执行后无法撤回</label>
          {/if}
          {#if exec.mode === 'clean'}<span class="muted small">移入隔离区的项目可以在「隔离区」页恢复；隔离不会立刻腾出空间，清空后才会释放。</span>{/if}
        </div>
        <div class="buttons">
          <button onclick={close}>取消</button>
          <button class="primary" disabled={!canGo} onclick={execute}>确认执行</button>
        </div>
      </footer>
    {:else if exec.phase === 'running'}
      <h2>正在执行…</h2>
      <div class="list">
        {#each accepted as a (a.key)}
          {@const r = exec.progress[a.key]}
          <div class="result">
            <span class="ic">
              {#if !r || r.status === 'running'}<span class="spinner" aria-hidden="true"></span>{:else}{statusIcon[r.status]}{/if}
            </span>
            <div class="rbody">
              <strong class="ell">{a.label}</strong>
              <span class="muted small">{r ? execText(r.detail) : '等待中…'}</span>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 150;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.45);
    animation: fade-in 0.15s;
  }
  .modal {
    width: min(760px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px 20px;
    box-shadow: var(--shadow-lg);
    animation: pop-in 0.22s var(--ease);
  }
  h2 {
    margin: 0;
    font-size: 17px;
  }
  p {
    margin: 0;
  }
  .small {
    font-size: 12px;
  }
  .warn {
    color: var(--warn);
  }
  .ell {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .center {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 40px 0;
  }
  .spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
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
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 8px;
    user-select: text;
  }
  .card {
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .card.off {
    background: var(--surface-2);
  }
  .head {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .titles {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .card.gone {
    background: var(--surface-2);
    opacity: 0.7;
  }
  .gone-row {
    align-self: flex-start;
    border: none;
    background: transparent;
    color: var(--muted);
    padding: 0;
  }
  .badge {
    font-size: 11px;
    padding: 1px 8px;
    border-radius: 10px;
    white-space: nowrap;
    background: var(--surface-2);
    color: var(--muted);
  }
  .badge.accent {
    background: var(--accent-soft);
    color: var(--accent);
  }
  .badge.warn {
    background: var(--warn-soft);
    color: var(--warn);
  }
  .size {
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .instr {
    white-space: pre-wrap;
  }
  .items {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 110px;
    overflow: auto;
  }
  .items li {
    display: flex;
    gap: 8px;
  }
  .items .p {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .items .mark {
    color: var(--safe);
  }
  .items li.bad .mark {
    color: var(--warn);
  }
  .code {
    margin: 0;
    padding: 6px 8px;
    background: var(--surface-2);
    border-radius: 4px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .warns {
    margin: 0;
    padding-left: 18px;
    color: var(--warn);
  }
  .inline {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  footer {
    display: flex;
    align-items: flex-end;
    gap: 16px;
    border-top: 1px solid var(--border);
    padding-top: 10px;
  }
  .summary {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .buttons {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: flex-end;
  }
  .result {
    display: flex;
    gap: 10px;
    padding: 6px 4px;
  }
  .ic {
    width: 18px;
    text-align: center;
    flex: none;
  }
  .rbody {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
  }
</style>
