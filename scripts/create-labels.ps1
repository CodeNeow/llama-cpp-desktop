# Issue 标签一键创建脚本（GitHub CLI）
# 用法：
#   1. 安装 gh CLI 并登录：gh auth login
#   2. 在仓库根目录执行：.\scripts\create-labels.ps1
# 幂等：已存在的标签会被 --force 更新描述与颜色。

$ErrorActionPreference = "Stop"

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Error "未找到 gh CLI，请先安装 https://cli.github.com/ 并执行 gh auth login"
    exit 1
}

$repo = gh repo view --json nameWithOwner --jq .nameWithOwner
if (-not $repo) {
    Write-Error "无法识别当前 GitHub 仓库，请确认在仓库目录内运行"
    exit 1
}

$labels = @(
    # 优先级（发现类 issue 恰好打一个）
    @{ Name = "P0-critical"; Color = "B60205"; Desc = "阻断核心功能或安全漏洞，必须立即修" }
    @{ Name = "P1-high";     Color = "D93F0B"; Desc = "高优先级，明显影响体验或安全" }
    @{ Name = "P2-medium";   Color = "FBCA04"; Desc = "中等，体验/一致性问题" }
    @{ Name = "P3-low";      Color = "0E8A16"; Desc = "低优先级，打磨项" }
    # 类型
    @{ Name = "bug";          Color = "D73A4A"; Desc = "非预期行为 / 缺陷" }
    @{ Name = "enhancement";  Color = "A2EEEF"; Desc = "新功能 / 改进" }
    @{ Name = "documentation"; Color = "0075CA"; Desc = "文档相关" }
    # 区域
    @{ Name = "frontend"; Color = "1D76DB"; Desc = "前端 (Vue 3 / Vite)" }
    @{ Name = "backend";  Color = "5319E7"; Desc = "后端 (Go / Wails)" }
    @{ Name = "models";   Color = "C5DEF5"; Desc = "模型扫描 / 参数配置" }
    @{ Name = "downloads"; Color = "7057FF"; Desc = "模型下载 (HF Mirror) / llama.cpp 安装" }
    @{ Name = "server";   Color = "008672"; Desc = "API 服务 / llama-server" }
    @{ Name = "config";   Color = "6F42C1"; Desc = "配置持久化 / 主题" }
    # 安全
    @{ Name = "security"; Color = "8B0000"; Desc = "安全相关（敏感漏洞请走 Security Advisory）" }
)

foreach ($l in $labels) {
    Write-Host "创建/更新标签 [$($l.Name)] ..." -ForegroundColor Cyan
    gh label create $l.Name --repo $repo --color $l.Color --description $l.Desc --force
}

Write-Host "`n完成！共 $($labels.Count) 个标签已就绪。" -ForegroundColor Green
