/**
 * Auto-tune summary helpers: build the interpolation params for the
 * tune-success toast shown after TuneModelConfig applies the computed
 * parameters to a model's persisted config, and pick the toast key — CPU-only
 * plans (GPULayers "0", e.g. the Android build) use the variant without the
 * GPU segment so the message never renders "GPU layers 0".
 */

/** Interpolation params for t('models.tuned') / t('models.tunedCpu'). */
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

/**
 * Pick the tune-success i18n key from the tuned GPU layer setting: the literal
 * "0" marks a CPU-only plan (no offload), whose toast drops the GPU segment;
 * every other value ("all", a layer count, "auto") keeps the full key.
 */
export function tunedToastKey(gpuLayers: string): 'models.tuned' | 'models.tunedCpu' {
  return gpuLayers === '0' ? 'models.tunedCpu' : 'models.tuned'
}
