// A deterministic fake disk for running the UI in a browser. Never used in the desktop app.
import type { ActionResult, BatchSummary, HistoryEntry, Execution, ExecutionPreview, PreparedAction, AIEvent, AISettingsView, Backend, Finding, CandidateGroup, CandidateItem, CandidateView, Drive, Evidence, NodeTree, NodeView, Progress, ScanSummary } from './api'
import { mt } from './mockText'

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
      path: '', name: mt('tree.others', { n: rest.length }), kind: 'others', logicalSize: total, allocationSize: total,
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

const providers = () => [
  { id: 'deepseek', name: 'DeepSeek', baseUrl: 'https://api.deepseek.com', defaultModel: 'deepseek-chat' },
  { id: 'qwen', name: mt('prov.qwen'), baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', defaultModel: 'qwen-plus' },
  { id: 'kimi', name: 'Kimi', baseUrl: 'https://api.moonshot.cn/v1', defaultModel: 'moonshot-v1-32k' },
  { id: 'zhipu', name: mt('prov.zhipu'), baseUrl: 'https://open.bigmodel.cn/api/paas/v4', defaultModel: 'glm-4-flash' },
  { id: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', defaultModel: 'gpt-4o-mini' },
  { id: 'custom', name: mt('prov.custom'), baseUrl: '', defaultModel: '' },
]
const aiSettings: AISettingsView = { provider: 'deepseek', baseUrl: providers()[0].baseUrl, model: providers()[0].defaultModel, hasKey: false, providers: [] }

const lancedbPaths = ['darwin-arm64', 'darwin-x64', 'linux-arm64-gnu', 'linux-arm64-musl', 'linux-x64-gnu', 'linux-x64-musl', 'win32-arm64-msvc'].map(
  (p) => `D:\\Projects\\pare\\node_modules\\@lancedb\\lancedb-${p}`,
)

function buildFindings(): Finding[] {
  return [
  {
    id: 'f1',
    title: mt('f1.title'),
    summary: mt('f1.summary'),
    cause: mt('f1.cause'),
    paths: ['C:\\Users\\me\\AppData\\Local\\Packages\\CanonicalGroupLimited.Ubuntu24.04LTS_79rhkp1fndgsc\\LocalState\\ext4.vhdx', 'C:\\Users\\me\\.wslconfig'],
    bytes: 11.08 * GB,
    risk: 'careful',
    confidence: 'high',
    regrowth: mt('f1.regrowth'),
    evidence: [mt('f1.ev1'), mt('f1.ev2'), mt('f1.ev3'), mt('f1.ev4')],
    actions: [
      { kind: 'template', label: mt('f1.a1.label'), explain: mt('f1.a1.explain'), bytes: 11.08 * GB, template: 'wsl_move', params: { distro: 'Ubuntu-24.04', target: 'D:\\WSL\\Ubuntu-24.04' } },
      { kind: 'template', label: mt('f1.a2.label'), explain: mt('f1.a2.explain'), bytes: 4.3 * GB, template: 'wsl_set_sparse', params: { distro: 'Ubuntu-24.04' } },
      { kind: 'config', label: mt('f1.a3.label'), explain: mt('f1.a3.explain'), bytes: 0, paths: ['C:\\Users\\me\\.wslconfig'] },
    ],
    prevention: [{ kind: 'template', label: mt('f1.p1.label'), explain: mt('f1.p1.explain'), bytes: 0, template: 'wsl_set_sparse', params: { distro: 'Ubuntu-24.04' } }],
  },
  {
    id: 'f2',
    title: mt('f2.title'),
    summary: mt('f2.summary'),
    cause: mt('f2.cause'),
    paths: ['C:\\Windows\\SystemTemp'],
    bytes: 35.8 * GB,
    risk: 'safe',
    confidence: 'high',
    regrowth: mt('f2.regrowth'),
    evidence: [mt('f2.ev1'), mt('f2.ev2'), mt('f2.ev3')],
    actions: [
      { kind: 'quarantine', label: mt('f2.a1.label'), explain: mt('f2.a1.explain'), bytes: 35.8 * GB, paths: ['C:\\Windows\\SystemTemp'] },
      { kind: 'manual', label: mt('f2.a2.label'), explain: mt('f2.a2.explain'), bytes: 0 },
    ],
    prevention: [{ kind: 'manual', label: mt('f2.p1.label'), explain: mt('f2.p1.explain'), bytes: 0 }],
  },
  {
    id: 'f3',
    title: mt('f3.title'),
    summary: mt('f3.summary'),
    cause: mt('f3.cause'),
    paths: lancedbPaths,
    bytes: 767 * MB,
    risk: 'rebuildable',
    confidence: 'medium',
    regrowth: mt('f3.regrowth'),
    evidence: [mt('f3.ev1'), mt('f3.ev2'), mt('f3.ev3')],
    actions: [
      { kind: 'quarantine', label: mt('f3.a1.label'), explain: mt('f3.a1.explain'), bytes: 767 * MB, paths: lancedbPaths },
      { kind: 'command', label: mt('f3.a2.label'), explain: mt('f3.a2.explain'), bytes: 0, command: 'npm rebuild @lancedb/lancedb' },
    ],
    prevention: [{ kind: 'command', label: mt('f3.p1.label'), explain: mt('f3.p1.explain'), bytes: 0, command: 'npm config set omit optional' }],
  },
  {
    id: 'r2-f1',
    title: mt('r2f1.title'),
    summary: mt('r2f1.summary'),
    cause: mt('r2f1.cause'),
    paths: ['C:\\Users\\me\\AppData\\Local\\Microsoft\\Edge\\User Data\\OptGuideOnDeviceModel\\2024.5.1.0'],
    bytes: 168 * MB,
    risk: 'rebuildable',
    confidence: 'medium',
    regrowth: mt('r2f1.regrowth'),
    evidence: [mt('r2f1.ev1'), mt('r2f1.ev2'), mt('r2f1.ev3')],
    actions: [
      { kind: 'quarantine', label: mt('r2f1.a1.label'), explain: mt('r2f1.a1.explain'), bytes: 168 * MB, paths: ['C:\\Users\\me\\AppData\\Local\\Microsoft\\Edge\\User Data\\OptGuideOnDeviceModel\\2024.5.1.0'] },
    ],
  },
  {
    id: 'r2-f2',
    title: mt('r2f2.title'),
    summary: mt('r2f2.summary'),
    cause: mt('r2f2.cause'),
    paths: ['C:\\Users\\me\\AppData\\Local\\Temp\\go1.22.3.windows-amd64.msi'],
    bytes: 75 * MB,
    risk: 'safe',
    confidence: 'high',
    regrowth: mt('r2f2.regrowth'),
    evidence: [mt('r2f2.ev1'), mt('r2f2.ev2'), mt('r2f2.ev3')],
    actions: [
      { kind: 'quarantine', label: mt('r2f2.a1.label'), explain: mt('r2f2.a1.explain'), bytes: 75 * MB, paths: ['C:\\Users\\me\\AppData\\Local\\Temp\\go1.22.3.windows-amd64.msi'] },
    ],
  },
  ]
}
// Rebuilt whenever demo data is produced so the text follows the current interface language.
let mockFindings: Finding[] = buildFindings()
function refreshFindings() {
  mockFindings = buildFindings()
}

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

let lastPreview: PreparedAction[] = []
const NEW_MANIFEST = 'C:\\.pare-quarantine\\20261024-150000-abc\\manifest.json'
const OLD_MANIFEST = 'C:\\.pare-quarantine\\20261019-092000-xyz\\manifest.json'
const mockBatches: BatchSummary[] = [
  { manifest: OLD_MANIFEST, createdAt: new Date(Date.now() - 5 * DAY).toISOString(), items: 3, quarantined: 3, restored: 0, purged: 0, other: 0, quarantinedBytes: 1.4 * GB },
]
let execRunning = false
let mockFree = 81.4 * GB
const mockHistory: HistoryEntry[] = [
  { time: new Date(Date.now() - 5 * DAY).toISOString(), kind: 'clean', volume: 'C:\\', freedBytes: 0, quarantinedBytes: 1.4 * GB, items: 2, actions: [] },
]
const seedHistory = mockHistory[0] // its action text is built in ListHistory so it follows the language

function mockExecution(manifest: string, state: string): Execution {
  const b = mockBatches.find((x) => x.manifest === manifest)
  const n = b?.quarantined || 1
  return {
    schema: 'pare.execution/1', plan_digest: 'sha256:mock', created_at: b?.createdAt ?? '', updated_at: new Date().toISOString(), quarantine_dir: manifest.replace(/\\manifest\.json$/, ''),
    items: Array.from({ length: n }, (_, i) => ({ original_path: `D:\\Users\\me\\AppData\\Local\\Temp\\item-${i + 1}.tmp`, quarantine_path: '', state, detail: '', allocation_size: '0' })),
  }
}

function prepareAction(f: Finding, index: number, prevent = false): PreparedAction {
  const a = (prevent ? f.prevention ?? [] : f.actions)[index]
  const base: PreparedAction = { key: `${f.id}:${prevent ? 'p' : ''}${index}`, findingTitle: f.title, kind: a.kind, label: a.label, explain: a.explain, bytes: a.bytes, items: [], steps: [], warnings: [], needsAdmin: false, runnable: true, error: '' }
  if (a.kind === 'quarantine') {
    const paths = a.paths ?? []
    base.items = paths.map((path) => {
      const prot = path.startsWith('C:\\Windows\\System32')
      return { path, kind: 'directory' as const, bytes: a.bytes / Math.max(1, paths.length), executable: !prot, reason: prot ? mt('prep.protected') : '' }
    })
    base.needsAdmin = paths.some((p) => p.startsWith('C:\\Windows'))
    base.runnable = base.items.some((i) => i.executable)
    if (!base.runnable) base.error = mt('prep.nothing')
  } else if (a.kind === 'template') {
    if (a.template === 'wsl_move') {
      base.steps = ['wsl.exe --terminate Ubuntu-24.04', 'wsl.exe --manage Ubuntu-24.04 --move D:\\WSL\\Ubuntu-24.04']
      base.warnings = [mt('prep.wslMove.w1'), mt('prep.wslMove.w2')]
    } else {
      base.steps = ['wsl.exe --terminate Ubuntu-24.04', 'wsl.exe --manage Ubuntu-24.04 --set-sparse true']
      base.warnings = [mt('prep.sparse.w1')]
    }
  } else if (a.kind === 'command') {
    base.steps = [a.command ?? '']
    base.warnings = [mt('prep.cmd.w1'), mt('prep.cmd.w2')]
  } else {
    base.runnable = false
  }
  return base
}

export const mockBackend: Backend = {
  async PrepareExecution(chosen, rulePaths) {
    refreshFindings()
    await new Promise((resolve) => setTimeout(resolve, 700))
    const actions: PreparedAction[] = []
    for (const c of chosen) {
      const f = mockFindings.find((x) => x.id === c.findingId)
      if (f && (c.prevent ? f.prevention ?? [] : f.actions)[c.actionIndex]) actions.push(prepareAction(f, c.actionIndex, c.prevent))
    }
    if (rulePaths.length) {
      const per = 38 * MB
      actions.push({
        key: 'rules', findingTitle: mt('rules.title'), kind: 'quarantine', label: mt('rules.label', { n: rulePaths.length }), explain: mt('rules.explain'),
        bytes: per * rulePaths.length, items: rulePaths.slice(0, 200).map((path) => ({ path, kind: 'file' as const, bytes: per, executable: true, reason: '' })), steps: [], warnings: [], needsAdmin: false, runnable: true, error: '',
      })
    }
    lastPreview = actions
    const preview: ExecutionPreview = { actions }
    return preview
  },
  async Execute(accepted) {
    refreshFindings()
    if (!lastPreview.length) throw new Error(mt('exec.noPlan'))
    if (execRunning) throw new Error(mt('exec.busy'))
    execRunning = true
    const acts = lastPreview.filter((a) => accepted.includes(a.key))
    const results: ActionResult[] = []
    acts.forEach((a, i) => {
      const at = 300 + i * 900
      const info = { key: a.key, findingTitle: a.findingTitle, kind: a.kind, label: a.label }
      setTimeout(() => mockEvents.emit('exec:progress', { ...info, status: 'running', detail: '', output: '', bytes: 0 } as ActionResult), at)
      setTimeout(() => {
        const partial = a.kind === 'quarantine' && a.items.length > 3
        const r: ActionResult = {
          ...info, status: partial ? 'partial' : 'done', bytes: partial ? a.bytes * 0.9 : a.bytes,
          detail: a.kind === 'quarantine' ? (partial ? mt('exec.movedPartial', { n: a.items.length - 1 }) : mt('exec.moved', { n: a.items.length })) : mt('exec.done'),
          output: a.steps.length ? a.steps.map((s) => `> ${s}\n${mt('exec.stepOk')}`).join('\n') : '',
        }
        results.push(r)
        mockEvents.emit('exec:progress', r)
      }, at + 700)
    })
    setTimeout(() => {
      execRunning = false
      const quarantined = acts.filter((a) => a.kind === 'quarantine')
      const manifests = quarantined.length ? [NEW_MANIFEST] : []
      if (manifests.length) {
        const n = quarantined.reduce((s, a) => s + a.items.length, 0)
        mockBatches.unshift({ manifest: NEW_MANIFEST, createdAt: new Date().toISOString(), items: n, quarantined: n, restored: 0, purged: 0, other: 0, quarantinedBytes: quarantined.reduce((s, a) => s + a.bytes, 0) })
      }
      const quarantinedBytes = results.filter((r) => r.kind === 'quarantine').reduce((s, r) => s + r.bytes, 0)
      const freed = results.filter((r) => r.kind !== 'quarantine').reduce((s, r) => s + r.bytes, 0)
      const tips = acts
        .filter((a) => !a.key.includes(':p'))
        .map((a) => mockFindings.find((f) => a.key.startsWith(f.id + ':')))
        .filter((f, i, all): f is Finding => !!f && f.regrowth !== '' && all.indexOf(f) === i)
        .map((f) => ({ findingId: f.id, findingTitle: f.title, text: f.regrowth }))
      const report = { time: new Date().toISOString(), volume: 'C:\\', freeBefore: mockFree, freeAfter: mockFree + freed, results, manifests, freedBytes: freed, quarantinedBytes, tips, error: '' }
      mockFree += freed
      mockHistory.unshift({ time: report.time, kind: 'clean', volume: 'C:\\', freedBytes: freed, quarantinedBytes, items: results.length, actions: results.map((r) => ({ findingTitle: r.findingTitle, label: r.label, kind: r.kind, status: r.status, bytes: r.bytes })) })
      mockEvents.emit('exec:done', report)
    }, 300 + acts.length * 900 + 800)
  },
  async ListQuarantine() {
    return mockBatches.map((b) => ({ ...b }))
  },
  async UndoBatch(manifest) {
    const b = mockBatches.find((x) => x.manifest === manifest)
    if (!b) throw new Error(mt('exec.noBatch'))
    const ex = mockExecution(manifest, 'restored')
    const conflict = ex.items.length > 2
    if (conflict) ex.items[0].state = 'conflict'
    b.restored += b.quarantined - (conflict ? 1 : 0)
    b.quarantined = conflict ? 1 : 0
    return ex
  },
  async PurgeBatch(manifest) {
    const b = mockBatches.find((x) => x.manifest === manifest)
    if (!b) throw new Error(mt('exec.noBatch'))
    const freed = b.quarantinedBytes
    const ex = mockExecution(manifest, 'purged')
    b.purged += b.quarantined
    b.quarantined = 0
    b.quarantinedBytes = 0
    mockFree += freed
    mockHistory.unshift({ time: new Date().toISOString(), kind: 'purge', volume: 'C:\\', freedBytes: freed, quarantinedBytes: 0, items: ex.items.length, actions: [] })
    return { execution: ex, freedBytes: freed }
  },
  async ExportLog() {
    await new Promise((resolve) => setTimeout(resolve, 400))
    return 'C:\\Users\\me\\Desktop\\pare-log-demo.txt'
  },
  async SetLanguage() {},
  async OpenIssuePage() {
    window.open('https://github.com/DRAG0NM/Pare/issues/new', '_blank')
  },
  async ListHistory() {
    return mockHistory.map((h) => (h === seedHistory ? { ...h, actions: [{ findingTitle: mt('rules.title'), label: mt('rules.label', { n: 36 }), kind: 'quarantine', status: 'done', bytes: 1.4 * GB }] } : { ...h }))
  },
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
    return { ...aiSettings, providers: providers() }
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
    if (!aiSettings.hasKey) throw new Error(mt('ai.notConfigured'))
    return mt('ai.connOk')
  },
  async StartInvestigation(scope, _note, _depth, continueRun) {
    if (!aiSettings.hasKey) throw new Error(mt('ai.needKey'))
    refreshFindings()
    aiTimers.forEach(clearTimeout)
    aiTimers = []
    aiRunning = true
    aiTokens = 0
    if (continueRun && emittedFindings.length) {
      aiEmit({ kind: 'thinking', step: 1 })
      aiAt(900, () => aiEmit({ kind: 'tool', step: 1, tool: 'list_dir', args: j({ path: 'C:\\Users\\me\\AppData\\Local\\Microsoft\\Edge\\User Data' }) }))
      aiAt(1600, () => aiFinding(3, 1))
      aiAt(2600, () => {
        aiEmit({ kind: 'thinking', step: 2 })
        aiEmit({ kind: 'tool', step: 2, tool: 'list_dir', args: j({ path: 'C:\\Users\\me\\AppData\\Local\\Temp' }) })
      })
      aiAt(3300, () => aiFinding(4, 2))
      aiAt(4200, () => {
        aiEmit({ kind: 'message', step: 3, text: mt('ai.more') })
        aiEmit({ kind: 'done', step: 3 })
        aiFinish(false)
      })
      return
    }
    emittedFindings = []
    const dir = scope || root.path
    const envPath = 'C:\\Users\\me\\project\\.env'
    aiEmit({ kind: 'thinking', step: 1 })
    aiAt(800, () => aiEmit({ kind: 'tool', step: 1, tool: 'scan_overview', args: j({ path: dir, depth: 2 }) }))
    aiAt(1300, () => aiEmit({ kind: 'result', step: 1, tool: 'scan_overview', text: mt('ai.overview', { dir }) }))
    aiAt(1800, () => aiEmit({ kind: 'thinking', step: 2 }))
    aiAt(2500, () => aiEmit({ kind: 'tool', step: 2, tool: 'list_dir', args: j({ path: 'C:\\Users\\me', limit: 20 }) }))
    aiAt(3000, () => aiEmit({ kind: 'result', step: 2, tool: 'list_dir', text: mt('ai.listHome') }))
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
      mockEvents.emit('ai:consent', { id: 'consent-1', path: envPath, reason: mt('ai.consentReason') })
      new Promise<boolean>((resolve) => (consentResolve = resolve)).then((allow) => {
        if (!aiRunning) return
        aiEmit({ kind: 'result', step: 5, tool: 'file_head', text: allow ? mt('ai.envAllowed') : mt('ai.envDenied') })
        aiAt(1200, () => {
          aiEmit({ kind: 'tool', step: 6, tool: 'list_dir', args: j({ path: 'C:\\Windows\\SystemTemp' }) })
          aiEmit({ kind: 'result', step: 6, tool: 'list_dir', text: mt('ai.listSysTemp') })
          aiFinding(1, 6)
        })
        aiAt(2400, () => {
          aiEmit({ kind: 'thinking', step: 7 })
          aiEmit({ kind: 'tool', step: 7, tool: 'list_dir', args: j({ path: 'D:\\Projects\\pare\\node_modules\\@lancedb' }) })
          aiFinding(2, 7)
        })
        aiAt(3600, () => {
          aiEmit({ kind: 'message', step: 8, text: mt('ai.final') })
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
