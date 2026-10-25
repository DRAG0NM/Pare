// Shared state of the confirm-and-execute flow, so both Decide tabs open the same dialog.
import { backend, onEvent, type ActionResult, type ChosenAction, type ExecutionPreview, type ExecutionReport } from './api'
import { inv } from './investigation.svelte'

export const exec = $state({
  phase: 'idle' as 'idle' | 'preparing' | 'confirm' | 'running',
  // A cleanup run ends on the summary page; a prevention run, started from that page,
  // only adds its results there.
  mode: 'clean' as 'clean' | 'prevent',
  preview: null as ExecutionPreview | null,
  accepted: {} as Record<string, boolean>,
  progress: {} as Record<string, ActionResult>,
  // summary is the report of the latest cleanup run, shown on the summary page.
  summary: null as ExecutionReport | null,
  // prevented holds results of prevention actions by action key ("f1:p0").
  prevented: {} as Record<string, ActionResult>,
  error: '',
})

let initialized = false
let onRulesDone: (() => void) | undefined
let onSummary: (() => void) | undefined

// App registers this to clear the rule selection after a run that included rule items.
export function setRulesDoneHandler(fn: () => void) {
  onRulesDone = fn
}

// App registers this to switch to the summary page when a cleanup run finishes.
export function setSummaryHandler(fn: () => void) {
  onSummary = fn
}

export const GONE_REASON = 'file no longer exists'
export function isGone(a: { items: { reason: string }[] }) {
  return a.items.length > 0 && a.items.every((it) => it.reason === GONE_REASON)
}

export function isPreventionKey(key: string) {
  return key.includes(':p')
}

export function initExecution() {
  if (initialized) return
  initialized = true
  onEvent<ActionResult>('exec:progress', (r) => {
    exec.progress[r.key] = r
  })
  onEvent<ExecutionReport>('exec:done', (r) => {
    exec.phase = 'idle'
    exec.preview = null
    if (exec.mode === 'prevent') {
      for (const res of r.results ?? []) exec.prevented[res.key] = res
      return
    }
    exec.summary = r
    exec.prevented = {}
    let hadRules = false
    for (const res of r.results ?? []) {
      if (res.key === 'rules') {
        hadRules = true
        continue
      }
      if (res.status !== 'done' && res.status !== 'partial') continue
      const id = res.key.slice(0, res.key.lastIndexOf(':'))
      if (!id) continue
      // A partial result must not downgrade a finding already fully handled.
      if (!(res.status === 'partial' && inv.handled[id] === 'done')) inv.handled[id] = res.status
      inv.chosen[id] = -1
    }
    if (hadRules) onRulesDone?.()
    onSummary?.()
  })
}

function clean(e: unknown) {
  return String(e).replace(/^Error:\s*/, '')
}

async function prepare(chosen: ChosenAction[], rulePaths: string[], mode: 'clean' | 'prevent') {
  exec.mode = mode
  exec.error = ''
  exec.progress = {}
  exec.accepted = {}
  exec.preview = null
  exec.phase = 'preparing'
  try {
    const preview = await backend.PrepareExecution(chosen, rulePaths)
    exec.preview = preview
    const accepted: Record<string, boolean> = {}
    for (const a of preview.actions) accepted[a.key] = a.runnable && !isGone(a) && (a.kind === 'quarantine' || a.kind === 'template')
    exec.accepted = accepted
    exec.phase = 'confirm'
  } catch (e) {
    exec.error = clean(e)
    exec.phase = 'confirm'
    exec.preview = { actions: [] }
  }
}

export function openConfirm(rulePaths: Iterable<string>) {
  const chosen = inv.findings
    .map((f) => ({ findingId: f.id, actionIndex: inv.chosen[f.id] ?? 0 }))
    .filter((c) => c.actionIndex >= 0)
  return prepare(chosen, [...rulePaths], 'clean')
}

// openPrevention shows one prevention action of a finding in the same confirmation dialog.
export function openPrevention(findingId: string, actionIndex: number) {
  return prepare([{ findingId, actionIndex, prevent: true }], [], 'prevent')
}

export async function execute() {
  if (!exec.preview) return
  const keys = exec.preview.actions.filter((a) => a.runnable && exec.accepted[a.key]).map((a) => a.key)
  exec.error = ''
  exec.progress = {}
  exec.phase = 'running'
  try {
    await backend.Execute(keys)
  } catch (e) {
    exec.error = clean(e)
    exec.phase = 'confirm'
  }
}

// close dismisses the dialog; the summary page keeps its report.
export function close() {
  exec.phase = 'idle'
  exec.preview = null
  exec.progress = {}
  exec.accepted = {}
  exec.error = ''
}
