The "Preferences" page centralizes the app's global configuration, grouped at the top of the page into four tabs: **Appearance**, **Downloads & Directories**, **Server** and **App & Updates**. Most changes save instantly and take effect immediately.

## Appearance

- **Theme Mode**: one-click dark / light;
- **Interface Language**: Auto (follow system), 中文 or English — switching takes effect immediately (the language of these docs follows it too).

## Downloads & Directories

**Model download source**: three sources: **HF Mirror** (hf-mirror.com), **ModelScope**, and **Hugging Face Official** (huggingface.co). The setting drives both search and download on the Downloads page; on Chinese networks prefer the mirror or ModelScope.

**Directories**:

- **llama.cpp Download Path**: where newly downloaded llama.cpp installs are extracted;
- **Model Download Path**: where new model downloads are saved (default `LLM-Models` under the app directory).

These two paths decide where **new** content lands; to reuse model files already on disk, set an external path with "Choose Folder" on the Models page instead.

## Server

- **Server Access Scope**: `Local (127.0.0.1)` keeps the service to this machine; `LAN (0.0.0.0)` opens it to devices on the same network. Changes apply the next time the service starts;
- **API Key**: an optional Bearer token. When set, every inference request must carry it; leave empty for no authentication. Even for purely local use, a key stops other local programs from calling the service freely;
- **Inference GPU**: on multi-GPU machines, choose which NVIDIA card llama-server runs on (stored as a stable UUID and passed to the service process via `CUDA_VISIBLE_DEVICES`). With no NVIDIA GPU the selector is disabled and the service uses the default device. One-click auto-tune budgets VRAM against the card chosen here.

## App & Updates

**System tray (Windows)**: when enabled, clicking the close button minimizes the window to the system tray while llama-server keeps running in the background; the tray icon menu reopens the main window or quits the app. Note: re-enabling the tray after disabling it requires an app restart.

**API route mode**: switches the app into a background-only service (no window, just tray + server) — see the "Headless Mode" section. Windows only, and the system tray must be enabled first.

**Check for Updates**: check manually; the app also auto-checks every two days. When a new version is found you can download it and replace the original executable to update.

**About**: current version, license (GPL-3.0) and the repository URL.
