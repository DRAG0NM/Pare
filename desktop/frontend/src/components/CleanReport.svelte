<script lang="ts">
  import { Tween } from 'svelte/motion'
  import { cubicOut } from 'svelte/easing'
  import { backend, type ActionResult, type ExecutionReport } from '../lib/api'
  import { exec, openPrevention } from '../lib/execution.svelte'
  import { inv } from '../lib/investigation.svelte'
  import { bytes } from '../lib/format'
  import { execText } from '../lib/labels'
  import { t, numberLocale } from '../lib/i18n.svelte'
  import { bt } from '../lib/backendText'

  let {
    report,
    onUndoPage,
    onRescan,
    onContinue,
    onBack,
    onHome,
  }: { report: ExecutionReport; onUndoPage: () => void; onRescan: () => void; onContinue: () => void; onBack: () => void; onHome: () => void } = $props()

  const kindText: Record<string, string> = { quarantine: '移入隔离区', template: '受支持动作', command: '命令', config: '修改配置', manual: '手动操作' }
  const statusText: Record<ActionResult['status'], string> = { running: '执行中', done: '完成', partial: '部分完成', skipped: '未执行', failed: '失败' }
  const statusIcon: Record<ActionResult['status'], string> = { running: '…', done: '●', partial: '◐', skipped: '○', failed: '●' }

  let purge = $state<'idle' | 'ask' | 'busy' | 'done'>('idle')
  let purgeFreed = $state(0)
  let purgeError = $state('')

  const known = $derived(report.freeBefore > 0)
  const freeNow = $derived(report.freeAfter + purgeFreed)
  const gained = $derived(freeNow - report.freeBefore)
  const shownFree = Tween.of(() => freeNow, { duration: 900, easing: cubicOut })
  const shownGain = Tween.of(() => Math.max(0, gained), { duration: 900, easing: cubicOut })
  const waiting = $derived(purge === 'done' ? 0 : report.quarantinedBytes)
  const counts = $derived({
    ok: report.results.filter((r) => r.status === 'done').length,
    partial: report.results.filter((r) => r.status === 'partial').length,
    rest: report.results.filter((r) => r.status === 'skipped' || r.status === 'failed').length,
  })
  const busy = $derived(exec.phase !== 'idle')

  async function doPurge() {
    purge = 'busy'
    purgeError = ''
    try {
      let freed = 0
      for (const m of report.manifests) freed += (await backend.PurgeBatch(m)).freedBytes
      purgeFreed = Math.max(0, freed)
      purge = 'done'
    } catch (e) {
      purgeError = String(e).replace(/^Error:\s*/, '')
      purge = 'idle'
    }
  }

  function prevention(findingId: string) {
    return inv.findings.find((f) => f.id === findingId)?.prevention ?? []
  }
</script>

<div class="report">
  <div class="head">
    <h2>{t('清理完成')}</h2>
    <span class="muted small">{new Date(report.time).toLocaleString(numberLocale(), { hour12: false })}</span>
  </div>

  <div class="stats">
    <section class="stat hero">
      <span class="caption">{t('{drive} 盘可用空间', { drive: report.volume.replace('\\', '') })}</span>
      {#if known}
        <div class="flow">
          <span class="dot before">{bytes(report.freeBefore)}</span>
          <span class="arrow">→</span>
          <strong class="dot after">{bytes(shownFree.current)}</strong>
        </div>
        {#if gained > 0}<strong class="dot freed">+{bytes(shownGain.current)}</strong>{:else}<span class="muted small">{t('· 暂时没有变化')}</span>{/if}
      {:else}
        <strong class="dot after">—</strong>
        <span class="muted small">{t('读不到磁盘空间')}</span>
      {/if}
    </section>

    <section class="stat">
      <span class="caption">{t('隔离中')}</span>
      <strong class="dot q" class:dim={waiting === 0}>{bytes(waiting)}</strong>
      {#if purge === 'done'}
        <span class="small gain">{t('已永久清空，释放 {size}', { size: bytes(purgeFreed) })}</span>
      {:else if report.manifests.length === 0}
        <span class="muted small">{t('这次没有移入隔离区的文件')}</span>
      {:else if purge === 'ask' || purge === 'busy'}
        <span class="warn small">{t('将永久删除，无法恢复')}</span>
        <div class="row">
          <button disabled={purge === 'busy'} onclick={() => (purge = 'idle')}>{t('先不清空')}</button>
          <button class="primary" disabled={purge === 'busy'} onclick={doPurge}>{purge === 'busy' ? t('清空中…') : t('确定清空')}</button>
        </div>
      {:else}
        <span class="muted small">{t('文件还在盘上，清空后才会释放')}</span>
        <div class="row"><button class="purge" onclick={() => (purge = 'ask')}>{t('立即清空这批')}</button></div>
      {/if}
      {#if purgeError}<span class="warn small">{bt(purgeError)}</span>{/if}
    </section>

    <section class="stat">
      <span class="caption">{t('处理结果')}</span>
      <strong class="dot q">{counts.ok + counts.partial}<span class="unit mono">{t('/ {n} 项', { n: report.results.length })}</span></strong>
      <span class="small muted">{t('完成 {ok} · 部分 {partial} · 未完成 {rest}', { ok: counts.ok, partial: counts.partial, rest: counts.rest })}</span>
    </section>
  </div>

  {#if report.error}<p class="warn">{bt(report.error)}</p>{/if}

  <div class="cols">
    <section class="panel">
      <h3>{t('处理明细')}</h3>
      <ul class="results">
        {#each report.results as r, i (r.key)}
          <li style:animation-delay="{Math.min(i, 8) * 40}ms">
            <span class="ic {r.status}" title={t(statusText[r.status])}>{statusIcon[r.status]}</span>
            <div class="rbody">
              <span class="muted small ell">{bt(r.findingTitle)}</span>
              <div class="line">
                <strong class="ell">{bt(r.label)}</strong>
                <span class="tag">{kindText[r.kind] ? t(kindText[r.kind]) : r.kind}</span>
                {#if r.bytes > 0}<span class="size">{bytes(r.bytes)}</span>{/if}
              </div>
              {#if r.detail}<span class="small" class:warn={r.status !== 'done'}>{execText(r.detail)}</span>{/if}
              {#if r.output}
                <details>
                  <summary class="small muted">{t('查看输出')}</summary>
                  <pre class="mono code">{r.output}</pre>
                </details>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    </section>

    {#if report.tips.length}
      <section class="panel">
        <h3>{t('防止再长出来')}</h3>
        <p class="muted small">{t('这些问题以后还会出现。能自动处理的可以一键执行，执行前同样会给你看具体命令。')}</p>
        {#each report.tips as tip}
          {@const steps = prevention(tip.findingId)}
          <div class="tip">
            <strong class="small">{bt(tip.findingTitle)}</strong>
            <span class="small">{tip.text}</span>
            {#each steps as p, i}
              {@const key = `${tip.findingId}:p${i}`}
              {@const result = exec.prevented[key]}
              <div class="prevent">
                <div class="pbody">
                  <span>{p.label}</span>
                  <span class="muted small">{p.explain}</span>
                  {#if result}
                    <span class="small" class:gain={result.status === 'done'} class:warn={result.status !== 'done'}>
                      {statusIcon[result.status]} {result.status === 'done' ? t('已处理') : execText(result.detail) || t(statusText[result.status])}
                    </span>
                  {/if}
                </div>
                {#if p.kind === 'template' || p.kind === 'command'}
                  {#if result?.status !== 'done'}
                    <button class="primary small-btn" disabled={busy} onclick={() => openPrevention(tip.findingId, i)}>{t('一键处理')}</button>
                  {/if}
                {:else}
                  <span class="muted small nowrap">{t('需要你自己操作')}</span>
                {/if}
              </div>
            {/each}
          </div>
        {/each}
      </section>
    {/if}
  </div>

  <footer>
    <span class="muted small">{t('原来的扫描结果已经过时，重新扫描可以看到最新占用。')}</span>
    <div class="row">
      <button onclick={onUndoPage}>{t('去隔离区')}</button>
      <button onclick={onRescan}>{t('重新扫描')}</button>
      <button onclick={onContinue}>{t('继续深挖剩下的问题')}</button>
      <button onclick={onBack}>{t('返回清理页')}</button>
      <button class="primary" onclick={onHome}>{t('回到主页')}</button>
    </div>
  </footer>
</div>

<style>
  .report {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 20px;
    overflow: auto;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  h2 {
    margin: 0;
    font-size: 20px;
  }
  h3 {
    margin: 0 0 10px;
    font-family: var(--font-label);
    font-size: 11px;
    font-weight: 400;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }
  .small {
    font-size: 12px;
  }
  .warn {
    color: var(--warn);
  }
  button.purge {
    background: transparent;
    border: 1px solid var(--warn);
    color: var(--warn);
  }
  button.purge:hover:not(:disabled) {
    background: var(--warn-soft);
    border-color: var(--warn);
  }
  .gain {
    color: var(--accent);
    font-weight: 600;
  }
  .nowrap {
    white-space: nowrap;
  }
  .stats {
    display: grid;
    grid-template-columns: 1.7fr 1fr 1fr;
    gap: 12px;
  }
  .stat {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 16px 18px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 18px;
    animation: fade-up 0.35s var(--ease) both;
  }
  .stat:nth-child(2) {
    animation-delay: 60ms;
  }
  .stat:nth-child(3) {
    animation-delay: 120ms;
  }
  .stat.hero {
    border-color: var(--border-strong);
  }
  .flow {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px 14px;
    margin: 4px 0;
  }
  .before {
    font-size: 36px;
    line-height: 1;
    color: var(--muted);
  }
  .arrow {
    font-size: 22px;
    color: var(--muted);
  }
  .after {
    font-size: 44px;
    line-height: 1;
    font-weight: 400;
  }
  .freed {
    font-size: 28px;
    line-height: 1;
    font-weight: 400;
  }
  .q {
    font-size: 30px;
    line-height: 1.1;
    font-weight: 400;
  }
  .q.dim {
    color: var(--muted);
  }
  .unit {
    font-size: 14px;
    font-weight: 400;
    color: var(--muted);
    margin-left: 6px;
  }
  .row {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    margin-top: 4px;
  }
  .cols {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 12px;
    align-items: start;
  }
  .panel {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 18px;
    padding: 14px 16px;
    user-select: text;
  }
  .panel > p {
    margin: 0 0 8px;
  }
  .results {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .results li {
    display: flex;
    gap: 10px;
    padding: 8px 0;
    border-top: 1px dashed var(--border-strong);
    animation: fade-up 0.3s var(--ease) both;
  }
  .results li:first-child {
    border-top: none;
  }
  .ic {
    flex: none;
    width: 16px;
    line-height: 20px;
    text-align: center;
    font-size: 13px;
    color: var(--text);
  }
  .ic.skipped,
  .ic.running {
    color: var(--muted);
  }
  .ic.failed {
    color: var(--signal);
  }
  .rbody {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .ell {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag {
    flex: none;
    font-size: 11px;
    padding: 0 8px;
    border-radius: 999px;
    border: 1px solid var(--border-strong);
    background: transparent;
    color: var(--muted);
  }
  .size {
    margin-left: auto;
    flex: none;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .code {
    margin: 4px 0 0;
    padding: 6px 8px;
    background: var(--bg);
    border: 1px dashed var(--border-strong);
    border-radius: 10px;
    max-height: 140px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .tip {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px 0;
    border-top: 1px dashed var(--border-strong);
  }
  .prevent {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: 12px;
    background: var(--bg);
  }
  .pbody {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .small-btn {
    flex: none;
    padding: 4px 12px;
    font-size: 12px;
  }
  footer {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    margin-top: auto;
    padding-top: 10px;
    border-top: 1px solid var(--border);
  }
  footer .row {
    margin: 0 0 0 auto;
  }
  @media (max-width: 760px) {
    .stats {
      grid-template-columns: 1fr;
    }
  }
</style>
