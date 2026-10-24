<script lang="ts">
  import { onMount } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import Decide from './components/Decide.svelte'
  import { backend, isMock, onEvent, type Progress, type ScanSummary } from './lib/api'
  import Home from './components/Home.svelte'
  import Scanning from './components/Scanning.svelte'
  import Overview from './components/Overview.svelte'
  import AISettings from './components/AISettings.svelte'
  import { answerConsent, initInvestigation, inv, resetInvestigation } from './lib/investigation.svelte'

  type View = 'home' | 'scanning' | 'overview' | 'decide'

  let view = $state<View>('home')
  let target = $state('')
  let progress = $state<Progress>({ entries: 0, directories: 0, current: '', done: false })
  let summary = $state<ScanSummary | null>(null)
  let error = $state('')
  const selection = new SvelteSet<string>()
  let decideTab = $state<'ai' | 'rules'>('ai')
  let showSettings = $state(false)

  function investigate(path: string) {
    inv.scope = path
    decideTab = 'ai'
    view = 'decide'
  }

  onMount(() => {
    initInvestigation()
    const offProgress = onEvent<Progress>('scan:progress', (p) => (progress = p))
    const offDone = onEvent<ScanSummary>('scan:done', (s) => {
      if (s.cancelled) {
        view = summary ? 'overview' : 'home'
        return
      }
      if (s.error) {
        error = s.error
        view = 'home'
        return
      }
      summary = s
      view = 'overview'
    })
    return () => {
      offProgress()
      offDone()
    }
  })

  async function scan(root: string) {
    error = ''
    selection.clear()
    resetInvestigation()
    target = root
    progress = { entries: 0, directories: 0, current: '', done: false }
    try {
      await backend.StartScan(root)
      view = 'scanning'
    } catch (e) {
      error = String(e)
    }
  }

  const stages = [
    { id: 'see', label: '看清', hint: '扫描并查看空间去向' },
    { id: 'decide', label: '决定', hint: '按规则核对后挑选要清理的文件' },
    { id: 'undo', label: '撤回', hint: '下一版本：管理隔离区与撤回' },
  ]
  const stageIndex = $derived(view === 'decide' ? 1 : 0)

  function goStage(i: number) {
    if (!summary) return
    if (i === 0) view = 'overview'
    else if (i === 1) view = 'decide'
  }
</script>

<div class="shell">
  <header>
    <div class="brand">Pare</div>
    <nav>
      {#each stages as stage, i}
        <button class="stage" class:active={i === stageIndex} disabled={i === 2 || !summary || view === 'scanning'} onclick={() => goStage(i)} title={stage.hint}>
          <span class="step">{i + 1}</span>{stage.label}
        </button>
      {/each}
    </nav>
    <div class="right">
      {#if isMock}<span class="mock">演示数据（浏览器预览）</span>{/if}
      <button onclick={() => (showSettings = true)}>设置</button>
    </div>
  </header>

  <main>
    {#if view === 'home'}
      <Home onScan={scan} {error} />
    {:else if view === 'scanning'}
      <Scanning {target} {progress} onCancel={() => backend.CancelScan()} />
    {:else if view === 'decide'}
      <Decide {selection} bind:tab={decideTab} root={summary?.root ?? target} onOpenSettings={() => (showSettings = true)} onBack={() => (view = 'overview')} />
    {:else if summary}
      <Overview {summary} onInvestigate={investigate} onDecide={() => (view = 'decide')} onRescan={() => scan(summary!.root)} onHome={() => (view = 'home')} />
    {/if}
  </main>
</div>

{#if showSettings}
  <AISettings onClose={() => (showSettings = false)} />
{/if}

{#if inv.pendingConsent}
  <div class="backdrop">
    <div class="modal" role="alertdialog" aria-modal="true" aria-label="AI 请求读取文件">
      <h3>AI 想读取一个敏感文件的内容</h3>
      <p class="mono path" title={inv.pendingConsent.path}>{inv.pendingConsent.path}</p>
      <p>{inv.pendingConsent.reason}</p>
      <p class="muted">内容会发送给 AI 服务商。拒绝后 AI 只能根据文件名和大小判断。</p>
      <div class="buttons">
        <button onclick={() => answerConsent(false)}>拒绝</button>
        <button class="primary" onclick={() => answerConsent(true)}>允许这一次</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  header {
    display: flex;
    align-items: center;
    gap: 24px;
    padding: 10px 20px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .brand {
    font-weight: 700;
    font-size: 18px;
    letter-spacing: 0.5px;
  }
  nav {
    display: flex;
    gap: 6px;
  }
  .stage {
    border: none;
    background: transparent;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
  }
  .stage.active {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .step {
    display: inline-grid;
    place-items: center;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: 1px solid currentColor;
    font-size: 12px;
  }
  .right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 200;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.45);
  }
  .modal {
    width: min(460px, calc(100vw - 32px));
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px 20px;
  }
  .modal h3 {
    margin: 0 0 8px;
    font-size: 16px;
  }
  .modal p {
    margin: 6px 0;
  }
  .path {
    word-break: break-all;
    background: var(--surface-2);
    padding: 6px 8px;
    border-radius: 4px;
    user-select: text;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
  }
  .mock {
    font-size: 12px;
    color: var(--warn);
    background: var(--warn-soft);
    padding: 2px 10px;
    border-radius: 10px;
  }
  main {
    flex: 1;
    min-height: 0;
  }
</style>
