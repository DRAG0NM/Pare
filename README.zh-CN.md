<p align="center">
  <img src="docs/images/banner.svg" alt="Pare：Windows 上的 AI 磁盘调查员" width="100%">
</p>

<p align="center">
  <a href="README.md">English</a> · <b>简体中文</b>
</p>

<p align="center">
  <img alt="平台" src="https://img.shields.io/badge/platform-Windows%2011-0e7d60?style=flat-square">
  <img alt="版本" src="https://img.shields.io/badge/version-0.2.0--beta-3cc59b?style=flat-square">
  <img alt="许可证" src="https://img.shields.io/badge/license-MIT-2b3632?style=flat-square">
  <img alt="Go" src="https://img.shields.io/badge/Go-core-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="Svelte" src="https://img.shields.io/badge/Svelte-UI-FF3E00?style=flat-square&logo=svelte&logoColor=white">
</p>

常见的清理软件清的是每台电脑都有的缓存。真正让 C 盘越来越满的，往往是只属于这台电脑的问题：更新器一遍遍重复下载同一个几百 MB 的安装包，WSL 虚拟磁盘只增不减，配置写错了位置、一直没生效，还有为这台电脑永远用不上的平台准备的原生二进制。

**Pare 让 AI 去调查这类浪费。** 它读扫描结果，打开需要辨认的文件，核对已安装的软件和系统状态，然后把每个问题讲清楚：是什么、为什么会出现、会不会再长出来、可以怎么处理。你看过具体的文件或命令并确认之前，磁盘上什么都不会改动；文件类的清理先移入隔离区，随时可以撤回。

<p align="center">
  <img src="docs/images/demo.gif" alt="扫描、AI 深挖、确认执行和清理统计" width="100%">
</p>
<p align="center"><sub>画面来自内置的演示模式，数据仅作示意。</sub></p>

## 主要特点

- **调查，而不是套规则。** 模型只拿到只读工具：查询扫描索引、看文件头和文本内容、列出已安装软件和正在运行的进程，以及一组固定的诊断命令（WSL、驱动库、组件存储、卷影副本、Docker、休眠）。每条发现都附带证据。
- **找根因，也防复发。** 每条发现都会说明会不会再长出来。会的话，Pare 给出防止复发的动作，比如给 WSL 里的 journald 日志设上限，清理完可以一键执行。
- **默认可撤回。** 文件和文件夹移入同一块盘上的隔离区，可以按批次恢复。永久删除是单独的一步，需要你明确操作。
- **执行前再核对一次。** 受保护的位置、文件身份、扫描后有没有变化、有没有程序占用，生成方案时检查一遍，真正移动前再检查一遍。
- **数字是实测的。** 清理前后的可用空间来自实际读数，不是估算；还留在隔离区里的空间单独列出。
- **扫得快，也扫得全。** 目录并行读取；以管理员身份运行时启用备份权限，只对系统开放的文件夹也会被统计，不会悄悄漏掉。

## 界面一览

<table>
  <tr>
    <td width="50%"><img src="docs/images/overview.png" alt="目录树与 treemap 联动"><br><sub><b>扫描</b>：目录树和 treemap 联动，读不到的位置单独列出，不会算成 0。</sub></td>
    <td width="50%"><img src="docs/images/investigate.png" alt="AI 发现：原因、证据和处理方案"><br><sub><b>AI 深挖</b>：每条发现的原因、证据、会不会复发和可选方案。</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/images/confirm.png" alt="确认执行：列出具体文件和命令"><br><sub><b>确认执行</b>：具体的文件和命令、风险提示，以及是否需要管理员权限。</sub></td>
    <td width="50%"><img src="docs/images/summary.png" alt="清理统计：实测可用空间和防止复发"><br><sub><b>清理统计</b>：实测的可用空间、隔离区里还剩多少，以及一键防止复发。</sub></td>
  </tr>
</table>

## 开始使用

1. 从 [Releases](https://github.com/DRAG0NM/Pare/releases) 下载 `pare-desktop.exe`。它是单个可执行文件，不需要安装。
2. 右键选择 **以管理员身份运行**，扫描结果才完整。不以管理员身份运行时，只对系统开放的文件夹会显示为读不到。
3. 选择一个磁盘。第一次进入「清理」时，Pare 会请你填写一个兼容 OpenAI 接口的 API Key。默认是 DeepSeek，也可以选通义千问、Kimi、智谱、OpenAI 或自定义地址。
4. 开始深挖，查看发现，为每一条选好处理方案，然后确认执行。

**系统要求。** Windows 11 x64（已测试）。装有 [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/) 的 Windows 10 理论上可以运行，但没有测试过。程序暂时没有代码签名，第一次打开时 SmartScreen 可能会要求确认。

**时间和费用。** 在开发机上，一次标准档深挖约两分钟，消耗 130 万到 150 万 tokens。DeepSeek 有缓存命中折扣，五次运行一共花了 ¥0.73，平均每次约 ¥0.15。更深的档位用时更长。

## 工作流程

```text
 扫描 ──► AI 深挖 ──► 确认执行 ──► 执行 ──► 清理统计 ──► 隔离区
 只读索引   只读工具        你逐项确认    先隔离       实测空间      恢复或
            敏感文件需同意                再跑模板                   永久清空
```

| 阶段 | 做什么 |
|---|---|
| **扫描** | 并行读取磁盘或文件夹，建立大小、文件 ID 和修改时间的索引。重解析点只列出，不跟进。 |
| **AI 深挖** | 模型先看总览和本地算出的异常线索（超大文件、虚拟磁盘、同尺寸文件扎堆、多版本并存、多平台二进制），再用工具逐步缩小范围，证据够了就报告。有三个深度可选；「继续深挖」会去找还没报告过的问题。 |
| **确认执行** | 对每个选中的方案，列出要移动的文件或要运行的命令、风险提示和是否需要管理员权限。模板以外的命令必须勾选“已读懂”才能执行。 |
| **执行** | 先执行隔离。受支持的模板以校验过的参数数组运行，不经过 shell。 |
| **清理统计** | 清理前后的可用空间、隔离中的大小和清空按钮、每个方案的结果，以及防止复发的动作。 |
| **隔离区** | 列出本地各盘的隔离批次；恢复时不会覆盖原位置上已经出现的新文件。页面底部是一份简单的清理记录。 |

**受支持的动作模板：** 把 WSL 发行版迁到其他盘、把它的虚拟磁盘设为稀疏、压缩或限制 WSL 内的 journald 日志、清理 WSL 内的 apt 缓存、清理 npm 和 pip 缓存、`docker system prune`、DISM 组件清理、删除一个驱动包、关闭休眠。

## 隐私与安全

- **哪些内容会发出去。** 目录结构、文件名，以及模型选择读取的文件内容，会发送给你配置的服务商。读取密钥类文件（SSH 密钥、`.env`、浏览器登录数据、证书、凭据库）的内容，每次都要你同意。
- **模型能做什么。** 它的工具都是只读的。它负责提建议，Pare 负责校验，最后由你决定。被扫描文件里的文字只当作数据，不会被当成指令。
- **API Key 存在哪里。** 用 Windows DPAPI 加密保存在 `%APPDATA%\Pare\advisor.json`，只有你的 Windows 账户能解密。
- **哪些位置不会动。** 卷根目录、`Windows`（其中的临时、更新下载和转储文件夹除外）、`System Volume Information`、回收站、`Recovery`、用户目录和 `AppData` 的根、页面文件和休眠文件、`ProgramData\Microsoft`、`Program Files\WindowsApps`。无论模型怎么建议，执行器都会拒绝处理这些位置。

## 已知限制

- 每次深挖的结果不完全一样。WinSxS 组件存储、重复的驱动包这类大项不是每次都会报告；「继续深挖」通常能补上一部分。
- 模型判断“某个文件是否仍被程序需要”时不够稳定。标为「需谨慎」的项目值得多看一眼；移入隔离区后仍可恢复。
- 模板和命令执行后，Pare 无法撤回。请仔细阅读确认框。
- 目录枚举看不到 NTFS 元数据（`$MFT` 等）和系统保留存储，所以整盘扫描列出的总量会小于已用空间。WizTree 这类直接读 MFT 的工具能看到这部分。
- 界面目前只有简体中文。

## 从源码构建

```powershell
pnpm --dir desktop/frontend install
pnpm --dir desktop/frontend build
go build -tags desktop,production -ldflags "-w -s -H windowsgui" -o pare-desktop.exe ./desktop
```

不需要 Wails CLI，Wails v2 只作为 Go 库使用。`pnpm --dir desktop/frontend dev` 会在浏览器里用内置演示数据打开界面，方便调整布局。程序图标和版本信息由 [go-winres](https://github.com/tc-hib/go-winres) 根据 `desktop/winres` 生成。

命令行核心（`pare scan`、`plan`、`apply`、`undo`）的说明见 [docs/cli.zh-CN.md](docs/cli.zh-CN.md)。设计说明和分阶段计划见 [docs/design.md](docs/design.md) 和 [docs/development-plan.md](docs/development-plan.md)。

## 许可证

Pare 以 [MIT 许可证](LICENSE) 发布。编译进程序的所有依赖的许可证汇总在 [desktop/THIRD_PARTY_LICENSES.txt](desktop/THIRD_PARTY_LICENSES.txt)；核心依赖的说明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
