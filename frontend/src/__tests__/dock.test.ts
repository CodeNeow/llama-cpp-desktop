import { describe, it, expect } from 'vitest'
import { activeLlamaCppDownload, activeModelTasks, activeUpdateDownload, dockShowsCompactSkin, homeResidentShowsUnload, residentReleaseMode, shouldShowDock } from '../lib/dock'
import { LLAMA_CPP_DOWNLOAD_STATUSES, MODEL_TASK_STATUSES, UPDATE_DOWNLOAD_STATUSES } from '../lib/downloadStatus'
import type { LlamaCppDownloadStatus, ModelTaskStatus, UpdateDownloadStatus } from '../lib/downloadStatus'

// Tables keyed by the canonical union types make an unlisted new backend status a compile
// error (vue-tsc fails on the missing Record key), and iterating the canonical lists makes
// a wrongly-listed status a runtime test failure.

describe('activeLlamaCppDownload', () => {
  // fetching / downloading / paused / extracting / error count; idle / done hidden.
  const llamaActiveTable: Record<LlamaCppDownloadStatus, boolean> = {
    idle: false,
    fetching: true,
    downloading: true,
    paused: true,
    extracting: true,
    done: false,
    error: true,
  }

  it('maps every canonical llama.cpp download status', () => {
    for (const status of LLAMA_CPP_DOWNLOAD_STATUSES) {
      expect(activeLlamaCppDownload(status), `status=${status}`).toBe(llamaActiveTable[status])
    }
  })
})

describe('activeModelTasks', () => {
  // queued / fetching / downloading / paused / extracting are shown;
  // done / error / cancelled are hidden.
  const taskShownTable: Record<ModelTaskStatus, boolean> = {
    queued: true,
    fetching: true,
    downloading: true,
    paused: true,
    extracting: true,
    done: false,
    error: false,
    cancelled: false,
  }

  it('keeps or filters each canonical model task status (single-task array)', () => {
    for (const status of MODEL_TASK_STATUSES) {
      const out = activeModelTasks([{ status }])
      if (taskShownTable[status]) {
        expect(out, `status=${status} should be kept`).toEqual([{ status }])
      } else {
        expect(out, `status=${status} should be filtered out`).toEqual([])
      }
    }
  })

  const tasks = [
    { id: '1', status: 'queued' },
    { id: '2', status: 'downloading' },
    { id: '3', status: 'paused' },
    { id: '4', status: 'done' },
    { id: '5', status: 'error' },
    { id: '6', status: 'cancelled' },
  ]

  it('filter active tasks (queued/fetching/downloading/paused/extracting) from a mixed list', () => {
    const out = activeModelTasks(tasks)
    expect(out.map(t => t.id)).toEqual(['1', '2', '3'])
  })

  it('empty array returns empty array', () => {
    expect(activeModelTasks([])).toEqual([])
  })
})

describe('activeUpdateDownload', () => {
  // downloading / installing / done / error count; idle hidden.
  const updateActiveTable: Record<UpdateDownloadStatus, boolean> = {
    idle: false,
    downloading: true,
    installing: true,
    done: true,
    error: true,
  }

  it('maps every canonical update download status (outcome stays visible after backgrounding)', () => {
    for (const status of UPDATE_DOWNLOAD_STATUSES) {
      expect(activeUpdateDownload(status), `status=${status}`).toBe(updateActiveTable[status])
    }
  })

  it('empty/unknown return false', () => {
    expect(activeUpdateDownload('')).toBe(false)
    // statuses belonging to other download kinds must not leak in
    expect(activeUpdateDownload('paused')).toBe(false)
    expect(activeUpdateDownload('extracting')).toBe(false)
  })
})

describe('shouldShowDock', () => {
  it('do not show when all zero', () => {
    expect(shouldShowDock(false, 0, 0, false)).toBe(false)
  })

  it('show when llamaActive is true', () => {
    expect(shouldShowDock(true, 0, 0, false)).toBe(true)
  })

  it('show when activeTaskCount > 0', () => {
    expect(shouldShowDock(false, 1, 0, false)).toBe(true)
  })

  it('show when loadedCount > 0', () => {
    expect(shouldShowDock(false, 0, 1, false)).toBe(true)
  })

  it('show when only updateActive is true', () => {
    expect(shouldShowDock(false, 0, 0, true)).toBe(true)
  })
})

describe('residentReleaseMode', () => {
  it('stops the service on Android (direct mode has no per-model unload route)', () => {
    expect(residentReleaseMode(true)).toBe('stop-server')
  })

  it('unloads through the router on desktop platforms', () => {
    expect(residentReleaseMode(false)).toBe('router-unload')
  })
})

describe('homeResidentShowsUnload', () => {
  it('Android shows status only: no unload button on the Home resident card', () => {
    expect(homeResidentShowsUnload(true)).toBe(false)
  })

  it('desktop platforms keep the unload action', () => {
    expect(homeResidentShowsUnload(false)).toBe(true)
  })
})

describe('dockShowsCompactSkin', () => {
  // Cases keyed to the tier classifier (lib/platform.ts): isTablet covers the
  // portrait tablet band 768..1099, so one (isMobile, isTablet) pair covers
  // each draft frame.
  it('phone tier renders the capsule skin (frame ⑲)', () => {
    expect(dockShowsCompactSkin(true, false)).toBe(true)
  })

  it('tablet tier (portrait band) renders the capsule skin (frame ⑲)', () => {
    expect(dockShowsCompactSkin(false, true)).toBe(true)
  })

  it('desktop tier keeps the segments pill unchanged', () => {
    expect(dockShowsCompactSkin(false, false)).toBe(false)
  })
})
