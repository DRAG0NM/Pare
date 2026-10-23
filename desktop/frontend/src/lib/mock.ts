// A deterministic fake disk for running the UI in a browser. Never used in the desktop app.
import type { Backend, CandidateGroup, CandidateItem, CandidateView, Drive, Evidence, NodeTree, NodeView, Progress, ScanSummary } from './api'

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
}
