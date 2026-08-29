/**
 * Auto-tune summary helper: builds the interpolation params for the
 * t('models.tuned') success message shown after TuneModelConfig applies
 * the computed parameters to a model's persisted config.
 */

/** Interpolation params for t('models.tuned'). */
// Type alias (not interface): t() takes Record<string, string | number> and
// only object-literal type aliases get an implicit index signature.
export type TuneSummaryParams = {
  gpu: string
  ctx: number
  cache: string
  threads: number
}

/**
 * Build the t('models.tuned') interpolation params from a tuned config.
 *
 * The parameter is intentionally a structural type holding exactly the fields
 * this helper reads: lib/ modules must not import types from views/ (that
 * would invert the dependency direction), and the ModelConfig interface in
 * ModelSettings.vue satisfies this shape structurally.
 */
export function tunedSummaryParams(cfg: {
  gpuLayers: string
  ctxSize: number
  cacheTypeK: string
  threads: number
}): TuneSummaryParams {
  return {
    gpu: cfg.gpuLayers,
    ctx: cfg.ctxSize,
    // Empty cacheTypeK means the backend default f16
    cache: cfg.cacheTypeK || 'f16',
    threads: cfg.threads,
  }
}
