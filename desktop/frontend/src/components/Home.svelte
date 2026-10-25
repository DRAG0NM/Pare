<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type Drive } from '../lib/api'
  import { t } from '../lib/i18n.svelte'
  import { bytes, percent } from '../lib/format'

  let { onScan, error }: { onScan: (root: string) => void; error: string } = $props()
  let drives = $state<Drive[]>([])
  let elevated = $state<boolean | null>(null)

  onMount(async () => {
    drives = (await backend.Drives()) ?? []
    elevated = await backend.Elevated()
  })

  async function chooseFolder() {
    const folder = await backend.ChooseFolder()
    if (folder) onScan(folder)
  }
</script>

<div class="home">
  <h1>{t('看看空间都去哪了')}</h1>
  <p class="muted">{t('选择一个磁盘或文件夹开始扫描。扫描只读取文件信息，不会改动任何文件。')}</p>

  <div class="drives">
    {#each drives as drive}
      {@const full = drive.total > 0 && drive.free / drive.total < 0.1}
      <button class="drive" onclick={() => onScan(drive.root)}>
        <div class="name">
          <strong class="dot letter">{drive.root.replace('\\', '')}</strong>
          <span class="muted">{drive.label || t('本地磁盘')}</span>
          <span class="pct caption" class:full>{percent(drive.total - drive.free, drive.total)}</span>
        </div>
        <div class="meter dotted" class:full><div style:width={percent(drive.total - drive.free, drive.total)}></div></div>
        <div class="muted small">{t('可用 {free}，共 {total}', { free: bytes(drive.free), total: bytes(drive.total) })}</div>
      </button>
    {/each}
    <button class="drive folder" onclick={chooseFolder}>
      <span class="plus" aria-hidden="true">+</span>
      <strong>{t('选择文件夹…')}</strong>
      <span class="muted small">{t('只扫描某个目录')}</span>
    </button>
  </div>

  {#if error}<p class="error">{t('扫描未能开始：{error}', { error })}</p>{/if}

  {#if elevated === true}
    <p class="muted small note">{t('已以管理员身份运行，可以读取只对系统开放的目录。')}</p>
  {:else if elevated === false}
    <p class="muted small note warn-note">
      <span class="led" aria-hidden="true"></span>
      {t('当前不是管理员身份，System Volume Information 等只对系统开放的目录会读不到。它们会在结果里单独列出，不会被算成 0；需要完整结果时，请右键“以管理员身份运行”。')}
    </p>
  {/if}
</div>

<style>
  .home {
    max-width: 920px;
    margin: 0 auto;
    padding: 9vh 24px 56px;
  }
  h1 {
    font-size: 34px;
    font-weight: 600;
    letter-spacing: -0.01em;
    margin: 0 0 8px;
  }
  .drives {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 12px;
    margin: 36px 0 28px;
  }
  .drive {
    display: flex;
    flex-direction: column;
    gap: 14px;
    align-items: stretch;
    text-align: left;
    padding: 18px 20px;
    border-radius: 20px;
    border-color: var(--border);
    font-weight: 400;
    animation: fade-up 0.3s var(--ease) both;
  }
  .letter {
    font-size: 30px;
    line-height: 1;
  }
  .pct {
    margin-left: auto;
  }
  .pct.full {
    color: var(--signal);
  }
  .plus {
    font-family: var(--font-label);
    font-size: 24px;
    line-height: 1;
    color: var(--muted);
  }
  .drive:nth-child(2) {
    animation-delay: 0.04s;
  }
  .drive:nth-child(3) {
    animation-delay: 0.08s;
  }
  .drive:nth-child(n + 4) {
    animation-delay: 0.12s;
  }
  .drive:hover:not(:disabled) {
    border-color: var(--text);
    background: var(--surface);
    transform: translateY(-1px);
  }
  .drive.folder {
    justify-content: center;
    gap: 6px;
    border-style: dashed;
    border-color: var(--border-strong);
    background: transparent;
  }
  .name {
    display: flex;
    gap: 10px;
    align-items: baseline;
    font-size: 14px;
  }
  .meter {
    height: 7px;
    background: var(--surface-2);
  }
  .meter div {
    height: 100%;
    background: var(--bar);
    animation: grow 0.8s steps(24) both;
    transform-origin: left;
  }
  .meter.full div {
    background: var(--signal);
  }
  @keyframes grow {
    from {
      transform: scaleX(0);
    }
  }
  .small {
    font-size: 12px;
  }
  .error {
    color: var(--warn);
  }
  .note {
    border-top: 1px dashed var(--border-strong);
    padding-top: 16px;
  }
  .warn-note .led {
    margin-right: 6px;
    vertical-align: 1px;
  }
</style>
