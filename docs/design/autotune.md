# 设计参考：硬件感知一键调优（auto-tune）

MyLlama 的硬件感知一键调优：读 GGUF 真实指标 + 本地硬件快照，为每个模型自动规划最优 llama-server 参数。

> 权威来源：`core/autotune.go`（策略 doc-comment + 常量 + 纯函数 `tuneModelConfig`）、
> `core/benchbw.go`（实测 RAM 带宽校准）、`core/app.go`（`TuneModelConfig` 绑定）、
> `frontend/src/views/ModelSettings.vue` + `frontend/src/lib/modelTune.ts`（前端联动）。
> 本文描述**当前工作区**实现；与代码冲突时以代码为准。
>
> 注：「serving-GPU 规划目标」（`tunePlanTarget`，按服务器配置里的 DeviceID 选卡）是另一批
> **进行中（未提交）的工作区改动**，本文按工作区现状记录；评审合并前若该特性被回退，
> 请同步删除相关段落。

## 目标与纯函数契约

读模型 GGUF 头里的真实指标（层数、GQA/MLA KV 几何、训练上下文、MoE 专家几何），加上本地硬件
快照（GPU 厂商 / 显存、内存、物理核、实测 RAM 带宽），为单个模型计算一组最优 llama-server
参数。Sizing 核心是纯函数 **`tuneModelConfig(hw, m) → ModelConfig`：确定性、无全局、无时间**——
同样输入永远得到同样输出。

为什么坚持纯函数：每条规则都能在单测里用构造输入直接验证（不需要真机、真 GGUF 文件或注入带宽）；
所有副作用被隔离在边缘——`tuneHardware()`（经 Home 页同一条硬件检测链取快照，
`getCalibratedRAMBandwidth` 读缓存/测量）、`readGGUFModelMetrics`（文件解析）、
`buildTuneModel`（把指标折算成输入）。核心没有任何可变性可出 bug。

绑定链路（`TuneModelConfig(modelID)`）：按扫描名找模型 → 解析 GGUF → `buildTuneModel` →
`tuneModelConfig` → **`SaveModelConfig` 持久化**并返回 `ModelConfig`；GGUF 不可读时打固定措辞的
`[INFO]`（`tuneFallbackLogLine`，可 grep）并走保守回退。前端 `ModelSettings.vue` 调
`wails.ts` 的 `tuneModelConfig`，把返回值套进表单后由用户确认保存；成功提示的插值参数由
`lib/modelTune.ts` 的 `tunedSummaryParams` 生成（`cacheTypeK` 为空串 → 展示为 "f16"，空串即
后端默认 f16，不写 cache-type 行）。

## 输入折算要点

- **GPU 规划目标**（进行中特性，`tunePlanTarget`）：默认沿用历史行为——显存取 GPU 列表最大值，
  厂商整机投票（任何名字含 nvidia 的卡或有 CUDA 驱动 → nvidia；否则 amd/radeon → amd；否则
  none）。但服务器配置里持久化的 serving-GPU 选择（`ServerConfig.DeviceID`，一个稳定的
  nvidia-smi UUID——与 llama-server 子进程经 `CUDA_VISIBLE_DEVICES` 钉死的是同一个值）若能
  **精确匹配**（区分大小写）到某张探测到的卡，则由**那张卡**决定显存与厂商，使规划目标与
  llama-server 实际使用的 GPU 在多卡主机上保持一致；匹配卡的 vendor 由它自己的名字决定
  （名字无法分类时保留整机投票结果）。规划目标经与 Home 页相同的检测链取数，匹配时打
  `[INFO] tune: planning against GPU …`。
- **专家/稠密拆分**：优先走张量表逐张量求和（`readGGUFTensorSplit`，重读同一文件的张量区，
  `ggmlBlockTable` 给出各量化块布局），仅当其总和与文件大小差 ≤ `tuneTensorSplitTol`(5%) 才采纳；
  否则退回元数据估算 `estimateExpertBytes`（`bpw × MoE层数 × (expert_count+shared) × 3 × ffn ×
  embedding`，钳制到文件大小）；再不行按全 dense 处理。`isExpertTensor` 按张量名判类
  （`_exps` / `ffn_exp` / `.exp`+`ffn`），router（`ffn_*_inp`）与共享专家（`ffn_*_shexp`）算稠密，
  与 llama-server `--cpu-moe` 的显存驻留一致。
- **KV 折算（混合注意力一致性）**：`tuneModelConfig` 用 `每层KV × Layers` 算每 token 总量、
  `每层KV × ctx` 算 partial offload 的每层成本。混合注意力模型（`full_attention_interval>1`，如
  qwen3.5）只有 `ceil(block_count/interval)` 层带 KV，因此 `buildTuneModel` 把每层成本按
  `kvCacheLayers/BlockCount` 折算——两个公式同时保持正确，且每层权重 `W/L` 仍用真实层数。
- **保守回退**（GGUF 不可读）：32 层 / 1024 B/层/token / ctx 32768 / 全 dense。

## 五步决策树

```
tuneHardware(GPU厂商/VRAM/RAM/物理核/实测带宽) + tuneModel(权重/层数/KV/专家字节)
      │
      ▼
[1] 无可用 GPU (vendor==none 或 VRAM < 1536MB) ──────────▶ CPU-only 方案，返回
      │ 有 GPU
      ▼
[2] full offload 试装：A=f16 / B=q8_0(仅 NVIDIA) 在 VRAM 预算内选 ctx
      ├─ 装下 且 fullCtx ≥ 8192 ─────────────────────────▶ full offload 直接胜出
      ├─ 装下 但 fullCtx < 8192（局促）且 实测带宽>0
      │    且估得 cpu-moe ≥ 3.0 t/s 且 cpu-moe 方案可行 ──▶ 翻转到 cpu-moe，返回
      │    否则 ─────────────────────────────────────────▶ full offload 胜出
      ▼
[3] 装不下：MoE 且专家字节 ≤ 可用 RAM ───────────────────▶ cpu-moe 方案，返回
      ▼
[4] 否则对 ctx∈{8192,4096,2048} 逐层算 n=floor((VRAM−computeBuf)/每层)，
    CPU 侧余量 ≤ 可用 RAM ──────────────────────────────▶ partial offload，返回
      ▼
[5] 全不行 ─────────────────────────────────────────────▶ CPU-only 方案
```

排序理由：**全 offload 永远最快**（没有任何权重走 PCIe/DRAM），所以它是默认赢家，唯一例外由
实测数据解锁（局促的大上下文专家入 RAM 计划 > 局促的全 offload——上下文对可用性影响更大）；
cpu-moe 只在专家放得进内存时成立（专家横跨两侧时 partial offload 更好）；partial offload 是
稠密模型的兜底；CPU-only 是最后保障。上下文梯子（降序）`131072/65536/32768/16384/8192/4096/2048`
按训练上下文过滤（缺失按 32768 折算；`ctxLadderFor` 对 ≤0 输入按上限处理——只有手工构造输入才会
走到），至少保留最小档，每个方案必有 ctx。

cpu-moe 方案细节：`GPULayers="all"` + `CPUMoe=true`（专家驻留系统 RAM，GPU 承担稠密权重 + KV +
计算缓冲），**`Threads=0` 省略 threads 行**——专家 GEMM 落在 CPU 时，llama-server 的自动线程
分配实测 ≥ 手动钉死物理核数（与步骤 2 的物理核规则刻意不同，两处都写明理由）。

## 常量表（含义速查）

| 常量 | 值 | 含义 |
| --- | --- | --- |
| `tuneRAMTotalCapRatio` | 0.90 | 可用 RAM = min(空闲, 总量×0.90)，再乘安全系数 |
| `tuneRAMSafetyFactor` | 0.85 | 15% 留给 OS 与其他进程 |
| `tuneMinUsableRAM` | 1 GiB | 退化快照的兜底下限 |
| `tuneGPUActiveMinVRAMMB` | 1536 | 低于此不算可用 GPU（512MB 级共享内存 iGPU 视为无 GPU） |
| `tuneNvidiaVRAMReserveMB` | 512 | NVIDIA 显存预留（CUDA 上下文比 ROCm 小） |
| `tuneOtherVRAMReserveMB` | 700 | AMD/其他预留 |
| `tuneQ8KVSizeRatio` | 0.53125 | q8_0 KV = 8.5 bit/权重 ÷ f16 的 16 bit |
| `tuneComputeBufBaseMB` | 384 | 计算缓冲（logits + 激活工作区）基线 |
| `tuneComputeBufLargeMB` | +256 @ ctx≥16384 | 长上下文追加 |
| `tuneComputeBufHugeMB` | +512 @ ctx≥65536 | 巨上下文再追加（超线性增长） |
| `tuneHeadroomGuardRatio` | 0.15 | f16 方案剩余 VRAM <15% 且 q8_0 买得到更大 ctx → 换 q8_0 |
| `tuneHugeCtxSafetyRatio` | 0.92 | ≥65536 档必须留 8% 余量（估算误差在 128k 档被放大，实测出现过仅剩 ~2% 的边缘案例；VRAM OOM 加载失败的代价远大于少一档 ctx） |
| `tuneCtxMax` / `tuneCtxMin` | 131072 / 2048 | 上下文梯子边界 |
| `tuneFullOffloadCrampedCtx` | 8192 | full offload 最优 ctx 低于此 = 局促（8192 是聊天尚不局促的最小档） |
| `tuneCPUMoeMinTPS` | 3.0 | 交互速度下限（t/s），翻转必须达标 |
| `tuneTensorSplitTol` | 0.05 | 张量表拆分总和与文件大小的容差 |

A/B 缓存合并（`mergeCachePlans`，full offload 与 cpu-moe 共用同一份 tie-break）：默认 f16；
q8_0 仅 NVIDIA，仅在能买到**严格更大** ctx、或 f16 已紧（剩余 <15%）而 q8_0 更大时胜出；
平局归 f16。

## 实测 RAM 带宽校准（`core/benchbw.go`）

动机：cpu-moe 的解码速度受 DRAM 读带宽限制，静态规则分不清 12 GB/s 单通道 DDR4 笔记本和
60 GB/s 双通道 DDR5 台式机——而这两台机器的翻转决策应当不同。设计移植自 FreeToken 的
`moe/benchbw.py`（哲学层面）。

- **基准形状**：`GOMAXPROCS` 个 worker 各读一段不相交、8 字节对齐的区域；**写 first-touch**
  （把页变私有脏页——从未写过的零页会走共享零页映射，测不出真 DRAM）；1 次未计时 warmup +
  3 次计时取**中位数**；工作集 512 MiB（封顶为空闲 RAM 的 1/4，下限 8 MiB——全 cache 工作集会
  报出虚高的缓存带宽）；展开的 uint64 读 + 8 条独立累加链保证计算不拖累到 DRAM 速度之下；
  **整数求和**避免任意内存内容引发 denormal/NaN 停顿；`benchSink` 让每次归约可被观测
  （校验和进了合理性错误消息），防死代码消除；`runtime.GC()` 先行。单位是**十进制 GB/s**
  （1e9 B/s，与内存营销口径一致，可直接对比）。
- **指纹缓存**：`llama-desktop-benchcache.json`（原子写 0644）。键 = SHA-256 前 16 hex，
  输入 `v1|gpu|vram|ramTotal|cores|logical|cpu`——**含 CPU 型号**（CPU 封装是内存子系统最能标识
  组件，只有六字段会在同板换 CPU 时碰撞）、**排除 RAMFreeGB**（每秒都变，会让缓存永久失效，而
  实测带宽取决于内存子系统而非当前占用）。版本字段 `1` 让未来格式变更干净失效。
- **single-flight**：`benchMu` 临界区横跨整个「查缓存→测量→落盘」——并发两次调优点击只测一次，
  后来者阻塞后直接命中刚写好的缓存。
- **合理性窗口 [2.0, 500.0] GB/s**：窗外值视为坏计时器/沙箱，拒绝而不翻转决策（取反形式
  `!(v>=min && v<=max)` 同时挡掉 NaN/Inf）。**失败不缓存**——瞬时抖动下次点击重试一次即可。
- **fail-open**：任何失败（文件坏、版本不符、指纹不匹配、窗口外、测量出错）→ 带宽 0 →
  `tuneModelConfig` 所有规则原样走静态行为，翻转永不触发。基准只花首次点击 ~1-2 s，**永远不
  fail 整个 tune**。
- **吞吐估算**：`estimateCPUMoeTPS = 带宽×1e9 / 每token活跃专家字节`（单位自洽：B/s ÷ B/t = t/s）；
  任一输入 ≤0 返回 0 =「无估计」。活跃字节 = `ExpertBytes × ExpertUsedFrac`（缺失按整池算——
  慢方向）。

## 已知保守性（刻意为之）

1. **共享专家不计入**：`expert_shared_count` 标记的共享专家每个 token 都被读取，但不进
   `expertBytesPerToken` → t/s 估计偏乐观；固定 3.0 t/s 下限吸收这份松弛。
2. **活跃占比缺失按整池算**：`ExpertUsedFrac` 缺失时假设整个专家池都在流式读取——只会压制翻转
   （慢方向），绝不会错误启用翻转。
3. **CUDA Blackwell 下限的保守比较**（硬件快照侧，`core/llamacpp.go` / `core/sysinfo.go`）：
   compute capability ≥ 12.0（RTX 50 系）要求 CUDA ≥ 12.8，否则二进制报 "no kernel image"；
   cudart 只能从 DLL 文件名解析主版本（如 "12"），**裸主版本无法证明 ≥ 12.8，故 12.x 一律不满足
   下限**。该结论经 `GetCUDA` 进入 `tuneHardware` 的厂商判定（`hasNVIDIA || cuda.Available`）。
4. **元数据专家估算只求量级正确**：混合量化文件（如 Q4_K_M 混 Q4_K/Q6_K）无法精确折算，
   `moeBytesPerWeight` 是粗表——精确路径是张量表，且带 5% 校验，不合格即退回或按 dense。
5. **GGUF 不可读 → 全保守回退**（32 层 / 1024 B/层/token / ctx 32768 / 全 dense），宁可给出
   能跑的保守参数，不给猜出来的激进参数。
6. **巨上下文档强制留 8% 余量**：KV/计算缓冲估算误差在 128k 档被放大（见常量表）。
7. **基准全链路 fail-open**：可疑即 0，静态规则兜底（见上）。
