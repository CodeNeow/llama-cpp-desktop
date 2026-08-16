# Llama Desktop 开发规范

## 项目概览

Llama Desktop 是一个本地大模型桌面管理工具：基于 **Wails v2**（Go 1.25 后端 + WebView2 前端），前端为 **Vue 3 + TypeScript + Vite 5**（无第三方 UI 库，手写 CSS 变量主题），推理引擎为 **llama.cpp**（llama-server 路由器模式）。

核心链路：`LLM-Models/` 目录扫描 GGUF → 逐模型推理参数配置 → 生成 llama-server 模型预设（INI）→ 启动 OpenAI 兼容服务（默认 `127.0.0.1:8080`）。

## 常用命令

在仓库根目录执行：

```bash
wails dev                 # 开发模式：Go 后端 + Vite 前端（:5173）热重载
wails build               # 生产构建，产物 build/bin/llama-desktop.exe（build/ 已 gitignore）
cd frontend && npm run build   # 仅构建前端（vue-tsc 类型检查 + vite build）
cd frontend && npm run dev     # 仅起 Vite（无 Wails 运行时，后端调用会失败，见下文）
```

验证门（改动后按需执行，见「提交前测试分级」）：

```bash
go build ./...                                   # 后端编译
go test ./...                                    # 后端单测（标准库 testing）
gofmt -l .                                       # Go 格式化检查，必须无输出
golangci-lint run                                # Go 静态检查（govet / ineffassign / unused）
cd frontend && npm run build                     # 前端类型检查 + 构建（vue-tsc --noEmit 零错误）
cd frontend && npm test                          # 前端单测（vitest）
make check                                       # 组合验证门（POSIX）
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1  # 组合验证门（Windows）
```

> 前端产物 `frontend/dist` 是 `go:embed` 的编译依赖（已 gitignore）。本机前端未构建时先执行 `npm run build` 或 `make check-frontend`，再跑后端验证门。

## 架构与代码导航

| 文件 | 职责 |
| --- | --- |
| `main.go` | Wails 入口：窗口配置（1200×800、Frameless）、资源嵌入、绑定 `core.App`（根目录仅此一个源文件） |
| `core/app.go` | 全部 Wails 绑定方法：配置 / 系统信息 / 模型 / 服务 / 下载 / 监控 / 更新（薄封装），以及 `Startup` / `Shutdown` 生命周期 |
| `core/engine.go` | 核心逻辑：环境检测、GGUF 扫描、llama.cpp 下载（断点续传）、模型下载任务队列、HF Mirror 搜索、配置持久化、模型预设生成 |
| `core/monitor.go` | 实时监控：服务日志 TPS 解析与 CPU / 内存 / GPU 采样 |
| `core/modelscope.go` | ModelScope（魔搭）模型源：搜索、文件列表、描述与下载 URL 构建 |
| `core/bridge.go` | 服务启停与下载触发的桥接实现 |
| `core/hidewindow_windows.go` / `core/hidewindow_other.go` | 子进程隐藏控制台窗口（Windows 实现 / 其他平台 no-op） |
| `core/*_test.go` | 后端单测（`runCmd` 跨平台、`hideWindow` 平台分支、config 持久化、GGUF 解析、模型扫描、预设生成、下载/HF/ModelScope 网络测试、监控、更新等） |
| `frontend/src/wails.ts` | 前端调用后端的唯一入口（`window.go.core.App.*` 桥接层） |
| `frontend/src/views/` | 页面：`Home`（系统状态）、`Downloads`（HF Mirror / ModelScope 搜索与下载队列）、`Models`（模型列表+设置弹窗）、`Api`（服务启停与实时监控，原 `Monitor` 页已并入）、`Settings`（主题 / 下载源 / 检查更新） |
| `frontend/src/components/` | `Sidebar`、`ModelSettings`（模型参数弹窗）、`UpdateModal`（更新下载弹窗）、`TaskDock`（全局右下角任务卡片：下载进度 + 内存模型卸载） |
| `frontend/src/__tests__/` | 前端单测（vitest，覆盖 `store.ts` 配置加载与 `lib/` 纯函数：格式化、下载队列 / 任务状态、监控采样、更新等） |
| `frontend/wailsjs/` | Wails 自动生成的绑定，**勿手改**，`wails build` 时自动重新生成 |

## Wails 绑定机制（重要）

- 后端方法在 `core/app.go` 中以 `func (a *App) Xxx(...)` 声明，根 `main.go` 将 `core.NewApp()` 加入 `Bind` 列表暴露给前端。`App` 生命周期方法为导出的 `Startup` / `Shutdown`，由 `main.go` 的 `OnStartup` / `OnShutdown` 引用。
- 前端统一在 `frontend/src/wails.ts` 中用 `window.go.core.App.Xxx(...)` 调用（方法返回 Promise）。**命名空间 `core` 来自 Go 包名 `core`**：若未来改包名或换绑定类型，`wails.ts` 的 `app()` 与 `wailsjs/go/` 下生成的绑定都要同步变化。新增 / 修改后端方法时，必须同步更新 `wails.ts` 中的封装，并保持命名一致。
- `window.go` 仅由 Wails 运行时注入。单独运行 `vite`（无 `wails dev`）时所有后端调用会抛错——这是预期行为，不是 bug。若需脱离 Wails 单独调试前端界面，可在页面加载前注入一个实现全部绑定方法的 `window.go` mock（仓库不含该 mock，需自行准备）。
- 后端返回结构体时，JSON 字段名以结构体 tag 为准（如 `DlTask` 的 `sizeHuman`）。修改返回结构后检查前端对应 interface 是否同步。

## 多角色协作与工作流

本文件面向所有 Agent 工具，但下列角色规则不是同时适用于每个工具。角色由用户在**任务开头的明确分工**决定：用户说「你是主审」，该会话承担主审职责；用户说「你是 agent 实施」（或交付实施任务包），承担实施 Agent 职责；用户说「你是 issues 发现者」，承担 Issues 发现者职责。实施 Agent 默认由 `general-purpose` 子代理承担。仅仅读取 `AGENTS.md`、使用某个厂商的模型，或完成实现和自测，都不会自动获得主审权限。

未声明角色时，按本文件其余章节的规则正常执行任务（含按「本地提交策略」直接提交）。除非用户在当前任务中明确改派角色，同一批次只能有一个主审。实施 Agent 不得自行升级为主审、批准自己的改动或把自报完成视为最终验收；主审也不得因自己能够修改代码而默认取代用户已选择的实施 Agent。

所有 Agent 都必须保护无关用户改动，遵守本文件的范围纪律、验证门、仓库卫生与远程/破坏性操作限制；不得通过删除失败测试、跳过验证门或扩大豁免获得绿色结果。

在不降低架构边界、行为正确性、测试覆盖与验证门的前提下，所有方案和实施都应优化端到端完成时间：优先消除可预见的返工，合并同一所有权范围内必需的生产改动与测试，并行执行互不冲突的只读检查；不得通过扩大豁免、降低断言、跳过测试或扩大未审查范围换取速度。

本文件只保留持久不变量与验证纪律；阶段状态、任务包、债务清单、决策与带日期证据放在 issue 评论或 `docs/` 权威文档中。任何 Agent 的完成报告都不是最终验收的事实来源。

### 主审职责（用户声明「你是主审」时适用）

- 主审的核心输入是**用户下达的命令与目标**，而不是 issues。用户下达任务后，主审负责把目标转化为可执行的方案，并在整个任务生命周期内**持久制定方案、持久验证**：方案随实施反馈持续更新，验证覆盖每一轮实施而非只在结尾。
- 完整闭环：理解用户命令与目标 → 制定方案 → 拆任务包交给实施 Agent → 审查实际 diff → 运行验证门 → 验收 → 按「本地提交策略」创建本地提交。
- 主审**不得自行实施**：实施一律由实施 Agent（默认 `general-purpose` 子代理）承担，避免自审自批；仅当用户在当前任务中明确要求主审直接实施时例外。
- 拉取与核对 issues 是主审的**可选输入**，不是默认起点：当用户要求、或任务涉及修复既有 issue 时，使用 `gh issue list` / `gh issue view <N>` 拉取远程 issues；对每个 issue 复现或定位到 `file:line` 证据，确认问题真实存在。不存在的在评论中说明结论，不规划修复。
- 制定方案：按用户目标、影响与风险排出优先级，规划修复方案；方案可作为一个整体任务包，也可拆成多个任务包，均交给实施 Agent 执行。每个任务包必须写明：目标、允许修改的路径、禁止触碰的路径、预期行为、必须新增或更新的测试、必跑验证命令、交付证据与明确退出条件。
- 审查顺序固定为先审查实际 diff、授权路径、关键行为与聚焦测试，再运行昂贵的完整验证门（`make check` / `scripts/check.ps1`，见「提交前测试分级」）。静态审查或聚焦复核发现阻塞问题时，先形成修复任务包，不提前运行完整门。
- 持久验证：每一轮实施返回后，主审都必须重新读取实际 diff，按规范符合性、实现质量、边界与行为、测试充分性逐项审查，并亲自运行适用验证命令，不得仅依据实施 Agent 的自报总结。发现阻塞问题、测试失败、验收证据缺失或债务增长时不得提交，必须形成带文件、行为和命令证据的修复任务包继续处理，直到本批次全部退出条件满足。
- 只有主审可以在验收通过后按「本地提交策略」创建本地提交；提交正文记录验证门实际结果。

### 实施 Agent 职责（默认由 `general-purpose` 子代理承担；用户声明「你是 agent 实施」时由该会话承担）

- 实施 Agent 负责按任务包（或用户指示）实现与验证：只修改授权范围内的路径和行为；实现预期行为与测试；运行指定验证命令；报告修改文件、关键设计决定、实际命令与完整结果、已知限制与未完成项。自报通过仅是待主审复核的交付证据。
- 授权范围内可确定性验证的机械修复、测试补齐、格式收口应直接完成，不因可自行解决的小问题暂停；只有需要扩大授权路径、改变行为或验证契约、降低验证门，或执行远程/破坏性操作时才停止并请求方向。
- 实施 Agent 不得执行 `git add`、commit、push、PR、merge 等操作；实现和自测完成后停止在未暂存工作树，等待主审审查或用户指示。发现的问题由主审形成修复任务包后在下一轮继续。

### Issues 发现者职责（用户声明「你是 issues 发现者」时适用）

- Issues 发现者负责查找项目中真实存在的问题，验证后按「Issue 跟踪」章节规范提交到远程仓库 issues。
- **验证流程（交互优先）**：优先使用 Playwright MCP 以真实用户交互行为发现功能缺陷——按核心流程逐项操作，例如：主页系统信息加载、模型扫描与参数设置、服务启停与 API 调用、下载任务管理、主题切换等。交互层面确认问题存在后，再回到代码定位根因（`file:line` 证据）。
- 仅当交互验证通过（核心功能无问题）时，才转向代码审计查找非交互类问题（样式、a11y、i18n、安全、死代码、文档缺口等）。
- 用户指定该角色即视为授权创建远程 issue，但提交前必须核对仓库现有 issues 避免重复，且遵循「Issue 跟踪」章节全部规范：表单模板、严重度前缀与 label、`file:line` 证据、验收标准、正文脱敏。批量发现先创建 Tracker 总览 issue，P0/P1 单独建子 issue，P2/P3 合并汇总。
- 敏感安全漏洞（凭据泄漏、注入、越权等）不开公开 issue，通过 GitHub Security Advisories 私密上报。
- 该授权仅覆盖创建/维护 issue；推送、开 PR、merge 等其他远程操作仍需用户明确授权。
- 创建/更新远程 issue 同样适用「Git 工作流」的重试规则：网络类失败间隔一段时间重试，多次仍失败或属认证/权限类错误时报告暂停。

## Git 工作流

- 默认在本地 `main` 分支上直接提交，非必要不新建分支。仅当用户明确要求、或改动需要隔离评审（如准备发 PR 给外部协作者）时才创建特性分支，并在任务前说明理由。
- 角色化协作时（见「多角色协作与工作流」），本地提交仅由承担主审职责的会话执行；被声明为实施 Agent 的会话停止在未暂存工作树，不提交。
- 不要自作主张切换或创建分支；提交前若对分支选择有疑问，先与用户确认。
- 推送、合并到远程、开 PR 等远程操作必须经用户明确授权后才执行。
- 特性分支是临时性的，PR 合并后必须立即删除（远程 `git push origin --delete <branch>` 与本地 `git branch -D <branch>` 都要删）；合并时可尝试 `gh pr merge --delete-branch`，若未生效需手动补删，不留已合并的残留分支。
- 远程操作（推送、`gh issue create` 等）可能因网络不稳定而失败：失败后不要立即判定为失败，应等待一段时间（如 30~60 秒）后重试，不同时间多试几次通常可以成功；仅当确认是认证、权限、冲突等非网络错误，或连续多次仍失败时，才报告并暂停等待用户指示。

## 代码规范

### Go 后端

- 所有 Go 代码通过 `gofmt`、`go build ./...` 与 `golangci-lint run`（govet / ineffassign / unused 零诊断）；错误不得吞掉，用 `fmt.Errorf("...: %w", err)` 包装上下文。
- 并发共享状态使用显式互斥（项目惯例：`configMu` / `serverLogsMu` / `downloadMu` / `dlTasksMu` 等），不得新增无锁共享变量。
- 下载类状态机（`downloadState.Status`）取值：`idle / fetching / downloading / paused / extracting / done / error`，新增状态需同步前端 `statusLabel` 映射。
- 日志统一 `log.Println` 并加前缀（`[INFO]` / `[WARN]` / `[ERROR]` / `[OK]`），服务日志走 `serverLogs` 环形缓冲。
- 启动外部子进程必须调用 `hideWindow(cmd)`，避免 GUI 应用弹出控制台窗口（见 `hidewindow_windows.go`）。
- 新行为必须带聚焦测试：测试使用标准库 `testing`，放同包 `*_test.go`，中文注释说明被测行为与断言依据。

### Vue / TypeScript 前端

- `npm run build`（vue-tsc strict）必须零错误；不留未使用变量、导入与死代码。
- 样式统一使用 `var(--cn-*)` 之外本项目自定义的语义 token（`--bg-primary` / `--surface` / `--text-primary` / `--border` 等），深浅主题通过 `html[data-theme]` 切换；新增颜色必须同时提供深 / 浅两套取值。
- 组件 props / emits 使用 `defineProps<T>` / `defineEmits` 类型化；不出现 `any`（测试与 mock 除外）。
- 页面新增必须在 `router/index.ts` 注册路由并添加侧边栏导航项（`Sidebar.vue` 的 `navItems`）。
- 用户可见文案使用中文，与现有页面风格保持一致。
- 行为改动必须新增或更新 vitest 聚焦测试（`src/__tests__/`）；只 mock 有副作用的依赖链（Wails 桥接、网络、时间），不 mock 纯函数或纯数据模块。

### 通用

- 修复 bug 时改动仅限故障点及其配套文件，不混入无关重构。
- 改动跨前后端（如新增绑定方法）时，`app.go`、`wails.ts` 与前端调用方必须一次提交到位，避免中间态。
- 涉及下载状态机、服务启停、配置文件结构（`llama-desktop-config.json`）的改动，需同时检查旧数据兼容（`loadConfig` 的默认值兜底逻辑）。
- 新行为必须带聚焦测试；修改或删除既有测试需说明原因，不得为通过验证门而删除断言。

## 提交前测试分级

验证强度以行为风险和所有权边界为主，文件数量只作为兜底信号，不能单独证明风险高低。

- **免测试提交**：仅修改文档，或仅修改不影响编译、lint 与运行时行为的注释。跳过测试套件，执行 `git diff --check` 及适用的文档检查。
- **局部提交**：同一模块或同一所有权范围内通常不超过 3 个非文档文件，且不命中高风险条件。运行受影响一侧的验证门（后端：`go build` + `go test` + `gofmt -l` + `golangci-lint run`；前端：`npm run build` + `npm test`）与受影响的聚焦测试。
- **中等提交**：4-5 个非文档文件但仍局限于同一模块、行为单一且有明确聚焦覆盖时，可继续采用局部验证；必须在提交正文说明范围与未跑全量的依据。
- **全量提交**：跨模块或跨所有权联动、高风险行为变更，或者修改 6 个及以上非文档文件且不能证明只是局部机械变更。运行完整 `make check` / `scripts/check.ps1`。
- 无论文件数量多少，以下情况都属于全量提交：
  - 改动配置持久化结构、共享状态、服务启停逻辑、下载状态机；
  - 新增后端绑定方法、API 端点或对外协议/响应结构；
  - 改动共享测试基础设施、CI 配置或验证门脚本本身。
- 文件数量统计排除纯文档、纯注释和由同一命令机械生成且已做漂移验证的产物；新增测试、fixture、快照和配置属于非文档文件。
- 要求执行的验证只要有一项不通过，就绝对不能提交；必须修复后重新运行相应验证并确认通过。
- **当前测试基线**：后端已有覆盖 `core` 包的聚焦测试（config 持久化、GGUF 解析、模型扫描、预设生成、下载/HF/ModelScope 网络测试、系统解析、服务命令构建等）；前端有 `store.ts` 与 `lib/` 纯函数的 vitest。新增行为必须补聚焦测试，不得以「没有测试框架」为由跳过。

## 本地提交策略

使用 `type(scope): 中文主题` 的 Conventional Commit 格式，主题保持一行；既有类型：`feat` / `fix` / `docs` / `chore` / `refactor` / `test` / `perf` / `security`，scope 如 `backend` / `frontend` / `build` / `models` / `server` / `downloads` / `config` 等，仅在有意义时使用。

详细正文必须包含以下结构：

```text
Summary:
- <主要变化，按域分段：Backend / Frontend / Tests / Docs 等>

Verification:
- <实际执行的验证命令与通过结果，全量提交必须记录完整验证门结果>

Remaining gaps:
- <明确说明未包含的后续工作；没有则写 None>
```

- 按明确路径逐个暂存文件。提交前检查 `git status --short`、`git diff --cached --stat` 与 `git diff --cached --check`；提交后工作树必须干净。
- 外部实施 Agent 不得暂存或提交；只有主审在重新审查实际 diff、确认任务包退出条件全部满足并亲自完成所需验证后可以提交。审查有问题时先进入下一轮修复，不提交。

## 版本发布

- 更新日志的权威来源是根目录 `CHANGELOG.md`：发版前先新增对应版本条目（含日期与逐提交核心改动），**tag 注解消息与 GitHub Release 正文均从该条目复制**，保持一致；不依赖 GitHub「自动生成发布说明」（本仓库直接提交 main、无 PR，自动生成只产生 compare 链接）。
- 版本标签为注解标签 `git tag -a vX.Y.Z`，消息取自 CHANGELOG 条目；发版前先确认验证门通过，推送 tag 属远程操作需用户授权。
- GitHub Release 正文不会自动同步 tag 消息：创建/更新 Release 时从 CHANGELOG 条目粘贴正文，该操作属远程操作需用户明确授权。

## 仓库卫生

- 提交前 `git status --short` 只包含本次任务有意改动的文件；`git diff --check` 无错误。
- 不得提交：`node_modules/`、`frontend/dist/`、`build/`（含编译产物 exe）、`LLM-Models/` 下的模型文件、`llama-cpp/`、`llama-desktop-config.json`（本地配置，可能含本机路径）、`*.log`、`.zcode/plans/`。
- 截图等文档资源提交到 `docs/` 目录（如 `docs/screenshots/`）。
- 新增忽略类型时同步更新 `.gitignore`。

## Issue 跟踪

Issue 位于 `https://github.com/CodeNeow/llama-cpp-desktop/issues`。任何非平凡的缺陷或计划内工作都建议创建 issue，使进度可见；保持列表高信噪比。本章节是「多角色协作与工作流」中 Issues 发现者角色提交远程 issue 时的执行规范，也适用于任何需要创建 issue 的场合。

### 创建

- 网页创建的 issue 使用 `.github/ISSUE_TEMPLATE/` 下的表单模板：日常 bug 用 `bug-report.yml`，审计或代码评审的结构化发现用 `audit-finding.yml`（含 `file:line` 证据与验收标准）。
- 批量发现（如完整审计）：创建一个置顶的 Tracker 总览 issue，在优先级表格中链接每个子 issue；为每个 P0/P1 发现单独建子 issue；P2/P3 可合并为汇总 issue。
- 敏感安全漏洞（凭据泄漏、注入、越权等）不要开公开 issue。通过 GitHub Security Advisories（`Security > Report a vulnerability`）私密上报。修复落地后可再开非敏感跟踪 issue。
- issue 正文绝不能包含密钥与本机绝对路径（`llama-desktop-config.json` 中可能含本机路径）。提交前对 token、密钥、DSN、路径脱敏，即便在日志或截图中也是如此。

### 标题与 Label

- 发现类标题加严重度前缀：`[P0]` / `[P1]` / `[P2]` / `[P3]`，其他用 `[Bug]` / `[Tracker]`。标题保持一行并指明受影响组件或区域，如 `[P1] 启动时子进程控制台窗口频繁闪现`。
- 恰好打一个优先级 label（`P0-critical` / `P1-high` / `P2-medium` / `P3-low`）与至少一个区域 label（`frontend` / `backend` / `models` / `downloads` / `server` / `config` / `security`）。标签由 `scripts/create-labels.ps1` 一键创建。不得自创新 label，先提出申请。
- 严重度反映影响而非工作量：P0 阻断核心功能或为安全漏洞；P1 实质损害体验或安全；P2 为一致性或打磨缺口；P3 为吹毛求疵的小问题。

### 必填内容（发现类）

一个发现 issue 必须包含：现象、复现步骤、带 `file:line` 的证据、影响、作为 `- [ ]` 勾选清单的验收标准。修复建议（含 diff）鼓励但非必需。若该发现属于某批次，在正文或评论中引用 Tracker issue 编号。

### 生命周期

- PR 描述中用 `Fixes #N` 关联修复，合并时自动关闭对应 issue。
- 验收标准未可验证地满足、且相关验证门未通过前，不得关闭 issue；验证门结果记录在 PR 中，而非仅记在 issue 中。
- issue 被取代或重复时，评论指向规范 issue 后关闭；不要删除。

## 常见坑

- **`wails dev` 报端口占用**：Vite 固定 `localhost:5173`（见 `wails.json` 的 `frontend:dev:serverUrl`），先结束占用进程再启动。
- **改了 Go 结构体但前端拿不到新字段**：检查 `wails.ts` 封装与页面 interface 是否同步；`wailsjs/` 由构建自动更新，不需要手动改。
- **API 页"启动服务"失败**：`startServerInternal` 会先扫描 `LLM-Models` 并生成预设，目录为空会直接报错（`LLM-Models 目录中没有模型`）——这是预期行为，先在「模型」页确认有 GGUF 文件。
- **Windows 下停止服务**：`stopServerInternal` 通过 `cmd.Process.Signal(os.Kill)` 结束 llama-server；不要在外部用 `taskkill /IM llama-server.exe` 宽范围强杀，避免误杀其他实例。
- **本机 `gofmt -l` 报大量既有文件**：仓库提交为 LF，Windows 检出（`core.autocrlf`）为 CRLF 会导致 gofmt 报告整个文件。判断格式问题时用 `git -c core.autocrlf=false diff` 核对实际改动，不要顺手改写未改动的行尾。
- **本机前端未构建时报 `frontend/dist: no matching files found`**：`go:embed` 依赖 `frontend/dist`，先执行 `cd frontend && npm run build` 再跑后端命令。
- **GitHub Release 页面只有 `Full Changelog` 对比链接**：自动生成发布说明按 PR 汇总，本仓库直接提交 main 无 PR，生成结果只剩 compare 链接；tag 消息不会自动同步到 Release 正文，需从 `CHANGELOG.md` 对应条目粘贴（见「版本发布」）。
