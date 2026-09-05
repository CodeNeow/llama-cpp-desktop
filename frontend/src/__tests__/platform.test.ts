import { describe, it, expect } from 'vitest'
import {
  parseOs,
  buildPlatformState,
  usePlatform,
  setPlatform,
  showTraySetting,
  showApiRouteSetting,
  showServingGpuSetting,
  updateSectionMode,
  showUpdateCheckActions,
  showGpuCards,
  showCudaCompat,
  showCudaRuntimeComponent,
  accelBuildKey,
  showMultiGpuPanel,
  showGpuOffloadParam,
  loadModeOptions,
  type OsId,
  type PlatformState,
} from '../lib/platform'
import { MOBILE_MAX, TABLET_MAX } from '../lib/layout'

describe('parseOs', () => {
  it.each<[string | null | undefined, OsId]>([
    ['windows', 'windows'],
    ['linux', 'linux'],
    ['darwin', 'darwin'],
    ['android', 'android'],
    ['ios', 'ios'],
    ['', 'other'],
    [null, 'other'],
    [undefined, 'other'],
    ['freebsd', 'other'],
    ['Windows', 'other'],
  ])('maps %j to %j', (raw, expected) => {
    expect(parseOs(raw)).toBe(expected)
  })
})

describe('buildPlatformState viewport tier boundaries', () => {
  it.each<[number, boolean, boolean, boolean]>([
    [MOBILE_MAX, true, false, false], // 767: mobile tier (inclusive)
    [MOBILE_MAX + 1, false, true, false], // 768: tablet tier starts
    [TABLET_MAX, false, true, false], // 1099: tablet tier (inclusive)
    [TABLET_MAX + 1, false, false, true], // 1100: desktop tier starts
    [1920, false, false, true],
  ])(
    'desktop OS at width %i -> mobile=%s tablet=%s desktop=%s',
    (width, isMobile, isTablet, isDesktop) => {
      const state = buildPlatformState('windows', width)
      expect(state.isMobile).toBe(isMobile)
      expect(state.isTablet).toBe(isTablet)
      expect(state.isDesktop).toBe(isDesktop)
    },
  )
})

describe('buildPlatformState tier classification is width-driven only', () => {
  // The OS never bends the tier: an Android tablet must get the tablet layout
  // and an Android phone the phone layout, exactly like the same widths on a
  // desktop OS (the old "android/ios forces isMobile at any width" rule is
  // gone — see the companion capability tests for the OS-scoped flags).
  it('android phone width (<= MOBILE_MAX) lands in the mobile tier', () => {
    for (const width of [320, 390, 500, MOBILE_MAX]) {
      const state = buildPlatformState('android', width)
      expect(state.isMobile).toBe(true)
      expect(state.isTablet).toBe(false)
      expect(state.isDesktop).toBe(false)
      expect(state.isAndroid).toBe(true)
    }
  })

  it('android tablet width (MOBILE_MAX+1..TABLET_MAX) lands in the tablet tier', () => {
    for (const width of [MOBILE_MAX + 1, 800, 900, TABLET_MAX]) {
      const state = buildPlatformState('android', width)
      expect(state.isMobile).toBe(false)
      expect(state.isTablet).toBe(true)
      expect(state.isDesktop).toBe(false)
    }
  })

  it('android desktop-tier width (> TABLET_MAX) lands in the desktop tier', () => {
    for (const width of [TABLET_MAX + 1, 1280, 1920]) {
      const state = buildPlatformState('android', width)
      expect(state.isMobile).toBe(false)
      expect(state.isTablet).toBe(false)
      expect(state.isDesktop).toBe(true)
    }
  })

  it('ios follows the same width-driven tiers as android', () => {
    expect(buildPlatformState('ios', 390).isMobile).toBe(true)
    expect(buildPlatformState('ios', 900).isTablet).toBe(true)
    expect(buildPlatformState('ios', 1920).isDesktop).toBe(true)
    expect(buildPlatformState('ios', 900).isIOS).toBe(true)
  })

  it('desktop OS in a small window lands in the mobile tier via the same width rule', () => {
    // Semantics note: the tier only shapes LAYOUT. The frameless title bar is
    // a separate OS-scoped capability — a desktop OS keeps it at every width
    // (see the capability matrix below), so "mobile tier" here never means
    // "no title bar" for windows/linux/darwin.
    const state = buildPlatformState('windows', 375)
    expect(state.isMobile).toBe(true)
    expect(state.isTablet).toBe(false)
    expect(state.isDesktop).toBe(false)
    expect(state.isAndroid).toBe(false)
    expect(state.isIOS).toBe(false)
  })
})

describe('buildPlatformState width-only rule covers Android tablets', () => {
  // Android tablets get NO dedicated tier: App.vue switches the viewport meta
  // upstream of this classifier — tablet PORTRAIT forces a 430px layout
  // viewport (phone layout, upscaled), tablet LANDSCAPE keeps the default
  // device-width meta (wide viewport). This classifier therefore stays purely
  // width-driven: an Android width > TABLET_MAX is the desktop tier (the
  // landscape meta case), and the 768..1099 band keeps serving desktop-OS
  // windows resized into it (lib/layout viewportMetaContent covers the meta
  // matrix).
  it('android width > TABLET_MAX classifies as desktop (landscape tablet case)', () => {
    const state = buildPlatformState('android', 1280)
    expect(state.isTablet).toBe(false)
    expect(state.isDesktop).toBe(true)
    expect(state.isAndroid).toBe(true)
  })

  it('android tablet band (768..1099) classifies as tablet (portrait meta gives width 430 → mobile)', () => {
    // Under the scaled 430px meta the width never reaches this band on a real
    // tablet; the band exists for desktop-OS windows resized into it.
    const state = buildPlatformState('android', 900)
    expect(state.isTablet).toBe(true)
  })

  it('desktop OSes keep the same width-driven classification at any size', () => {
    const landscape = buildPlatformState('windows', 1280)
    expect(landscape.isDesktop).toBe(true)
    expect(landscape.isTablet).toBe(false)

    const narrowTablet = buildPlatformState('windows', 900)
    expect(narrowTablet.isTablet).toBe(true)
  })
})

describe('buildPlatformState capability matrix', () => {
  it.each<[OsId, number, boolean, boolean]>([
    ['windows', 1920, true, true], // desktop windows: tray + frameless titlebar
    ['windows', MOBILE_MAX, true, true], // both capabilities are OS-scoped: a narrow frameless window still needs its titlebar
    ['darwin', 1920, true, true], // macOS desktop: NSStatusItem tray + frameless ok
    ['linux', 1920, false, true], // linux desktop: tray hidden (DE-dependent DBUS), frameless ok
    ['android', 390, false, false], // phone: native chrome, no custom titlebar
    ['android', 900, false, false], // tablet tier: still no window chrome on the OS
    ['android', 1920, false, false], // desktop-tier width changes nothing: the flag is OS-scoped
    ['ios', 900, false, false],
    ['other', 1920, false, false], // unknown OS degrades to no fake titlebar
  ])('%s @ %i -> supportsTray=%s supportsFramelessTitlebar=%s', (os, width, tray, titlebar) => {
    const state = buildPlatformState(os, width)
    expect(state.supportsTray).toBe(tray)
    expect(state.supportsFramelessTitlebar).toBe(titlebar)
  })
})

describe('buildPlatformState arch field', () => {
  it('carries the backend arch when given and defaults to unknown ("")', () => {
    expect(buildPlatformState('darwin', 1920, 'arm64').arch).toBe('arm64')
    expect(buildPlatformState('windows', 1920, 'amd64').arch).toBe('amd64')
    expect(buildPlatformState('linux', 1920).arch).toBe('')
  })
})

describe('hardware-capability render gates', () => {
  // Backend facts these encode (core/sysinfo.go + release asset matrix):
  // GPU probes: windows = nvidia-smi; linux = nvidia-smi + PCI display
  // controllers (vulkan build accelerates AMD/Intel too); darwin = one Apple
  // GPU entry on arm64 (embedded Metal), none on the CPU-only x64 release;
  // android probes unsupported (GPUs always empty). cudart asset =
  // windows-only; llama.cpp builds: windows CPU/CUDA, linux Vulkan, macOS
  // Metal (arm64) / CPU-only (x64), android CPU arm64.
  it.each<[OsId, string, boolean]>([
    ['windows', '', true],
    ['linux', '', true],
    ['darwin', 'arm64', true], // Apple Silicon: one Apple GPU entry (Metal)
    ['darwin', 'amd64', false], // x64 release is CPU-only: no probe results
    ['darwin', '', false], // unknown arch degrades to hiding (safe fallback)
    ['android', '', false], // probes unsupported
    ['ios', '', false],
    ['other', '', false],
  ])('showGpuCards: %s arch=%j -> %s', (os, arch, expected) => {
    expect(showGpuCards(buildPlatformState(os, 1920, arch))).toBe(expected)
  })

  it.each<[OsId, number[], boolean]>([
    ['windows', [86], true], // NVIDIA GPU with compute capability
    ['windows', [0], false], // GPU present but no compute capability
    ['windows', [], false], // no GPU at all
    ['linux', [86], false], // Vulkan-only: CUDA compat is meaningless
    ['darwin', [86], false],
    ['android', [86], false],
  ])('showCudaCompat: %s with compute caps %j -> %s', (os, caps, expected) => {
    const gpus = caps.map((computeCapability) => ({ computeCapability }))
    expect(showCudaCompat(buildPlatformState(os, 1920), gpus)).toBe(expected)
  })

  it.each<[OsId, boolean]>([
    ['windows', true],
    ['linux', false], // no cudart step in the vulkan-only release
    ['darwin', false],
    ['android', false],
    ['other', false],
  ])('showCudaRuntimeComponent: %s -> %s', (os, expected) => {
    expect(showCudaRuntimeComponent(buildPlatformState(os, 1920))).toBe(expected)
  })

  it.each<[OsId, 'windows' | 'linux' | 'darwin' | 'cpu']>([
    ['windows', 'windows'],
    ['linux', 'linux'],
    ['darwin', 'darwin'],
    ['android', 'cpu'],
    ['ios', 'cpu'],
    ['other', 'cpu'],
  ])('accelBuildKey: %s -> %j', (os, expected) => {
    expect(accelBuildKey(buildPlatformState(os, 1920))).toBe(expected)
  })

  it.each<[OsId, boolean]>([
    ['windows', true],
    ['linux', true],
    ['darwin', false], // Metal: single GPU, nothing to split
    ['android', false], // CPU-only
    ['ios', false],
  ])('showMultiGpuPanel: %s -> %s', (os, expected) => {
    expect(showMultiGpuPanel(buildPlatformState(os, 1920))).toBe(expected)
  })

  it.each<[OsId, string, boolean]>([
    ['windows', '', true],
    ['windows', 'arm64', true],
    ['linux', '', true],
    ['darwin', 'arm64', true], // macOS arm64 release embeds Metal (-ngl works)
    ['darwin', 'amd64', false], // macOS x64 release is CPU-only
    ['darwin', '', false], // unknown arch degrades to hiding (safe fallback)
    ['android', 'arm64', false], // CPU-only: offload selector must not render
    ['ios', 'arm64', false],
  ])('showGpuOffloadParam: %s arch=%j -> %s', (os, arch, expected) => {
    expect(showGpuOffloadParam(buildPlatformState(os, 1920, arch))).toBe(expected)
  })
})

describe('loadModeOptions', () => {
  // Full vocabulary (default mmap / mmap / mlock / mmap+mlock / none / dio)
  // everywhere, except 'dio' which is not a meaningful DirectIO option on
  // android and darwin.
  const allValues = ['', 'mmap', 'mlock', 'mmap+mlock', 'none', 'dio']

  it.each<[OsId]>([['windows'], ['linux'], ['ios'], ['other']])(
    '%s keeps the full load-mode list including dio',
    (os) => {
      const opts = loadModeOptions(buildPlatformState(os, 1920))
      expect(opts.map((o) => o.value)).toEqual(allValues)
    },
  )

  it.each<[OsId]>([['android'], ['darwin']])('%s excludes dio', (os) => {
    const opts = loadModeOptions(buildPlatformState(os, 1920))
    expect(opts.map((o) => o.value)).toEqual(allValues.filter((v) => v !== 'dio'))
    expect(opts.every((o) => o.label.length > 0)).toBe(true)
  })
})

describe('OS-scoped setting gates (Settings.vue visibility)', () => {
  // Gates follow each feature's own capability matrix: tray is windows+darwin
  // (NSStatusItem; linux DBUS is too desktop-environment-dependent), headless
  // relaunch and CUDA device pinning stay Windows-only, the native
  // check-for-updates action renders on Windows (NSIS installer) and Android
  // (system PackageInstaller for the in-app downloaded APK) — linux/darwin/ios/other
  // short-circuit the update probe to "no update" so the action cluster would
  // be a no-op there (Android's settings row shows the GitHub Releases link as
  // a supplementary footnote, not as the primary discovery path); its
  // automatic update check + in-app modal are independent of these gates.
  // None of the gates are viewport-scoped.
  it.each<[OsId, boolean, boolean, boolean, boolean]>([
    ['windows', true, true, true, true],
    ['darwin', true, false, false, false],
    ['linux', false, false, false, false],
    ['android', false, false, false, true],
    ['ios', false, false, false, false],
    ['other', false, false, false, false],
  ])(
    '%s -> tray=%s apiRoute=%s gpu=%s showCheckActions=%s',
    (os, tray, apiRoute, gpu, showCheck) => {
      for (const width of [390, MOBILE_MAX, TABLET_MAX, TABLET_MAX + 1, 1920]) {
        const state: PlatformState = buildPlatformState(os, width)
        expect(showTraySetting(state)).toBe(tray)
        expect(showApiRouteSetting(state)).toBe(apiRoute)
        expect(showServingGpuSetting(state)).toBe(gpu)
        expect(showUpdateCheckActions(state)).toBe(showCheck)
      }
    },
  )

  it.each<[OsId, 'native' | 'link']>([
    ['windows', 'native'],
    ['darwin', 'link'],
    ['linux', 'link'],
    ['android', 'link'],
    ['ios', 'link'],
    ['other', 'link'],
  ])('%s -> updateSectionMode=%s', (os, mode) => {
    for (const width of [390, MOBILE_MAX, TABLET_MAX, TABLET_MAX + 1, 1920]) {
      expect(updateSectionMode(buildPlatformState(os, width))).toBe(mode)
    }
  })
})

describe('reactive platform singleton', () => {
  it('starts from a desktop-windows default before any wiring', () => {
    const platform = usePlatform()
    expect(platform.value.os).toBe('windows')
    expect(platform.value.isDesktop).toBe(true)
    expect(platform.value.supportsTray).toBe(true)
  })

  it('reflects setPlatform updates in the shared readonly ref', () => {
    const android: PlatformState = buildPlatformState('android', 412)
    setPlatform(android)
    expect(usePlatform().value.isMobile).toBe(true)
    expect(usePlatform().value.supportsFramelessTitlebar).toBe(false)

    // Restore the default so other suites observe the pristine singleton.
    setPlatform(buildPlatformState('windows', Number.POSITIVE_INFINITY))
    expect(usePlatform().value.isDesktop).toBe(true)
  })
})
