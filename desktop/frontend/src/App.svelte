<script lang="ts">
  import { onMount } from 'svelte'
  import { onLocaleChange, t } from './lib/i18n.svelte'
  import { bt } from './lib/backendText'
  import { SvelteSet } from 'svelte/reactivity'
  import Decide from './components/Decide.svelte'
  import { backend, isMock, onEvent, type Progress, type ScanSummary } from './lib/api'
  import Home from './components/Home.svelte'
  import Logo from './components/Logo.svelte'
  import Scanning from './components/Scanning.svelte'
  import Overview from './components/Overview.svelte'
  import AISettings from './components/AISettings.svelte'
  import UndoPage from './components/UndoPage.svelte'
  import ConfirmExecution from './components/ConfirmExecution.svelte'
  import CleanReport from './components/CleanReport.svelte'
  import CleanGuide from './components/CleanGuide.svelte'
  import { exec, initExecution, setRulesDoneHandler, setSummaryHandler } from './lib/execution.svelte'
  import { answerConsent, initInvestigation, inv, resetInvestigation, startInvestigation } from './lib/investigation.svelte'

  type View = 'home' | 'scanning' | 'overview' | 'decide' | 'report' | 'undo'

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
  let showGuide = $state(false)
  let guidePending = false

  // The clean-page walkthrough opens once, unless the user unticked "don't show again".
  function maybeGuide() {
    try {
      if (localStorage.getItem('pare.guide.clean') === 'seen') return
    } catch {}
    showGuide = true
  }

  const introText = '要让 AI 帮你找出这台电脑上的空间浪费，需要先填写一个 AI 服务的 API Key。推荐 DeepSeek：注册后在控制台创建 Key，粘贴到下面即可。'

  $effect(() => {
    if (view !== 'decide' || autoPrompted) return
    autoPrompted = true
    backend
      .GetAISettings()
      .then((s) => {
        if (!s.hasKey) {
          settingsIntro = t(introText)
          showSettings = true
          guidePending = true
        } else maybeGuide()
      })
      .catch(() => maybeGuide())
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
    onLocaleChange((locale) => backend.SetLanguage(locale).catch(() => {}))
    initInvestigation()
    initExecution()
    setRulesDoneHandler(() => selection.clear())
    setSummaryHandler(() => (view = 'report'))
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
    decideTab = 'ai'
    view = 'decide'
    startInvestigation(true)
  }

  const stages = [
    { id: 'see', label: '扫描', hint: '扫描并查看空间去向' },
    { id: 'decide', label: '清理', hint: '让 AI 深挖空间浪费，或按规则清理常规缓存' },
    { id: 'undo', label: '隔离区', hint: '撤回清理过的文件，或永久清空' },
  ]
  const stageIndex = $derived(view === 'decide' || view === 'report' ? 1 : view === 'undo' ? 2 : 0)

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
      <Logo size={22} />
      <span class="wordmark">PARE</span>
    </div>
    <nav>
      {#each stages as stage, i}
        {#if i > 0}<span class="sep" aria-hidden="true"></span>{/if}
        <button class="stage" class:active={i === stageIndex} disabled={(i === 1 && !summary) || view === 'scanning'} onclick={() => goStage(i)} title={t(stage.hint)}>
          <span class="step">0{i + 1}</span>{t(stage.label)}{#if i === stageIndex}<span class="led" aria-hidden="true"></span>{/if}
        </button>
      {/each}
    </nav>
    <div class="right">
      {#if isMock}<span class="mock">{t('演示数据（浏览器预览）')}</span>{/if}
      <button onclick={openSettings}>{t('设置')}</button>
    </div>
  </header>

  <main>
    {#if view === 'home'}
      <Home onScan={scan} error={bt(error)} />
    {:else if view === 'scanning'}
      <Scanning {target} {progress} onCancel={() => backend.CancelScan()} />
    {:else if view === 'decide'}
      <Decide {selection} bind:tab={decideTab} root={summary?.root ?? target} onOpenSettings={openSettings} onGuide={() => (showGuide = true)} onBack={() => (view = 'overview')} />
    {:else if view === 'report' && exec.summary}
      <CleanReport
        report={exec.summary}
        onUndoPage={() => (view = 'undo')}
        onRescan={() => scan(summary?.root ?? target)}
        onContinue={continueInvestigation}
        onBack={() => (view = 'decide')}
        onHome={() => (view = 'home')}
      />
    {:else if view === 'undo'}
      <UndoPage />
    {:else if summary}
      <Overview {summary} onInvestigate={investigate} onDecide={() => (view = 'decide')} onRescan={() => scan(summary!.root)} onHome={() => (view = 'home')} />
    {/if}
  </main>
</div>

{#if showSettings}
  <AISettings
    intro={settingsIntro}
    onClose={() => {
      showSettings = false
      if (guidePending) (guidePending = false), maybeGuide()
    }}
  />
{/if}

{#if showGuide}
  <CleanGuide onClose={() => (showGuide = false)} />
{/if}

{#if exec.phase !== 'idle'}
  <ConfirmExecution />
{/if}

{#if inv.pendingConsent}
  <div class="backdrop">
    <div class="modal" role="alertdialog" aria-modal="true" aria-label={t('AI 请求读取文件')}>
      <h3>{t('AI 想读取一个敏感文件的内容')}</h3>
      <p class="mono path" title={inv.pendingConsent.path}>{inv.pendingConsent.path}</p>
      <p>{bt(inv.pendingConsent.reason)}</p>
      <p class="muted">{t('内容会发送给 AI 服务商。拒绝后 AI 只能根据文件名和大小判断。')}</p>
      <div class="buttons">
        <button onclick={() => answerConsent(false)}>{t('拒绝')}</button>
        <button class="primary" onclick={() => answerConsent(true)}>{t('允许这一次')}</button>
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
    padding: 9px 20px;
    background: var(--surface);
    border-bottom: 1px solid var(--border);
    position: relative;
    z-index: 1;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .wordmark {
    font-family: var(--font-label);
    font-weight: 700;
    font-size: 15px;
    letter-spacing: 0.32em;
  }
  nav {
    display: flex;
    align-items: center;
    gap: 2px;
  }
  .sep {
    width: 14px;
    height: 1px;
    background: repeating-linear-gradient(90deg, var(--border-strong) 0 2px, transparent 2px 4px);
  }
  .stage {
    border: 1px solid transparent;
    background: transparent;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .stage:hover:not(:disabled) {
    color: var(--text);
    border-color: transparent;
  }
  .stage.active,
  .stage.active:hover:not(:disabled) {
    color: var(--text);
    border-color: var(--border-strong);
    font-weight: 600;
  }
  .step {
    font-family: var(--font-label);
    font-size: 11px;
    letter-spacing: 0.04em;
    opacity: 0.7;
  }
  .stage .led {
    animation: blink 2.4s steps(1) infinite;
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
    background: rgb(0 0 0 / 0.5);
    backdrop-filter: blur(6px);
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
    color: var(--muted);
    border: 1px dashed var(--border-strong);
    padding: 2px 10px;
    border-radius: 999px;
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
