<script lang="ts">
  import { onMount } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import Decide from './components/Decide.svelte'
  import { backend, isMock, onEvent, type Progress, type ScanSummary } from './lib/api'
  import Home from './components/Home.svelte'
  import Scanning from './components/Scanning.svelte'
  import Overview from './components/Overview.svelte'
  import AISettings from './components/AISettings.svelte'
  import UndoPage from './components/UndoPage.svelte'
  import ConfirmExecution from './components/ConfirmExecution.svelte'
  import { close as closeExec, exec, initExecution, setRulesDoneHandler } from './lib/execution.svelte'
  import { answerConsent, initInvestigation, inv, resetInvestigation, startInvestigation } from './lib/investigation.svelte'

  type View = 'home' | 'scanning' | 'overview' | 'decide' | 'undo'

  let view = $state<View>('home')
  let target = $state('')
  let progress = $state<Progress>({ entries: 0, directories: 0, current: '', done: false })
  let summary = $state<ScanSummary | null>(null)
  let error = $state('')
  const selection = new SvelteSet<string>()
  let decideTab = $state<'ai' | 'rules'>('ai')
  let showSettings = $state(false)
  let settingsIntro = $state('')
  let autoPrompted = false

  const introText = '要让 AI 帮你找出这台电脑上的空间浪费，需要先填写一个 AI 服务的 API Key。推荐 DeepSeek：注册后在控制台创建 Key，粘贴到下面即可。'

  $effect(() => {
    if (view !== 'decide' || autoPrompted) return
    autoPrompted = true
    backend
      .GetAISettings()
      .then((s) => {
        if (!s.hasKey) {
          settingsIntro = introText
          showSettings = true
        }
      })
      .catch(() => {})
  })

  function openSettings() {
    settingsIntro = ''
    showSettings = true
  }

  function investigate(path: string) {
    inv.scope = path
    decideTab = 'ai'
    view = 'decide'
  }

  onMount(() => {
    initInvestigation()
    initExecution()
    setRulesDoneHandler(() => selection.clear())
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

  function continueInvestigation() {
    closeExec()
    decideTab = 'ai'
    view = 'decide'
    startInvestigation(true)
  }

  const stages = [
    { id: 'see', label: '扫描', hint: '扫描并查看空间去向' },
    { id: 'decide', label: '清理', hint: '让 AI 深挖空间浪费，或按规则清理常规缓存' },
    { id: 'undo', label: '隔离区', hint: '撤回清理过的文件，或永久清空' },
  ]
  const stageIndex = $derived(view === 'decide' ? 1 : view === 'undo' ? 2 : 0)

  // 扫描 always works: without a scan result it goes back to the drive list.
  function goStage(i: number) {
    if (i === 2) view = 'undo'
    else if (i === 0) view = summary ? 'overview' : 'home'
    else if (summary) view = 'decide'
  }
</script>

<div class="shell">
  <header>
    <div class="brand">
      <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
        <rect x="2" y="2" width="20" height="20" rx="6" fill="var(--accent)" />
        <path d="M7 16.5V7.5h5.2a3.2 3.2 0 0 1 0 6.4H7" fill="none" stroke="var(--on-accent)" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
      Pare
    </div>
    <nav>
      {#each stages as stage, i}
        {#if i > 0}<span class="sep" aria-hidden="true"></span>{/if}
        <button class="stage" class:active={i === stageIndex} disabled={(i === 1 && !summary) || view === 'scanning'} onclick={() => goStage(i)} title={stage.hint}>
          <span class="step">{i + 1}</span>{stage.label}
        </button>
      {/each}
    </nav>
    <div class="right">
      {#if isMock}<span class="mock">演示数据（浏览器预览）</span>{/if}
      <button onclick={openSettings}>设置</button>
    </div>
  </header>

  <main>
    {#if view === 'home'}
      <Home onScan={scan} {error} />
    {:else if view === 'scanning'}
      <Scanning {target} {progress} onCancel={() => backend.CancelScan()} />
    {:else if view === 'decide'}
      <Decide {selection} bind:tab={decideTab} root={summary?.root ?? target} onOpenSettings={openSettings} onBack={() => (view = 'overview')} />
    {:else if view === 'undo'}
      <UndoPage />
    {:else if summary}
      <Overview {summary} onInvestigate={investigate} onDecide={() => (view = 'decide')} onRescan={() => scan(summary!.root)} onHome={() => (view = 'home')} />
    {/if}
  </main>
</div>

{#if showSettings}
  <AISettings intro={settingsIntro} onClose={() => (showSettings = false)} />
{/if}

{#if exec.phase !== 'idle'}
  <ConfirmExecution
    onUndoPage={() => (view = 'undo')}
    onRescan={() => scan(summary?.root ?? target)}
    onContinue={continueInvestigation}
  />
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
    padding: 8px 20px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    box-shadow: var(--shadow);
    position: relative;
    z-index: 1;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 700;
    font-size: 17px;
    letter-spacing: 0.3px;
  }
  nav {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .sep {
    width: 18px;
    height: 1px;
    background: var(--border-strong);
  }
  .stage {
    border: none;
    background: transparent;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 12px 5px 6px;
    border-radius: 999px;
    color: var(--muted);
    white-space: nowrap;
  }
  .stage:hover:not(:disabled) {
    color: var(--text);
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
    background: var(--surface-2);
    font-size: 12px;
    font-weight: 600;
    transition: background-color 0.2s, color 0.2s;
  }
  .stage.active .step {
    background: var(--accent);
    color: var(--on-accent);
  }
  .right button {
    white-space: nowrap;
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
    animation: fade-in 0.15s;
  }
  .modal {
    width: min(460px, calc(100vw - 32px));
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 16px 20px;
    box-shadow: var(--shadow-lg);
    animation: pop-in 0.22s var(--ease);
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
    white-space: nowrap;
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
  /* Each view remounts on navigation, so this replays on every page change. */
  main > :global(*) {
    animation: fade-up 0.25s var(--ease);
  }
</style>
