# Llama Desktop Development Guidelines

## Project Overview

Llama Desktop is a local LLM desktop management tool built on **Wails v2** (Go 1.25 backend + WebView2 frontend). The frontend uses **Vue 3 + TypeScript + Vite 5** (no third-party UI library, hand-written CSS variable theming). The inference engine is **llama.cpp** (llama-server router mode).

Core pipeline: scan `LLM-Models/` for GGUF files → configure per-model inference parameters → generate llama-server model presets (INI) → launch an OpenAI-compatible service (default `127.0.0.1:8080`).

An optional API-route (headless) mode (Windows) relaunches the app as a pure background process — Go backend + system tray + llama-server, no GUI — with the llama-server process kept running uninterrupted across GUI ↔ headless switches (see `core/headless.go` / `core/handover.go`).

## Common Commands

Run these from the repository root:

```bash
wails dev                 # Dev mode: Go backend + Vite frontend (:5173) hot-reload
wails build               # Production build; output at build/bin/llama-desktop.exe (build/ is gitignored)
cd frontend && npm run build   # Frontend only (vue-tsc type-check + vite build)
cd frontend && npm run dev     # Vite only (without Wails runtime, backend calls will fail; see below)
```

Quality gates (run as needed after changes; see "Pre-commit Test Tiers"):

```bash
go build ./...                                   # Backend compilation
go test ./...                                    # Backend unit tests (standard library testing)
gofmt -l .                                       # Go formatting check; must produce no output
golangci-lint run                                # Go static analysis (govet / ineffassign / unused)
cd frontend && npm run build                     # Frontend type-check + build (vue-tsc --noEmit zero errors)
cd frontend && npm test                          # Frontend unit tests (vitest)
make check                                       # Combined quality gate (POSIX)
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1  # Combined quality gate (Windows)
```

> The frontend artifact `frontend/dist` is a compile-time dependency embedded via `go:embed` (gitignored). If the frontend has not been built locally, run `npm run build` or `make check-frontend` before running backend quality gates.

## Architecture and Code Navigation

| File | Responsibility |
| --- | --- |
| `main.go` | Wails entry point: window config (1200×800, Frameless), asset embedding, binding `core.App`, tray icon embed + startup/shutdown wiring, `--headless` / `--gui` mode flags (API-route headless mode vs forced GUI) and the single-instance mutex (only source file at repo root) |
| `core/app.go` | All Wails binding methods: config / system info / models / service / download / monitor / update / router models (thin wrappers), plus `Startup` / `Shutdown` lifecycle |
| `core/engine.go` | Core logic: environment detection, GGUF scanning, llama.cpp download (resumable; main program + CUDA runtime packages), model download task queue, HF Mirror search, config persistence, model preset generation, app version (embedded `core/VERSION`) |
| `core/monitor.go` | Real-time monitoring: service log TPS parsing and CPU / memory / GPU / disk sampling |
| `core/modelscope.go` | ModelScope source: search, file listing, description and download URL construction |
| `core/bridge.go` | Service start/stop and download trigger bridge implementation |
| `core/router.go` | llama-server router API wrapper (GET /models, POST /models/unload): loaded-model listing, model-type classification, unload requests, server port bookkeeping |
| `core/headless.go` | API-route (headless) mode: startup decision core (`ShouldRunHeadless`: `--headless` / `--gui` flags win; non-Windows and dev builds never follow the persisted preference) and `RunHeadless` background startup (Go backend + tray + llama-server, no WebView2) |
| `core/handover.go` | GUI ↔ headless zero-downtime switch: the exiting process writes a handover record (llama-server pid + port, `llama-desktop-server-handover.json` next to the config file); the relaunched process probes it and adopts the still-running llama-server instead of starting a new one |
| `core/i18n.go` | Backend UI language: `tr(zh, en)` translations for user-facing strings |
| `core/locale_windows.go` / `core/locale_other.go` | System locale detection (kernel32 `GetUserDefaultLocaleName` on Windows / env-based lookup on other platforms) |
| `core/tray_windows.go` / `core/tray_other.go` | Windows system tray (fyne.io/systray: show main window / quit; `quitOnce` prevents re-`Run` in one process) / other-platform no-op stubs |
| `core/hidewindow_windows.go` / `core/hidewindow_other.go` | Hide child-process console windows (Windows implementation / other platforms no-op) |
| `core/crossdevice_windows.go` / `core/crossdevice_other.go` | Cross-device rename detection (Windows `ERROR_NOT_SAME_DEVICE` / other platforms `EXDEV`) for the download move-file fallback |
| `core/singleinstance_windows.go` / `core/singleinstance_other.go` | Single-instance named mutex (blocks double launch; the retry window also covers the headless → GUI handover) / other-platform stubs |
| `core/headlessalert_windows.go` / `core/headlessalert_other.go` | Native MessageBox alert when llama-server fails to start in headless mode (no GUI to surface the error) / other-platform stubs |
| `core/installer_launch_windows.go` / `core/installer_launch_other.go` | App-update installer launch with UAC elevation (ShellExecute `runas` verb; plain fork/exec fails with `ERROR_ELEVATION_REQUIRED`) / other-platform plain launch |
| `core/diskusage_windows.go` / `core/diskusage_other.go` | Disk usage for the volume containing a path (`GetDiskFreeSpaceEx`) / other-platform fallback |
| `core/devbuild_dev.go` / `core/devbuild_release.go` | `isDevBuild` via the `dev` build tag (`wails dev` sets it, `wails build` does not); declared as a var so plain `go test` runs can exercise dev branches |
| `core/*_test.go` | Backend unit tests (`runCmd` cross-platform, `hideWindow` platform branch, config persistence, GGUF parsing, model scanning, preset generation, download/HF/ModelScope network tests, monitor, router, update, tray, cross-device, etc.) |
| `frontend/src/wails.ts` | Sole frontend entry for calling the backend (`window.go.core.App.*` bridge layer) |
| `frontend/src/views/` | Pages: `Home` (system info: CPU / memory / GPU / CUDA), `Runtime` (runtime environment: llama.cpp per-component status — main program + CUDA runtime — with resumable download / custom directory; former llama.cpp card on Home), `Chat` (local chat: direct SSE streaming to llama-server), `Downloads` (HF Mirror / ModelScope search), `ModelDetail` (download subpage: file list + description; not in sidebar), `Models` (model list; merges the model download path and imported external directories, each model labeled with its source dir), `ModelSettings` (standalone route page for per-model inference params; not in sidebar), `Api` (service start/stop + real-time monitor; former `Monitor` page merged in and redirected), `Settings` (theme / language / download source / directories: llama.cpp & model download paths + external import dirs / tray / API-route mode / check for updates). Sidebar order: Home → Runtime → Chat → Downloads → Models → API → Settings |
| `frontend/src/components/` | `Sidebar` (navigation + collapse + system-ready status), `UpdateModal` (update download modal), `TaskDock` (global bottom-right task card: download progress + in-memory model unload), `ThemedSelect` (themed custom dropdown: button trigger + in-app option list; variants `field` / `toolbar`) |
| `frontend/src/lib/` | Pure/state modules: `i18n.ts` (bilingual dictionary), `format.ts` (bytes / percent / uptime), `monitor.ts` (TPS formatting, uptime), `chat.ts` + `chatState.ts` (SSE parsing, chat request building, module-level chat state + localStorage persistence), `dock.ts` / `downloadStatus.ts` / `taskStatus.ts` / `llamaDownload.ts` (download-state vocabularies and display rules), `modelFiles.ts` (file sorting + quantization guessing), `downloadsState.ts` (search context), `update.ts` (update check/download state), `selectOptions.ts` (option shape + display-label resolution for ThemedSelect), `dockSpace.ts` (shared bottom-space reservation for the floating TaskDock pill so overlaid controls stay clickable), `limitedQueue.ts` / `latestOnly.ts` / `systemReady.ts` (task queue, polling, readiness) |
| `frontend/src/__tests__/` | Frontend unit tests (vitest, covering `store.ts` config loading and `lib/` pure functions: formatting, download queue / task status, chat SSE, monitor sampling, update, system readiness, i18n, etc.) |
| `frontend/wailsjs/` | Wails auto-generated bindings — **do not edit manually**; regenerated on `wails build` |

## Wails Binding Mechanism (Important)

- Backend methods are declared in `core/app.go` as `func (a *App) Xxx(...)`. The root `main.go` adds `core.NewApp()` to the `Bind` list to expose them to the frontend. `App` lifecycle methods are exported as `Startup` / `Shutdown`, referenced by `main.go`'s `OnStartup` / `OnShutdown`.
- The frontend calls exclusively through `frontend/src/wails.ts` using `window.go.core.App.Xxx(...)` (methods return Promise). **The `core` namespace comes from the Go package name `core`**: if the package name or binding type ever changes, `wails.ts`'s `app()` and the generated bindings under `wailsjs/go/` must all be updated in sync. When adding or modifying backend methods, always update the wrappers in `wails.ts` and keep naming consistent.
- `window.go` is injected only by the Wails runtime. Running `vite` standalone (without `wails dev`) causes all backend calls to throw — this is expected behavior, not a bug. To debug the frontend UI standalone without Wails, inject a `window.go` mock that implements all binding methods before page load (the repo does not include this mock; provide your own).
- When the backend returns structs, JSON field names follow struct tags (e.g., `DlTask`'s `sizeHuman`). After modifying a returned struct, verify that the corresponding frontend interface stays in sync.

## Multi-role Collaboration and Workflows

This file covers all Agent tools, but the role rules below do not all apply to every tool simultaneously. Roles are determined by **explicit task instructions from the user at the start of the task**: if the user says "you are the reviewer", that session assumes reviewer duties; if the user says "you are the implementation agent" (or delivers an implementation task package), that session assumes implementation agent duties; if the user says "you are the issues finder", that session assumes issues finder duties. The implementation agent defaults to a `general-purpose` sub-agent. Simply reading AGENTS.md, using a vendor model, or completing implementation and self-testing does not automatically grant reviewer privileges.

When no role is declared, follow the remaining rules in this file normally (including direct commits per the "Local Commit Strategy"). Unless the user explicitly reassigns roles within the current task, only one reviewer may exist per batch. Implementation agents must not promote themselves to reviewer, approve their own changes, or treat their own completion report as final acceptance; reviewers must not assume the user's chosen implementation agent's role just because they could edit code themselves.

All agents must protect unrelated user changes and respect this file's scope discipline, quality gates, repo hygiene, and remote/destructive operation limits. Do not delete failing tests, skip quality gates, or expand exemptions to get a green result.

Without compromising architecture boundaries, behavioral correctness, test coverage, or quality gates, all plans and implementations should optimize end-to-end completion time: eliminate foreseeable rework first, merge required production changes and tests within the same ownership scope, and parallelize non-conflicting read-only checks. Do not trade speed for expanded exemptions, lowered assertions, skipped tests, or widened unreviewed scope.

This file retains only persistent invariants and verification discipline; stage status, task packages, debt lists, decisions, and date-stamped evidence belong in issue comments or `docs/` authoritative documents. No agent's completion report is a final-acceptance source of truth.

### Reviewer Responsibilities (applies when the user declares "you are the reviewer")

- The reviewer's core input is **the commands and goals issued by the user**, not issues. After the user assigns a task, the reviewer converts the goal into an executable plan and **persistently drives the plan and verification** across the task lifecycle: the plan evolves with implementation feedback, and verification covers every implementation round, not just the final one.
- Full closed loop: understand the user's command and goal → formulate a plan → break into task packages for the implementation agent → review actual diffs → run quality gates → accept → create a local commit per the "Local Commit Strategy".
- The reviewer **must not self-implement**: all implementation is done by the implementation agent (default `general-purpose` sub-agent) to avoid self-review. This exception applies only when the user explicitly requests the reviewer to implement directly in the current task.
- Pulling and reviewing issues is an **optional input** for the reviewer, not the default starting point: when the user requests it, or when the task involves fixing existing issues, use `gh issue list` / `gh issue view <N>` to pull remote issues. For each issue, reproduce or locate `file:line` evidence to confirm it is real. For non-existent issues, explain the conclusion in a comment and do not plan a fix.
- Plan formulation: prioritize by user goals, impact, and risk; the plan may be a single task package or multiple packages, all delivered to the implementation agent. Each task package must state: goal, allowed modification paths, forbidden paths, expected behavior, required new or updated tests, mandatory verification commands, delivery evidence, and explicit exit criteria.
- Review order is fixed: review actual diff, authorized paths, key behavior, and focused tests first, then run expensive full quality gates (`make check` / `scripts/check.ps1`; see "Pre-commit Test Tiers"). When static review or focused review reveals blocking issues, form a fix task package first; do not run the full gate early.
- Persistent verification: after every implementation round, the reviewer must re-read the actual diff, review against specification compliance, implementation quality, boundary and behavior, and test sufficiency item by item, and personally run applicable verification commands. Do not rely solely on the implementation agent's self-report. Blocking issues, test failures, missing acceptance evidence, or growing technical debt mean no submission — form a fix task package with file, behavior, and command evidence and continue until all exit criteria for the batch are met.
- Only the reviewer may create a local commit after acceptance is verified; the commit body records actual quality gate results.

### Implementation Agent Responsibilities (defaults to `general-purpose` sub-agent; applies when the user declares "you are the implementation agent")

- The implementation agent is responsible for implementing and verifying per the task package (or user instructions): only modify paths and behavior within the authorized scope; implement expected behavior and tests; run specified verification commands; report modified files, key design decisions, actual commands and full results, known limitations, and unfinished items. Self-reported pass is only delivery evidence pending reviewer re-validation.
- Mechanical fixes, test additions, and formatting cleanups that are deterministically verifiable within the authorized scope should be completed directly without pausing for minor self-resolvable issues; only stop and request direction when the work requires expanding authorized paths, changing behavior or verification contracts, lowering quality gates, or executing remote/destructive operations.
- The implementation agent must not run `git add`, `commit`, `push`, PR, `merge`, or similar operations. After implementation and self-testing, stop at an unstaged working tree and wait for reviewer review or user instructions. Issues found by the implementation agent become fix task packages in the next round by the reviewer.

### Issues Finder Responsibilities (applies when the user declares "you are the issues finder")

- The issues finder is responsible for finding real problems in the project, verifying them, and submitting remote issues per the "Issue Tracking" chapter rules.
- **Verification process (interaction-first)**: prefer Playwright MCP to discover functional defects via real user interactions — operate through the core flows item by item, such as: home page system info loading, model scanning and parameter settings, service start/stop and API calls, download task management, theme switching, etc. After interactive confirmation that the problem exists, return to the code to locate the root cause (`file:line` evidence).
- Only when interactive verification passes (core functionality is sound) does the issues finder shift to code audit for non-interactive issues (style, a11y, i18n, security, dead code, documentation gaps, etc.).
- When the user assigns this role, it is treated as authorization to create remote issues. Before submitting, cross-check existing remote issues to avoid duplicates, and follow all rules in the "Issue Tracking" chapter: form templates, severity prefixes and labels, `file:line` evidence, acceptance criteria, and body redaction. For batch findings, create a top-level Tracker overview issue first; create separate child issues for each P0/P1 finding; P2/P3 may be merged into a summary issue.
- Sensitive security vulnerabilities (credential leaks, injection, privilege escalation, etc.) must not be filed as public issues. Report them privately via GitHub Security Advisories (`Security > Report a vulnerability`). After the fix lands, a non-sensitive tracking issue may be opened.
- This authorization covers only issue creation and maintenance; push, open PR, merge, and other remote operations still require explicit user authorization.
- Creating or updating remote issues is also subject to the "Git Workflow" retry rules: for network failures, wait a period (e.g., 30–60 seconds) and retry; multiple retries at different times usually succeed. Only report and pause when the failure is confirmed to be an authentication, permission, or conflict error, or when consecutive retries still fail.

## Git Workflow

- By default, commit directly to the local `main` branch; only create feature branches when the user explicitly requests it or when changes need isolated review (e.g., preparing a PR for an external collaborator), and state the reason before the task.
- In role-based collaboration (see "Multi-role Collaboration and Workflows"), local commits are made only by the session holding reviewer duties; sessions declared as implementation agents stop at an unstaged working tree and do not commit.
- Do not switch or create branches on your own initiative; if unsure about branch choice before committing, confirm with the user first.
- Remote operations (push, `gh issue create`, etc.) require explicit user authorization before execution.
- Feature branches are temporary and must be deleted immediately after PR merge (both remote `git push origin --delete <branch>` and local `git branch -D <branch>`); on merge, try `gh pr merge --delete-branch`; if that does not take effect, manually clean up to avoid leftover merged branches.
- Remote operations (push, `gh issue create`, etc.) may fail due to unstable network: do not declare failure immediately; wait a period (e.g., 30–60 seconds) and retry. Multiple retries at different times usually succeed. Only report and pause when the failure is confirmed to be a non-network error such as authentication, permission, or conflict, or when consecutive retries still fail.

## Code Standards

### Go Backend

- All Go code must pass `gofmt`, `go build ./...`, and `golangci-lint run` (zero diagnostics for govet / ineffassign / unused); never swallow errors — wrap them with context using `fmt.Errorf("...: %w", err)`.
- Concurrent shared state uses explicit mutexes (project convention: `configMu` / `serverLogsMu` / `downloadMu` / `dlTasksMu`, etc.); do not introduce new unlocked shared variables.
- Download state machine (`downloadState.Status`) values: `idle / fetching / downloading / paused / extracting / done / error`. The frontend mirrors all download-status vocabularies (llama.cpp download / model tasks / app update) in `frontend/src/lib/downloadStatus.ts`; adding or renaming any status requires updating that module, the exhaustive Record tables in `frontend/src/__tests__/` (llamaDownload / dock / taskStatus — vue-tsc fails on a missing key), and the UI `statusLabel` / i18n maps (Runtime.vue, TaskDock.vue).
- Logs use `log.Println` with a consistent prefix (`[INFO]` / `[WARN]` / `[ERROR]` / `[OK]`); service logs flow through the `serverLogs` ring buffer.
- Launching external child processes must call `hideWindow(cmd)` to prevent GUI applications from flashing a console window (see `hidewindow_windows.go`).
- New behavior must include focused tests: use the standard library `testing`, placed in same-package `*_test.go`; write concise English comments describing the behavior under test.

### Vue / TypeScript Frontend

- `npm run build` (vue-tsc strict) must produce zero errors; no unused variables, imports, or dead code.
- Styles use the project's own semantic tokens beyond `var(--cn-*)` (`--bg-primary` / `--surface` / `--text-primary` / `--border`, etc.); light/dark themes are switched via `html[data-theme]`; new colors must provide both light and dark values.
- Component props / emits use `defineProps<T>` / `defineEmits` typed; `any` must not appear (tests and mocks are exempt).
- New pages must register a route in `router/index.ts` and add a sidebar navigation item (`Sidebar.vue`'s `navItems`).
- User-visible copy is bilingual via the i18n dictionary (`lib/i18n.ts`); never hardcode UI strings. Comments are in English throughout.
- Behavior changes must add or update focused vitest tests (`src/__tests__/`); only mock side-effect-bearing dependency chains (Wails bridge, network, time). Do not mock pure functions or pure data modules.

### General

- When fixing bugs, limit changes to the fault site and its related files; do not mix in unrelated refactoring.
- For cross-frontend-backend changes (e.g., adding binding methods), `app.go`, `wails.ts`, and the frontend caller must be updated in a single commit to avoid intermediate states.
- For changes involving the download state machine, service start/stop, or config file structure (`llama-desktop-config.json`), also verify backward compatibility of old data (`loadConfig`'s default-value fallback logic).
- New behavior must include focused tests; when modifying or deleting existing tests, explain the reason. Do not delete assertions to pass quality gates.

## Pre-commit Test Tiers

Verification strength is determined primarily by behavioral risk and ownership boundaries; file count is only a fallback signal and cannot alone prove risk level.

- **No-test commit**: changes documentation only, or changes only comments that do not affect compilation, lint, or runtime behavior. Skip the test suite; run `git diff --check` and applicable documentation checks.
- **Local commit**: typically no more than 3 non-documentation files within the same module or ownership scope, and does not hit high-risk conditions. Run the affected side's quality gates (backend: `go build` + `go test` + `gofmt -l` + `golangci-lint run`; frontend: `npm run build` + `npm test`) and the affected focused tests.
- **Medium commit**: 4–5 non-documentation files but still confined to the same module, with a single behavior and clear focused coverage; local verification may continue. The commit body must explain the scope and the rationale for not running the full suite.
- **Full commit**: cross-module or cross-ownership linkage, high-risk behavioral changes, or 6+ non-documentation files that cannot be proven to be purely local mechanical changes. Run the full `make check` / `scripts/check.ps1`.
- Regardless of file count, the following always qualify as full commits:
  - Changes to config persistence structure, shared state, service start/stop logic, or the download state machine;
  - New backend binding methods, API endpoints, or external protocol/response structures;
  - Changes to shared test infrastructure, CI configuration, or the quality gate scripts themselves.
- File count excludes pure documentation, pure comments, and artifacts mechanically generated by the same command that have been verified for drift; new tests, fixtures, snapshots, and configs count as non-documentation files.
- If any required verification command fails, the commit must not proceed; fix the failure, re-run the applicable verification, and confirm it passes before proceeding.
- **Current test baseline**: the backend has focused tests covering the `core` package (config persistence, GGUF parsing, model scanning, preset generation, download/HF/ModelScope network tests, system parsing, service command construction, etc.); the frontend has vitest for `store.ts` and `lib/` pure functions. New behavior must include focused tests; do not skip tests citing "no test framework".

## Local Commit Strategy

Use Conventional Commits in the form `type(scope): English subject`, with the subject on a single line. Existing types: `feat` / `fix` / `docs` / `chore` / `refactor` / `test` / `perf` / `security`. Scopes such as `backend` / `frontend` / `build` / `models` / `server` / `downloads` / `config` etc., used only when meaningful.

The detailed body must include the following structure:

```text
Summary:
- <primary changes, grouped by domain: Backend / Frontend / Tests / Docs, etc.>

Verification:
- <actual verification commands run and pass results; full commits must record complete quality gate results>

Remaining gaps:
- <explicitly state unfinished follow-up work; write None if there are none>
```

- Stage files by explicit path. Before committing, check `git status --short`, `git diff --cached --stat`, and `git diff --cached --check`; the working tree must be clean after commit.
- External implementation agents must not stage or commit; only the reviewer may do so after re-reviewing the actual diff, confirming all task package exit criteria are met, and personally completing the required verification. If review reveals issues, enter the next fix round without committing.

## Versioning and Releases

- The authoritative source for changelogs is the root `CHANGELOG.md`: before a release, add the corresponding version entry (with date). **Tag annotation messages and GitHub Release bodies are copied from this entry** to keep them in sync; do not rely on GitHub "auto-generate release notes" (this repo commits directly to main with no PRs, so auto-generation only produces a compare link).
- Entries summarize the release as a whole: a short paragraph plus a few bullets covering **source-code changes only** (Backend / Frontend behavior, build/CI); documentation-only edits (README, screenshots, CHANGELOG itself, AGENTS.md) are not listed. Do NOT write per-commit breakdowns or commit hashes; keep each entry a concise, user-facing digest.
- Entries are bilingual with Chinese on top: a `## 中文` section (top) followed by an `## English` section (bottom) — starting with **v0.3.3**; v0.3.0–v0.3.2 entries are English-first per-commit records and are preserved as-is, while entries before v0.3.0 were removed along with their tags/releases (only the "历史版本" note remains in CHANGELOG.md). The frontend update modal extracts the section matching the UI language (see `frontend/src/lib/update.ts` `extractReleaseNotes`, which handles both marker orders); tag messages and Release bodies keep both sections so the in-app notes stay language-appropriate.
- Version tags are annotated tags (`git tag -a vX.Y.Z`) with the message taken from the CHANGELOG entry. Before tagging, confirm quality gates pass. Pushing tags is a remote operation and requires user authorization.
- GitHub Release bodies are not automatically synced from tag messages: when creating or updating a Release, paste the body from the corresponding CHANGELOG entry. This is a remote operation requiring explicit user authorization.
- Releases must keep `core/VERSION`, `wails.json` `info.productVersion` and `frontend/package.json` `version` in sync — all three carry the app version (`core/VERSION` and release tags use the `v` prefix; the other two do not).

## Documentation Language

- `README.md` is the Chinese primary documentation. `README_en.md` is the English counterpart.
- Screenshots live under `docs/screenshots/zh/` and `docs/screenshots/en/`; each document references the set matching its own language.
- `CHANGELOG.md` entries are bilingual with Chinese on top (`## 中文` first, `## English` below) and summarize the release as a whole; entries before v0.3.0 were removed along with their tags/releases, leaving only a short historical note.

## Repository Hygiene

- Before committing, `git status --short` must contain only intentionally modified files for the current task; `git diff --check` must produce no errors.
- Do not commit: `node_modules/`, `frontend/dist/`, `build/` (including compiled exe artifacts), model files under `LLM-Models/`, `llama-cpp/`, `llama-desktop-config.json` (local config, may contain machine-specific paths), `*.log`, `.zcode/plans/`.
- Screenshot and other documentation assets are committed under `docs/` (e.g., `docs/screenshots/`).
- When adding new ignore patterns, update `.gitignore` in sync.

## Issue Tracking

Issues live at `https://github.com/CodeNeow/llama-cpp-desktop/issues`. Any non-trivial defect or planned work should be filed as an issue to make progress visible; keep the list high signal-to-noise. This chapter applies as execution rules when the Issues finder role submits remote issues, and to anyone creating issues.

### Creating Issues

- Web-created issues use the form templates under `.github/ISSUE_TEMPLATE/`: daily bugs use `bug-report.yml`; structured findings from audits or code reviews use `audit-finding.yml` (includes `file:line` evidence and acceptance criteria).
- Batch findings (e.g., a full audit): create a top-level Tracker overview issue, with each child issue linked in a priority table. Create separate child issues for each P0/P1 finding; P2/P3 may be merged into a summary issue.
- Sensitive security vulnerabilities (credential leaks, injection, privilege escalation, etc.) must not be filed as public issues. Report them privately via GitHub Security Advisories. After the fix lands, a non-sensitive tracking issue may be opened.
- Issue bodies must never contain secrets or machine-specific absolute paths (e.g., paths from `llama-desktop-config.json`). Redact tokens, secrets, DSNs, and paths before submitting, including in logs and screenshots.

### Titles and Labels

- Finding-class titles add a severity prefix: `[P0]` / `[P1]` / `[P2]` / `[P3]`; other titles use `[Bug]` / `[Tracker]`. Keep the title on one line and name the affected component or area, e.g., `[P1] child-process console window flashes frequently at startup`.
- Apply exactly one priority label (`P0-critical` / `P1-high` / `P2-medium` / `P3-low`) and at least one area label (`frontend` / `backend` / `models` / `downloads` / `server` / `config` / `security`). Labels are created in one shot via `scripts/create-labels.ps1`. Do not invent new labels; apply first.
- Severity reflects impact, not effort: P0 blocks core functionality or is a security vulnerability; P1 materially harms experience or security; P2 is a consistency or polish gap; P3 is a nitpick-level minor issue.

### Required Content (Findings)

A finding issue must include: phenomenon, reproduction steps, `file:line` evidence, impact, and acceptance criteria as a `- [ ]` checklist. Fix suggestions (including diffs) are encouraged but not required. If the finding belongs to a batch, reference the Tracker issue number in the body or a comment.

### Lifecycle

- Use `Fixes #N` in the PR description to associate the fix; the linked issue closes automatically on merge.
- Do not close an issue until acceptance criteria are verifiably met and the relevant quality gates pass; quality gate results are recorded in the PR, not only in the issue.
- When an issue is superseded or duplicated, comment linking to the canonical issue and close it; do not delete.

## Common Pitfalls

- **`wails dev` reports port-in-use**: Vite binds `localhost:5173` (see `wails.json`'s `frontend:dev:serverUrl`); end the occupying process before starting.
- **Go struct changed but frontend does not receive new fields**: verify `wails.ts` wrappers and page interfaces are in sync; `wailsjs/` is auto-updated by build and does not need manual changes.
- **"Start Service" fails on the API page**: `startServerInternal` scans `LLM-Models` and generates presets first; an empty directory returns an error (`no models found in the LLM-Models directory`) — this is expected behavior. Confirm GGUF files exist on the "Models" page first.
- **Stopping service on Windows**: `stopServerInternal` terminates llama-server via `cmd.Process.Signal(os.Kill)`; do not use `taskkill /IM llama-server.exe` externally with broad force-kill, to avoid killing other instances.
- **`gofmt -l` reports many existing files on this machine**: the repo commits as LF; Windows checkout (`core.autocrlf`) as CRLF causes `gofmt` to report the entire file. When judging formatting issues, use `git -c core.autocrlf=false diff` to inspect actual changes; do not rewrite untouched line endings as a side effect.
- **`frontend/dist: no matching files found` when running backend commands locally**: `go:embed` depends on `frontend/dist`; run `cd frontend && npm run build` before running backend commands.
- **GitHub Release page shows only a "Full Changelog" compare link**: auto-generated release notes aggregate by PR; this repo commits directly to main with no PRs, so the generated result is only a compare link. Tag messages are not synced to the Release body automatically; paste from the corresponding `CHANGELOG.md` entry (see "Versioning and Releases").
