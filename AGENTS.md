# Llama GUI 开发规范

## 项目概览

Llama GUI 是一个本地大模型桌面管理工具：基于 **Wails v2**（Go 1.22 后端 + WebView2 前端），前端为 **Vue 3 + TypeScript + Vite 5**（无第三方 UI 库，手写 CSS 变量主题），推理引擎为 **llama.cpp**（llama-server 路由器模式）。

核心链路：`LLM-Models/` 目录扫描 GGUF → 逐模型推理参数配置 → 生成 llama-server 模型预设（INI）→ 启动 OpenAI 兼容服务（默认 `127.0.0.1:8080`）。

## 常用命令

在仓库根目录执行：

```bash
wails dev                 # 开发模式：Go 后端 + Vite 前端（:5173）热重载
wails build               # 生产构建，产物 build/bin/llama-gui.exe（build/ 已 gitignore）
cd frontend && npm run build   # 仅构建前端（vue-tsc 类型检查 + vite build）
cd frontend && npm run dev     # 仅起 Vite（无 Wails 运行时，后端调用会失败，见下文）
```

验证门（改动后按需执行）：

```bash
go build ./...                                   # 后端编译
cd frontend && npm run build                     # 前端类型检查 + 构建（vue-tsc --noEmit 零错误）
gofmt -l .                                       # Go 格式化检查，必须无输出
```

## 架构与代码导航

| 文件 | 职责 |
| --- | --- |
| `main.go` | Wails 入口：窗口配置（1200×800、Frameless）、资源嵌入、绑定 App |
| `app.go` | 全部 Wails 绑定方法：配置 / 系统信息 / 模型 / 服务 / 下载（薄封装） |
| `engine.go` | 核心逻辑：环境检测、GGUF 扫描、llama.cpp 下载（断点续传）、HF Mirror 搜索、配置持久化、模型预设生成 |
| `bridge.go` | 服务启停与下载触发的桥接实现 |
| `frontend/src/wails.ts` | 前端调用后端的唯一入口（`window.go.main.App.*` 桥接层） |
| `frontend/src/views/` | 页面：`Home`（系统状态）、`Models`（模型列表+设置弹窗）、`Api`（服务启停）、`Downloads`（HF Mirror）、`Settings`（主题） |
| `frontend/src/components/` | `Sidebar`、`ModelSettings`（模型参数弹窗） |
| `frontend/wailsjs/` | Wails 自动生成的绑定，**勿手改**，构建时自动重新生成 |

## Wails 绑定机制（重要）

- 后端方法通过 `app.go` 中以 `func (a *App) Xxx(...)` 声明并加入 `main.go` 的 `Bind` 列表暴露给前端。
- 前端统一在 `frontend/src/wails.ts` 中用 `window.go.main.App.Xxx(...)` 调用（方法返回 Promise）。**新增 / 修改后端方法时，必须同步更新 `wails.ts` 中的封装**，并保持命名一致。
- `window.go` 仅由 Wails 运行时注入。单独运行 `vite`（无 `wails dev`）时所有后端调用会抛错——这是预期行为，不是 bug。若需脱离 Wails 单独调试前端界面，可在页面加载前注入一个实现全部绑定方法的 `window.go` mock（仓库不含该 mock，需自行准备）。
- 后端返回结构体时，JSON 字段名以结构体 tag 为准（如 `DlTask` 的 `sizeHuman`）。修改返回结构后检查前端对应 interface 是否同步。

## 代码规范

### Go 后端

- 所有 Go 代码通过 `gofmt` 与 `go build ./...`；错误不得吞掉，用 `fmt.Errorf("...: %w", err)` 包装上下文。
- 并发共享状态使用显式互斥（项目惯例：`configMu` / `serverLogsMu` / `downloadMu` / `dlTasksMu` 等），不得新增无锁共享变量。
- 下载类状态机（`downloadState.Status`）取值：`idle / fetching / downloading / paused / extracting / done / error`，新增状态需同步前端 `statusLabel` 映射。
- 日志统一 `log.Println` 并加前缀（`[INFO]` / `[WARN]` / `[ERROR]` / `[OK]`），服务日志走 `serverLogs` 环形缓冲。

### Vue / TypeScript 前端

- `npm run build`（vue-tsc strict）必须零错误；不留未使用变量、导入与死代码。
- 样式统一使用 `var(--cn-*)` 之外本项目自定义的语义 token（`--bg-primary` / `--surface` / `--text-primary` / `--border` 等），深浅主题通过 `html[data-theme]` 切换；新增颜色必须同时提供深 / 浅两套取值。
- 组件 props / emits 使用 `defineProps<T>` / `defineEmits` 类型化；不出现 `any`（测试与 mock 除外）。
- 页面新增必须在 `router/index.ts` 注册路由并添加侧边栏导航项（`Sidebar.vue` 的 `navItems`）。
- 用户可见文案使用中文，与现有页面风格保持一致。

## 变更纪律

- 修复 bug 时改动仅限故障点及其配套文件，不混入无关重构。
- 改动跨前后端（如新增绑定方法）时，`app.go`、`wails.ts` 与前端调用方必须一次提交到位，避免中间态。
- 涉及下载状态机、服务启停、配置文件结构（`llama-gui-config.json`）的改动，需同时检查旧数据兼容（`loadConfig` 的默认值兜底逻辑）。

## 仓库卫生

- 提交前 `git status --short` 只包含本次任务有意改动的文件；`git diff --check` 无错误。
- 不得提交：`node_modules/`、`frontend/dist/`、`build/`（含编译产物 exe）、`LLM-Models/` 下的模型文件、`llama-cpp/`、`llama-gui-config.json`（本地配置，可能含本机路径）、`*.log`。
- 截图等文档资源提交到 `docs/` 目录（如 `docs/screenshots/`）。
- 新增忽略类型时同步更新 `.gitignore`。

## 常见坑

- **`wails dev` 报端口占用**：Vite 固定 `localhost:5173`（见 `wails.json` 的 `frontend:dev:serverUrl`），先结束占用进程再启动。
- **改了 Go 结构体但前端拿不到新字段**：检查 `wails.ts` 封装与页面 interface 是否同步；`wailsjs/` 由构建自动更新，不需要手动改。
- **API 页"启动服务"失败**：`startServerInternal` 会先扫描 `LLM-Models` 并生成预设，目录为空会直接报错（`LLM-Models 目录中没有模型`）——这是预期行为，先在「模型」页确认有 GGUF 文件。
- **Windows 下停止服务**：`stopServerInternal` 通过 `cmd.Process.Signal(os.Kill)` 结束 llama-server；不要在外部用 `taskkill /IM llama-server.exe` 宽范围强杀，避免误杀其他实例。
