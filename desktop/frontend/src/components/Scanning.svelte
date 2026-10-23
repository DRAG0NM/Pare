<script lang="ts">
  import { onMount } from 'svelte'
  import type { Progress } from '../lib/api'
  import { count, middle, seconds } from '../lib/format'

  let { target, progress, onCancel }: { target: string; progress: Progress; onCancel: () => void } = $props()
  let elapsed = $state(0)
  let cancelling = $state(false)

  onMount(() => {
    const started = Date.now()
    const timer = setInterval(() => (elapsed = (Date.now() - started) / 1000), 200)
    return () => clearInterval(timer)
  })
</script>

<div class="scanning">
  <div class="spinner" aria-hidden="true"></div>
  <h2>正在扫描 {target}</h2>
  <div class="stats">
    <div><strong>{count(progress.entries)}</strong><span class="muted">个条目</span></div>
    <div><strong>{count(progress.directories)}</strong><span class="muted">个目录</span></div>
    <div><strong>{seconds(elapsed)}</strong><span class="muted">已用时间</span></div>
  </div>
  <p class="mono muted current">{middle(progress.current || '准备中…', 90)}</p>
  <button disabled={cancelling} onclick={() => ((cancelling = true), onCancel())}>
    {cancelling ? '正在取消…' : '取消扫描'}
  </button>
</div>

<style>
  .scanning {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding-top: 12vh;
    gap: 18px;
  }
  h2 {
    margin: 0;
    font-weight: 600;
  }
  .stats {
    display: flex;
    gap: 48px;
  }
  .stats div {
    display: flex;
    flex-direction: column;
    align-items: center;
  }
  .stats strong {
    font-size: 28px;
    font-variant-numeric: tabular-nums;
  }
  .current {
    height: 20px;
    margin: 0;
  }
  .spinner {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: 3px solid var(--surface-2);
    border-top-color: var(--accent);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
