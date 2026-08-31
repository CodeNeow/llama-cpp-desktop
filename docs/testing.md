# 质量验证体系

本文说明本项目如何验证「每个平台、每个功能都正常」:哪些由 CI 自动证明,哪些必须人工验收,以及如何本地复现。

## 分层结构

| 层 | 内容 | 运行位置 | 触发 |
| --- | --- | --- | --- |
| L1 单元测试 | Go(`go test ./...`)+ 前端(vitest) | ubuntu、windows、macos(各自真实 OS 代码分支) | 每次推送 |
| L2 服务链路集成测试 | `core/servchain_e2e_test.go`:真实 llama-server + 真实 tiny GGUF,走「模型扫描 → 预设生成 → 进程拉起 → 日志采集 → `/health` → `/v1/models` → `/v1/completions` 真实推理 → 优雅停止」全链 | ubuntu、windows、macos | 每次推送 |
| L3 安卓模拟器冒烟 | x86_64 调试 APK 装入 API 30 模拟器,断言进程存活、无崩溃、JNI 存储锚点解析成功(非 cwd 回退),留存截图 | CI 模拟器 | 每次推送 |
| L4 前端 E2E | 计划中:vite + `window.go` mock 驱动页面流程 | — | — |
| L5 人工验收 | 见下方清单,打 tag 发版前执行 | 真机 / 真硬件 | 发版前 |

平台 × 功能覆盖矩阵(L1–L3 自动覆盖 ●;L5 人工 ○;该平台无此功能 —):

| 功能 | Windows | Linux | macOS | Android |
| --- | --- | --- | --- | --- |
| Go 单元测试(本 OS 分支) | ● | ● | ● | ○(分支经 seams 在桌面端测试) |
| 前端 vitest | ● | ● | ●(JS 无平台差) | ●(同左) |
| 服务启动 / 推理 / 停止 | ● | ● | ● | ○(链路与桌面共用代码,模拟器验证启动路径) |
| llama.cpp 运行时下载 / 校验 | ○ | ○ | ○ | ○(网络流程同构,资产名按平台) |
| 模型下载(HF / ModelScope) | ○ | ○ | ○ | ○ |
| 图形界面(布局 / 交互) | ○ | ○ | ○ | ○(手机 ○、平板 ○ 分开验) |
| 系统托盘 | ○ | —(不提供) | ○ | — |
| API 路由(无头模式) | ○ | — | — | — |
| 推理显卡选择 / CUDA 兼容 | ○(需真实 N 卡) | — | — | — |
| 应用自更新 | ○ | —(引导链接) | —(引导链接) | —(引导链接) |
| 开机自启 / 安装器 | ○ | ○(deb 安装) | ○(.app 拖装) | ○(侧载) |

## 本地复现服务链路集成测试

```bash
# 1) 准备一个 llama-server(已有安装目录即可)和一个任意 .gguf:
curl -L -o stories260K.gguf \
  "https://huggingface.co/ggml-org/models/resolve/main/tinyllamas/stories260K.gguf"
# (国内可用 https://hf-mirror.com/ggml-org/models/resolve/main/tinyllamas/stories260K.gguf)

# 2) 运行(约 1 秒;测试会用临时目录,不污染本机配置):
LLAMA_DESKTOP_E2E=1 \
LLAMA_DESKTOP_E2E_LLAMA_SERVER="D:/llama-desktop/llama-cpp" \
LLAMA_DESKTOP_E2E_MODEL="$PWD/stories260K.gguf" \
go test ./core/ -run TestServiceChainE2E -count=1 -timeout 10m -v
```

不设 `LLAMA_DESKTOP_E2E` 时该测试自动跳过,常规 `go test` 不受影响。CI 固定使用 llama.cpp **b10695** 的 CPU 构建与 stories260K 模型(约 2 MB),通过 actions/cache 复用;刷新固定版本时同步更新缓存键 `e2e-assets-llama-b10695-stories260K`。

直连模式变体(Android 真机实际使用的启动路径):在上述环境变量基础上追加 `LLAMA_DESKTOP_E2E_MODE=direct`,只运行 `TestServiceChainE2EDirect`——测试内把 `platformGOOS` seam 固定为 `android`,用同一个真实 llama-server(桌面二进制即可)以 `-m`/`--alias` 直连参数拉起单个模型,断言模型 id 为净化别名、路由 `/models` 404 后回退 `/v1/models` 列表(单一驻留模型、状态 loaded),再走真实补全推理与优雅停止:

```bash
LLAMA_DESKTOP_E2E=1 \
LLAMA_DESKTOP_E2E_MODE=direct \
LLAMA_DESKTOP_E2E_LLAMA_SERVER="D:/llama-desktop/llama-cpp" \
LLAMA_DESKTOP_E2E_MODEL="$PWD/stories260K.gguf" \
go test ./core/ -run TestServiceChainE2EDirect -count=1 -timeout 10m -v
```

CI 的 backend 任务(ubuntu)在路由模式 E2E 步骤之后追加了直连模式步骤,复用同一份缓存资产;直连启动路径与 OS 无关(经 seam 驱动),单一 OS 覆盖即可。

## 安卓模拟器冒烟

CI 的 `smoke-android` 任务会为 x86_64 模拟器单独构建调试 APK(`wails3 task android:build:go ABI=x86_64` + `gradlew -PtargetAbi=x86_64`),装入 API 30 模拟器后:轮询进程存活(≤90 秒)、扫描 logcat 断言无 `FATAL EXCEPTION`、断言未出现 `keeping cwd-relative app paths`(即 JNI 存储锚点正常解析),并留存模拟器截图工件。

本地模拟(装有 Android SDK/模拟器的机器):

```bash
wails3 task android:build:go ABI=x86_64
cd build/android && ./gradlew -PtargetAbi=x86_64 assembleDebug
emulator -avd test & adb install -r app/build/outputs/apk/debug/app-debug.apk
adb shell am start -W -n com.wails.app/com.wails.app.MainActivity
```

## 发版前人工验收清单(L5)

自动化无法覆盖的项目(真实硬件、桌面环境交互、厂商 ROM 差异)。打 tag 前逐项勾选:

**Windows**
- [ ] 安装器安装 / 卸载干净,开始菜单图标正确
- [ ] 首页:CPU / 内存 / GPU / CUDA 卡片数据正确(有 N 卡机器验证显卡选择与 CUDA 三态)
- [ ] 运行环境:llama.cpp 下载(含 CUDA 运行时配对)、版本识别、自定义目录
- [ ] 模型:下载(断点续传)、导入目录、参数设置、一键调优、深度实测
- [ ] 聊天:发送 / 流式回复 / 切换模型自动卸载
- [ ] API 页:启动 / 停止 / Token 速度折线 / 日志控制台
- [ ] 设置:主题 / 语言 / 托盘(关闭最小化到托盘、重启后恢复)/ API 路由模式
- [ ] 应用内检查更新

**Linux(Ubuntu 22.04 / 24.04)**
- [ ] deb 安装,应用菜单出现图标,启动无缺库
- [ ] 首页硬件卡片(无 CUDA 卡片,符合能力矩阵)
- [ ] llama.cpp ubuntu-vulkan 下载后服务可启动、聊天可推理
- [ ] AMD / Intel 显卡机器:下载同样选中 Vulkan 资产(`accel=vulkan`),首页显卡卡片显示对应卡(无显存行)
- [ ] 模型下载 / 参数设置 / 主题切换
- [ ] 无托盘与 API 路由设置项(符合能力矩阵)

**macOS(Apple Silicon,arm64)**
- [ ] .app 拖入 Applications,右键打开一次通过 Gatekeeper
- [ ] 首页显卡卡片显示 Apple GPU 并带「Metal 统一内存」标注(无显存条);菜单栏托盘开关可用
- [ ] llama.cpp macOS 下载后服务可启动、聊天可推理(加速标注为 Metal)
- [ ] 一键调优产出 Metal 方案:GPU 层 `all`、Flash Attention 关、上下文按内存预算
- [ ] 模型下载 / 设置项与能力矩阵一致

**macOS(Intel,x64)**
- [ ] 无显卡卡片(官方 x64 版为纯 CPU);加速标注为 CPU;GPU 卸载参数不显示

**Android(真机,arm64)**
- [ ] 侧载安装,启动器图标为品牌图,应用名正确
- [ ] 首启动:JNI 存储锚点生效(下载不再报 read-only)
- [ ] 运行环境:llama.cpp android-arm64 下载、可执行(llama-server 启动成功)
- [ ] 模型:通过 HF 下载小模型到外部存储,列表可见
- [ ] 聊天:加载模型 → 流式回复 → 停止服务释放内存
- [ ] 模型参数设置页:不显示 Flash Attention / cpu-moe / n-cpu-moe / GPU 层数;加载方式无 `dio` 选项
- [ ] 布局:手机单列无横向溢出;平板(如有)双列与侧边导航轨
- [ ] 设置页:目录只读展示、无 Windows 专属项;深色模式
- [ ] 后台切回、旋转屏幕、TaskDock 任务卡不遮挡输入框

### 响应式检查清单(Responsive checklist)

前端三档断点(手机 ≤767 / 平板 768–1099 / 桌面 >1099,唯一来源 `frontend/src/lib/layout.ts`)× 关键页面,各平台通用,可用浏览器开发者工具或拖拽窗口宽度逐档验证:

**手机(≤767)**
- [ ] 首页:硬件卡片单列堆叠,页面无横向溢出
- [ ] 聊天:唤起键盘后输入框仍可见,TaskDock 任务卡不遮挡发送按钮
- [ ] 模型 / 我的模型:标签行过长时在自身容器内横向滑动(滚动条隐藏),滚动时粘性表头下无内容透出
- [ ] 模型参数设置:表单单列、控件全宽,保存按钮可达
- [ ] API:监视卡片单列,日志控制台可独立滚动
- [ ] 设置:标签行横向滑动,目录长路径换行不撑宽页面
- [ ] 文档:目录收起为内容上方换行芯片,代码块在自身内部横向滚动

**平板(768–1099)**
- [ ] 侧边导航自动收起为图标轨,可正常展开 / 收回
- [ ] 文档目录收起为芯片行(与模型详情、API 页同在 ≤1099 收起双栏)
- [ ] 聊天 / API / 模型详情等双栏布局收为单列,无挤压细条、无横向溢出

**桌面(>1099)**
- [ ] 各页保持既有桌面渲染:侧边栏完整展开,文档目录为左侧粘性栏
- [ ] 窗口在 767/768 与 1099/1100 临界宽度拖动时,相邻档位样式正确接管,无闪烁或混合态
