// The messages below mirror Chinese text produced by the Go backend (desktop/*.go,
// internal/actions, internal/plan and internal/advisor). Update this list whenever those
// messages change, otherwise they stay untranslated in the UI.
import { t } from './i18n.svelte'

const exact = new Set<string>([
  '请先完成一次扫描',
  '已有调查在进行',
  '已有操作在执行',
  '没有待执行的方案，请重新确认',
  '不是 Pare 隔离区里的记录',
  '没有可以执行的项目',
  '扫描结果里没有可以处理的内容，可能已经清理过或扫描后有变化，请重新扫描后再试',
  '常规缓存（规则）',
  '移入隔离区',
  'AI 尚未配置：请在设置里填写接口地址、模型和 API Key',
  'Pare 无法撤回这个操作',
  'Pare 无法撤回这个操作，但可以用同样的命令搬回来',
  '搬迁要复制整个虚拟磁盘，可能需要几分钟，期间不要关闭 Pare',
  '部分 WSL 版本出于数据安全考虑可能拒绝转换，失败时会显示原因',
  '会删除较早的系统日志',
  '删除这个文件即可恢复原来的设置',
  'apt 之后需要时会重新下载安装包',
  'npm 之后会重新下载需要的包',
  'pip 之后会重新下载需要的包',
  '会删除已停止的容器、未使用的网络、悬空镜像和构建缓存',
  '可能需要十几分钟',
  '清理后已安装的更新可能无法再卸载',
  '正在被设备使用的驱动包会被系统拒绝删除（不会强制）',
  '会关闭休眠和快速启动，可以用 powercfg /hibernate on 恢复',
  '不是受支持的动作模板',
  '命令为空或包含换行',
  '这是 AI 写的命令，不在 Pare 的受支持动作里，Pare 无法检查它会做什么',
  '请逐字读懂后再决定是否执行',
  '没有选择任何路径',
  '不在受保护的位置',
  '扫描结果里没有这个路径，请重新扫描后再选',
  '目录身份已记录',
  '链接或特殊文件，不处理',
  '已不是普通目录',
  '目录仍然存在',
  '调查结束（步数用完）。',
  '通义千问',
  '智谱',
  '自定义',
  'Windows 系统目录',
  '系统还原和卷信息',
  '回收站（请用系统清空）',
  '恢复环境',
  '启动文件',
  '系统升级文件',
  '安装程序回滚数据',
  '页面文件（请在系统设置中调整）',
  '休眠文件（请用 powercfg 关闭休眠）',
  '交换文件',
  'Pare 隔离区',
  '不是完整的绝对路径',
  '卷根目录',
  '系统必需的顶层目录',
  '用户配置文件根目录',
  '用户文件夹本身（可以处理其中的内容）',
  'AppData 根目录',
  '用户注册表文件',
  '应用商店程序目录',
  'Windows 系统数据',
])

// More specific templates come first; the broad "{x} 失败：{err}" style ones go last.
const templates: string[] = [
  '已移入隔离区 {n} 项',
  '移入 {moved} 项，{left} 项未处理：{list}',
  '其他 {n} 项',
  '请求 {url} 失败：{err}',
  '接口返回 {code}：{body}',
  '无法解析接口返回：{err}；内容：{body}',
  '接口没有返回结果：{body}',
  '写入失败：{err}',
  '发行版名 "{name}" 不合法',
  '目标位置 "{path}" 需要是完整的 Windows 路径',
  '目标位置是受保护的位置：{reason}',
  '目标位置 {path} 已经存在，请换一个新文件夹',
  '目标位置的上级文件夹 {path} 不存在',
  '日志大小 "{size}" 不合法，例如 100M',
  '日志大小 "{size}" 不合法，例如 200M',
  '驱动包名 "{name}" 不合法，应类似 oem27.inf',
  '不是受支持的动作模板：{name}',
  '会先关闭正在运行的 {name}，其中未保存的工作会丢失',
  '会先关闭正在运行的 {name}',
  '会在 {name} 里写入 /etc/systemd/journald.conf.d/pare.conf，之后日志最多占用 {size}',
  '受保护的位置：{reason}',
  '已包含在另一个选中的目录里：{path}',
  '已用完 {n} 步调查，正在整理已确认的问题…',
  'AI 建议：{title}',
  '{step} 失败：{err}',
  '{source}（{dir} 中的内容）',
]

// Placeholder values that are themselves backend messages.
const nested = new Set(['reason'])

const compiled = templates.map((template) => {
  const names: string[] = []
  const source = template
    .split(/(\{\w+\})/)
    .map((part) => {
      const m = part.match(/^\{(\w+)\}$/)
      if (m) {
        names.push(m[1])
        return '([\\s\\S]+?)'
      }
      return part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    })
    .join('')
  return { template, names, re: new RegExp('^' + source + '$') }
})

export function bt(text: string): string {
  if (!text) return text
  const s = text.replace(/^Error: /, '')
  if (exact.has(s)) return t(s)
  for (const { template, names, re } of compiled) {
    const m = s.match(re)
    if (!m) continue
    const params: Record<string, string> = {}
    names.forEach((name, i) => {
      params[name] = nested.has(name) ? bt(m[i + 1]) : m[i + 1]
    })
    return t(template, params)
  }
  return text
}
