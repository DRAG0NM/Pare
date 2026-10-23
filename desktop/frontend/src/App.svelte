<script lang="ts">
  import { onMount } from 'svelte'
  import { SvelteSet } from 'svelte/reactivity'
  import Decide from './components/Decide.svelte'
  import { backend, isMock, onEvent, type Progress, type ScanSummary } from './lib/api'
  import Home from './components/Home.svelte'
  import Scanning from './components/Scanning.svelte'
  import Overview from './components/Overview.svelte'

  type View = 'home' | 'scanning' | 'overview' | 'decide'

  let view = $state<View>('home')
  let target = $state('')
  let progress = $state<Progress>({ entries: 0, directories: 0, current: '', done: false })
  let summary = $state<ScanSummary | null>(null)
  let error = $state('')
  const selection = new SvelteSet<string>()

  onMount(() => {
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
    {#if isMock}<span class="mock">演示数据（浏览器预览）</span>{/if}
  </header>

  <main>
    {#if view === 'home'}
      <Home onScan={scan} {error} />
    {:else if view === 'scanning'}
      <Scanning {target} {progress} onCancel={() => backend.CancelScan()} />
    {:else if view === 'decide'}
      <Decide {selection} onBack={() => (view = 'overview')} />
    {:else if summary}
      <Overview {summary} onDecide={() => (view = 'decide')} onRescan={() => scan(summary!.root)} onHome={() => (view = 'home')} />
    {/if}
  </main>
</div>

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
  .mock {
    margin-left: auto;
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
