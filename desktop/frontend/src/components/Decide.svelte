<script lang="ts">
  import type { SvelteSet } from 'svelte/reactivity'
  import Investigate from './Investigate.svelte'
  import RuleCandidates from './RuleCandidates.svelte'

  let {
    selection,
    onBack,
    root,
    onOpenSettings,
    tab = $bindable('ai'),
  }: { selection: SvelteSet<string>; onBack: () => void; root: string; onOpenSettings: () => void; tab?: 'ai' | 'rules' } = $props()
</script>

<div class="decide-page">
  <div class="tabs" role="tablist">
    <button role="tab" aria-selected={tab === 'ai'} class:active={tab === 'ai'} onclick={() => (tab = 'ai')}>AI 深挖</button>
    <button role="tab" aria-selected={tab === 'rules'} class:active={tab === 'rules'} onclick={() => (tab = 'rules')}>常规清理</button>
  </div>
  <div class="body">
    {#if tab === 'ai'}
      <Investigate {root} {onOpenSettings} {selection} />
    {:else}
      <RuleCandidates {selection} {onBack} />
    {/if}
  </div>
</div>

<style>
  .decide-page {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .tabs {
    display: flex;
    gap: 4px;
    padding: 10px 20px 0;
    border-bottom: 1px solid var(--border);
  }
  .tabs button {
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    background: transparent;
    padding: 8px 14px;
    color: var(--muted);
  }
  .tabs button:hover:not(.active) {
    background: transparent;
    color: var(--text);
  }
  .tabs button.active {
    color: var(--accent);
    font-weight: 600;
    border-bottom-color: var(--accent);
  }
  .body {
    flex: 1;
    min-height: 0;
  }
</style>
