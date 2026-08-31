# 平台能力适配地图（Platform Capability Map）

本文是各平台（Windows / Linux / macOS / Android）功能差异的速查地图：应用在哪个平台提供什么能力、由哪个文件/函数决定。涉及跨平台改动时，先查这里再动手。

## 平台 × 功能矩阵

| 能力 | Windows | Linux | macOS (arm64) | macOS (x64) | Android |
| --- | --- | --- | --- | --- | --- |
| llama.cpp 资产 | CPU / CUDA zip（另配 cudart 运行时包） | ubuntu Vulkan tarball（NVIDIA / AMD / Intel 通用） | Metal（内嵌，arm64 包） | CPU 包（官方 x64 版无 Metal） | android arm64 CPU tarball |
| 加速检测（`accel`） | `ggml-cuda.dll`→cuda / `ggml-vulkan.dll`→vulkan / 否则 cpu | `libggml-vulkan.so`→vulkan / 否则 cpu | 恒为 metal（按架构判定） | 恒为 cpu（按架构判定） | 恒为 cpu |
| GPU 探测 | nvidia-smi（NVIDIA，含显存） | nvidia-smi + PCI 显示控制器（AMD / Intel / 其他，无显存字段） | system_profiler（Apple GPU，统一内存，无显存） | 无（返回空列表） | 无（探测短路，恒为空） |
| 显卡参数（gpu-layers / flash-attn / cpu-moe / n-cpu-moe） | 显示（NVIDIA 驱动） | 显示（Vulkan） | 仅 arm64 显示（Metal）；arch 未知时隐藏 | 隐藏（CPU-only） | 隐藏（CPU-only，后端直接参数同步剔除） |
| 加载方式 `dio` | 提供 | 提供 | 隐藏 | 隐藏 | 隐藏 |
| 一键调优方案 | CUDA：全卸载 / cpu-moe / 部分卸载 / 纯 CPU | Vulkan：同 CUDA 规则 | Metal 方案：全卸载 + 按内存选上下文（Flash Attention 关、不跑 bench 链） | 纯 CPU 方案 | 纯 CPU 方案 |
| 系统托盘 | 有 | 无（桌面环境依赖大） | 有（NSStatusItem） | 有（NSStatusItem） | 无 |
| API 路由（无头）模式 | 可用（依赖托盘） | 不可用 | 不可用 | 不可用 | 不可用 |
| 应用自更新 | 应用内更新 | 指向 GitHub Releases 链接 | 同左 | 同左 | 同左 |
| 多 GPU 面板（切分 / 张量切分 / 主 GPU） | 显示 | 显示 | 隐藏（单 GPU） | 隐藏 | 隐藏 |
| 服务模式 | router 子进程 | router 子进程 | router 子进程 | router 子进程 | direct 直连参数（无 router 子进程） |
| 存储锚点 | 路径相对 cwd | 用户配置目录 `llama-desktop/` 下 | 同 Linux | 同 Linux | JNI files 目录（`androidpath_android.go`） |

> Linux CUDA 兼容性卡片、推理显卡选择（`CUDA_VISIBLE_DEVICES`）与 Windows cudart 组件行均为 Windows 专属。

## 平台决策触点地图

| 决策 | 归属 |
| --- | --- |
| 前端平台状态分类（tier / 托盘 / 标题栏 / arch 门控） | `frontend/src/lib/platform.ts` `buildPlatformState(os, width, arch?)` |
| 前端各项 UI 门控（显卡卡片、CUDA 卡、多 GPU 面板、offload 参数、加载方式列表、加速标签回退） | `platform.ts` `showGpuCards` / `showCudaCompat` / `showMultiGpuPanel` / `showGpuOffloadParam` / `loadModeOptions` / `accelBuildKey` |
| 安装的加速构建检测（cuda / vulkan / metal / cpu） | `core/llamacpp.go` `detectAccel`（读取 llama-server 所在目录的后端库） |
| Linux PCI 显示控制器探测 | `core/gpuprobe.go`（纯解析）+ `gpuprobe_linux.go` / `gpuprobe_other.go`（IO 与桩） |
| GPU 列表（Vendor 字段 / darwin Metal 条目 / linux PCI 追加） | `core/sysinfo.go` `getGPUInfo` / `parseGPUInfoCSV` / `probeDarwinGPUs` |
| 发布资产选择（win cuda / linux vulkan / macos / android） | `core/llamacpp.go` `pickBestAssetFor`（`hasGPU` = 探测到的任意厂商 GPU） |
| 调优方案（CUDA / Vulkan / Metal / 纯 CPU 分支） | `core/autotune.go` `tuneModelConfig` + `tunePlanTarget`（Vendor: nvidia / amd / apple / none） |
| Android 直连参数剔除 GPU 专属旗标 | `core/preset.go` `modelDirectArgs`（`platformGOOS` 门控；INI 预设不受影响） |
| 托盘平台门控（windows / darwin） | `core/app.go` `trayPlatformSupported` + `ServiceStartup` / `SetTrayEnabled`；实现在 `core/tray.go`（v3 SystemTray）与 `tray_other.go`（非桌面桩） |
| Go 侧 OS 分支测试缝 | `core/bridge.go` `platformGOOS`、`core/paths.go` `pathsGOOS`（测试用 `withPlatformGOOS`） |
| 构建标签文件对 | `tray*.go`、`gpuprobe_linux.go` / `gpuprobe_other.go`、`proctime_*`、`installer_launch_*`、`crossdevice_*`、`singleinstance_*`、`hidewindow_*`、`locale_*`、`diskusage_*`、`headlessalert_*`、`tray_headless_*`、`androidpath_*`、`devbuild_*` |
