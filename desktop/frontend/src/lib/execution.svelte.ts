// Shared state of the confirm-and-execute flow, so both Decide tabs open the same dialog.
import { backend, onEvent, type ActionResult, type ExecutionPreview, type ExecutionReport } from './api'
import { inv } from './investigation.svelte'

export const exec = $state({
  phase: 'idle' as 'idle' | 'preparing' | 'confirm' | 'running' | 'done',
  preview: null as ExecutionPreview | null,
  accepted: {} as Record<string, boolean>,
  progress: {} as Record<string, ActionResult>,
  report: null as ExecutionReport | null,
  error: '',
})

let initialized = false

export function initExecution() {
  if (initialized) return
  initialized = true
  onEvent<ActionResult>('exec:progress', (r) => {
    exec.progress[r.key] = r
  })
  onEvent<ExecutionReport>('exec:done', (r) => {
    exec.report = r
    if (r.error) exec.error = r.error
    exec.phase = 'done'
  })
}

function clean(e: unknown) {
  return String(e).replace(/^Error:\s*/, '')
}

export async function openConfirm(rulePaths: Iterable<string>) {
  const chosen = inv.findings
    .map((f) => ({ findingId: f.id, actionIndex: inv.chosen[f.id] ?? 0 }))
    .filter((c) => c.actionIndex >= 0)
  exec.error = ''
  exec.report = null
  exec.progress = {}
  exec.accepted = {}
  exec.preview = null
  exec.phase = 'preparing'
  try {
    const preview = await backend.PrepareExecution(chosen, [...rulePaths])
    exec.preview = preview
    const accepted: Record<string, boolean> = {}
    for (const a of preview.actions) accepted[a.key] = a.runnable && (a.kind === 'quarantine' || a.kind === 'template')
    exec.accepted = accepted
    exec.phase = 'confirm'
  } catch (e) {
    exec.error = clean(e)
    exec.phase = 'confirm'
    exec.preview = { actions: [] }
  }
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

export function close() {
  exec.phase = 'idle'
  exec.preview = null
  exec.report = null
  exec.progress = {}
  exec.accepted = {}
  exec.error = ''
}
