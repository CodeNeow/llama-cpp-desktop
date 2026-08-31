The "Preferences" page centralizes the app's global configuration. Top to bottom it contains: a gradient "Help & Tutorial" entry card, a "Device" info card, and three rounded setting groups (Appearance & Preferences, Directories & Services, About). Most changes save instantly and take effect immediately.

## Help & Tutorial

The purple gradient card at the top of the page is the entry to the in-app tutorial — click it to open the "Documentation" page (the bilingual guide you are reading right now). The docs update online, so fixes reach you without waiting for an app release.

## Device

The "Device" card below the entry card summarizes this machine: operating system, CPU architecture (`arm64` / `amd64` / ...), the llama-server acceleration build (CUDA / Vulkan / Metal / CPU; CPU-only on Android) and the current app version. It shows on both desktop and mobile.

## Appearance & Preferences

- **Theme Mode**: one-click dark / light (gradient capsule switch);
- **Interface Language**: Auto (follow system), 中文 or English — switching takes effect immediately (the language of these docs follows it too);
- **Model download source**: three sources: **HF Mirror** (hf-mirror.com), **ModelScope**, and **Hugging Face Official** (huggingface.co). The setting drives both search and download on the "Download" tab of the Models page; on Chinese networks prefer the mirror or ModelScope.

## Directories & Services

**Download paths**:

- **llama.cpp Download Path**: where newly downloaded llama.cpp installs are extracted;
- **Model Download Path**: where new model downloads are saved (default `LLM-Models` under the app directory).

These two paths decide where **new** content lands; to reuse model files already on disk, set an external path with "Choose Folder" on the "My Models" tab of the Models page instead. On Android these paths are app-managed and read-only.

**Server Access Scope**: `Local (127.0.0.1)` keeps the service to this machine; `LAN (0.0.0.0)` opens it to devices on the same network. Changes apply the next time the service starts.

**API Key**: an optional Bearer token. When set, every inference request must carry it; leave empty for no authentication. Even for purely local use, a key stops other local programs from calling the service freely.

**Inference GPU**: on multi-GPU machines, choose which NVIDIA card llama-server runs on (stored as a stable UUID and passed to the service process via `CUDA_VISIBLE_DEVICES`). With no NVIDIA GPU the selector is disabled and the service uses the default device. One-click auto-tune budgets VRAM against the card chosen here. Windows only.

**System tray (Windows / macOS)**: when enabled, clicking the close button minimizes the window to the system tray while llama-server keeps running in the background; the tray icon menu reopens the main window or quits the app. The row shows only where the capability is reliable (Linux desktop-environment tray support is incomplete, so it is not offered there). Note: re-enabling the tray after disabling it requires an app restart.

**API route mode**: switches the app into a background-only service (no window, just tray + server) — see the "Headless Mode" section. Windows only, and the system tray must be enabled first.

## About

- **Updates**: check manually; the app also auto-checks every two days. When a new version is found you can download it and replace the original executable to update. The in-app self-update action is Windows-only; other platforms show a link to GitHub Releases instead;
- **Version / License / Repository**: the current version, the GPL-3.0 license and the repository URL.
