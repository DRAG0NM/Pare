<script lang="ts">
  import { t } from '../lib/i18n.svelte'

  let { onClose }: { onClose: () => void } = $props()

  // Raw Chinese keys; translated where rendered.
  const steps = [
    {
      icon: '1',
      title: '先设定，再开始深挖',
      points: [
        '范围：留空就是整个扫描位置；只想查某个文件夹时填它的路径。',
        '补充说明：告诉 AI 重点看哪里、哪些不要碰，例如“别动游戏目录”。',
        '深度：标准档几分钟就够；想挖得更细选深入或彻底，用时更长。',
      ],
      tip: '这三项要在开始前设好，深挖过程中不能修改。',
    },
    {
      icon: '2',
      title: '逐条查看发现，选一个处理方案',
      points: [
        '左边是 AI 找到的问题，点一条在右边看原因、证据和会不会再长出来。',
        '每条都已默认选中推荐方案；不想处理的，选最下面的「暂不处理」。',
        '「移入隔离区」可以撤回；模板和命令执行后无法撤回，要多看一眼。',
      ],
      tip: '标着「需谨慎」的项目可能还在被使用，拿不准就先暂不处理。',
    },
    {
      icon: '3',
      title: '确认执行',
      points: [
        '点右下角「下一步：确认执行」，会列出每一项要移动的文件或要运行的命令。',
        '执行前 Pare 会再检查一遍文件是否被占用、有没有变化。',
        '完成后可以看到实际腾出了多少空间，以及怎么防止它再长出来。',
      ],
      tip: '旁边的「常规清理」页按固定规则清理临时文件和包缓存，可以和 AI 深挖一起提交。',
    },
  ]

  let index = $state(0)
  let dontShow = $state(true)
  const step = $derived(steps[index])

  function close() {
    if (dontShow) {
      try {
        localStorage.setItem('pare.guide.clean', 'seen')
      } catch {}
    }
    onClose()
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') close()
    if (e.key === 'ArrowRight' && index < steps.length - 1) index++
    if (e.key === 'ArrowLeft' && index > 0) index--
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="backdrop">
  <div class="modal" role="dialog" aria-modal="true" aria-label={t('怎么使用清理')}>
    <div class="top">
      <span class="kicker">{t('怎么使用清理')}</span>
      <div class="dots" aria-hidden="true">
        {#each steps as _, i}<span class:on={i === index} class:done={i < index}></span>{/each}
      </div>
    </div>
    {#key index}
      <div class="body">
        <div class="badge">{step.icon}</div>
        <div class="text">
          <h2>{t(step.title)}</h2>
          <ul>
            {#each step.points as p}<li>{t(p)}</li>{/each}
          </ul>
          <p class="tip">{t(step.tip)}</p>
        </div>
      </div>
    {/key}
    <div class="foot">
      <label class="small muted"><input type="checkbox" bind:checked={dontShow} />{t('下次不再自动显示')}</label>
      <div class="buttons">
        {#if index > 0}<button onclick={() => index--}>{t('上一步')}</button>{/if}
        {#if index < steps.length - 1}
          <button class="primary" onclick={() => index++}>{t('下一步')}</button>
        {:else}
          <button class="primary" onclick={close}>{t('开始使用')}</button>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 120;
    display: grid;
    place-items: center;
    background: rgba(0, 0, 0, 0.45);
    animation: fade-in 0.15s;
  }
  .modal {
    width: min(560px, calc(100vw - 32px));
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 14px;
    box-shadow: var(--shadow-lg);
    padding: 18px 22px 16px;
    animation: pop-in 0.22s var(--ease);
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .kicker {
    font-size: 12px;
    color: var(--accent);
    font-weight: 600;
    letter-spacing: 0.5px;
  }
  .dots {
    display: flex;
    gap: 6px;
  }
  .dots span {
    width: 18px;
    height: 4px;
    border-radius: 2px;
    background: var(--surface-2);
    transition: background-color 0.2s;
  }
  .dots span.done,
  .dots span.on {
    background: var(--accent);
  }
  .body {
    display: flex;
    gap: 16px;
    margin: 14px 0 6px;
    animation: fade-up 0.25s var(--ease);
    min-height: 210px;
  }
  .badge {
    flex: none;
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    font-size: 20px;
    font-weight: 700;
    background: var(--accent-soft);
    color: var(--accent);
  }
  h2 {
    margin: 4px 0 10px;
    font-size: 18px;
  }
  ul {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .tip {
    margin: 12px 0 0;
    padding: 8px 10px;
    border-radius: 8px;
    background: var(--warn-soft);
    color: var(--warn);
    font-size: 12px;
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border-top: 1px solid var(--border);
    padding-top: 12px;
  }
  .foot label {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .small {
    font-size: 12px;
  }
  .buttons {
    display: flex;
    gap: 8px;
  }
</style>
