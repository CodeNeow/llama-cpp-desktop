/**
 * Platform classification foundation for upcoming cross-platform support
 * (Linux/macOS desktop + Android full-parity mobile later).
 *
 * Pure state module: no side effects at import time, no backend access,
 * fully testable without mocks. `buildPlatformState` is a pure classifier;
 * the module-level reactive singleton starts from a desktop-windows default
 * so current behavior is unchanged. App.vue wires it: once the backend
 * getOS() binding resolves it publishes the state via setPlatform and keeps
 * the viewport tier in sync with window resizes. The showXxxSetting /
 * updateSectionMode helpers below are the OS-scoped UI gates used by
 * Settings.vue to hide Windows-only features on other platforms.
 */

import { ref, readonly } from 'vue'
import type { Ref } from 'vue'
import { MOBILE_MAX, TABLET_MAX } from './layout'

/** Operating system identifiers, aligned with the backend getOS() strings. */
export type OsId = 'windows' | 'linux' | 'darwin' | 'android' | 'ios' | 'other'

/**
 * Map a raw getOS() backend string to an OsId. 'windows' / 'linux' /
 * 'darwin' come from Go runtime.GOOS; empty and unknown values degrade to
 * 'other' so callers never have to handle parse failures.
 */
export function parseOs(raw: string | null | undefined): OsId {
  switch (raw) {
    case 'windows':
    case 'linux':
    case 'darwin':
    case 'android':
    case 'ios':
      return raw
    default:
      return 'other'
  }
}

/** Derived platform facts computed from an OS id and the viewport width. */
export interface PlatformState {
  os: OsId
  isMobile: boolean
  isTablet: boolean
  isDesktop: boolean
  isAndroid: boolean
  isIOS: boolean
  /**
   * System tray integration: reliable on Windows (taskbar) and macOS
   * (NSStatusItem); Linux is excluded — DBUS StatusNotifierItem depends on the
   * desktop environment (GNOME shows nothing by default).
   */
  supportsTray: boolean
  /**
   * Whether the custom frameless title bar should render. OS-scoped (constant
   * across viewport widths): true only for desktop OSes, which run as frameless
   * windows that need the drag/window-control band; android/ios have no window
   * chrome, so the bar would waste space there.
   */
  supportsFramelessTitlebar: boolean
}

/**
 * Pure three-tier classifier (mobile / tablet / desktop; see layout.ts).
 * Tier classification is VIEWPORT-WIDTH-DRIVEN ONLY — the OS never bends the
 * tier — so an Android tablet (768..1099px) gets the roomier tablet layout
 * and an Android phone (<= MOBILE_MAX) gets the phone layout:
 *   - isMobile: viewport <= MOBILE_MAX
 *   - isTablet: MOBILE_MAX < viewport <= TABLET_MAX
 *   - isDesktop: viewport > TABLET_MAX
 * A desktop OS in a small window therefore lands in the mobile tier too;
 * whether the custom frameless title bar renders is a separate, OS-scoped
 * capability below (the tier only shapes layout, never window chrome).
 */
export function buildPlatformState(os: OsId, viewportWidth: number): PlatformState {
  const isAndroid = os === 'android'
  const isIOS = os === 'ios'
  const isMobile = viewportWidth <= MOBILE_MAX
  const isTablet = !isMobile && viewportWidth <= TABLET_MAX
  const isDesktop = !isMobile && !isTablet
  return {
    os,
    isMobile,
    isTablet,
    isDesktop,
    isAndroid,
    isIOS,
    // Tray capability: windows (taskbar) and darwin (NSStatusItem) have
    // reliable, first-class tray integration in the v3 runtime; linux is kept
    // hidden for now — the DBUS StatusNotifierItem protocol depends heavily on
    // the desktop environment (GNOME shows nothing by default), so the setting
    // would promise something the platform may not deliver.
    supportsTray: os === 'windows' || os === 'darwin',
    // OS-scoped (never viewport-scoped): the fake title bar exists to drag /
    // close / maximize a FRAMELESS desktop window, so it is wanted exactly
    // when the app runs as one. Android/iOS have no window chrome at all —
    // the bar would only waste space — and unknown OSes degrade to hiding it.
    // A desktop OS keeps the bar at every viewport width: shrinking a frameless
    // window to phone width still leaves it a window that needs controls.
    supportsFramelessTitlebar:
      os === 'windows' || os === 'linux' || os === 'darwin',
  }
}

/** Desktop-windows default keeps current behavior until setPlatform is wired. */
const state = ref(buildPlatformState('windows', Number.POSITIVE_INFINITY))

/** Readonly view exposed to components; writes go through setPlatform only. */
const platform: Readonly<Ref<PlatformState>> = readonly(state)

/**
 * Publish a new platform state (OS detection + viewport observation).
 * Called by App.vue after getOS() resolves and on window resizes.
 */
export function setPlatform(next: PlatformState): void {
  state.value = next
}

/** Readonly reactive view of the current platform state for components. */
export function usePlatform(): Readonly<Ref<PlatformState>> {
  return platform
}

// ─── OS-scoped setting gates ─────────────────────────────────────────────────
// These decide whether Settings-page items render at all. They are OS-scoped
// by design (NOT viewport-scoped): the features are bound to OS-level backend
// behavior, so they stay constant across window resizes. Kept as separate
// named helpers (not one flag) so each feature can follow its own capability
// matrix: tray is windows+darwin today, api-route / serving-GPU / native
// self-update remain windows-only.

/** System-tray setting row: tray is reliable on Windows and macOS (NSStatusItem);
 * Linux stays hidden — DBUS StatusNotifierItem support is too desktop-environment-dependent. */
export function showTraySetting(state: PlatformState): boolean {
  return state.supportsTray
}

/**
 * API-route (headless) mode setting: the backend ShouldRunHeadless decision
 * and the tray-only return path are Windows-only today.
 */
export function showApiRouteSetting(state: PlatformState): boolean {
  return state.os === 'windows'
}

/**
 * Serving-GPU (推理显卡) selector: pinning llama-server to an NVIDIA device
 * via CUDA_VISIBLE_DEVICES is a Windows + NVIDIA feature today; the backend
 * cudaDeviceEnv no-ops on other platforms, so the selector is hidden there.
 */
export function showServingGpuSetting(state: PlatformState): boolean {
  return state.os === 'windows'
}

/** How the update section offers new versions to the user. */
export type UpdateSectionMode = 'native' | 'link'

/**
 * Update section mode: 'native' renders the in-app check-for-updates action
 * (self-update installer is Windows-only); 'link' renders a hint pointing at
 * the GitHub Releases page instead.
 */
export function updateSectionMode(state: PlatformState): UpdateSectionMode {
  return state.os === 'windows' ? 'native' : 'link'
}

// ─── Hardware-capability render gates ────────────────────────────────────────
// Platform parity does NOT mean showing identical UI everywhere: a section
// backed by a probe that cannot succeed on a platform must not render there at
// all (no empty states, no "N/A" cards). Like the setting gates above, these
// are OS-scoped and constant across resizes. Backend facts they encode (see
// core/sysinfo.go and the llama.cpp release asset matrix):
//   - nvidia-smi probe: windows + linux only; android probes are unsupported
//     (gpuProbesUnsupported → GPUs always empty); darwin has no GPU probe.
//   - llama.cpp assets: windows = CPU/CUDA builds + separate cudart runtime;
//     linux = vulkan-only (no CUDA variant, no cudart step); macOS = Metal;
//     android = CPU-only arm64.

/** Platforms where a real GPU probe exists and GPU cards are meaningful. */
export function showGpuCards(state: PlatformState): boolean {
  return state.os === 'windows' || state.os === 'linux'
}

/** Minimal GPU shape the CUDA-compat gate needs (GpuStaticInfo satisfies it). */
interface GpuCompatSource {
  computeCapability: number
}

/**
 * CUDA compatibility card: Windows only (Linux llama.cpp is Vulkan-only, so
 * CUDA compat is a meaningless concept there; macOS is Metal; Android is
 * CPU-only) AND only when at least one GPU reports a compute capability
 * (a > 0 value means an NVIDIA card the probe could actually classify).
 */
export function showCudaCompat(state: PlatformState, gpus: readonly GpuCompatSource[]): boolean {
  return state.os === 'windows' && gpus.some((g) => g.computeCapability > 0)
}

/**
 * CUDA runtime (cudart) component row / download step: only the Windows
 * release ships a separate cudart asset; Linux (vulkan) / macOS (Metal) /
 * Android (CPU) never have it.
 */
export function showCudaRuntimeComponent(state: PlatformState): boolean {
  return state.os === 'windows'
}

/**
 * i18n key suffix describing the platform's llama-server acceleration build,
 * shown beside the main-program component row. windows → CPU/CUDA,
 * linux → Vulkan, darwin → Metal, everything else (android/ios/unknown) → CPU.
 */
export function accelBuildKey(state: PlatformState): 'windows' | 'linux' | 'darwin' | 'cpu' {
  if (state.os === 'windows' || state.os === 'linux' || state.os === 'darwin') {
    return state.os
  }
  return 'cpu'
}

/**
 * Multi-GPU panel (split mode / tensor split / main GPU): needs multiple
 * devices to split across. Windows (CUDA) and Linux (Vulkan multi-device)
 * qualify; macOS Metal drives a single GPU and Android is CPU-only, so the
 * panel must not render there.
 */
export function showMultiGpuPanel(state: PlatformState): boolean {
  return state.os === 'windows' || state.os === 'linux'
}

/**
 * GPU-offload parameter (gpu layers, -ngl): meaningful on every platform with
 * a possible GPU — Windows (CUDA), Linux (Vulkan), macOS (Metal). Hidden on
 * android/ios where only CPU inference exists (the backend tuner already
 * degrades to CPU-only there).
 */
export function showGpuOffloadParam(state: PlatformState): boolean {
  return (
    state.os === 'windows' || state.os === 'linux' || state.os === 'darwin'
  )
}
