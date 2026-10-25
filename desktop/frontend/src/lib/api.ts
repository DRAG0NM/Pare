// Bridge to the Go backend. Inside the desktop app Wails injects window.go and
// window.runtime; in a plain browser (vite dev) a mock backend stands in so the UI
// can be developed and reviewed without Windows APIs.
import { mockBackend, mockEvents } from './mock'

export type NodeView = {
  path: string
  name: string
  kind: 'file' | 'directory' | 'reparse' | 'others' | string
  logicalSize: number
  allocationSize: number
  files: number
  directories: number
  incomplete: boolean
  hasChildren: boolean
  others: number
}

export type NodeTree = NodeView & { children: NodeTree[] }

export type Progress = { entries: number; directories: number; current: string; done: boolean }

export type ScanError = { path: string; operation: string; code: string }

export type ScanSummary = {
  root: string
  complete: boolean
  cancelled: boolean
  error: string
  seconds: number
  entries: number
  files: number
  directories: number
  logicalSize: number
  allocationSize: number
  uniqueAllocation: number
  volumeUsed: number
  gapCount: number
  gaps: ScanError[]
}

export type Drive = { root: string; label: string; total: number; free: number }

export type EvidenceStatus = 'pass' | 'fail' | 'unknown' | 'error'
export type Evidence = { kind: string; status: EvidenceStatus; detail: string }

export type CandidateItem = {
  path: string
  name: string
  directory: string
  logicalSize: number
  allocationSize: number
  lastWriteTime: string
  risk: string
  executable: boolean
  evidence: Evidence[]
}

export type CandidateGroup = {
  ruleId: string
  description: string
  risk: string
  minAgeDays: number
  candidates: number
  executable: number
  executableAllocation: number
  items: CandidateItem[]
}

export type CandidateView = {
  digest: string
  createdAt: string
  expiresAt: string
  rulesVersion: string
  groups: CandidateGroup[]
  rules: CandidateGroup[]
}

export type Provider = { id: string; name: string; baseUrl: string; defaultModel: string }
export type AISettingsView = { provider: string; baseUrl: string; model: string; hasKey: boolean; providers: Provider[] }
export type AISettingsInput = { provider: string; baseUrl: string; model: string; apiKey: string; clearKey: boolean }

export type Action = {
  kind: 'quarantine' | 'template' | 'command' | 'config' | 'manual'
  label: string
  explain: string
  bytes: number
  paths?: string[]
  template?: string
  params?: Record<string, string>
  command?: string
}

export type Finding = {
  id: string
  title: string
  summary: string
  cause: string
  paths: string[]
  bytes: number
  risk: 'safe' | 'rebuildable' | 'careful' | 'system'
  confidence: 'high' | 'medium' | 'low'
  regrowth: string
  evidence: string[]
  actions: Action[]
  prevention?: Action[]
}

export type AIEvent = {
  kind: 'thinking' | 'tool' | 'result' | 'finding' | 'message' | 'error' | 'done'
  step: number
  tool?: string
  args?: string
  text?: string
  finding?: Finding
  tokens?: number
}

export type InvestigationDepth = 'standard' | 'deep' | 'thorough'
export type AIConsent = { id: string; path: string; reason: string }
export type AIFinished = { findings: Finding[]; error: string; cancelled: boolean }

export type QuarantineItemView = { path: string; kind: 'file' | 'directory'; bytes: number; executable: boolean; reason: string }
export type PreparedAction = {
  key: string
  findingTitle: string
  kind: 'quarantine' | 'template' | 'command' | 'config' | 'manual'
  label: string
  explain: string
  bytes: number
  items: QuarantineItemView[]
  steps: string[]
  warnings: string[]
  needsAdmin: boolean
  runnable: boolean
  error: string
}
export type ExecutionPreview = { actions: PreparedAction[] }
export type ActionResult = {
  key: string
  findingTitle: string
  kind: PreparedAction['kind']
  label: string
  status: 'running' | 'done' | 'partial' | 'skipped' | 'failed'
  detail: string
  output: string
  bytes: number
}
export type RegrowthTip = { findingId: string; findingTitle: string; text: string }
export type ExecutionReport = {
  time: string
  volume: string
  freeBefore: number
  freeAfter: number
  results: ActionResult[]
  manifests: string[]
  freedBytes: number
  quarantinedBytes: number
  tips: RegrowthTip[]
  error: string
}
export type HistoryAction = { findingTitle: string; label: string; kind: string; status: string; bytes: number }
export type HistoryEntry = { time: string; kind: 'clean' | 'purge'; volume: string; freedBytes: number; quarantinedBytes: number; items: number; actions: HistoryAction[] }
export type BatchSummary = { manifest: string; createdAt: string; items: number; quarantined: number; restored: number; purged: number; other: number; quarantinedBytes: number }
export type ExecutionItem = { original_path: string; quarantine_path: string; kind?: string; state: string; detail: string; allocation_size: string }
export type Execution = { schema: string; plan_digest: string; created_at: string; updated_at: string; quarantine_dir: string; items: ExecutionItem[] }
export type ChosenAction = { findingId: string; actionIndex: number; prevent?: boolean }

export interface Backend {
  Elevated(): Promise<boolean>
  Drives(): Promise<Drive[]>
  ChooseFolder(): Promise<string>
  StartScan(root: string): Promise<void>
  CancelScan(): Promise<void>
  Children(path: string, limit: number): Promise<NodeView[]>
  Subtree(path: string, depth: number, limit: number): Promise<NodeTree>
  Reveal(path: string): Promise<void>
  BuildCandidates(refresh: boolean): Promise<CandidateView>
  GetAISettings(): Promise<AISettingsView>
  SaveAISettings(input: AISettingsInput): Promise<void>
  TestAI(): Promise<string>
  StartInvestigation(scope: string, note: string, depth: InvestigationDepth, continueRun: boolean): Promise<void>
  StopInvestigation(): Promise<void>
  AnswerConsent(id: string, allow: boolean): Promise<void>
  Findings(): Promise<Finding[]>
  PrepareExecution(chosen: ChosenAction[], rulePaths: string[]): Promise<ExecutionPreview>
  Execute(accepted: string[], admin: string[]): Promise<void>
  ListQuarantine(): Promise<BatchSummary[]>
  UndoBatch(manifest: string): Promise<Execution>
  PurgeBatch(manifest: string): Promise<{ execution: Execution; freedBytes: number }>
  ListHistory(): Promise<HistoryEntry[]>
  ExportLog(): Promise<string>
  OpenIssuePage(): Promise<void>
  SetLanguage(locale: string): Promise<void>
}

const host = window as unknown as {
  go?: { main?: { App?: Backend } }
  runtime?: { EventsOn(name: string, callback: (data: unknown) => void): () => void }
}

export const isMock = !host.go?.main?.App

export const backend: Backend = host.go?.main?.App ?? mockBackend

export function onEvent<T>(name: string, callback: (data: T) => void): () => void {
  const handler = callback as (data: unknown) => void
  return host.runtime ? host.runtime.EventsOn(name, handler) : mockEvents.on(name, handler)
}
