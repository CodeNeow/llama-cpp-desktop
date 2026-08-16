/**
 * Downloads page shared search-state module.
 *
 * Hoists the search-related state previously defined inside the Downloads.vue
 * component to module level, so unmounting the component (e.g. entering the model
 * detail page and coming back) does not lose context — the input, result list and
 * model-size cache re-render as-is without new requests.
 */

import { ref, reactive } from 'vue'

/** Current search input */
export const searchQuery = ref('')

/** Whether a search has been performed (drives the empty-state display) */
export const searched = ref(false)

/** Search result list */
export const searchResults = ref<HFResult[]>([])

/** Cache of each model's largest GGUF file size (bytes); 0 means a query was attempted but yielded nothing or failed */
export const modelSizes = reactive<Record<string, number>>({})

/** Search result entry type (unchanged from the original in Downloads.vue) */
export interface HFResult {
  modelId: string
  author: string
  downloads?: number
  likes?: number
  pipelineTag?: string
  tags?: string[]
}
