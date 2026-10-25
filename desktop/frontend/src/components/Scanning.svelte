<script lang="ts">
  import { onMount } from 'svelte'
  import type { Progress } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
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
  <div class="spinner" aria-hidden="true"><span></span></div>
  <h2>{t('正在扫描 {target}', { target })}</h2>
  <div class="stats">
    <div><strong>{count(progress.entries)}</strong><span class="muted">{t('个条目')}</span></div>
    <div><strong>{count(progress.directories)}</strong><span class="muted">{t('个目录')}</span></div>
    <div><strong>{seconds(elapsed)}</strong><span class="muted">{t('已用时间')}</span></div>
  </div>
  <p class="mono muted current">{middle(progress.current || t('准备中…'), 90)}</p>
  <button disabled={cancelling} onclick={() => ((cancelling = true), onCancel())}>
    {cancelling ? t('正在取消…') : t('取消扫描')}
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
    gap: 14px;
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
    position: relative;
    width: 56px;
    height: 56px;
  }
  .spinner::before,
  .spinner span {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: 50%;
  }
  .spinner::before {
    background: var(--accent-soft);
    animation: halo 1.8s var(--ease) infinite;
  }
  .spinner span {
    border: 3px solid var(--surface-2);
    border-top-color: var(--accent);
    border-right-color: var(--accent);
    animation: spin 1s linear infinite;
  }
  @keyframes halo {
    0% {
      transform: scale(0.85);
      opacity: 0.9;
    }
    100% {
      transform: scale(1.6);
      opacity: 0;
    }
  }
  .stats div {
    min-width: 110px;
    padding: 10px 16px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
</style>
