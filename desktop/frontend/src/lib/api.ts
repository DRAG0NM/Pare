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
  gapCount: number
  gaps: ScanError[]
}

export type Drive = { root: string; label: string; total: number; free: number }

export interface Backend {
  Drives(): Promise<Drive[]>
  ChooseFolder(): Promise<string>
  StartScan(root: string): Promise<void>
  CancelScan(): Promise<void>
  Children(path: string, limit: number): Promise<NodeView[]>
  Subtree(path: string, depth: number, limit: number): Promise<NodeTree>
  Reveal(path: string): Promise<void>
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
