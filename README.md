# Llama Desktop

[简体中文](README_zh.md)

[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Ubuntu-0078D6?logo=windows&logoColor=white)](https://github.com/CodeNeow/llama-cpp-desktop/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/CodeNeow/llama-cpp-desktop?logo=github&color=blue)](https://github.com/CodeNeow/llama-cpp-desktop/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/CodeNeow/llama-cpp-desktop/.github/workflows/ci.yml?branch=main&logo=githubactions&logoColor=white)](https://github.com/CodeNeow/llama-cpp-desktop/actions)

A user-friendly desktop GUI for [llama.cpp](https://github.com/ggml-org/llama.cpp) — visually configure cutting-edge GGUF models like Qwen3.8-27B, serve them all on one OpenAI-compatible endpoint, with built-in model downloads, local chat and real-time monitoring. Built with Wails v2 (Go backend + Vue 3 frontend, no third-party UI framework).

## Highlights

- **One endpoint, many models** — runs llama-server in router mode (`--models-dir` / `--models-preset` / `--models-max`), serving every GGUF in your models directory over a single OpenAI-compatible API (default `http://127.0.0.1:8080/v1`); models are loaded and unloaded on demand.
- **Built-in chat** — streaming conversations against the local endpoint, with image attachments for multimodal models and per-session sampling controls (temperature, top-p / top-k, repeat penalty, max tokens, system prompt).
- **Model discovery and downloads** — search HF Mirror (hf-mirror.com) or ModelScope, expand repositories into file lists, and batch-download through a resumable queue (pause / resume / cancel) that survives restarts.
- **Per-model inference presets** — GPU layers, KV cache types, long-context RoPE settings, speculative decoding and more, persisted per model and written into the llama-server preset on save.
- **Live service monitor** — server log console, prompt-processing and generation token speed with charts, and CPU / memory / GPU (utilization + VRAM) sampling, refreshed every second.
- **Desktop niceties** — Windows system tray, in-app update check, light / dark themes, and a zh / en / auto UI language.

## Screenshots

![System Info](docs/screenshots/en/home.png)

| Local Chat | Model Manager |
| :---: | :---: |
| ![Local Chat](docs/screenshots/en/chat.png) | ![Model Manager](docs/screenshots/en/models.png) |
| ![Model Downloads](docs/screenshots/en/downloads.png) | ![API Router](docs/screenshots/en/api.png) |

## Getting Started

### Prerequisites

- Windows 10+ or Ubuntu 20.04+ (on Windows, WebView2 is installed automatically with the app)
- [Git](https://git-scm.com/), [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org/) 18+
- Wails CLI v2.14+:

  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### Run in dev mode

```bash
git clone https://github.com/CodeNeow/llama-cpp-desktop.git
cd llama-cpp-desktop
wails dev
```

`wails dev` starts the Go backend and the Vite dev server (`http://localhost:5173`) together, with hot reload on both sides.

### First run

1. On **System Info**, click "Download llama.cpp" to fetch the latest release from GitHub (resumable), or point the app at an existing llama.cpp directory.
2. On **Model Downloads**, search HF Mirror or ModelScope and download a GGUF file into the models directory (`LLM-Models/` by default); progress shows up in the task dock at the bottom-right corner.
3. On **API Router**, confirm the host / port (default `127.0.0.1:8080`) and click "Start".
4. Open **Local Chat**, pick the model, and start talking — or point any OpenAI-compatible client at the endpoint.

## Usage

- **System Info** — detects CPU, memory, GPU and CUDA, and shows the llama.cpp installation status; one-click download or a custom directory.
- **Local Chat** — streaming chat with image attachments and sampling parameters; requires the API router to be running.
- **Model Downloads** — dual-source search (HF Mirror / ModelScope, switchable in Preferences) with file-level selection and a persistent, resumable download queue.
- **Model Manager** — scans the models directory for GGUF files (architecture, quantization, multimodal / embedding detection) and links each model to its settings page (basic / inference / memory / multi-GPU / long-context / advanced tabs).
![Model Settings](docs/screenshots/en/model-settings.png)
- **API Router** — start / stop / restart llama-server, watch logs and live metrics, edit host / port / access mode / max concurrent models / prompt cache, and see which models are currently in memory.
- **Preferences** — theme, UI language (zh / en / auto), download source, Windows tray toggle, and check for updates.

Once the service is running, any OpenAI-compatible client can connect:

```bash
OPENAI_BASE_URL="http://127.0.0.1:8080/v1"
OPENAI_API_KEY="sk-any-placeholder"   # the local service does not authenticate
```

Set `model` to the GGUF file name (for example `qwen2.5-7b-instruct-q4_k_m.gguf`); llama-server loads and unloads models on demand. In-memory models can also be unloaded from the task dock.

## Configuration

Runtime settings are persisted to `llama-desktop-config.json` in the project root. Key fields:

| Field | Meaning | Default |
| --- | --- | --- |
| `theme` | UI theme: `light` / `dark` | `light` |
| `language` | UI language: `zh` / `en` / `auto` (auto follows the OS locale) | `auto` |
| `downloadSource` | Default model source: `hf` / `modelscope` | `hf` |
| `trayEnabled` | Windows system tray (show / quit menu) | `true` |
| `sidebarCollapsed` | Whether the sidebar starts collapsed | `true` |
| `serverConfig` | `accessMode` (`local` / `lan`), `host`, `port`, `maxModels`, `cacheRam` (MiB) | `127.0.0.1:8080`, `maxModels` 1, `cacheRam` 8192 |

Also stored: `llamaCppDir` / `modelDir` (custom directories), `modelConfigs` (per-model inference parameters) and `downloadTasks` (the download queue, recovered on restart).

## Development

Combined quality gate:

```bash
make check                                                                  # POSIX
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\check.ps1     # Windows
```

The gate runs `go build` / `go test` / `gofmt` / `golangci-lint` on the backend and `npm run build` (vue-tsc + vite) on the frontend; the PowerShell script also runs the vitest suite (`npm test`). Backend tests live in `core/*_test.go` (standard library `testing`), frontend tests in `frontend/src/__tests__/`. See [AGENTS.md](AGENTS.md) and [CONTRIBUTING.md](CONTRIBUTING.md) for conventions, commit format and the collaboration workflow.

## Building

```bash
wails build
```

The production binary is written to `build/bin/llama-desktop.exe` (Windows) or `build/bin/llama-desktop` (Linux). The frontend is embedded into the binary via `go:embed`; `wails build` compiles it automatically.

## FAQ

**`wails dev` reports the port is already in use.**
The Vite dev server binds `localhost:5173` (see `frontend:dev:serverUrl` in `wails.json`). End the process occupying it and retry.

**"Start" on the API Router page fails with "no models found".**
Startup scans the models directory and generates presets first, so an empty directory is an error. Put GGUF files into `LLM-Models/` (check the Model Manager page) and try again. Also confirm llama.cpp is installed, as shown on the System Info page.

**Every backend call throws when running the frontend with `npm run dev` standalone.**
`window.go` is injected by the Wails runtime, so Vite without `wails dev` has no bridge to the Go backend — this is expected. Use `wails dev` to debug the UI with the backend attached.

**Downloading llama.cpp is slow or fails.**
The download comes from GitHub Releases; it supports pause / resume with resumable transfers. On a restricted network, download the release for your platform manually, extract it, and select the directory via "Custom" on the System Info page.

## License

This project is licensed under the [MIT License](LICENSE).
