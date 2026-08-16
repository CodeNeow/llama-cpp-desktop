/**
 * 系统就绪判定：llama.cpp 已安装且本地至少有一个模型。
 *
 * 两条硬条件同时满足才视为就绪：
 * 1) llama.cpp 已安装在本地（Installed = true）
 * 2) LLM-Models/ 下至少有一个模型文件（modelCount > 0）
 */

/** 系统就绪判定：llama.cpp 已安装且本地至少有一个模型 */
export function isSystemReady(llamaInstalled: boolean, modelCount: number): boolean {
  return llamaInstalled && modelCount > 0
}
