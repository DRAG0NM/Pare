// Shared state of the AI investigation, so it survives page and tab switches.
import { backend, onEvent, type AIConsent, type AIEvent, type AIFinished, type Finding } from './api'

export const inv = $state({
  scope: '',
  note: '',
  running: false,
  finished: false,
  events: [] as AIEvent[],
  findings: [] as Finding[],
  summary: '',
  error: '',
  tokens: 0,
  step: 0,
  pendingConsent: null as AIConsent | null,
  chosen: {} as Record<string, number>,
})

let initialized = false

export function initInvestigation() {
  if (initialized) return
  initialized = true
  onEvent<AIEvent>('ai:event', (e) => {
    inv.events.push(e)
    if (e.step > inv.step) inv.step = e.step
    if (e.tokens !== undefined) inv.tokens = e.tokens
    if (e.kind === 'finding' && e.finding) {
      inv.findings.push(e.finding)
      inv.chosen[e.finding.id] = 0
    } else if (e.kind === 'message' && e.text) {
      inv.summary = e.text
    } else if (e.kind === 'error' && e.text) {
      inv.error = e.text
    }
  })
  onEvent<AIConsent>('ai:consent', (c) => (inv.pendingConsent = c))
  onEvent<AIFinished>('ai:finished', (f) => {
    inv.running = false
    inv.finished = true
    inv.pendingConsent = null
    if (f.error) inv.error = f.error
    // The final list from the backend is authoritative.
    if (f.findings?.length) {
      inv.findings = f.findings
      for (const x of f.findings) if (!(x.id in inv.chosen)) inv.chosen[x.id] = 0
    }
  })
}

export function resetInvestigation() {
  inv.events = []
  inv.findings = []
  inv.summary = ''
  inv.error = ''
  inv.tokens = 0
  inv.step = 0
  inv.finished = false
  inv.pendingConsent = null
  inv.chosen = {}
}

export async function startInvestigation() {
  resetInvestigation()
  inv.running = true
  try {
    await backend.StartInvestigation(inv.scope, inv.note)
  } catch (e) {
    inv.running = false
    inv.error = String(e).replace(/^Error:\s*/, '')
  }
}

export async function stopInvestigation() {
  try {
    await backend.StopInvestigation()
  } catch (e) {
    inv.error = String(e)
  }
}

export async function answerConsent(allow: boolean) {
  const c = inv.pendingConsent
  if (!c) return
  inv.pendingConsent = null
  try {
    await backend.AnswerConsent(c.id, allow)
  } catch (e) {
    inv.error = String(e)
  }
}
