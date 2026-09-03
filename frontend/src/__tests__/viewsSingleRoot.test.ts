// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

// Route views render inside App.vue's <transition mode="out-in">, which
// requires a SINGLE-root child. In dev builds the SFC compiler keeps
// top-level HTML comments as vnodes, so a template like
//   <template><!-- doc comment --><div>...</div></template>
// compiles to a two-node fragment. A fragment-rooted child breaks the
// out-in leave/enter handoff (BaseTransition's state.isLeaving never
// resets) and every navigation after visiting that page renders an empty
// placeholder — exactly the "blank after leaving Chat" defect (fixed by
// moving Chat's lane-mirroring comment inside its root div).
//
// The contract only binds components rendered DIRECTLY under App.vue's
// transition — the views imported by router/index.ts. Views nested in a
// shell's inner router-view (Home tabs, Models tabs) render under
// <keep-alive> without a transition, so top-level comments are harmless
// there and out of scope. The tested file set is derived from the router's
// imports so newly routed views are covered automatically.

const here = dirname(fileURLToPath(import.meta.url))
const viewsDir = join(here, '..', 'views')

/** View file names routed as TOP-LEVEL routes (the transition children).
 * router/index.ts formats top-level route properties at 4-space indent, so
 * `component: X` on a 4-space indent is a transition child; deeper indents
 * are child routes mounted inside a shell's <keep-alive> router-view, where
 * the contract does not apply. */
function routedViewFiles(): string[] {
  const routerSrc = readFileSync(join(here, '..', 'router', 'index.ts'), 'utf8')
  const names = [...routerSrc.matchAll(/^ {4}component: ([A-Za-z0-9]+),?\s*$/gm)].map((m) => m[1])
  expect(names.length, 'router/index.ts must declare top-level route components').toBeGreaterThan(0)
  return [...new Set(names)].sort()
}

/** Extract the raw template body of an SFC file. */
function templateBody(src: string): string {
  const match = src.match(/<template[^>]*>([\s\S]*)<\/template>/)
  expect(match, 'SFC must contain a <template> block').toBeTruthy()
  return match![1]
}

/**
 * Return the template's top-level nodes. A tiny stack-based tag scanner:
 * comments, elements (self-closing and void tags handled), and tag-end
 * detection that survives quoted attributes containing '>'.
 */
function topLevelNodes(tpl: string): Array<{ kind: 'comment' | 'element'; name: string }> {
  const nodes: Array<{ kind: 'comment' | 'element'; name: string }> = []
  const voidTags = new Set(['br', 'img', 'input', 'hr', 'meta', 'link'])
  let depth = 0
  let i = 0
  while (i < tpl.length) {
    const lt = tpl.indexOf('<', i)
    if (lt < 0) break
    if (tpl.startsWith('<!--', lt)) {
      const end = tpl.indexOf('-->', lt)
      expect(end, 'unterminated comment in template').toBeGreaterThanOrEqual(0)
      if (depth === 0) nodes.push({ kind: 'comment', name: '#comment' })
      i = end + 3
      continue
    }
    const m = /^<\/?([a-zA-Z][a-zA-Z0-9-]*)/.exec(tpl.slice(lt))
    if (!m) {
      i = lt + 1
      continue
    }
    const closing = tpl[lt + 1] === '/'
    const name = m[1]
    if (closing) {
      depth--
      i = lt + 1
      continue
    }
    if (depth === 0) nodes.push({ kind: 'element', name })
    let j = lt
    let quote: string | null = null
    while (j < tpl.length) {
      const c = tpl[j]
      if (quote) {
        if (c === quote) quote = null
      } else if (c === '"' || c === "'") {
        quote = c
      } else if (c === '>') {
        break
      }
      j++
    }
    const selfClosing = tpl[j - 1] === '/'
    if (!selfClosing && !voidTags.has(name)) depth++
    i = j + 1
  }
  return nodes
}

describe('routed views keep a single template root (transition out-in contract)', () => {
  it('every top-level route view has exactly one root node and it is an element', () => {
    // Cross-check the scan set against the real views directory so a renamed
    // router import cannot silently shrink coverage.
    const onDisk = readdirSync(viewsDir).filter((f) => f.endsWith('.vue')).map((f) => f.replace(/\.vue$/, ''))
    const routed = routedViewFiles()
    for (const name of routed) {
      expect(onDisk, `${name} routed but missing from src/views/`).toContain(name)
    }
    for (const name of routed) {
      const src = readFileSync(join(viewsDir, name + '.vue'), 'utf8')
      const nodes = topLevelNodes(templateBody(src))
      const summary = nodes.map((n) => n.kind + ':' + n.name).join(', ')
      expect(nodes.length, `${name}.vue: top-level nodes [${summary}] — exactly one root element required under <transition mode="out-in">`).toBe(1)
      expect(nodes[0].kind, `${name}.vue: the single root must be an element, not a comment`).toBe('element')
    }
  })

  it('Chat.vue keeps its lane-mirroring comment inside the root div', () => {
    // Pins the historical offender so the comment is not hoisted back
    // to template top level.
    const src = readFileSync(join(viewsDir, 'Chat.vue'), 'utf8')
    expect(topLevelNodes(templateBody(src))).toEqual([{ kind: 'element', name: 'div' }])
  })
})
