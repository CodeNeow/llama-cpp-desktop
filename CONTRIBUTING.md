# 贡献指南

感谢你参与 Llama GUI 的开发！本指南覆盖环境准备、提交规范、验证门与 Issue / PR 流程。完整的开发规范见 [AGENTS.md](./AGENTS.md)。

## 1. 环境准备

| 依赖 | 版本 | 说明 |
| --- | --- | --- |
| [Go](https://go.dev/dl/) | 1.22+ | 后端（`go.mod` 声明） |
| [Node.js](https://nodejs.org/) | 18+ | 前端构建（CI 使用 24） |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| [golangci-lint](https://golangci-lint.run/) | v2 | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.9.0` |

本地开发：

```bash
wails dev          # Go 后端 + Vite 前端（:5173）热重载
```

> 前端通过 `window.go.core.App` 调用后端，单独运行 `npm run dev`（无 Wails 运行时）时后端调用会抛错，这是预期行为。

## 2. 分支与提交

### 2.1 分支

- 默认在 `main` 分支上直接提交；仅当改动需要隔离评审（如发 PR）时才新建特性分支。

### 2.2 Commit Message

使用 `type(scope): 中文主题` 格式，主题保持一行。既有类型：`feat` / `fix` / `docs` / `chore` / `refactor` / `test` / `perf` / `security`，scope 如 `backend` / `frontend` / `build` 等。

```text
feat(backend): 支持模型自定义 llama.cpp 目录
fix(backend): 修复下载任务暂停后无法恢复的问题
docs: 更新 API 页使用说明
```

详细正文必须包含以下结构：

```text
Summary:
- <主要变化，按域分段：Backend / Frontend / Tests / Docs 等>

Verification:
- <实际执行的验证命令与通过结果>

Remaining gaps:
- <明确说明未包含的后续工作；没有则写 None>
```

- 跨前后端的改动（如新增绑定方法）必须一次提交到位，避免中间态。
- 提交前 `git status --short` 只包含本次任务有意改动的文件；`git diff --check` 无错误。

### 2.3 多工具协作角色

使用多 Agent / 多会话协作时，角色由用户在任务开头声明：**主审**（制定方案、审查 diff、验收后提交）、**实施 Agent**（按任务包实现与自测，停在未暂存工作树）、**Issues 发现者**（交互验证后按规范创建远程 issue）。详细职责见 [AGENTS.md](./AGENTS.md)「多角色协作与工作流」。

## 3. 验证门（提交前必过）

| 范围 | 命令 | 要求 |
| --- | --- | --- |
| 后端 | `go build ./...` | 编译通过 |
| 后端 | `go test ./...` | 单测通过 |
| 后端 | `gofmt -l .` | 无输出 |
| 后端 | `golangci-lint run` | 零诊断（govet / ineffassign / unused） |
| 前端 | `cd frontend && npm run build` | vue-tsc 类型检查 + vite 构建零错误 |
| 前端 | `cd frontend && npm test` | vitest 单测通过 |

组合验证门：POSIX 用 `make check`；Windows 用 `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1`。按改动范围选择局部 / 全量验证，分级规则见 [AGENTS.md](./AGENTS.md)「提交前测试分级」。

> CI（`.github/workflows/ci.yml`）会在每次 push / PR 自动执行上述检查，并在 Windows 上完成 `wails build` 桌面打包验证。本地通过不能替代 CI 的最终判定。

## 4. Issue 流程

### 4.1 何时开 Issue

非平凡的缺陷或计划内工作都建议创建 issue 使进度可见；使用问题、设计讨论请优先走 Discussions。

### 4.2 如何开

- 日常 bug 用「Bug 报告」模板（`.github/ISSUE_TEMPLATE/bug-report.yml`）。
- 审计 / 代码评审的结构化发现用「审计发现」模板（`audit-finding.yml`），必须含 `文件:行号` 证据与验收标准。
- 批量发现（如完整审计）：先建 Tracker 总览 issue，P0/P1 单独建子 issue，P2/P3 可合并汇总。

### 4.3 优先级与 Label

标题加严重度前缀：`[P0]` / `[P1]` / `[P2]` / `[P3]` / `[Bug]` / `[Tracker]`。优先级反映**影响**而非工作量：

- `P0`：阻断核心功能或安全漏洞
- `P1`：实质损害体验或安全
- `P2`：一致性 / 打磨缺口
- `P3`：吹毛求疵的小问题

标签体系（含 P0–P3 与区域标签）由 `scripts/create-labels.ps1` 一键创建：

```powershell
gh auth login   # 首次需登录
.\scripts\create-labels.ps1
```

### 4.4 敏感安全漏洞

涉及凭据、注入、越权等漏洞**不要开公开 issue**，通过 GitHub Security Advisory 私密上报。

### 4.5 Issue 内容脱敏

issue 正文绝不能包含 token、密钥、本机绝对路径（如 `llama-gui-config.json` 中的路径），提交前先脱敏。

## 5. Pull Request 流程

1. 从 `main` 切出特性分支，提交规范见第 2 节。
2. 确保本地验证门（第 3 节）全部通过。
3. 创建 PR：`Fixes #N` 关联对应 issue（如有），在描述中附验证命令与结果。
4. CI 全绿后由维护者 review 并合并。

## 6. 行为准则

- 修复 bug 时改动仅限故障点及其配套文件，不混入无关重构。
- 新增 / 修改后端绑定方法时，必须同步更新 `frontend/src/wails.ts` 与调用方。
- 新行为必须带聚焦测试（Go `*_test.go` / vitest）；不得通过删除失败测试或跳过验证门获得绿色结果。
- 不得提交生成产物与本地配置：`node_modules/`、`frontend/dist/`、`build/`、`LLM-Models/` 下的模型文件、`llama-gui-config.json`、`*.log`。
- 用户可见文案使用中文，与现有页面风格保持一致。
