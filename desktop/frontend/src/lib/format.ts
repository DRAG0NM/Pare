const units = ['B', 'KB', 'MB', 'GB', 'TB']

// Sizes use 1024-based units with Windows Explorer's labels.
export function bytes(value: number): string {
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  const digits = unit === 0 || size >= 100 ? 0 : 1
  return `${size.toFixed(digits)} ${units[unit]}`
}

export function count(value: number): string {
  return value.toLocaleString('zh-CN')
}

export function percent(part: number, whole: number): string {
  if (whole <= 0) return '0%'
  const value = (part / whole) * 100
  return value >= 10 ? `${value.toFixed(0)}%` : `${value.toFixed(1)}%`
}

export function seconds(value: number): string {
  return value < 60 ? `${value.toFixed(1)} 秒` : `${Math.floor(value / 60)} 分 ${Math.round(value % 60)} 秒`
}

// Keeps both ends of a long path visible.
export function middle(text: string, max = 72): string {
  if (text.length <= max) return text
  const keep = Math.floor((max - 1) / 2)
  return `${text.slice(0, keep)}…${text.slice(text.length - keep)}`
}

export const gapReasons: Record<string, string> = {
  access_denied: '没有权限',
  changed_entry: '扫描时被修改或删除',
  unsupported_information_class: '文件系统不支持',
  io_error: '读取出错',
}
