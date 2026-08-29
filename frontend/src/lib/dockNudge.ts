/**
 * Tiny cross-component nudge bus for the TaskDock.
 *
 * TaskDock polls backend state every 1s, but chat-driven model loads/unloads
 * and dock-row unload actions want immediate visual feedback. Writers call
 * nudgeDock(); the dock watches the counter and re-polls right away. A plain
 * module-level ref keeps the implementation observable in unit tests with
 * standard Vue reactivity (no custom event wiring needed).
 */

import { ref, readonly } from 'vue'
import type { Ref } from 'vue'

/** Internal nudge counter; monotonically increasing. */
const counter = ref(0)

/** Request an immediate TaskDock refresh; fire-and-forget, never throws. */
export function nudgeDock(): void {
  counter.value++
}

/** Readonly view of the nudge counter for watchers (TaskDock). */
export const dockNudgeCounter: Readonly<Ref<number>> = readonly(counter)
