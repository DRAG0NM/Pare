// A deterministic fake disk for running the UI in a browser. Never used in the desktop app.
import type { AIEvent, AISettingsView, Backend, Finding, CandidateGroup, CandidateItem, CandidateView, Drive, Evidence, NodeTree, NodeView, Progress, ScanSummary } from './api'

type FakeNode = { name: string; kind: string; size: number; children: FakeNode[]; incomplete?: boolean }

const GB = 1024 ** 3
const MB = 1024 ** 2

function folder(name: string, children: FakeNode[], incomplete = false): FakeNode {
  return { name, kind: 'directory', size: 0, children, incomplete }
}
function file(name: string, size: number): FakeNode {
  return { name, kind: 'file', size, children: [] }
}
function many(prefix: string, count: number, base: number): FakeNode[] {
  return Array.from({ length: count }, (_, i) => file(`${prefix}-${i + 1}.bin`, base * (1 + ((i * 7) % 11))))
}

const disk = folder('D:\\', [
  folder('Steam', [
    folder('steamapps', [
      folder('common', [
        folder('Cyberpunk 2077', many('archive', 24, 2.4 * GB)),
        folder('Baldurs Gate 3', many('pak', 18, 3.1 * GB)),
        folder('Detroit Become Human', many('data', 12, 2.2 * GB)),
        folder('Hades', many('pkg', 30, 40 * MB)),
      ]),
      folder('shadercache', many('cache', 40, 30 * MB)),
    ]),
  ]),
  folder('Users', [
    folder('me', [
      folder('Videos', many('clip', 40, 600 * MB)),
      folder('Downloads', [...many('setup', 14, 180 * MB), file('dataset.zip', 4.2 * GB)]),
      folder('AppData', [folder('Local', [folder('Temp', many('tmp', 120, 4 * MB)), folder('npm-cache', many('pkg', 300, 300 * 1024))])]),
    ]),
  ]),
  folder('Projects', [folder('pare', many('src', 80, 20 * 1024)), folder('node_modules', many('mod', 200, 900 * 1024))]),
  folder('System Volume Information', [], true),
  file('pagefile-like.sys', 8 * GB),
])

type Indexed = FakeNode & { path: string; total: number; files: number; directories: number; incompleteTree: boolean }
const index = new Map<string, Indexed>()

function build(node: FakeNode, path: string): Indexed {
  const indexed = node as Indexed
  indexed.path = path
  indexed.total = node.size
  indexed.files = node.kind === 'file' ? 1 : 0
  indexed.directories = 0
  indexed.incompleteTree = !!node.incomplete
  for (const child of node.children) {
    const built = build(child, path.endsWith('\\') ? path + child.name : `${path}\\${child.name}`)
    indexed.total += built.total
    indexed.files += built.files
    indexed.directories += built.directories + (child.kind === 'directory' ? 1 : 0)
    indexed.incompleteTree ||= built.incompleteTree
  }
  node.children.sort((a, b) => (b as Indexed).total - (a as Indexed).total)
  index.set(path, indexed)
  return indexed
}
const root = build(disk, 'D:\\')

function view(node: Indexed): NodeView {
  return {
    path: node.path,
    name: node.path === root.path ? node.path : node.name,
    kind: node.kind,
    logicalSize: node.total * 0.98,
    allocationSize: node.total,
    files: node.files,
    directories: node.directories,
    incomplete: node.incompleteTree,
    hasChildren: node.children.length > 0,
    others: 0,
  }
}

function trimmed(children: FakeNode[], limit: number): NodeView[] {
  const shown = (children as Indexed[]).slice(0, limit).map(view)
  const rest = (children as Indexed[]).slice(limit)
  if (rest.length) {
    const total = rest.reduce((sum, child) => sum + child.total, 0)
    shown.push({
      path: '', name: `其他 ${rest.length} 项`, kind: 'others', logicalSize: total, allocationSize: total,
      files: rest.reduce((sum, child) => sum + child.files, 0), directories: 0, incomplete: false, hasChildren: false, others: rest.length,
    })
  }
  return shown
}

function subtree(node: Indexed, depth: number, limit: number): NodeTree {
  const result: NodeTree = { ...view(node), children: [] }
  if (depth <= 0) return result
  for (const child of (node.children as Indexed[]).slice(0, limit)) result.children.push(subtree(child, depth - 1, limit))
  for (const other of trimmed(node.children, limit).slice(Math.min(limit, node.children.length))) result.children.push({ ...other, children: [] })
  return result
}

const DAY = 86_400_000

function mockGroup(ruleId: string, description: string, minAgeDays: number, folder: string): CandidateGroup {
  const files = (index.get(folder)?.children ?? []) as Indexed[]
  const items: CandidateItem[] = files.map((file, i) => {
    const age = i % 4 === 0 ? 2 : 30 + (i % 50)
    const evidence: Evidence[] = [
      { kind: 'path_rule', status: 'pass', detail: description },
      { kind: 'attributes', status: 'pass', detail: 'not a system file' },
      i === 7
        ? { kind: 'file_identity', status: 'fail', detail: 'file ID appears at 2 paths (hard link)' }
        : { kind: 'file_identity', status: 'pass', detail: 'single path for file ID' },
      { kind: 'age', status: age >= minAgeDays ? 'pass' : 'fail', detail: `written ${age} days ago, rule requires ${minAgeDays}` },
    ]
    if (evidence.every((e) => e.status === 'pass')) {
      const locked = i % 13 === 5
      evidence.push(
        locked ? { kind: 'unchanged', status: 'unknown', detail: 'file is open in another process' } : { kind: 'unchanged', status: 'pass', detail: 'file ID, size and last write time match the scan' },
        locked ? { kind: 'unlocked', status: 'fail', detail: 'file is open in another process' } : { kind: 'unlocked', status: 'pass', detail: 'exclusive read open succeeded' },
      )
    }
    return {
      path: file.path, name: file.name, directory: folder, logicalSize: file.total * 0.98, allocationSize: file.total,
      lastWriteTime: new Date(Date.now() - age * DAY).toISOString(), risk: 'E1',
      executable: evidence.length === 6 && evidence.every((e) => e.status === 'pass'), evidence,
    }
  })
  items.sort((a, b) => Number(b.executable) - Number(a.executable) || b.allocationSize - a.allocationSize)
  const executable = items.filter((item) => item.executable)
  return {
    ruleId, description, risk: 'E1', minAgeDays, candidates: items.length, executable: executable.length,
    executableAllocation: executable.reduce((sum, item) => sum + item.allocationSize, 0), items,
  }
}

function mockCandidates(): CandidateView {
  const rules = [
    { ruleId: 'user-temp', description: 'Per-user temporary files', days: 7 },
    { ruleId: 'npm-cache', description: 'npm content cache; npm re-downloads missing entries', days: 30 },
    { ruleId: 'pip-cache', description: 'pip download and wheel cache', days: 30 },
    { ruleId: 'go-build-cache', description: 'Go build cache; go rebuilds missing entries', days: 30 },
  ]
  const groups = [
    mockGroup('user-temp', rules[0].description, 7, 'D:\\Users\\me\\AppData\\Local\\Temp'),
    mockGroup('npm-cache', rules[1].description, 30, 'D:\\Users\\me\\AppData\\Local\\npm-cache'),
  ].sort((a, b) => b.executableAllocation - a.executableAllocation)
  return {
    digest: 'sha256:mock', createdAt: new Date().toISOString(), expiresAt: new Date(Date.now() + DAY).toISOString(), rulesVersion: 'builtin/1',
    groups,
    rules: rules.map((r) => ({ ruleId: r.ruleId, description: r.description, risk: 'E1', minAgeDays: r.days, candidates: 0, executable: 0, executableAllocation: 0, items: [] })),
  }
}

const listeners = new Map<string, Set<(data: unknown) => void>>()
export const mockEvents = {
  on(name: string, callback: (data: unknown) => void) {
    if (!listeners.has(name)) listeners.set(name, new Set())
    listeners.get(name)!.add(callback)
    return () => listeners.get(name)!.delete(callback)
  },
  emit(name: string, data: unknown) {
    listeners.get(name)?.forEach((callback) => callback(data))
  },
}

let timer: ReturnType<typeof setInterval> | undefined

const providers = [
  { id: 'deepseek', name: 'DeepSeek', baseUrl: 'https://api.deepseek.com', defaultModel: 'deepseek-chat' },
  { id: 'qwen', name: '通义千问', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', defaultModel: 'qwen-plus' },
  { id: 'kimi', name: 'Kimi', baseUrl: 'https://api.moonshot.cn/v1', defaultModel: 'moonshot-v1-32k' },
  { id: 'zhipu', name: '智谱', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', defaultModel: 'glm-4-flash' },
  { id: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', defaultModel: 'gpt-4o-mini' },
  { id: 'custom', name: '自定义', baseUrl: '', defaultModel: '' },
]
const aiSettings: AISettingsView = { provider: 'deepseek', baseUrl: providers[0].baseUrl, model: providers[0].defaultModel, hasKey: false, providers }

const lancedbPaths = ['darwin-arm64', 'darwin-x64', 'linux-arm64-gnu', 'linux-arm64-musl', 'linux-x64-gnu', 'linux-x64-musl', 'win32-arm64-msvc'].map(
  (p) => `D:\\Projects\\pare\\node_modules\\@lancedb\\lancedb-${p}`,
)

const mockFindings: Finding[] = [
  {
    id: 'f1',
    title: 'WSL 虚拟磁盘 11.1 GB，其中约 4.3 GB 是死空间',
    summary: 'Ubuntu-24.04 的 ext4.vhdx 占 11.1 GB，里面实际数据只有约 6.8 GB，其余是删除文件后没有回收的空间。',
    cause: '.wslconfig 里写了 sparseVhd=true，但当前 WSL 版本不识别这个键，所以虚拟磁盘从不自动收缩。',
    paths: ['C:\\Users\\me\\AppData\\Local\\Packages\\CanonicalGroupLimited.Ubuntu24.04LTS_79rhkp1fndgsc\\LocalState\\ext4.vhdx', 'C:\\Users\\me\\.wslconfig'],
    bytes: 11.08 * GB,
    risk: 'careful',
    confidence: 'high',
    regrowth: '会。虚拟磁盘只增不减，之后在 WSL 里写入删除大量文件还会再次膨胀，需要定期压缩或启用稀疏模式。',
    evidence: ['ext4.vhdx 文件大小 11.08 GB', 'WSL 内 df 显示已用 6.8 GB', '.wslconfig 中存在 sparseVhd=true', 'wsl --version 为 1.2.5，低于支持稀疏盘的版本'],
    actions: [
      { kind: 'template', label: '把 Ubuntu-24.04 迁移到 D 盘', explain: '导出并导入到 D 盘，C 盘立刻腾出空间，导入时顺便压缩。', bytes: 11.08 * GB, template: 'wsl_move', params: { distro: 'Ubuntu-24.04', target: 'D:\\WSL\\Ubuntu-24.04' } },
      { kind: 'template', label: '开启稀疏磁盘', explain: '对该发行版启用 sparse，之后删除的文件会自动回收。', bytes: 4.3 * GB, template: 'wsl_set_sparse', params: { distro: 'Ubuntu-24.04' } },
      { kind: 'config', label: '修正 .wslconfig', explain: '把无效的 sparseVhd 键移到受支持的位置。', bytes: 0, paths: ['C:\\Users\\me\\.wslconfig'] },
    ],
  },
  {
    id: 'f2',
    title: '罗技 G HUB 更新包残留 35.8 GB',
    summary: '系统临时目录里堆了 14 个 G HUB 下载的更新包，都已安装过，不会再被使用。',
    cause: 'G HUB 的自动更新下载后不清理安装包。',
    paths: ['C:\\Windows\\SystemTemp'],
    bytes: 35.8 * GB,
    risk: 'safe',
    confidence: 'high',
    regrowth: '会。只要 G HUB 自动更新开着，每次更新都会留下新的安装包。',
    evidence: ['SystemTemp 下 14 个 lghub_*.exe / .zip', '文件修改时间集中在最近半年', '对应版本已安装，没有进程占用'],
    actions: [
      { kind: 'quarantine', label: '移入隔离区', explain: '整个目录内容移走，可随时撤回。', bytes: 35.8 * GB, paths: ['C:\\Windows\\SystemTemp'] },
      { kind: 'manual', label: '在 G HUB 设置里关闭自动更新', explain: '避免之后再次堆积。', bytes: 0 },
    ],
  },
  {
    id: 'f3',
    title: 'lancedb 装了 8 个平台的原生二进制，浪费 767 MB',
    summary: 'node_modules 里 @lancedb 下有 8 个平台的原生包，本机只会用到 win32-x64。',
    cause: 'npm 安装时把所有可选平台依赖一并下载。',
    paths: lancedbPaths,
    bytes: 767 * MB,
    risk: 'rebuildable',
    confidence: 'medium',
    regrowth: '重新执行 npm install 可能再次装回，可用 --omit=optional 避免。',
    evidence: ['node_modules/@lancedb 下共 8 个平台目录', '当前系统为 win32-x64', '其余 7 个目录合计 767 MB'],
    actions: [
      { kind: 'quarantine', label: '移走 7 个其他平台的包', explain: '保留 win32-x64，其余移入隔离区。', bytes: 767 * MB, paths: lancedbPaths },
      { kind: 'command', label: '重新构建依赖', explain: '让 npm 只保留需要的平台。', bytes: 0, command: 'npm rebuild @lancedb/lancedb' },
    ],
  },
]

let aiTimers: ReturnType<typeof setTimeout>[] = []
let aiRunning = false
let emittedFindings: Finding[] = []
let consentResolve: ((allow: boolean) => void) | undefined
let aiTokens = 0

function aiEmit(e: AIEvent) {
  aiTokens += 400 + Math.round(Math.random() * 300)
  mockEvents.emit('ai:event', { ...e, tokens: aiTokens })
}
function aiAt(ms: number, fn: () => void) {
  aiTimers.push(setTimeout(() => aiRunning && fn(), ms))
}
function aiFinish(cancelled: boolean) {
  aiRunning = false
  consentResolve = undefined
  aiTimers.forEach(clearTimeout)
  aiTimers = []
  mockEvents.emit('ai:finished', { findings: emittedFindings, error: '', cancelled })
}
function aiFinding(i: number, step: number) {
  emittedFindings.push(mockFindings[i])
  aiEmit({ kind: 'finding', step, finding: mockFindings[i] })
}
const j = (v: unknown) => JSON.stringify(v)

export const mockBackend: Backend = {
  async Elevated() {
    return false
  },
  async Drives(): Promise<Drive[]> {
    return [
      { root: 'C:\\', label: 'System', total: 200 * GB, free: 81.4 * GB },
      { root: 'D:\\', label: 'Data', total: 731 * GB, free: 107.4 * GB },
    ]
  },
  async ChooseFolder() {
    return 'D:\\'
  },
  async StartScan() {
    let step = 0
    const started = Date.now()
    timer = setInterval(() => {
      step++
      const progress: Progress = { entries: step * 9137, directories: step * 811, current: `D:\\Steam\\steamapps\\common\\folder-${step}`, done: false }
      mockEvents.emit('scan:progress', progress)
      if (step === 25) {
        clearInterval(timer)
        const summary: ScanSummary = {
          root: root.path, complete: false, cancelled: false, error: '', seconds: (Date.now() - started) / 1000,
          entries: root.files + root.directories, files: root.files, directories: root.directories,
          logicalSize: root.total * 0.98, allocationSize: root.total, uniqueAllocation: root.total * 0.97,
          volumeUsed: root.total * 0.97 + 23 * GB,
          gapCount: 1, gaps: [{ path: 'D:\\System Volume Information', operation: 'enumerate', code: 'access_denied' }],
        }
        mockEvents.emit('scan:done', summary)
      }
    }, 80)
  },
  async CancelScan() {
    clearInterval(timer)
    mockEvents.emit('scan:done', { root: root.path, cancelled: true } as ScanSummary)
  },
  async Children(path, limit) {
    const node = index.get(path)
    if (!node) throw new Error(`${path} is not in the scan result`)
    return trimmed(node.children, limit)
  },
  async Subtree(path, depth, limit) {
    const node = index.get(path)
    if (!node) throw new Error(`${path} is not in the scan result`)
    return subtree(node, depth, limit)
  },
  async Reveal() {},
  async BuildCandidates() {
    await new Promise((resolve) => setTimeout(resolve, 600))
    return mockCandidates()
  },
  async GetAISettings() {
    return { ...aiSettings }
  },
  async SaveAISettings(input) {
    aiSettings.provider = input.provider
    aiSettings.baseUrl = input.baseUrl
    aiSettings.model = input.model
    if (input.clearKey) aiSettings.hasKey = false
    if (input.apiKey) aiSettings.hasKey = true
  },
  async TestAI() {
    await new Promise((resolve) => setTimeout(resolve, 500))
    if (!aiSettings.hasKey) throw new Error('AI 尚未配置')
    return '连接正常'
  },
  async StartInvestigation(scope) {
    if (!aiSettings.hasKey) throw new Error('AI 尚未配置，请先在设置里填写 API Key')
    aiTimers.forEach(clearTimeout)
    aiTimers = []
    aiRunning = true
    emittedFindings = []
    aiTokens = 0
    const dir = scope || root.path
    const envPath = 'C:\\Users\\me\\project\\.env'
    aiEmit({ kind: 'thinking', step: 1 })
    aiAt(800, () => aiEmit({ kind: 'tool', step: 1, tool: 'scan_overview', args: j({ path: dir, depth: 2 }) }))
    aiAt(1300, () => aiEmit({ kind: 'result', step: 1, tool: 'scan_overview', text: `${dir} 共 1.2 TB，最大的子目录：Steam 412 GB、Users 233 GB、Projects 31 GB。` }))
    aiAt(1800, () => aiEmit({ kind: 'thinking', step: 2 }))
    aiAt(2500, () => aiEmit({ kind: 'tool', step: 2, tool: 'list_dir', args: j({ path: 'C:\\Users\\me', limit: 20 }) }))
    aiAt(3000, () => aiEmit({ kind: 'result', step: 2, tool: 'list_dir', text: 'AppData 190 GB\nProjects 31 GB\n.wslconfig 112 B\n.env 0.3 KB\n……' }))
    aiAt(3400, () => aiEmit({ kind: 'thinking', step: 3 }))
    aiAt(4000, () => aiEmit({ kind: 'tool', step: 3, tool: 'run_diagnostic', args: j({ name: 'wsl_disk_usage' }) }))
    aiAt(4600, () => aiEmit({ kind: 'result', step: 3, tool: 'run_diagnostic', text: 'Ubuntu-24.04: vhdx 11.08 GB, used 6.8 GB, sparse=false\nwsl 1.2.5' }))
    aiAt(5000, () => {
      aiEmit({ kind: 'tool', step: 4, tool: 'file_head', args: j({ path: 'C:\\Users\\me\\.wslconfig', bytes: 2000 }) })
      aiEmit({ kind: 'result', step: 4, tool: 'file_head', text: '[wsl2]\nmemory=8GB\nsparseVhd=true' })
      aiFinding(0, 4)
    })
    aiAt(5800, () => {
      aiEmit({ kind: 'thinking', step: 5 })
      aiEmit({ kind: 'tool', step: 5, tool: 'file_head', args: j({ path: envPath, bytes: 500 }) })
      mockEvents.emit('ai:consent', { id: 'consent-1', path: envPath, reason: '读取文本内容' })
      new Promise<boolean>((resolve) => (consentResolve = resolve)).then((allow) => {
        if (!aiRunning) return
        aiEmit({ kind: 'result', step: 5, tool: 'file_head', text: allow ? 'NODE_ENV=development\n（内容已读取，与清理无关）' : '用户拒绝读取，仅有文件名和大小' })
        aiAt(1200, () => {
          aiEmit({ kind: 'tool', step: 6, tool: 'list_dir', args: j({ path: 'C:\\Windows\\SystemTemp' }) })
          aiEmit({ kind: 'result', step: 6, tool: 'list_dir', text: 'lghub_installer_2024.1.exe 2.4 GB\nlghub_update_2024.2.zip 2.6 GB\n……（共 14 项）' })
          aiFinding(1, 6)
        })
        aiAt(2400, () => {
          aiEmit({ kind: 'thinking', step: 7 })
          aiEmit({ kind: 'tool', step: 7, tool: 'list_dir', args: j({ path: 'D:\\Projects\\pare\\node_modules\\@lancedb' }) })
          aiFinding(2, 7)
        })
        aiAt(3600, () => {
          aiEmit({ kind: 'message', step: 8, text: '共发现 3 个问题，合计可回收约 47.7 GB。其中罗技更新包最大且最安全；WSL 虚拟磁盘建议先迁移，再开启稀疏模式。' })
          aiEmit({ kind: 'done', step: 8 })
          aiFinish(false)
        })
      })
    })
  },
  async StopInvestigation() {
    if (aiRunning) aiFinish(true)
  },
  async AnswerConsent(_id, allow) {
    consentResolve?.(allow)
    consentResolve = undefined
  },
  async Findings() {
    return [...emittedFindings]
  },
}
