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
import { MOBILE_MAX, TABLET_MAX, isTabletLandscapeViewport, viewportOrientation } from './layout'
import { t } from './i18n'

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

/** Derived platform facts computed from an OS id, the viewport width and the backend arch. */
export interface PlatformState {
  os: OsId
  /**
   * Backend architecture (Go runtime.GOARCH, e.g. 'amd64' / 'arm64'; ''
   * when unknown — e.g. before the getOS() probe resolves). Gates the
   * darwin Metal capability: only the macOS arm64 release embeds Metal,
   * the x64 release is CPU-only. An unknown arch degrades to hiding
   * Metal-gated UI (safe fallback — a capability that is not provable is
   * not offered).
   */
  arch: string
  isMobile: boolean
  isTablet: boolean
  isDesktop: boolean
  /**
   * Viewport orientation ('portrait' | 'landscape'; landscape iff width is
   * strictly greater than height — a square ties to 'portrait'). Feeds the
   * Android landscape-tablet classification; on every other platform it is
   * informational only and never bends the tier.
   */
  orientation: 'portrait' | 'landscape'
  /**
   * True exactly for Android in the landscape-tablet band (width in
   * TABLET_MAX+1..TABLET_LANDSCAPE_MAX with width > height): an Android
   * tablet rotated to landscape keeps the tablet layout instead of jumping
   * to desktop. Never true on desktop OSes (a same-sized desktop window
   * stays desktop) and never true for Android portrait >TABLET_MAX or
   * Android widths beyond TABLET_LANDSCAPE_MAX.
   */
  isTabletLandscape: boolean
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
 * Pure tier classifier (mobile / tablet / desktop; see layout.ts). Tier
 * classification is VIEWPORT-WIDTH-DRIVEN ONLY — the OS never bends the
 * tier — with ONE Android-gated extension: an Android tablet in landscape
 * orientation (see layout.ts's landscape band) classifies back into the
 * tablet tier instead of desktop, so the design draft's 1280x800 landscape
 * panel gets the tablet layout:
 *   - isMobile: viewport <= MOBILE_MAX
 *   - isTablet: MOBILE_MAX < viewport <= TABLET_MAX, OR Android inside the
 *     landscape-tablet band (TABLET_MAX < width <= TABLET_LANDSCAPE_MAX and
 *     width > height)
 *   - isDesktop: everything else
 * Explicit non-goals (byte-identical to the old width-only rule):
 *   - desktop OSes are NEVER re-classified: a 1280x800 desktop window stays
 *     desktop (the landscape band is Android-gated);
 *   - Android portrait above TABLET_MAX stays desktop (orientation tie-break:
 *     square viewports count as portrait);
 *   - Android at widths beyond TABLET_LANDSCAPE_MAX stays desktop.
 * A desktop OS in a small window therefore lands in the mobile tier too;
 * whether the custom frameless title bar renders is a separate, OS-scoped
 * capability below (the tier only shapes layout, never window chrome).
 *
 * arch is the backend architecture string (Go runtime.GOARCH); it defaults to
 * '' (unknown) and only gates the darwin Metal capability (see
 * showGpuOffloadParam / showGpuCards).
 *
 * viewportHeight defaults to Number.POSITIVE_INFINITY so every legacy 2/3-arg
 * call classifies as 'portrait' orientation — the landscape band can never
 * trigger without a real height, keeping existing behavior unchanged.
 */
export function buildPlatformState(
  os: OsId,
  viewportWidth: number,
  arch = '',
  viewportHeight: number = Number.POSITIVE_INFINITY,
): PlatformState {
  const isAndroid = os === 'android'
  const isIOS = os === 'ios'
  const orientation = viewportOrientation(viewportWidth, viewportHeight)
  const isMobile = viewportWidth <= MOBILE_MAX
  const isTablet =
    !isMobile &&
    (viewportWidth <= TABLET_MAX ||
      (os === 'android' && isTabletLandscapeViewport(viewportWidth, viewportHeight)))
  const isTabletLandscape = isTablet && os === 'android' && viewportWidth > TABLET_MAX
  const isDesktop = !isMobile && !isTablet
  return {
    os,
    arch,
    isMobile,
    isTablet,
    isDesktop,
    orientation,
    isTabletLandscape,
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

/**
 * Whether the update section renders the manual check-for-updates action
 * cluster (error / "up to date" label / check button). This is distinct from
 * {@link updateSectionMode}: the latter decides the *resource mode* (Windows
 * downloads an NSIS installer, Android installs through the system
 * PackageInstaller after the in-app modal downloads the APK — both are
 * "native" install paths, but only Windows uses the self-update installer).
 * Linux, macOS and other OSes do not render the action cluster because the
 * backend `CheckForUpdateAt` gate short-circuits to "no update" there, so a
 * manual check would always be a no-op.
 */
export function showUpdateCheckActions(state: PlatformState): boolean {
  return state.os === 'windows' || state.os === 'android'
}

/** How the update section offers new versions to the user. */
export type UpdateSectionMode = 'native' | 'link'

/**
 * Update section mode: 'native' means Windows has the in-app self-update
 * install path (the downloaded NSIS installer takes over). Android's actual
 * install path is the system PackageInstaller (triggered by the in-app modal
 * after downloading the APK), but Android remains 'link' mode here — the
 * GitHub Releases pointer stays as a supplementary entry. The separate
 * {@link showUpdateCheckActions} gate controls whether the manual
 * check-for-updates action cluster renders; on Android it does, while
 * linux/darwin/other platforms render the hint + link only because the
 * backend `CheckForUpdateAt` gate short-circuits to "no update" there.
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
//   - GPU probes: windows = nvidia-smi; linux = nvidia-smi + PCI display
//     controllers (the vulkan build accelerates AMD/Intel too); darwin = one
//     Apple GPU entry on arm64 (embedded Metal), none on the CPU-only x64
//     release; android probes are unsupported (gpuProbesUnsupported → GPUs
//     always empty).
//   - llama.cpp assets: windows = CPU/CUDA builds + separate cudart runtime;
//     linux = vulkan-only (no CUDA variant, no cudart step); macOS = Metal
//     (arm64) / CPU-only (x64); android = CPU-only arm64.

/** Platforms where a real GPU probe exists and GPU cards are meaningful.
 * Windows (nvidia-smi) and Linux (nvidia-smi + PCI display controllers)
 * always probe; macOS probes on Apple Silicon only (arch-gated — the x64
 * release is CPU-only and reports no GPUs), with an unknown arch degrading
 * to hiding the card (safe fallback). */
export function showGpuCards(state: PlatformState): boolean {
  if (state.os === 'windows' || state.os === 'linux') return true
  return state.os === 'darwin' && state.arch === 'arm64'
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
 * a possible GPU — Windows (CUDA), Linux (Vulkan, NVIDIA/AMD/Intel alike) and
 * macOS arm64 (embedded Metal). macOS x64 ships the CPU-only release and an
 * unknown arch degrades to hiding (safe fallback); android/ios are CPU-only.
 */
export function showGpuOffloadParam(state: PlatformState): boolean {
  if (state.os === 'windows' || state.os === 'linux') return true
  return state.os === 'darwin' && state.arch === 'arm64'
}

/** Minimal option shape (structurally compatible with ThemedSelect's SelectOption). */
export interface ParamOption {
  value: string
  label: string
}

/**
 * Load-mode option list for the per-model settings page: the full mmap /
 * mlock / mmap+mlock / none / dio vocabulary, minus 'dio' on platforms where
 * DirectIO is not a meaningful llama-server option (android and darwin —
 * desktop macOS/Android sandboxes gain nothing from it). Values and labels
 * are otherwise unchanged.
 */
export function loadModeOptions(state: PlatformState): ParamOption[] {
  const options: ParamOption[] = [
    { value: '', label: t('modelSettings.loadDefaultMmap') },
    { value: 'mmap', label: t('modelSettings.loadMmap') },
    { value: 'mlock', label: t('modelSettings.loadMlock') },
    { value: 'mmap+mlock', label: t('modelSettings.loadMmapMlock') },
    { value: 'none', label: t('modelSettings.loadNone') },
    { value: 'dio', label: t('modelSettings.loadDio') },
  ]
  if (state.os === 'android' || state.os === 'darwin') {
    return options.filter((o) => o.value !== 'dio')
  }
  return options
}
