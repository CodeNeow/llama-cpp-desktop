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
  /** System tray integration is Windows-only today (macOS/Linux tray arrives with the v3 migration). */
  supportsTray: boolean
  /** Mobile form factors use native chrome, so no custom frameless title bar. */
  supportsFramelessTitlebar: boolean
}

/**
 * Pure three-tier classifier (mobile / tablet / desktop; see layout.ts):
 *   - isMobile: mobile OS (android/ios) at any width, or viewport <= MOBILE_MAX
 *   - isTablet: neither mobile nor desktop-tier width (> MOBILE_MAX, <= TABLET_MAX)
 *   - isDesktop: everything else
 */
export function buildPlatformState(os: OsId, viewportWidth: number): PlatformState {
  const isAndroid = os === 'android'
  const isIOS = os === 'ios'
  const isMobile = isAndroid || isIOS || viewportWidth <= MOBILE_MAX
  const isTablet = !isMobile && viewportWidth <= TABLET_MAX
  const isDesktop = !isMobile && !isTablet
  return {
    os,
    isMobile,
    isTablet,
    isDesktop,
    isAndroid,
    isIOS,
    // Windows is the only platform with tray support today; macOS/Linux
    // tray integration arrives with the v3 cross-platform migration.
    supportsTray: os === 'windows',
    supportsFramelessTitlebar: !isMobile,
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
// behavior, so they stay constant across window resizes. All Windows-only
// today, kept as separate named helpers so they can diverge per feature when
// macOS/Linux support arrives (e.g. tray on macOS first).

/** System-tray setting row: tray integration is Windows-only today. */
export function showTraySetting(state: PlatformState): boolean {
  return state.os === 'windows'
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
