# Issue label one-click creation script (GitHub CLI)
# Usage:
#   1. Install gh CLI and log in: gh auth login
#   2. Run from repo root: .\scripts\create-labels.ps1
# Idempotent: existing labels are --force updated with description and color.

$ErrorActionPreference = "Stop"

if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
    Write-Error "gh CLI not found. Install https://cli.github.com/ and run gh auth login"
    exit 1
}

$repo = gh repo view --json nameWithOwner --jq .nameWithOwner
if (-not $repo) {
    Write-Error "Cannot detect GitHub repo. Confirm you are in the repo directory."
    exit 1
}

$labels = @(
    # Priority (exactly one per issue)
    @{ Name = "P0-critical"; Color = "B60205"; Desc = "Blocks core functionality or security vulnerability, must fix immediately" }
    @{ Name = "P1-high";     Color = "D93F0B"; Desc = "High priority, noticeably affects experience or security" }
    @{ Name = "P2-medium";   Color = "FBCA04"; Desc = "Medium, experience/consistency issue" }
    @{ Name = "P3-low";      Color = "0E8A16"; Desc = "Low priority, polish item" }
    # Type
    @{ Name = "bug";          Color = "D73A4A"; Desc = "Unexpected behavior / defect" }
    @{ Name = "enhancement";  Color = "A2EEEF"; Desc = "New feature / improvement" }
    @{ Name = "documentation"; Color = "0075CA"; Desc = "Documentation" }
    # Area
    @{ Name = "frontend"; Color = "1D76DB"; Desc = "Frontend (Vue 3 / Vite)" }
    @{ Name = "backend";  Color = "5319E7"; Desc = "Backend (Go / Wails)" }
    @{ Name = "models";   Color = "C5DEF5"; Desc = "Model scan / parameter config" }
    @{ Name = "downloads"; Color = "7057FF"; Desc = "Model download (HF Mirror) / llama.cpp install" }
    @{ Name = "server";   Color = "008672"; Desc = "API service / llama-server" }
    @{ Name = "config";   Color = "6F42C1"; Desc = "Config persistence / theme" }
    # Security
    @{ Name = "security"; Color = "8B0000"; Desc = "Security (sensitive vulnerabilities go via Security Advisory)" }
)

foreach ($l in $labels) {
    Write-Host "Creating/updating label [$($l.Name)] ..." -ForegroundColor Cyan
    gh label create $l.Name --repo $repo --color $l.Color --description $l.Desc --force
}

    Write-Host "`nDone! $($labels.Count) labels ready." -ForegroundColor Green
