# Changelog

更新日志的**权威来源**（见 `AGENTS.md`「版本发布」）：发版时先在此新增版本条目（含日期与逐提交核心改动），`git tag` 注解消息与 GitHub Release 正文均从该条目复制，保持一致。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循语义化版本。

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

[v0.1.8]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.7...v0.1.8
[v0.1.7]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.6...v0.1.7
[v0.1.6]: https://github.com/CodeNeow/llama-cpp-desktop/compare/v0.1.5...v0.1.6
