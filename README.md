<div align="center">

<img src="docs/branding/icon.png" width="88" alt="Llama Desktop" />

# Llama Desktop

**本地大模型推理桌面 —— [llama.cpp](https://github.com/ggml-org/llama.cpp) 的友好图形客户端**，可视化调优 GGUF 模型，多模型共享一个 OpenAI 兼容端点，内置模型下载、本地聊天与实时监控。

基于 Wails v2（Go 后端 + Vue 3 前端） · Windows x64 · GPL-3.0

[![Platform](https://img.shields.io/badge/platform-Windows%20x64-0078D6?logo=windows&logoColor=white)](https://github.com/CodeNeow/llama-cpp-desktop/releases)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/CodeNeow/llama-cpp-desktop?logo=github&color=blue)](https://github.com/CodeNeow/llama-cpp-desktop/releases)
[![Downloads](https://img.shields.io/github/downloads/CodeNeow/llama-cpp-desktop/total?logo=github&label=downloads&color=blue)](https://github.com/CodeNeow/llama-cpp-desktop/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/CodeNeow/llama-cpp-desktop/.github/workflows/ci.yml?branch=main&logo=githubactions&logoColor=white)](https://github.com/CodeNeow/llama-cpp-desktop/actions)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Wails](https://img.shields.io/badge/Wails-v2-DF0000?logo=wails&logoColor=white)](https://wails.io/)

</div>

[English](README_en.md)

![Llama Desktop 界面预览](docs/screenshots/zh/chat.png)

## ✨ 功能亮点

- **一个端点，多个模型** — 以路由器模式运行 llama-server（`--models-dir` / `--models-preset` / `--models-max`），把模型目录下的所有 GGUF 汇聚到一个 OpenAI 兼容 API（默认 `http://127.0.0.1:8080/v1`）。
- **按需加载与一键卸载** — 模型仅在首次被请求时加载进显存 / 内存，无需手动预载；已加载模型在任务卡片中一目了然，空闲即点即卸载释放资源，多模型轮换不用重启服务。
- **无头 API 路由模式** — 一个开关即切换为纯后台运行（Go 后端 + 系统托盘 + llama-server，界面关闭）：GUI 模式下占用数百 MB 内存的 WebView2 界面进程整体退出，仅保留约 20 MB 的后台进程；推理全程不中断，OpenAI API 持续可用，托盘菜单「显示主窗口」随时回到完整界面。
- **模型 ID 所见即所得** — API 的 `model` 字段就是页面上显示的模型名（如 `Qwen3.6-29B-REAP-Opus-Reasoning-Distill-MTP-Q4_K_M`），从模型管理页、API 路由页或聊天页复制即可直接使用。
- **硬件感知一键调优** — 读取 GGUF 真实指标（层数、GQA/MLA KV 几何、训练上下文、MoE 专家占比），结合 GPU/CPU/内存快照为每个模型自动规划 GPU 层数、上下文长度、线程与缓存类型。
- **CUDA 兼容性提示** — 系统信息页自动比对显卡算力与已安装的 CUDA 运行时并给出兼容性判定，Blackwell 显卡会明确提示需要 CUDA 12.8+，避免装错运行时。
- **内置本地聊天** — 直连本地端点的流式对话，Markdown 渲染、思考过程实时展示，多模态模型支持发送图片，会话级采样参数可调（temperature、top-p / top-k、重复惩罚、最大 token 数、系统提示词）。
- **模型搜索与下载** — 支持 HF Mirror（hf-mirror.com）与 ModelScope 双源搜索，仓库可展开为文件列表，批量下载走可断点续传的任务队列（暂停 / 继续 / 取消），队列重启后自动恢复。
- **逐模型推理预设** — GPU 层数、KV 缓存类型、长上下文 RoPE、投机解码等参数按模型独立保存，保存后自动写入 llama-server 预设。
- **实时服务监控** — 服务日志控制台与提示词处理 / 生成 Token 速度双指标，每秒刷新，全部固定在视口内无需滚动。
- **灵动任务卡片** — 右下角悬浮的可收起卡片，一眼看清下载进度（llama.cpp / 模型文件 / 应用更新），同时列出当前内存中已加载的模型，每个模型带一键卸载按钮。
- **桌面体验** — Windows 系统托盘、应用内检查更新、深色 / 浅色主题、界面语言支持 zh / en / auto。
- **内置中英教程** — 帮助页内置中英双语教程，内容支持在线更新——无需升级应用即可获取最新文档。

## 📸 界面预览

| 系统信息 | 运行环境 |
| :---: | :---: |
| ![系统信息](docs/screenshots/zh/home.png) | ![运行环境](docs/screenshots/zh/runtime.png) |
| ![本地聊天](docs/screenshots/zh/chat.png) | ![模型管理](docs/screenshots/zh/models.png) |
| ![模型下载](docs/screenshots/zh/downloads.png) | ![API 路由](docs/screenshots/zh/api.png) |
| ![模型设置](docs/screenshots/zh/model-settings.png) | ![帮助文档](docs/screenshots/zh/docs.png) |

<div align="center">

右下角的灵动任务卡片：下载进度与已加载模型一键卸载。

![灵动任务卡片](docs/screenshots/zh/task-dock.png)

</div>

## 🚀 快速开始

### 方式一：下载安装版（推荐）

前往 [Releases 最新版](https://github.com/CodeNeow/llama-cpp-desktop/releases/latest) 下载 `llama-desktop-setup-*.exe` 安装包，双击安装即可；应用内置自动更新，后续新版本无需手动重装。

环境要求：Windows 10 及以上（仅 x64）；WebView2 随应用自动安装。

### 方式二：从源码构建

- [Git](https://git-scm.com/)、[Go](https://go.dev/dl/) 1.25+、[Node.js](https://nodejs.org/) 18+
- Wails CLI v2.14+：

  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

克隆仓库并进入开发模式：

```bash
git clone https://github.com/CodeNeow/llama-cpp-desktop.git
cd llama-cpp-desktop
wails dev
```

`wails dev` 会同时启动 Go 后端与 Vite 前端开发服务器（`http://localhost:5173`），前后端均支持热重载。

**首次使用（两种方式通用）：**

1. 在**运行环境**页点击「下载 llama.cpp」从 GitHub 获取最新版（支持断点续传），也可指定已有的 llama.cpp 目录。
2. 在**模型下载**页搜索 HF Mirror 或 ModelScope，将 GGUF 文件下载到模型目录（默认 `LLM-Models/`）；进度显示在右下角任务卡片中。
3. 在**API 路由**页点击「启动服务」（默认 `127.0.0.1:8080`）。
4. 打开**本地聊天**页选择模型即可对话，也可以让任意 OpenAI 兼容客户端接入该端点。

## 🧭 使用指南

- **系统信息** — 自动检测 CPU、内存、GPU 与 CUDA 环境，实时采样刷新，并给出 Blackwell 显卡与 CUDA 版本的兼容性提示。
- **运行环境** — 展示 llama.cpp 安装状态（主程序与 CUDA 运行时组件）；支持一键断点续传下载或自定义目录。
- **本地聊天** — 流式聊天，支持 Markdown 渲染与图片附件；需要 API 路由服务处于运行状态。
- **模型下载** — 双源搜索（HF Mirror / ModelScope，可在偏好设置中切换默认源），文件级选择，持久化的可断点续传下载队列。
- **模型管理** — 扫描模型目录下的 GGUF 文件（解析架构、量化等级，识别多模态 / 嵌入模型），支持硬件感知一键调优；每个模型可进入设置页（基础 / 推理 / 内存 / 多 GPU / 长上下文 / 高级六个标签页）。
- **API 路由** — 启动 / 停止 / 重启 llama-server，查看服务日志与 Token 速度双指标，配置 Port / 最大并发模型数 / Prompt 缓存，并查看当前已加载的模型；访问范围与推理显卡等服务器配置在偏好设置中调整。
- **偏好设置** — 主题、界面语言（zh / en / auto）、下载源、下载与导入目录、服务访问范围与推理显卡等服务器配置、Windows 托盘开关、API 路由模式、检查更新。
- **帮助** — 内置中英双语教程，内容自动在线更新。

服务启动后，任意 OpenAI 兼容客户端均可接入：

```bash
OPENAI_BASE_URL="http://127.0.0.1:8080/v1"
OPENAI_API_KEY="sk-任意占位值"   # 本地服务默认不鉴权；可在偏好设置中配置 API Key
```

`model` 字段直接填页面上显示的模型名（如 `Qwen3.6-29B-REAP-Opus-Reasoning-Distill-MTP-Q4_K_M`），API 路由页的模型标签可直接复制；llama-server 会按需加载 / 卸载模型，内存中的模型也可以在任务卡片中手动卸载。

## ⚙️ 配置

运行时配置持久化在项目根目录的 `llama-desktop-config.json`，主要字段：

| 字段 | 含义 | 默认值 |
| --- | --- | --- |
| `theme` | 主题：`light` / `dark` | `light` |
| `language` | 界面语言：`zh` / `en` / `auto`（auto 跟随系统语言） | `auto` |
| `downloadSource` | 默认下载源：`hf` / `modelscope` | `hf` |
| `trayEnabled` | Windows 系统托盘（显示主窗口 / 退出菜单） | `true` |
| `sidebarCollapsed` | 侧边栏是否默认收起 | `true` |
| `apiRouteMode` | API 路由（无头）模式：下次启动后仅以托盘 + llama-server 后台运行，不显示界面 | `false` |
| `serverConfig` | `accessMode`（`local` / `lan`）、`host`、`port`、`maxModels`、`cacheRam`（MiB） | `127.0.0.1:8080`，`maxModels` 1，`cacheRam` 8192 |

此外还保存：`llamaCppDownloadDir` / `modelDownloadDir`（下载路径）与 `llamaCppDir` / `modelDir`（外部导入目录）、`modelConfigs`（逐模型推理参数）、`downloadTasks`（下载任务队列，重启后恢复）。

## 🏗️ 架构

```mermaid
flowchart LR
    A["GGUF 模型目录<br/>(下载 + 外部导入)"] --> B["扫描与解析<br/>架构 · 量化 · 多模态"]
    B --> C["逐模型推理预设 (INI)"]
    C --> D["llama-server 路由模式"]
    D --> E["OpenAI 兼容端点<br/>127.0.0.1:8080/v1"]
    E --> F["内置本地聊天"]
    E --> G["任意 OpenAI 客户端"]
    H["硬件感知一键调优"] -.-> C
```

前端是 Vue 3 单页应用，经 Wails 桥接调用 Go 后端；后端负责扫描模型目录、解析 GGUF 元数据、生成逐模型推理预设并拉起 llama-server。llama-server 以路由模式运行，把目录下所有 GGUF 汇聚成一个 OpenAI 兼容端点，模型按需加载 / 卸载——内置聊天与任意 OpenAI 客户端都接在这同一个端点上。

## 🛠️ 开发

组合质量门禁：

```bash
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1     # Windows
make check                                                                  # POSIX
```

门禁会运行后端的 `go build` / `go test` / `gofmt` / `golangci-lint` 与前端的 `npm run build`（vue-tsc + vite）；PowerShell 脚本还会运行 vitest 测试套件（`npm test`）。后端测试位于 `core/*_test.go`（标准库 `testing`），前端测试位于 `frontend/src/__tests__/`。开发约定、提交格式与协作流程详见 [AGENTS.md](AGENTS.md) 与 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 📦 构建

```bash
wails build
```

生产构建产物输出到 `build/bin/llama-desktop.exe`。前端通过 `go:embed` 打进二进制，`wails build` 会自动完成前端编译。

## ❓ 常见问题

**`wails dev` 提示端口被占用。**
Vite 开发服务器绑定 `localhost:5173`（见 `wails.json` 的 `frontend:dev:serverUrl`）。请结束占用该端口的进程后重试。

**`wails dev` 启动后窗口一闪而过，提示已在运行。**
应用使用单实例互斥锁。请先关闭正在运行的 Llama Desktop（包括安装版和托盘里的后台实例），再重新启动开发模式。

**API 路由页「启动」失败，提示找不到模型。**
启动流程会先扫描模型目录并生成预设，目录为空时会报错。请先将 GGUF 文件放入 `LLM-Models/`（可在模型管理页确认），然后重试；同时请在运行环境页确认 llama.cpp 已安装。

**调用 API 报 `model not found`。**
`model` 字段必须与页面显示的模型名大小写完全一致（服务按精确匹配）。请从 API 路由页的模型标签或模型管理页复制粘贴，不要手打。

**单独运行 `npm run dev` 时所有后端调用都报错。**
`window.go` 由 Wails 运行时注入，脱离 `wails dev` 的 Vite 没有通往 Go 后端的桥接层，这是预期行为。调试 UI 请使用 `wails dev`。

**下载 llama.cpp 很慢或失败。**
下载源为 GitHub Releases，支持暂停 / 继续与断点续传。网络受限时可手动下载 Windows 发行包并解压，然后在运行环境页通过「自定义」选择该目录。

## 📄 协议

Copyright © 2026 [CodeNeow](https://github.com/CodeNeow/llama-cpp-desktop)

本项目基于 [GNU General Public License v3](LICENSE) 开源。
