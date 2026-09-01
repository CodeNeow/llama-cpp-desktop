/**
 * Model file pure-function utilities: sorting and quantization-name detection.
 * Extracted from Downloads.vue's original inline implementation for reuse by
 * ModelDetail and Downloads.
 */

/**
 * Sort by file size descending; a missing or <=0 size falls back to 0 (sorted after files of equal size).
 * @typeParam T object with filename and optional size properties (e.g. HFFile)
 */
export function sortModelFiles<T extends { filename: string; size?: number }>(files: T[]): T[] {
  return [...files].sort((a, b) => (b.size || 0) - (a.size || 0))
}

/**
 * Detect the quantization type from a filename (case-insensitive).
 * Returns the quant name (keeping the casing used in the table above), or an empty string when nothing matches.
 */
export function guessQuant(filename: string): string {
  const name = filename.toLowerCase()
  // Order matters: BF16 must precede F16 so "bf16" is not first matched by the "f16" substring.
  const quants = [
    'Q8_K', 'Q8_0', 'Q6_K', 'Q5_K_M', 'Q5_K_S', 'Q5_1', 'Q5_0',
    'Q4_K_M', 'Q4_K_S', 'Q4_1', 'Q4_0', 'Q3_K_L', 'Q3_K_M', 'Q3_K_S',
    'Q2_K', 'IQ4_NL', 'IQ4_XS', 'IQ3_M', 'IQ3_S', 'IQ3_XXS', 'IQ2_XS', 'IQ2_XXS',
    'BF16', 'F16', 'F32',
  ]
  for (const q of quants) {
    if (name.includes(q.toLowerCase())) return q
  }
  return ''
}

/**
 * Minimal local-model facts the loaded-model size lookup needs.
 * Mirrors the fields of the GetModels binding payload used here.
 */
export interface LocalModelFact {
  name: string
  sizeHuman: string
}

/**
 * Human size of the local model a loaded router id was loaded from, for the
 * dock's in-memory status line ("● 已加载 · 2.3 GB"). Matching mirrors the
 * chat page's model identity rules: the router id is the model file name (or
 * its sanitized alias), so an exact name match wins and a contained-name
 * match is the fallback. Returns '' when nothing matches or the size is
 * unknown — callers drop the segment instead of inventing one.
 */
export function matchLoadedModelSize(id: string, models: LocalModelFact[]): string {
  if (!id) return ''
  const exact = models.find((m) => m.name === id)
  if (exact?.sizeHuman) return exact.sizeHuman
  const contained = models.find((m) => id.includes(m.name) && m.sizeHuman)
  return contained?.sizeHuman ?? ''
}
