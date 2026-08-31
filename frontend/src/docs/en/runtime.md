The "Runtime Environment" tab of the System Environment page (side by side with the "System Info" tab) manages what the service depends on: the llama.cpp main program (llama-server) and, on Windows, the CUDA runtime (cudart) that ships alongside CUDA builds.

## Status and components

The tab shows llama.cpp information as a card:

- **Status**: Installed / Not found;
- **Version**: the detected version once installed;
- **Install path / Download path**: where the main program lives and where new downloads land;
- **Component list** (shown once installed):
  - `llama-server main program`: the inference service itself, annotated with the actually detected acceleration build (CUDA / Vulkan / Metal / CPU);
  - `CUDA runtime (cudart)`: Windows only, annotated with the detected CUDA major family (e.g. `CUDA 12`). It is downloaded automatically with CUDA builds of llama.cpp; without it, NVIDIA GPU acceleration cannot work.

## Downloading llama.cpp

When nothing is installed, it offers two actions:

- **Download llama.cpp**: fetches the latest release from GitHub — two packages in one download, the main program and the CUDA runtime (Windows), each with its own progress bar. Linux picks the Vulkan build (accelerating NVIDIA / AMD / Intel GPUs alike); macOS picks the Metal build (Apple Silicon) or the CPU build (Intel).
- **Custom**: browse to an existing llama.cpp directory and the app will use it instead of downloading.

While downloading you can **pause**, **resume** (resumable — it never starts over), or **cancel** at any time. Transient network failures are retried automatically (up to 3 times, 3 seconds apart); after a hard failure the tab shows the reason plus a retry button.

## Custom directory

After picking a directory via "Custom", the app immediately re-probes it:

- The directory must contain the llama-server binary; CUDA builds additionally need the matching cudart library in the same directory (or on the system PATH).
- The choice is persisted and used on every subsequent start.

The download path itself can be changed under "Preferences → Directories" to decide where new llama.cpp installs land.

## The three-state CUDA compatibility display

The CUDA card on the "System Info" tab of the System Environment page (shown only on Windows when an NVIDIA GPU is detected) has a "CUDA Compat" row with three possible states:

| State | Meaning |
| --- | --- |
| Compatible | Regular GPUs (compute capability below 12.0): the current CUDA runtime just works |
| Requires CUDA ≥12.8 | The GPU is Blackwell (RTX 50 series, compute capability ≥ 12.0) but the current CUDA runtime is not proven to meet 12.8 |
| Satisfied | Blackwell GPU with an installed CUDA runtime confirmed to meet 12.8 |

Why the "Requires CUDA ≥12.8" state exists: Blackwell GPUs need CUDA 12.8 or newer, otherwise llama-server fails with `no kernel image`. But the cudart version can only be parsed down to the major family from the DLL file name (e.g. `12`) — a bare major cannot prove the minor requirement, so unless 12.8 is explicitly proven, the app conservatively shows "Requires CUDA ≥12.8". If you see it, re-download the latest llama.cpp in the "Runtime Environment" tab (its bundled cudart satisfies the floor) or install a CUDA 13 series runtime.

The GPU card on the "System Info" tab is itself capability-gated: Windows / Linux detect GPUs (on Linux, AMD / Intel cards are listed too, without VRAM sampling), macOS Apple Silicon shows the Apple GPU with a "Metal unified memory" badge (no VRAM bar), and Android shows no GPU card at all.
