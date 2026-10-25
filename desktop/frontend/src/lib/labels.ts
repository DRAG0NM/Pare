// Plain-language wording for rules, risk levels and evidence. The core keeps its
// English, machine-oriented text; only this file decides what users read.
import type { Evidence } from './api'
import { t } from './i18n.svelte'
import { bt } from './backendText'

export const riskLabels: Record<string, { label: string; tone: 'safe' | 'check' | 'manual' | 'blocked' }> = {
  E0: { label: '可放心清理', tone: 'safe' },
  E1: { label: '建议确认', tone: 'check' },
  E2: { label: '需人工判断', tone: 'manual' },
  E3: { label: '禁止清理', tone: 'blocked' },
}

export function riskLabel(risk: string) {
  const r = riskLabels[risk] ?? riskLabels.E2
  return { label: t(r.label), tone: r.tone }
}

type RuleText = { name: string; explain: string }

const ruleTexts: Record<string, RuleText> = {
  'user-temp': { name: '用户临时文件', explain: '用户 Temp 文件夹里 {days} 天以上没改过的文件。程序运行时留下的临时文件，一般可以重新生成。' },
  'npm-cache': { name: 'npm 缓存', explain: 'npm 下载过的包缓存，{days} 天以上没用过的部分。删掉后 npm 需要时会重新下载。' },
  'pip-cache': { name: 'pip 缓存', explain: 'pip 下载和编译过的包缓存，{days} 天以上没用过的部分。删掉后会重新下载。' },
  'go-build-cache': { name: 'Go 编译缓存', explain: 'Go 编译的中间产物，{days} 天以上没用过的部分。删掉后下次编译会重新生成。' },
}

export function ruleName(id: string, fallback = ''): string {
  const name = ruleTexts[id]?.name
  return name ? t(name) : fallback || id
}

export function ruleExplain(id: string, days: number, fallback = ''): string {
  const explain = ruleTexts[id]?.explain
  return explain ? t(explain, { days }) : fallback
}

const evidenceNameTexts: Record<string, string> = {
  path_rule: '位置符合规则',
  attributes: '不是系统文件',
  file_identity: '文件身份唯一',
  age: '足够久没修改',
  unchanged: '扫描后没变化',
  unlocked: '没有程序在用',
}

// Values are translated on access, so callers can keep indexing it directly.
export const evidenceNames: Record<string, string> = new Proxy(evidenceNameTexts, {
  get: (target, prop) => (typeof prop === 'string' && prop in target ? t(target[prop]) : undefined),
})

const patterns: [RegExp, (m: RegExpMatchArray) => string][] = [
  [/^written (\d+) days ago, rule requires (\d+)$/, (m) => t('{days} 天前修改，规则要求至少 {min} 天', { days: m[1], min: m[2] })],
  [/^last write time is in the future$/, () => t('修改时间在未来，时间信息不可信')],
  [/^no last write time$/, () => t('没有修改时间')],
  [/^conflicting rules: (.+)$/, (m) => t('同时符合多条规则（{rules}），无法确定归属', { rules: m[1] })],
  [/^system file$/, () => t('系统文件')],
  [/^not a system file$/, () => t('普通文件')],
  [/^scan has no file ID$/, () => t('扫描时没拿到文件 ID')],
  [/^file ID appears at (\d+) paths \(hard link\)$/, (m) => t('这是硬链接，同一个文件出现在 {n} 个位置，清掉一处不会腾出空间', { n: m[1] })],
  [/^single path for file ID$/, () => t('只在一个位置出现')],
  [/^file ID, size and last write time match the scan$/, () => t('大小、修改时间和文件身份都与扫描时一致')],
  [/^changed since scan: (.+)$/, (m) => t('扫描后已经变化（{detail}），需要重新扫描', { detail: m[1] })],
  [/^file no longer exists$/, () => t('文件已经不在了')],
  [/^file is open in another process$/, () => t('正被其他程序使用')],
  [/^exclusive read open succeeded$/, () => t('没有程序在使用它')],
  [/^file probes are only available on Windows$/, () => t('只能在 Windows 上检查')],
  [/^probe cancelled$/, () => t('检查被取消')],
]

export function evidenceText(evidence: Evidence, ruleId = ''): string {
  if (evidence.kind === 'path_rule' && evidence.status === 'pass') return t('符合“{rule}”规则', { rule: ruleName(ruleId) })
  if (evidence.kind === 'unchanged' && evidence.detail === 'file is open in another process') return t('文件被占用，无法核对是否有变化')
  for (const [pattern, render] of patterns) {
    const match = evidence.detail.match(pattern)
    if (match) return render(match)
  }
  return evidence.detail
}

// The first piece of evidence that does not pass explains why an item cannot be cleaned.
export function blockedReason(evidence: Evidence[], ruleId = ''): string {
  const failing = evidence.find((e) => e.status !== 'pass')
  if (!failing) return t('证据不完整')
  return evidenceText(failing, ruleId)
}

export function daysAgo(iso: string): number {
  return Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000)
}

// Files unpacked from archives often keep timestamps from around 1980; a date reads
// better than "17000 天前" there.
export function ageText(iso: string): string {
  const days = daysAgo(iso)
  return days > 3650 ? iso.slice(0, 10) : t('{n} 天前', { n: days })
}

// Item reasons and result details from the execution core, shown in Chinese.
export function execText(s: string): string {
  if (s === 'file no longer exists') return t('已不存在（可能已清理）')
  if (s === 'file is open in another process') return t('正被其他程序使用')
  if (s.startsWith('changed since scan:')) return t('扫描后已变化，请重新扫描')
  if (s.startsWith('changed since plan:')) return t('生成方案后已变化，请重新扫描')
  if (s === 'system file') return t('系统文件')
  const m = s.match(/file ID appears at (\d+) paths/)
  if (m) return t('硬链接，同一文件在 {n} 个位置', { n: m[1] })
  if (s === 'a file inside the directory is in use') return t('文件夹里有文件正被使用')
  if (s.startsWith('protected location:')) return t('受保护的位置：{path}', { path: bt(s.slice('protected location:'.length).trimStart()) })
  return bt(s)
}
