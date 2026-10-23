<script lang="ts">
  import { onMount } from 'svelte'
  import { backend, type Drive } from '../lib/api'
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
  <h1>看看空间都去哪了</h1>
  <p class="muted">选择一个磁盘或文件夹开始扫描。扫描只读取文件信息，不会改动任何文件。</p>

  <div class="drives">
    {#each drives as drive}
      <button class="drive" onclick={() => onScan(drive.root)}>
        <div class="name">
          <strong>{drive.root.replace('\\', '')}</strong>
          <span class="muted">{drive.label || '本地磁盘'}</span>
        </div>
        <div class="meter"><div style:width={percent(drive.total - drive.free, drive.total)}></div></div>
        <div class="muted small">可用 {bytes(drive.free)}，共 {bytes(drive.total)}</div>
      </button>
    {/each}
    <button class="drive folder" onclick={chooseFolder}>
      <strong>选择文件夹…</strong>
      <span class="muted small">只扫描某个目录</span>
    </button>
  </div>

  {#if error}<p class="error">扫描未能开始：{error}</p>{/if}

  {#if elevated === true}
    <p class="muted small note">已以管理员身份运行，可以读取只对系统开放的目录。</p>
  {:else if elevated === false}
    <p class="muted small note warn-note">
      当前不是管理员身份，System Volume Information 等只对系统开放的目录会读不到。它们会在结果里单独列出，不会被算成 0；需要完整结果时，请右键“以管理员身份运行”。
    </p>
  {/if}
</div>

<style>
  .home {
    max-width: 880px;
    margin: 0 auto;
    padding: 56px 24px;
  }
  h1 {
    font-size: 26px;
    margin: 0 0 6px;
  }
  .drives {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 14px;
    margin: 28px 0;
  }
  .drive {
    display: flex;
    flex-direction: column;
    gap: 10px;
    align-items: stretch;
    text-align: left;
    padding: 16px;
    border-radius: var(--radius);
  }
  .drive.folder {
    justify-content: center;
    border-style: dashed;
  }
  .name {
    display: flex;
    gap: 10px;
    align-items: baseline;
    font-size: 16px;
  }
  .meter {
    height: 8px;
    border-radius: 4px;
    background: var(--surface-2);
    overflow: hidden;
  }
  .meter div {
    height: 100%;
    background: var(--accent);
  }
  .small {
    font-size: 12px;
  }
  .error {
    color: var(--warn);
  }
  .note {
    border-top: 1px solid var(--border);
    padding-top: 16px;
  }
  .warn-note {
    color: var(--warn);
  }
</style>
