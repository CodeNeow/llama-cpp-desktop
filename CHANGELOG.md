# Changelog

更新日志的**权威来源**（见 `AGENTS.md`「版本发布」）：发版时先在此新增版本条目（含日期与逐提交核心改动），`git tag` 注解消息与 GitHub Release 正文均从该条目复制，保持一致。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循语义化版本。

## [v0.2.7] - 2026-08-17

## English

v0.2.7: Fix the blank API page on machines without a discrete GPU and backfill release history (2 commits since v0.2.6, per-commit core changes):

1. `07f5730` fix(frontend): guard null gpus in API page monitor rendering
   - Core: On machines without an NVIDIA GPU / nvidia-smi, sampleGPUs returned a nil slice which Go serialized as "gpus": null; Api.vue's status.gpus.length then threw a TypeError that blanked the whole page. The backend now always returns a non-nil empty slice (both exits use make([]MonitorGPU, 0)) and the frontend condition is hardened to status.gpus?.length as defense in depth.
2. `38215e9` docs: backfill release changelog and refresh architecture navigation
   - Core: CHANGELOG backfills the v0.2.6 / v0.2.5 / v0.2.3 entries (v0.2.4 was never released); AGENTS.md architecture navigation table syncs the Chat page, ModelDetail/ModelSettings standalone routes, core/router.go, core/i18n.go, tray/cross-device platform branches and the lib/ module list.

## 中文

v0.2.7: 修复无独立显卡机器 API 页空白并回填发布记录（v0.2.6 以来 2 个提交，按提交逐一说明核心改动）：

1. `07f5730` fix(frontend): API 页监控渲染对 null gpus 防御
   - 核心：无 NVIDIA 独显 / nvidia-smi 不可用的机器上，后端 sampleGPUs 返回 nil slice，Go 序列化为 `"gpus": null`，Api.vue 模板 `status.gpus.length` 对 null 取值抛 TypeError 导致整页空白；后端 sampleGPUs 改为恒返回非 nil 空数组（两个出口均 make([]MonitorGPU,0)），前端 GPU 区块条件加固为 `status.gpus?.length` 双保险
2. `38215e9` docs: 补录发布历史并刷新架构导航
   - 核心：CHANGELOG 补录 v0.2.6 / v0.2.5 / v0.2.3 条目（v0.2.4 从未发布，跳过）；AGENTS.md 架构导航表同步 Chat 页、ModelDetail/ModelSettings 独立路由、core/router.go、core/i18n.go、tray/crossdevice 平台分支与 lib/ 模块清单

## [v0.2.6] - 2026-08-17

v0.2.6: 聊天思维链流式与图片附件、llama.cpp 资产选择修复、更新弹窗改版（v0.2.5 以来 11 个提交，按提交逐一说明核心改动）：

1. `2e2e629` docs(readme): 新增徽章块与模型设置截图
   - 核心：README 顶部加入徽章区块，补入模型设置页截图
2. `7aad601` docs(readme): 徽章统一为 shields.io flat 风格
   - 核心：替换徽章为通用 shields.io flat 样式，风格统一
3. `bda6f5b` docs(readme): 一行简介 feature Qwen3.8-27B
   - 核心：README 一句话简介改为主打 Qwen3.8-27B
4. `17cad40` feat(build): 移除 portable 发布产物，x64-only 支持明确化
   - 核心：CI Windows job 不再重命名/上传 portable exe，发布产物收敛为 4 个（setup exe + 3 个 Ubuntu deb）；更新路径 pickUpdateAsset 的 portable 分支回退到第一个 installer 资产（旧 portable 安装不再报 no-main-executable），按资产类型命名保存文件；README 前置条件明确 x64/amd64-only（llama.cpp 无 32 位 Windows 构建）
5. `98616f1` docs(screenshots): 侧边栏收起态重拍双语截图
   - 核心：全量重拍 Home/Downloads/Models/Settings 截图，反映默认的侧边栏收起态
6. `ffabd38` feat(frontend): 更新下载弹窗改版 + Dock 后台进度
   - 核心：UpdateModal 下载视图改为居中布局（渐变图标/版本行/粗进度条/大号百分比/已传大小行），确认视图用版本日期 chip + 可滚动更新说明区；下载中提供「取消」与「后台下载」（关闭弹窗后 dock 持续显示进度）；lib/update closeUpdateModal 在下载中保持轮询，关闭弹窗不再让后端下载失联
7. `8187456` fix(frontend): 回主页恢复暂停的 llama.cpp 下载状态
   - 核心：initialDownloadAction 对 paused 状态返回 poll 分支——暂停的下载仍在后台运行（goroutine 保活状态），主页轮询分支恢复暂停进度区与恢复/停止按钮，而非回退到空闲下载按钮
8. `72417ea` test(frontend): 下载状态词汇集中管理与穷举映射表
   - 核心：新增 lib/downloadStatus.ts 作为三套后端下载状态词汇（llama.cpp 下载 x7 / 模型任务 x8 / 应用自更新 x4）的权威前端镜像，const 断言数组 + 派生联合类型；llamaDownload / dock / taskStatus 测试改为表驱动，Record 表按联合类型穷举——漏状态即 vue-tsc TS2741 失败，多列状态运行时失败，杜绝丢失状态类回归
9. `9ebe100` fix(frontend): 侧边栏展开动画顺滑
   - 核心：过渡此前只覆盖 width 未覆盖 min-width，展开时 min-width 瞬间跳到 200px 掩盖 width 过渡（收起正常只因 64 低于动画宽度）；两者现在同时过渡；文字经 max-width:0 + opacity:0 隐藏（替代 display:none），展开时淡入且不会在 64px 窄轨内折行
10. `c6e7017` fix(backend): 修正 llama.cpp 资产选择适配当前发布命名
    - 核心：Linux 下载此前完全失效（上游资产名 ubuntu-* 不含 "linux" 子串导致匹配 nil），现同时接受 ubuntu 与历史 linux 关键词；架构过滤全平台强制（此前 Windows 豁免，x64 主机可能 tie-break 选中 arm64 zip）；无 NVIDIA GPU 的 Windows 明确选 -cpu- 构建；有 NVIDIA GPU 时按 compute capability 探测引入 CUDA 下限（Blackwell cc>=12.0 需 CUDA>=12.8，12.4 资产硬跳过），工具链精确匹配仍优先但受下限约束；cudart 运行库按所选主资产的版本/架构配对而非本地工具链；Linux+NVIDIA 选 ubuntu-vulkan；表驱动测试覆盖 b10453 资产清单 11 种主机画像
11. `7843ae2` feat(frontend): 聊天思维链流式、图片附件与分阶段速率统计
    - 核心：聊天页分轨渲染 deepseek reasoning_content 思维链（回答前独立区块），分阶段 token 速率按流式 chunk 精确计数并随消息持久化（思考文本与速率重启后保留）；支持内联图片附件——data URL 按 OpenAI 多模态 content 格式发送（text 在前 images 在后），持久化时剥离防 localStorage 膨胀；lib/chat.ts parseSSEChunks 兼取 reasoning_content，streamChatCompletion 新增 onReasoningDelta 回调，新增 tokenRates 纯函数，ChatMessage 新增 reasoning/stats 字段；wailsjs 绑定补全 GetLoadedModels/UnloadModel 导出（此前仅存在于 App.d.ts/models.ts，TaskDock 卸载流程缺运行时绑定）

## [v0.2.5] - 2026-08-16

v0.2.5: 页面重命名与系统就绪判定、模型设置独立页、聊天图片附件与采样参数、注释转英文（v0.2.3 以来 13 个提交，按提交逐一说明核心改动）：

1. `5475a04` feat(frontend): 模型设置改独立页面，聊天样式修复与图片附件
   - 核心：ModelSettings 从弹窗迁移为独立路由页 /models/settings/:modelName（sticky 页头 + tabs + 三重禁用保存按钮，保存成功返回模型页）；聊天页补齐缺失的 .page-title/.page-subtitle 样式（此前未定义渲染为浏览器默认大字）；支持图片选择（回形针按钮 + accept=image/* multiple）与粘贴（paste 读 clipboardData），预览条可移除，发送按 llama.cpp webui 同款多模态 content parts（text 前 image_url 后），气泡渲染图片，持久化剥离 images
2. `c7cacc5` feat(frontend): 聊天页新增采样参数设置
   - 核心：工具栏齿轮按钮弹出参数面板（温度/Top P/Top K/重复惩罚/最大 Token + 系统提示词），点外关闭、aria-expanded 联动、恢复默认一键重置；参数经 buildChatBody 注入请求体顶层（temperature/top_p/top_k/repeat_penalty/max_tokens），systemPrompt 非空时 messages 最前注入 system 消息；chatParams 模块级 reactive + watch deep 自动持久化（llama-desktop-chat-params）
3. `e416a9a` refactor(frontend): API 页服务器参数收进齿轮弹层、可用模型移入顶部状态卡
   - 核心：页面底部服务器参数区块收进 toolbar 齿轮「参数设置」弹层（对齐聊天页交互：点外关闭、aria-expanded、stop 冒泡），watch debounce 500ms 自动保存不变，服务运行中三输入禁用 + 锁定提示；可用模型列表移入状态卡第二行（标题 + 紧凑 tags，无模型空态）；删除底部 cfg-section/models-section 及死键
4. `9327bab` fix(frontend): API 页监控轮询联动刷新服务运行状态
   - 核心：1s 监控轮询同时刷新 serverRunning，llama-server 被外部终止时按钮态与参数锁定在 1s 内自动纠正（此前只在挂载与启停操作后刷新）
5. `79c6550` fix(frontend): 路由过渡去位移，修复切换页面文字左右抖动
   - 核心：路由过渡改为纯 opacity（.fade-enter/leave 无 translate），消除合成层/布局切换引起的居中文字亚像素抖动
6. `ea25562` refactor(frontend): API 页监控区删除磁盘并将 GPU 并入系统监控卡
   - 核心：监控区移除磁盘采样展示（disk 字段保留在后端契约），GPU 卡并入系统监控卡，字段同步精简
7. `e17bfd2` feat(frontend): 页面重命名与侧边栏系统就绪真实判定
   - 核心：六项统一重命名（主页→系统信息、聊天→本地聊天、下载→模型下载、模型→模型管理、API→API 路由、设置→偏好设置，i18n + router meta 同步）；系统就绪改为真实判定（lib/systemReady.ts：llama.cpp 已安装且本地至少一个模型，挂载即查 + 15s 轮询刷新），就绪=绿点呼吸、未就绪=灰点静止
8. `05cc1ec` feat(frontend): 侧边栏缩宽与窗口控制按钮平台原生适配
   - 核心：侧边栏展开宽度 240→200px；窗口控制按钮按平台适配——macOS 保持红黄绿圆点，Windows/Linux 改原生扁平按钮组（46×36px 贴标题栏，关闭 hover 红底白字），最大化支持还原双框图标切换（本地翻转 + WindowIsMaximised 150ms 校正）
9. `563ccfb` style(frontend): 最小化图标横线居中，模型管理页刷新按钮垂直居中
   - 核心：最小化按钮图标横线视觉居中；模型页刷新按钮与目录栏垂直对齐
10. `19ec9ca` docs(rules): 项目语言策略切换为英文
    - 核心：项目语言策略（文档/注释/提交信息）切换为英文，AGENTS.md 同步
11. `8a5285c` refactor(backend): 后端注释翻译为英文
    - 核心：core 包全部注释/文档串翻译为英文，行为零变化
12. `2e51591` refactor(frontend): 前端代码与脚本注释翻译为英文
    - 核心：frontend 源码注释翻译为英文，行为零变化
13. `70f48ae` docs(readme): README 改为英文主文档 + 双语截图集
    - 核心：README 重写为英文主文档（README_zh.md 为中文对照），截图按语言分目录 docs/screenshots/en|zh 并各引用本语言一套

## [v0.2.3] - 2026-08-16

v0.2.3: 聊天页直连 llama-server 与对话持久化、模型详情页、全局任务卡片（v0.2.2 以来 8 个提交，按提交逐一说明核心改动）：

1. `cf4c89c` style(frontend): 页头标题上移与侧边栏 logo 图标顶部平齐
   - 核心：全局 .page-header padding-top 36→0，页头标题顶部从 y≈82 上移到 y≈41（36px 标题栏 + 行高半距为物理下限），五页 .page-title 加 line-height:1.2；页头 sticky 零位移与侧边栏两态对齐不变量保持
2. `441efab` style(frontend): 主页信息卡改两列网格，内存卡补可用容量与使用率条
   - 核心：Home 六卡单列改两列网格（处理器｜内存、显卡｜CUDA、llama.cpp｜系统），卡内字段 auto-fit 自适应列；内存卡补使用率进度条 + 「已使用 X GB / Y GB」与百分比，首次展示后端已返回的 freeGb；format.ts 新增 usagePercent 纯函数
3. `ed6dba2` feat(server): 全局右下角任务卡片：下载进度与内存模型卸载
   - 核心：后端新增 core/router.go 封装 llama-server 路由 API（GET /models 过滤 loaded/loading/sleeping，POST /models/unload），classifyModelType 按 output_modalities 分类 chat/audio/image/video，serverPort（serverMu 保护）记录实际端口；app.go 新增 GetLoadedModels/UnloadModel 绑定；前端新增全局 TaskDock 组件（App.vue 挂载右下角 fixed 卡片，1s 轮询）：下载任务区显示 llama.cpp 与模型下载进度（活跃才显示），内存模型区按类型徽章 + 状态 + 卸载按钮，支持收起为小条；lib/dock.ts 纯函数 + 8 用例
4. `6714cce` fix(frontend): 任务卡片不透明化并消除 llama.cpp 幽灵行
   - 核心：TaskDock 卡片背景改 var(--bg-secondary)（不透明，浮在滚动内容上不再透底）；llama.cpp 行渲染条件改 llamaActive（仅活跃态显示），修复卡片因模型下载或内存模型可见时空状态幽灵行常驻；llamaStatus 初始值改 idle 态对象消除模板空值访问
5. `8a47136` fix(frontend): 页面操作区随页头固定、隐藏已取消任务、任务卡片缩小
   - 核心：sticky 职责上移至全局 .sticky-top 包装容器（top:0 z-index:20 flow-root），下载/模型/API 页操作栏随页头固定；任务弹窗经 lib/taskStatus.ts visibleTasks 纯函数过滤已取消任务；TaskDock 整体缩小一档（340→300px、圆角 14→12px、max-height 60→50vh）
6. `248d7b1` feat(frontend): 下载页搜索结果改为模型详情页，全选与下载按钮提升为 sticky 操作栏
   - 核心：新增 /downloads/model/:modelId 详情页（返回 + 模型说明 + 按大小降序文件列表 + 量化徽章），sticky 操作栏「已选 n 个文件 + 全选/取消全选 + 下载选中」；结果卡改纯导航删除原地展开逻辑（-275 行）；文件排序与量化识别抽为 lib/modelFiles.ts 纯函数 + 8 用例
7. `72ccae8` feat(frontend): 新增聊天页直连 llama-server，搜索状态保留与结果卡简约化
   - 核心：新增 /chat 页（侧边栏「主页」下方），原生界面直连本地 llama-server（主审实测其回显任意 Origin 的 CORS 头，前端 fetch 无需后端代理）：GET /models 拉取模型列表（排除 failed），POST /v1/chat/completions SSE 流式渲染（lib/chat.ts parseSSEChunks 纯函数处理跨块残片/[DONE]/异常行 + 10 用例），流式光标/停止保留已生成内容/输入框自适应，服务未运行显示离线卡；下载页搜索词/结果/大小缓存抽为 lib/downloadsState.ts 模块级状态
8. `2715056` fix(frontend): 聊天对话与所选模型持久化（模块级状态 + localStorage）
   - 核心：新增 lib/chatState.ts 模块级状态（messages/selectedModel/streaming/chatAbortController），切页往返不清空对话、流式中切页后台续写、返回可见完整内容；localStorage 持久化（重启恢复对话与所选模型，容量上限 200 条，流式高频期只改内存不落盘）；chatState.test.ts 新增 11 用例

## [v0.2.2] - 2026-08-16

v0.2.2: Linux .deb 多发行版构建与前端布局优化（v0.2.1 以来 8 个提交，按提交逐一说明核心改动）：

1. `aac5652` fix(build): 将 Ubuntu 20.04 的 wails build tag 从 webkit2_40 修正为 webkit2_36
   - 核心：20.04 的 libwebkit2gtk-4.0-dev 版本为 2.38.6，缺少 WebKitGTK 2.40 才引入的 C API `webkit_uri_scheme_request_get_http_body`，build tag webkit2_40 会直接调用该 C API 导致 go build 失败；修正为 webkit2_36 后 wails 走 Go 层 `http.NoBody` 兜底，不再依赖该符号
2. `ce56dda` fix(build): 修复 Ubuntu 20.04 构建卡死在 tzdata 交互提示
   - 核心：build-linux job 顶层注入 DEBIAN_FRONTEND=noninteractive 与 TZ=Etc/UTC，关闭裸容器内 apt-get 首次装入 tzdata 时的 debconf 交互，避免无 TTY 的 runner 降级到 Readline 前端后阻塞等待「选择地理区域」导致 job 卡死超时
3. `833b5e3` style(frontend): 页面改为流式布局减少两侧留白，截图同步重拍
   - 核心：各页面主容器改为流式布局并收窄最大宽度，减少宽屏下的两侧留白；同步重拍 Home / Downloads / Models / Settings 四页截图反映新布局
4. `c10312d` style(frontend): 页头零位移贴顶并去掉分隔横线
   - 核心：页头取消额外上边距与底部分隔横线，内容区紧贴窗口顶部；同步调整各页面样式保持视觉一致
5. `5ab14e7` style(frontend): 各页面标题区固定于内容区顶部，不随内容滚动
   - 核心：各页面标题区改为 sticky 定位，固定在内容区顶部；长内容滚动时标题始终可见，与侧边栏收起/展开状态解耦
6. `e386b9f` ci(build): 新增 Ubuntu 20.04/22.04/24.04 .deb 安装包构建与集中发布
   - 核心：build-linux job 以 matrix 容器方式构建三个 Ubuntu 发行版的 .deb 包（20.04 用 libwebkit2gtk-4.0-dev + webkit2_36，22.04/24.04 用 libwebkit2gtk-4.1-dev + webkit2_41）；release job 集中下载 5 个产物（Windows 2 exe + Linux 3 deb）并创建 GitHub Release；非 tag 构建生成 draft preview release
7. `ec692d7` fix(frontend): 修复服务日志瞬间增多时溢出顶开服务器配置区块
   - 核心：API 页服务日志区在大量日志瞬间涌入时保持固定高度，不再撑开上方服务器配置区块；日志滚动容器保持独立滚动条
8. `5fb096f` feat(frontend): API 页改版为顶部启停+左日志右监控面板布局，截图统一收起态
   - 核心：API 页重新布局为顶部服务启停按钮 + 下方左右分栏（左日志、右监控面板），截图统一为侧边栏收起态，反映默认用户体验

## [v0.2.1] - 2026-08-15

v0.2.1: 侧边栏收起（默认收起、偏好持久化）与配置迁移、测试竞态修复（v0.2.0 以来 6 个提交，按提交逐一说明核心改动）：

1. `127244a` test(downloads): 修复 TestPauseResumeDownloadTask 与下载 goroutine 的状态竞态
   - 核心：改用本地 404 服务入队（不再打外网）并以 waitTaskTerminal 等 goroutine 退出后再断言 Pause/Resume 状态机；根因是 downloadTask 开篇无条件写 Status=downloading，与测试手动伪造的状态在 CI 满载 runner 上偶发交错（v0.2.0 tag 首次运行即触发）；生产路径无此交错，产品代码未改动
2. `ab7f0a3` fix(config): 旧配置迁移改为读旧写新，避免 wails dev 监视器竞态崩溃
   - 核心：migrateLegacyConfig 从 os.Rename 改为 ReadFile + WriteFile 复制（0644），旧配置 llama-gui-config.json 保留原处不删除；起因是 wails dev 下应用启动改名项目根目录文件触发 Wails CLI 文件监视器 GetFileAttributesEx 竞态，wails dev 崩退出并遗留孤儿 dev 进程；测试断言随行为反转（旧文件保留且内容字节级一致）
3. `c3f53fc` feat(frontend): 侧边栏支持收起为图标栏并持久化状态
   - 核心：Sidebar 底部新增 « / » 切换按钮，240px ↔ 64px 图标栏宽度过渡（0.2s 缓动），收起态隐藏文字、图标居中、title 提示；appConfig 新增 sidebarCollapsed 字段与 SetSidebarCollapsed 绑定，store 双轨持久化（localStorage 首帧兜底 + 后端配置，主题同款模式）；浏览器 GUI 实测发现并修复收起态 footer 状态点与切换按钮重叠；后端往返测试 + 前端 store 6 用例
4. `ed34039` style(frontend): 侧边栏收起态隐藏状态区，footer 仅保留切换按钮
   - 核心：收起态状态点与「系统就绪」文字整体隐藏，footer 仅剩 » 展开按钮居中；删除状态点已无意义的条件 title 绑定，简化为单子项居中布局
5. `2800e5f` feat(config): 侧边栏默认收起，保存的展开偏好仍优先
   - 核心：后端 loadConfig 预置 SidebarCollapsed=true（镜像 trayEnabled 的「预置默认值再 Unmarshal」模式，区分旧配置缺字段与显式 false）；前端 readStoredSidebarCollapsed 改 !== '0'、loadConfig 改 !== false，首帧 / 配置加载 / 持久化往返三处口径一致为默认收起，用户显式展开的偏好仍优先
6. `9bf8f12` docs(readme): 更新界面截图与过时描述（侧边栏收起/真实搜索/设置 6 分页）
   - 核心：界面预览按新顺序重排并新增「侧边栏收起」小节；重拍 home / downloads / api / monitor 四图，模型设置改为 6 张分 tab 截图并删除被取代的单图（下载页为真实搜索 qwen 结果，API / 监控为仿真数据截图）；中英 README 同步修正技术栈 Wails v2.14 / Go 1.25、环境要求、store 职责、配置字段清单（补 language / trayEnabled / sidebarCollapsed）

## [v0.2.0] - 2026-08-15

v0.2.0: 项目更名为 llama-desktop（仓库 llama-cpp-desktop）与 Wails v2.14.0 升级（v0.1.8 以来 3 个提交，按提交逐一说明核心改动）：

1. `2183371` refactor(backend): 单参错误构造改用 errors.New 消除 vet 非常量格式串告警
   - 核心：core 包 6 处无格式化参数的 fmt.Errorf(tr(...)) 机械转换为 errors.New(tr(...))（app.go 2 处、engine.go 3 处、modelscope.go 1 处）；tr() 返回运行时翻译串属非常量格式串，Go 1.24+ vet 新检查会拒绝该写法；消息文本与翻译参数零变化，纯等价转换
2. `dcc359b` chore(build): 升级 Wails v2.12.0 → v2.14.0（go directive 1.25.0）
   - 核心：go directive 1.22.0 → 1.25.0（v2.14.0 依赖声明强制），golang.org/x/* 间接依赖与 Wails CLI 同步升级至 v2.14.0；动机为 v2.14.0 修复 WebView2 引导程序下载失败时 nil pointer 崩溃（应用直接退出而非显示安装错误对话框），本项目用到的 options/runtime/embed API 无破坏性变更
3. `fff4eef` chore(build): 项目更名为 llama-desktop（仓库 llama-cpp-desktop，含旧配置无损迁移）
   - 核心：go module 与 wails.json 更名 llama-desktop（exe 与 NSIS ProductName/快捷方式随之变化）；配置文件改名 llama-desktop-config.json，loadConfig 前置 migrateLegacyConfig，旧 llama-gui-config.json 整体迁移无损接续；主题与更新节流 localStorage 键写新读旧回退；HTTP User-Agent、应用内更新源与产物命名（llama-desktop-setup-/portable-vX.Y.Z-amd64.exe，llama-gui 三代旧命名资产仍兼容）、窗口标题/托盘 tooltip、CI 产物 glob 与全部文档统一新名称；GitHub 仓库同步重命名为 CodeNeow/llama-cpp-desktop，旧地址自动重定向

> 注：v0.2.0 起随项目更名重置发布历史——v0.1.x 全部 tag 与 Release 已删除，v0.2.0 为新名下首个发布。历史条目的 compare 链接随之失效，仅作史实记录。

## [v0.1.8] - 2026-08-14

v0.1.8: Windows 系统托盘与下载/界面修复（v0.1.7 以来 4 个提交，按提交逐一说明核心改动）：

1. `07e76bf` fix(downloads): 修复 Windows 跨盘保存下载文件失败
   - 核心：moveFile 跨设备判定改用平台常量 crossDeviceRenameErr（Windows 为真实错误码 ERROR_NOT_SAME_DEVICE，其他平台 EXDEV），修复 Go 在 Windows 的 syscall.EXDEV 是发明常量导致跨盘 rename 回退复制永不触发、模型/更新下载跨盘保存直接报错的问题；新增 crossdevice_windows/other 平台分支文件与 Windows 真实跨盘错误回归测试
2. `0429778` fix(frontend): 模型设置弹窗切换分类页时高度跳动
   - 核心：ModelSettings 弹窗新增确定高度 height: min(620px, 80vh)，六个分类页切换时弹窗高度恒定；.modal-body 加 scrollbar-gutter: stable，消除滚动条出现/消失导致的内容区宽度抖动
3. `4aacac2` feat(server): 新增 Windows 系统托盘（关闭缩到托盘、托盘退出）
   - 核心：引入 fyne.io/systray（Windows 纯 Win32 无 cgo，可与 Wails 主线程共存），托盘图标 + 菜单「显示主窗口 / 退出」，托盘退出经 OnShutdown 清理；Windows 关闭按钮改为缩到托盘（llama-server 可后台运行），其他平台保持直接退出（no-op 存根）
4. `00b2944` feat(config): 设置页新增系统托盘启用开关（持久化 + 按需启停）
   - 核心：appConfig 新增 trayEnabled（默认 true，旧配置缺字段兜底）；设置页新增「系统托盘」开关，持久化配置并即时启停托盘；禁用后重新启用需重启应用（systray quitOnce 限制，设置项有常驻提示）

## [v0.1.7] - 2026-08-14

v0.1.7: 多语言支持、服务访问范围、按安装类型更新与发布产物命名规范化（v0.1.6 以来 9 个提交，按提交逐一说明核心改动）：

1. `53b8f93` docs: 引入 CHANGELOG.md 并固化版本发布流程
   - 核心：新增根目录 CHANGELOG.md 作为更新日志权威来源；AGENTS.md 新增「版本发布」小节与「Release 页面只有 compare 链接」常见坑条目
2. `4f2d185` docs: README 中英文重写并新增 MIT 协议，更新全部界面截图
   - 核心：README.md 重写（中文默认）并新增 README.en.md 英文版，新增 MIT LICENSE，docs/screenshots/ 更新为 8 张界面截图（含监控页）
3. `ab1f178` docs: 修正发布产物文件名并同步 AGENTS.md 架构导航与测试基线
   - 核心：修正 README 中 NSIS 安装器文件名，AGENTS.md 架构导航表与测试基线描述同步当前现状
4. `e8f042b` feat(i18n): 引入多语言支持（中文/英文/自动）
   - 核心：新增三语言模式 zh/en/auto（默认 auto 跟随系统，中文环境中文、其余英文）；后端 tr(zh,en) 翻译全部用户可见错误串，语言检测按平台分支（Windows 注册表 / 其他环境变量）；前端手写轻量 i18n 字典，设置页新增「界面语言」三选一，切换即时生效
5. `065cf3e` feat(server): 新增服务访问范围选项（本地/局域网）
   - 核心：设置页新增「服务访问范围」本地（127.0.0.1，默认）/局域网（0.0.0.0）二选一；后端 ServerConfig 新增 AccessMode，SaveServerConfig 按白名单校验并派生 Host；API 页移除 Host 输入框
6. `fe70845` refactor(frontend): 移除主页内存卡片的监控页跳转提示
   - 核心：删除主页内存卡片下的「实时负载与推理速度前往监控页」跳转链接及配套样式
7. `e4ddf1d` fix(build): 覆盖安装时记住上次自定义安装路径
   - 核心：新增自定义 NSIS 模板 project.nsi，安装时把 InstallLocation 写回注册表、覆盖安装时读回上次路径，修复自定义路径安装后覆盖安装回到默认路径的问题
8. `e7505da` chore(build): 规范化发布产物命名（setup/portable 带版本号）
   - 核心：发布产物改为 llama-gui-setup-vX.Y.Z-amd64.exe（安装器）+ llama-gui-portable-vX.Y.Z-amd64.exe（便携版），版本号随 tag 动态注入
9. `e18ef6b` feat(update): 按安装类型下载对应更新产物
   - 核心：应用内更新按当前安装类型（setup/portable，以是否存在 uninstall.exe 判定）下载对应产物；setup 提示运行安装器、portable 提示替换 exe；资产挑选兼容新旧两种命名

## [v0.1.6] - 2026-08-13

v0.1.6: 实时推理监控、ModelScope 下载源与多项修复（v0.1.5 以来 11 个提交，按提交逐一说明核心改动）：

1. `bac0d6a` feat: 实时监控与TPS图表、ModelScope下载源、下载速度、队列持久化与模型新参数
   - 核心：本版本最大功能提交，一次性引入实时监控/TPS 图表、ModelScope 下载源、下载速度显示、下载队列持久化与模型新参数
2. `a3e1840` chore: 重新生成 wailsjs 绑定（新增监控/下载源/新模型参数绑定）
   - 核心：为上述新功能重新生成 Wails 前端绑定，同步新增的绑定方法
3. `aa421a0` refactor(frontend): 主页与监控页职责分离，实时指标收敛至监控页
   - 核心：实时运行指标从主页迁出至监控页，主页只保留系统状态，职责单一化
4. `6ecb52e` refactor(frontend): 监控页更名「推理监控」并移至主页正下方
   - 核心：页面更名「推理监控」，导航位置调整至主页正下方
5. `22cdea4` refactor(frontend): 推理服务区块置顶，位于推理监控页最上方
   - 核心：服务启停区块调整到推理监控页最上方
6. `880e567` refactor(frontend): 移除下载页搜索栏中无样式的源标签
   - 核心：清理下载页搜索栏中未样式化的来源标签
7. `3da94fe` fix(server): TPS 只取解码速度，排除提示词预填充行
   - 核心：修复 TPS 统计口径——只统计解码阶段速度，剔除预填充行误计
8. `e3c20ee` fix(server): 服务日志按行缓冲，根除 TPS 预填充漏入与日志拆行
   - 核心：服务日志改为按行缓冲输出，解决预填充行漏入 TPS 统计与日志被拆断两处问题
9. `0de44b0` feat(server): 监控页拆分推理(预填充)/生成(解码)双指标并实时刷新
   - 核心：推理监控拆分「推理(预填充)」「生成(解码)」双 TPS 指标并实时刷新
10. `0fc9f86` refactor(frontend): 监控页推理服务合并为单一区块并统一双指标样式
    - 核心：推理服务区块合并为单一区块，统一双指标的视觉样式
11. `1699907` fix(server): 兼容新版 llama.cpp 日志格式，预填充期间实时刷新提示词处理速度
    - 核心：适配新版 llama.cpp 日志格式变化，预填充期间提示词处理速度实时刷新

[v0.2.0]: https://github.com/CodeNeow/llama-cpp-desktop/compare/b240d4e...v0.2.0
[v0.1.8]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.7...v0.1.8
[v0.1.7]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.6...v0.1.7
[v0.1.6]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.5...v0.1.6
