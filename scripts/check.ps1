# Llama Desktop 组合验证门（Windows PowerShell）
# 用法：.\scripts\check.ps1                 # 全量（后端 + 前端）
#       .\scripts\check.ps1 -Scope backend # 仅后端
#       .\scripts\check.ps1 -Scope frontend# 仅前端

[CmdletBinding()]
param(
    [ValidateSet("all", "backend", "frontend")]
    [string]$Scope = "all"
)

$ErrorActionPreference = "Stop"
$RepositoryRoot = Split-Path -Parent $PSScriptRoot

function Invoke-ExternalCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$WorkingDirectory,
        [Parameter(Mandatory = $true)]
        [string]$Executable,
        [string[]]$CommandArguments = @()
    )

    Push-Location (Join-Path $RepositoryRoot $WorkingDirectory)
    try {
        Write-Host ("> {0} {1}" -f $Executable, ($CommandArguments -join " "))
        & $Executable @CommandArguments
        if ($LASTEXITCODE -ne 0) {
            throw "Command failed with exit code ${LASTEXITCODE}: $Executable $($CommandArguments -join ' ')"
        }
    }
    finally {
        Pop-Location
    }
}

if ($Scope -in @("all", "backend")) {
    Invoke-ExternalCommand -WorkingDirectory "." -Executable "go" -CommandArguments @("build", "./...")
    Invoke-ExternalCommand -WorkingDirectory "." -Executable "go" -CommandArguments @("test", "./...")

    Push-Location $RepositoryRoot
    try {
        Write-Host "> gofmt -l ."
        $UnformattedFiles = @(& gofmt -l .)
        if ($LASTEXITCODE -ne 0) {
            throw "gofmt failed with exit code ${LASTEXITCODE}"
        }
        if ($UnformattedFiles.Count -gt 0) {
            $UnformattedFiles | ForEach-Object { Write-Host $_ }
            throw "gofmt reported unformatted files"
        }
    }
    finally {
        Pop-Location
    }

    Invoke-ExternalCommand -WorkingDirectory "." -Executable "golangci-lint" -CommandArguments @("run")
}

if ($Scope -in @("all", "frontend")) {
    Invoke-ExternalCommand -WorkingDirectory "frontend" -Executable "npm" -CommandArguments @("run", "build")
    Invoke-ExternalCommand -WorkingDirectory "frontend" -Executable "npm" -CommandArguments @("test")
}
