<script lang="ts">
  import { onMount } from 'svelte'
  import type { Progress } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
  import { count, middle } from '../lib/format'

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
  <!-- Twelve dots light up in turn, like a ring of indicator LEDs. -->
  <div class="ring" aria-hidden="true">
    {#each Array(12) as _, i}
      <span style:transform="rotate({i * 30}deg) translateY(-26px)" style:animation-delay="{(i - 12) * 0.1}s" class:red={i === 0}></span>
    {/each}
  </div>
  <h2>{t('正在扫描 {target}', { target })}</h2>
  <div class="stats">
    <div><strong class="dot">{count(progress.entries)}</strong><span class="caption">{t('个条目')}</span></div>
    <div><strong class="dot">{count(progress.directories)}</strong><span class="caption">{t('个目录')}</span></div>
    <div><strong class="dot">{elapsed.toFixed(1)}</strong><span class="caption">{t('已用时间')} · s</span></div>
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
    font-size: 40px;
    line-height: 1.1;
  }
  .current {
    height: 20px;
    margin: 0;
  }
  .ring {
    position: relative;
    width: 64px;
    height: 64px;
  }
  .ring span {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 7px;
    height: 7px;
    margin: -3.5px;
    border-radius: 50%;
    background: var(--text);
    opacity: 0.12;
    animation: lit 1.2s steps(1) infinite;
  }
  .ring span.red {
    background: var(--signal);
  }
  @keyframes lit {
    0%,
    25% {
      opacity: 1;
    }
    26% {
      opacity: 0.12;
    }
  }
  .stats div {
    min-width: 150px;
    padding: 14px 20px 12px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 20px;
    gap: 2px;
  }
</style>
