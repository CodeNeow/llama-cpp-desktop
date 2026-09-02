# 设计参考：GUI ↔ 无头（API 路由）模式交接与接管

MyLlama 的 GUI ↔ 无头模式切换：退出侧写交接记录，接管侧探活后认领仍在运行的 llama-server。

> 权威来源：`core/handover.go`、`core/headless.go`、`core/bridge.go`（adoptedPid 状态）、
> `core/server.go`（日志 tailer、`serverTrueStart`）、`core/app.go`、`main.go`、
> `core/proctime_*.go`。与代码冲突时以代码为准。
>
> 注：`serverStartedAt` 字段与 PID 复用启动时间检查（含 `core/proctime_*.go`）是另一批
> **进行中（未提交）** 的工作区改动，本文按工作区现状记录；若该特性被回退，请同步删除相关段落。

## 为什么需要交接记录（唯一目标）

模式切换 = 当前进程退出 + 拉起一个新进程（GUI → 无头：`relaunchSelf("--headless")`；
无头 → GUI：`relaunchSelf()` 无参数）。正在服务的 llama-server 不能被顺带杀掉重启（模型加载
几十秒，OpenAI API 会中断）。因此：退出侧把**仍在运行的 llama-server 的身份**（pid + 端口 +
日志路径 + 进程创建时间）写进一个 JSON 文件，接管侧探测后**认领**它而不是启动新的。

```
   退出侧（当前进程）                              接管侧（重启后的进程）
   ──────────────────                            ─────────────────────
   SetApiRouteMode(true)（GUI→无头）              main.go: ShouldRunHeadless?
   或 托盘「显示主窗口」（无头→GUI）                   │ 是 → RunHeadless
        │ writeHandover(pid, port, startedAt,      │ 否 → App.Startup
        │   serverStartedAt, logPath)              ▼
        ▼                                     evaluateHandover()
   llama-desktop-server-handover.json          ├─ 缺失 / 损坏 / 不健康 → 删记录+全新启动
        ── relaunchSelf ──▶                    └─ 健康（探活 ∧ pid ∧ 启动时间）
                                                     → adoptHandover()
   llama-server 子进程  ◀── 全程不停止；serverCmd=nil, adoptedPid=pid；
   （父进程已退出，无管道可达）      日志写入同一文件，tailer 从 EOF 续读
```

## 交接记录 schema

文件 `llama-desktop-server-handover.json`，与配置文件同为进程工作目录下的裸文件名（var，
测试重定向到临时目录）。`atomicWriteFile`（tmp + fsync + 原子重命名，0644）+ `MarshalIndent`。

| 字段 | 类型 | 含义 |
| --- | --- | --- |
| `pid` | int | llama-server pid（子进程 `cmd.Process.Pid`，或被接管时沿用 `adoptedPid`） |
| `port` | int | 监听端口（取 `serverPort`，不读当前配置——改配置不能污染记录） |
| `startedAt` | string | **记录写入时刻**（RFC3339），不是 llama-server 的启动时间，不参与判定 |
| `serverStartedAt` | string | llama-server 进程的**真实创建时间**（RFC3339，`omitempty`，取自 `serverTrueStart`）；供后继做 PID 复用检测。旧版本记录可能缺失 → 缺失 = 跳过启动时间检查（fail-open） |
| `logPath` | string | 日志文件**绝对路径**（`omitempty`；`absServerLogPath` 保证后继不依赖退出侧工作目录）。缺失 = 接管但不 tail |

## 接管决策流（纯函数核心）

`evaluateHandover` 读文件并探活，`decideHandoverAction(fileExists, rec, healthy, startMatches)`
是纯决策核心（单测直接喂输入，不开 socket）：

```
文件不存在     → 空计划（全新启动，无文件可删）
存在不可解析   → RemoveFile（删除记录 + 全新启动）
存在且解析成功 → healthy      = probeHandoverHealth(port) && handoverPidAlive(pid)
                startMatches = !healthy || handoverStartMatches(rec)
                （启动时间探针只在健康探针通过后才运行——服务已死时记录必然过期，
                  再做 OS 查询与 [WARN] 只是噪音）
                healthy && startMatches → Adopt{PID, Port, LogPath, ServerStartedAt}
                否则                    → RemoveFile
```

三个探活都是可注入的 var（测试不碰真实网络/进程）：

- `probeHandoverHealth(port)`：GET `http://127.0.0.1:{port}/health`，2 秒超时；**任何 HTTP
  响应即健康（不看状态码）**——未授权请求也会回 4xx，端口上有响应就足以证明服务还活着。
- `handoverPidAlive(pid)` → 平台 `processAlive`（Windows：`OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)`）。
- `handoverProcStartTime(pid)` → 平台 `processStartTime`（`core/proctime_windows.go`：
  `GetProcessTimes` 取创建时间；非 Windows 为 stub——无头模式本就 Windows-only）。查询失败返回
  ok=false，调用方视为「检查不可用」而 fail-open。

两条启动路径的差异：**GUI**（`App.Startup` → `adoptOrCleanHandover`）健康 → 接管、过期 → 删
记录，**绝不自动启动服务**（保持「GUI 打开 ≠ 服务必须开」）；**无头**（`RunHeadless` →
`startOrAdoptServer`）接管 / 删记录 + **自动** `startServerInternal`；启动失败只 `[WARN]` +
原生 MessageBox（`notifyHeadlessServerStartFailed`），无头进程继续运行——托盘是回到 GUI 的
唯一回路，不能死。

## Fail-open 矩阵

| 现场情况 | 计划 | 理由 |
| --- | --- | --- |
| 无记录文件 | 全新启动 | 正常路径，无文件可删，绝不报错 |
| 记录损坏 / 不可解析 | 删除记录 + 全新启动 | 宁可重开服务，不可基于坏数据行动 |
| /health 无响应 或 pid 已死 | 删除记录 + 全新启动 | 记录已过期 |
| `serverStartedAt` 与查询到的创建时间不匹配 | 删除记录 + 全新启动 | pid 已被复用：活着且应答的 pid 不是当初那台服务 |
| 探活 + pid + 启动时间均通过 | 接管（pid/port/logPath/serverStartedAt） | 唯一接管路径 |
| 记录缺 `serverStartedAt`（legacy） | 接管，跳过启动时间检查 | legacy 容忍：字段 `omitempty` |
| `serverStartedAt` 不可解析 | 接管，跳过检查（`[WARN]`） | 数据坏 ≠ 现场坏，端口探活仍在守门 |
| 平台查不到进程创建时间 | 接管，跳过检查（`[WARN]`） | 检查不可用 ≠ 检查失败 |
| 记录缺 `logPath`（legacy） | 接管，**不做日志 tail** | legacy 容忍 |
| `logPath` 打不开 | 接管，跳过 tail（`[WARN]`） | 接管绝不因日志捕获失败而失败 |

原则：**fail-open 永远指向「全新启动」或「按现状接管」，绝不指向「误接管一个无关进程」**——
误接管意味着停止路径会去 kill 它。「做不了检查」的情况全部跳过该检查；只有**确定的反证**
（探活失败、时间不匹配）才判过期。

## PID 复用防御

只查「pid 存活」不可信：Windows 很快复用 pid。判定因此是**三重合取**：pid 存活；
记录端口 `/health` 有响应（端口上那台服务还在的最强证据）；记录的 `serverStartedAt`
与 OS 查询到的该 pid 创建时间之差在 `handoverStartTolerance`（**5 秒**——记录在进程创建后
毫秒级内写就，5 秒吸收读钟与调度延迟）内。`handoverStartMatches` 对差值超出 ±5 秒打 `[INFO]`
并判过期；每类「做不了检查」的情况打 `[WARN]` 后放行。代价是无头启动多一次最多 2 秒的本地
HTTP 探测加一次 OS 进程查询——可接受。

## 接管后的状态与停止路径

`adoptHandover` 在 `serverMu` 下写：`serverRunning=true`、`serverPort=port`、`adoptedPid=pid`、
**`serverCmd=nil`**（子进程属于已退出的前代，没有可 Signal/Wait 的句柄）、
`serverStartTime=time.Now()`（uptime 从接管时刻起算）、`serverTrueStart=记录的 serverStartedAt`。
`serverTrueStart` 的**链式传递**让启动时间检查能跨多次交接成立：接管后再交接时，`writeHandover`
从 `serverTrueStart` 取值，重新记录这台服务的**原始**创建时间而非最近一次接管的时刻。

已知边界（进行中特性）：legacy 记录（无 `serverStartedAt`）被接管时 `serverTrueStart` 退化为
**接管时刻**——链上下一个记录因此携带接管时刻而非真实创建时间，下一次交接的启动时间检查会合法
地判不匹配（通常远超 5 秒容差）→ 删除记录 + 全新启动。方向安全（代价是多一次模型加载，绝不会
误接管无关进程），全新启动后恢复为真实创建时间——legacy 链最多引入一次多余重启。正常子进程路径
在启动成功时令 `serverTrueStart = serverStartTime`（bridge.go）。

**硬性不变量：`adoptedPid > 0 ⟹ serverCmd == nil`**（`core/bridge.go`）；正常子进程路径的
对偶不变量是 `serverRunning==true ⟹ serverCmd.Process != nil`。两种形态互斥，所有依赖
「有句柄/没句柄」的代码都以此分派。`serverTrueStart` 与生命周期状态同受 `serverMu` 保护。
**锁序：`serverMu` → `serverLogsMu`**（`core/server.go`），同时持两锁的路径必须按此序。

- 停止：`stopServerInternal` 先走 `stopAdoptedServerIfAny`（running ∧ cmd==nil ∧ adopted>0）
  → `killProcessByPid`（`os.FindProcess(pid).Kill()`，注入点）→ 清状态 → 停 tailer →
  `removeHandover`。`App.Shutdown` 同样先走它。
- 下次启动（`startServerInternal`）：先停掉遗留 tailer，避免新旧 tailer 双写 ring。

## 接管侧的日志 tail

为什么是**文件**而不是管道：接管来的子进程属于已退出的前代，没有管道可达。启动路径把子进程
stdout/stderr 指向**同一个** `*os.File`（一个 open file description → 共享写偏移，并发写不会
行内交错），于是「spawn 的孩子」与「接管的孩子」产生同构日志流——任何进程重开文件即可续上。

`adoptServerLogTail(logPath)`：空路径 → 返回（legacy，无 tail）；`os.Stat` 失败 → `[WARN]`
返回；`startServerLogTailer(logPath, /*fromStart=*/false)` **先 seek 到 EOF**——被接管服务的
既有日志绝不能回放进 ring；新 tailer 装好后再 `Stop()` 旧的（防双写）。

tailer 行为契约（`core/server.go`）：100 ms 轮询；EOF 不是终点（日志还在长）；`Stop` 后先
drain 剩余并 flush 行装配器（不丢最后半行）；goroutine 内 `recover`——tailer 永不拖垮应用；
partial 重绘 400 ms 节流，完整行即时入 ring（监控 TPS 解析依赖完整行）。

## 退出侧流程与失败策略

| 方向 | 入口 | 失败策略 |
| --- | --- | --- |
| GUI → 无头 | `SetApiRouteMode(true)`：dev 构建 / 未开托盘直接拒绝 → 持久化偏好 → `writeServerHandover` → `relaunchSelf("--headless")` → `switchRestartPending=true` → `wailsRuntime.Quit` | relaunch 失败：返回错误、**不置标记**、GUI 照常继续（之后正常退出会停服务） |
| 无头 → GUI | 托盘回调 `returnToGuiFromHeadless`：持久化 `apiRouteMode=false` → `writeServerHandover` → `relaunchSelf()`（无参数）→ `switchRestartPending=true` → `requestHeadlessExit` | relaunch 失败：**留在无头模式**——此时退出会让用户无路可回 |

`writeServerHandover` 边界：服务未运行、pid/port 为 0 时**不写文件**（静默 nil）——没有服务就
没有需要交接的东西，留旧记录反而会让接管侧误判。

共同出口不变量：**必须先成功 relaunch 再置 `switchRestartPending` 并退出**。
`switchRestartPending==true` 时 `Shutdown` 跳过停服与下载取消（app.go）——服务交给后继进程，
下载从持久化队列恢复。单实例互斥锁的 retry 窗口覆盖交接期（后继进程可能在旧进程尚未完全退出时
启动，短暂重试即可拿到锁）。
