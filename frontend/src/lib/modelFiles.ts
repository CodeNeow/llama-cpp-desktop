/**
 * 模型文件纯函数工具库：排序与量化名识别。
 * 抽自 Downloads.vue 原有内联实现，供 ModelDetail 与 Downloads 复用。
 */

/**
 * 按文件大小降序排列；size 缺失或 <=0 时以 0 兜底（排在同大小文件之后）。
 * @typeParam T 含 filename 与可选 size 属性的对象（如 HFFile）
 */
export function sortModelFiles<T extends { filename: string; size?: number }>(files: T[]): T[] {
  return [...files].sort((a, b) => (b.size || 0) - (a.size || 0))
}

/**
 * 从文件名中识别量化类型（大小写不敏感）。
 * 返回量化名（保持原始大小写表中的写法），无识别结果时返回空串。
 */
export function guessQuant(filename: string): string {
  const name = filename.toLowerCase()
  // 注意顺序：BF16 必须在 F16 之前，避免 "bf16" 先被 "f16" 的 substring 命中。
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
