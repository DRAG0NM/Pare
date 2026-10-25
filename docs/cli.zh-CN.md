# Pare 命令行

[English](cli.md)

命令行工具是桌面版所依赖的核心，每一步都以证据为准。`pare scan` 遍历指定目录，记录文件元数据和读不到的位置，输出带版本号的 JSON 报告。`pare plan` 用内置清理规则匹配报告，为每个候选文件生成方案，并附上判断依据。`pare apply` 把审阅过的方案中可执行的文件移入同一块盘上的隔离区，`pare undo` 再把它们移回原处。命令行工具从不删除文件。

## 构建

Pare 目前只支持 Windows，需要 Go 1.27 或更新版本。

```powershell
go test ./...
go vet ./...
go build -o pare.exe ./cmd/pare
```

## 扫描

```powershell
.\pare.exe scan --root 'C:\Users\me\Downloads' --json '.\downloads-scan.json'
```

`--json -` 把报告写到标准输出。写入文件时，先在目标目录写临时文件，再重命名到位。报告的 schema 是 `pare.scan/1`；可能超出 JavaScript 精确整数范围的文件 ID 和字节数以十进制字符串表示。

退出码：

| 代码 | 含义 |
|---:|---|
| 0 | 扫描完成，没有读不到的位置 |
| 2 | 参数无效或平台不支持 |
| 3 | 报告已写出，但有读不到的位置 |
| 4 | 扫描或写报告失败 |

退出码不为 0 不代表没有报告：代码 3 会和不完整的报告一起出现。使用前请检查报告的 `complete` 和 `errors` 字段。

## 生成方案

```powershell
.\pare.exe plan --scan '.\downloads-scan.json' --json '.\plan.json'
```

方案的 schema 是 `pare.plan/1`。每一项对应一个文件，写明匹配的规则、风险等级和一组证据，每条证据的状态是 `pass`、`fail`、`unknown` 或 `error`。只有所有必需的证据都通过，这一项才是 `executable`（可执行）：

| 证据 | 检查内容 |
|---|---|
| `path_rule` | 路径恰好匹配一条规则 |
| `attributes` | 不是系统文件 |
| `file_identity` | 扫描时拿到了文件 ID，且只出现在一个路径上（没有硬链接） |
| `age` | 距最后修改已满规则要求的天数，且修改时间不在未来 |
| `unchanged` | 文件 ID、大小和修改时间仍与扫描时一致 |
| `unlocked` | 能以独占方式打开读取，说明没有其他进程占用 |

最后两项是实时检查，只对其他证据都通过的项目执行，不读写文件内容。总量按文件 ID 去重统计可执行的字节数。`--ttl` 设置方案的有效期（默认 `24h`）。`digest` 字段是把 digest 置空后整份方案的 SHA-256，用来发现改动，不是签名。

内置规则（`builtin/1`，风险等级均为 E1）：各用户 `AppData\Local\Temp` 中 7 天以上没改过的文件，以及 npm、pip、Go 编译缓存中 30 天以上没用过的部分。

退出码：0 方案已写出，2 参数无效，4 读不了扫描报告或写方案失败。

## 执行与撤回

```powershell
.\pare.exe apply --plan '.\plan.json'
.\pare.exe apply --plan '.\plan.json' --confirm 'sha256:...'
.\pare.exe undo --manifest 'C:\.pare-quarantine\20261023-141444-d242abdf9fd5\manifest.json'
```

重复使用 `--item <path>` 可以只隔离指定的项目。每个指定的路径都必须是方案里的可执行项，否则整次执行在移动任何文件之前就会被拒绝。不加 `--item` 时处理所有可执行项。

不加 `--confirm` 时，`apply` 只显示方案会移动多少个文件，以及确认时要用的 digest，不移动任何东西。加上匹配的 digest 后，它会再次校验方案（digest 和有效期），然后逐项处理可执行项：

1. 在清单里把这一项记为 `intent`。清单写不进去时立即停止，不再移动其他文件。
2. 确认所有上级目录都不是重解析点。
3. 以不共享的方式打开文件，期间其他进程无法使用、替换或删除它；再通过这个句柄，把文件 ID、大小、修改时间和属性与方案对比。
4. 通过同一个句柄把文件重命名到批次目录。重命名不会覆盖已有文件，也不会跨盘。

没通过检查的文件记为 `skipped`，留在原处。隔离区默认在文件所在盘根目录的 `\.pare-quarantine`；`--quarantine` 可以指定同一块盘上的其他目录。每次执行会新建一个批次目录，里面是 `manifest.json`（schema `pare.execution/1`）和被移走的文件。

`undo` 会恢复状态为 `quarantined` 的项目，以及执行中断时留下的 `intent` 项目。它会重建缺失的上级目录；原路径上已经有文件时不会覆盖，而是把这一项记为 `conflict`。清理掉原路径上的文件后再运行一次 `undo`，就能恢复这些项目。

退出码：0 所有项目都已移动（或恢复），2 参数无效或缺少确认，3 有项目被跳过、失败或冲突，4 方案或清单被拒绝、无法读取。

## 服务模式

```powershell
.\pare.exe serve
```

`pare serve` 供智能体宿主使用，例如 Pare-DSH 插件。它是一个常驻进程，在内存里保留一份扫描索引，通过标准输入输出收发按行分隔的 JSON，协议名为 `pare.serve/1`。它不接受其他参数，输入关闭后以退出码 0 结束。

每个请求占一行：`{"id","method","params"}`；每个应答也占一行：`{"id","result"}`，失败时是 `{"id","error"}`，`error` 为错误信息字符串。请求按顺序逐个处理，所以 `scan` 之后发出的调用看到的就是这次扫描的索引。

| 方法 | 作用 |
|---|---|
| `hello` | 返回协议名、核心版本、只读调查工具（名称、说明、参数 schema）和支持的动作模板 |
| `scan` | 扫描一个绝对 Windows 路径并保留索引，返回摘要。重新扫描会丢弃之前准备的所有清理 |
| `tool` | 对当前索引运行一个调查工具（`name`、`args`），需要先扫描 |
| `read_sensitive` | 读取被标为敏感的文件内容，只对这一次调用有效 |
| `prepare` | 接收要隔离的路径和动作模板，返回 `id`、`digest`、有效期和逐项预览，不改动任何东西 |
| `execute` | 按 `id` 和 `digest` 执行已准备的清理：先隔离，再运行模板 |
| `batches` | 列出本地盘上的隔离批次 |
| `undo` | 按 `manifest` 路径恢复一个隔离批次 |
| `purge` | 按 `manifest` 路径永久删除一个隔离批次 |

宿主应当在 `execute`、`undo`、`purge` 以及 `read_sensitive` 之前取得用户批准。核心不依赖这一点：它拒绝受保护的位置，对每一项做与 `apply` 相同的证据和逐项检查，并拒绝 digest 不匹配、超过 30 分钟、已经执行过（清理在运行前就被标记为已使用）或根本不存在的预览。`undo` 和 `purge` 只接受 Pare 隔离区内的 `manifest.json`。
