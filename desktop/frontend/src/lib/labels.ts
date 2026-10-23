// Plain-language wording for rules, risk levels and evidence. The core keeps its
// English, machine-oriented text; only this file decides what users read.
import type { Evidence } from './api'

export const riskLabels: Record<string, { label: string; tone: 'safe' | 'check' | 'manual' | 'blocked' }> = {
  E0: { label: '可放心清理', tone: 'safe' },
  E1: { label: '建议确认', tone: 'check' },
  E2: { label: '需人工判断', tone: 'manual' },
  E3: { label: '禁止清理', tone: 'blocked' },
}

export function riskLabel(risk: string) {
  return riskLabels[risk] ?? riskLabels.E2
}

type RuleText = { name: string; explain: (days: number) => string }

const ruleTexts: Record<string, RuleText> = {
  'user-temp': { name: '用户临时文件', explain: (d) => `用户 Temp 文件夹里 ${d} 天以上没改过的文件。程序运行时留下的临时文件，一般可以重新生成。` },
  'npm-cache': { name: 'npm 缓存', explain: (d) => `npm 下载过的包缓存，${d} 天以上没用过的部分。删掉后 npm 需要时会重新下载。` },
  'pip-cache': { name: 'pip 缓存', explain: (d) => `pip 下载和编译过的包缓存，${d} 天以上没用过的部分。删掉后会重新下载。` },
  'go-build-cache': { name: 'Go 编译缓存', explain: (d) => `Go 编译的中间产物，${d} 天以上没用过的部分。删掉后下次编译会重新生成。` },
}

export function ruleName(id: string, fallback = ''): string {
  return ruleTexts[id]?.name ?? (fallback || id)
}

export function ruleExplain(id: string, days: number, fallback = ''): string {
  return ruleTexts[id]?.explain(days) ?? fallback
}

export const evidenceNames: Record<string, string> = {
  path_rule: '位置符合规则',
  attributes: '不是系统文件',
  file_identity: '文件身份唯一',
  age: '足够久没修改',
  unchanged: '扫描后没变化',
  unlocked: '没有程序在用',
}

const patterns: [RegExp, (m: RegExpMatchArray) => string][] = [
  [/^written (\d+) days ago, rule requires (\d+)$/, (m) => `${m[1]} 天前修改，规则要求至少 ${m[2]} 天`],
  [/^last write time is in the future$/, () => '修改时间在未来，时间信息不可信'],
  [/^no last write time$/, () => '没有修改时间'],
  [/^conflicting rules: (.+)$/, (m) => `同时符合多条规则（${m[1]}），无法确定归属`],
  [/^system file$/, () => '系统文件'],
  [/^not a system file$/, () => '普通文件'],
  [/^scan has no file ID$/, () => '扫描时没拿到文件 ID'],
  [/^file ID appears at (\d+) paths \(hard link\)$/, (m) => `这是硬链接，同一个文件出现在 ${m[1]} 个位置，清掉一处不会腾出空间`],
  [/^single path for file ID$/, () => '只在一个位置出现'],
  [/^file ID, size and last write time match the scan$/, () => '大小、修改时间和文件身份都与扫描时一致'],
  [/^changed since scan: (.+)$/, (m) => `扫描后已经变化（${m[1]}），需要重新扫描`],
  [/^file no longer exists$/, () => '文件已经不在了'],
  [/^file is open in another process$/, () => '正被其他程序使用'],
  [/^exclusive read open succeeded$/, () => '没有程序在使用它'],
  [/^file probes are only available on Windows$/, () => '只能在 Windows 上检查'],
  [/^probe cancelled$/, () => '检查被取消'],
]

export function evidenceText(evidence: Evidence, ruleId = ''): string {
  if (evidence.kind === 'path_rule' && evidence.status === 'pass') return `符合“${ruleName(ruleId)}”规则`
  if (evidence.kind === 'unchanged' && evidence.detail === 'file is open in another process') return '文件被占用，无法核对是否有变化'
  for (const [pattern, render] of patterns) {
    const match = evidence.detail.match(pattern)
    if (match) return render(match)
  }
  return evidence.detail
}

// The first piece of evidence that does not pass explains why an item cannot be cleaned.
export function blockedReason(evidence: Evidence[], ruleId = ''): string {
  const failing = evidence.find((e) => e.status !== 'pass')
  if (!failing) return '证据不完整'
  return evidenceText(failing, ruleId)
}

export function daysAgo(iso: string): number {
  return Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000)
}
