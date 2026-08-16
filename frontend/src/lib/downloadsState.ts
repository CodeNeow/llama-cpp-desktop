/**
 * 下载页搜索状态共享模块。
 *
 * 将原本定义在 Downloads.vue 组件内的搜索相关状态提升到模块级，
 * 使组件卸载（如进入模型详情页再返回）后上下文不丢失——输入词、结果
 * 列表、模型大小缓存原样呈现，无需重新请求。
 */

import { ref, reactive } from 'vue'

/** 当前搜索输入词 */
export const searchQuery = ref('')

/** 是否已执行过搜索（用于空状态显示） */
export const searched = ref(false)

/** 搜索结果列表 */
export const searchResults = ref<HFResult[]>([])

/** 模型最大 GGUF 文件大小缓存（字节），0 表示已尝试查询但无结果或失败 */
export const modelSizes = reactive<Record<string, number>>({})

/** 搜索结果条目类型（与 Downloads.vue 中原来一致） */
export interface HFResult {
  modelId: string
  author: string
  downloads?: number
  likes?: number
  pipelineTag?: string
  tags?: string[]
}
