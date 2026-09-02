# MyLlama combined validation gate (Windows PowerShell)
# Usage: .\scripts\check.ps1                 # full (version sync + backend + frontend)
#       .\scripts\check.ps1 -Scope backend  # backend only
#       .\scripts\check.ps1 -Scope frontend # frontend only
#       .\scripts\check.ps1 -VersionOnly    # version sync check only

[CmdletBinding()]
param(
    [ValidateSet("all", "backend", "frontend")]
    [string]$Scope = "all",

    [switch]$VersionOnly
)

$ErrorActionPreference = "Stop"
$RepositoryRoot = Split-Path -Parent $PSScriptRoot

function Assert-VersionSync {
    # Fails the gate unless core/VERSION, wails.json info.productVersion and
    # frontend/package.json version describe the same release version
    # (AGENTS.md "Versioning and Releases" requires the three files to stay in sync).

    $VersionFilePath = Join-Path $RepositoryRoot "core\VERSION"
    $WailsFilePath = Join-Path $RepositoryRoot "wails.json"
    $PackageFilePath = Join-Path $RepositoryRoot "frontend\package.json"

    # core/VERSION is the only file carrying the v prefix: trim, then strip one leading v/V.
    $CoreVersion = (Get-Content -Raw -Path $VersionFilePath).Trim()
    $CoreVersion = $CoreVersion -replace "^[vV]", ""

    $WailsVersion = [string]((Get-Content -Raw -Path $WailsFilePath | ConvertFrom-Json).info.productVersion)
    $PackageVersion = [string]((Get-Content -Raw -Path $PackageFilePath | ConvertFrom-Json).version)

    if ([string]::IsNullOrEmpty($WailsVersion) -or [string]::IsNullOrEmpty($PackageVersion)) {
        throw "Version sync check failed: wails.json info.productVersion and frontend/package.json version must exist and be non-empty"
    }

    $WailsVersion = $WailsVersion.Trim()
    $PackageVersion = $PackageVersion.Trim()

    if ($CoreVersion -cne $WailsVersion -or $CoreVersion -cne $PackageVersion) {
        throw ("Version files are out of sync: core/VERSION = '{0}', wails.json info.productVersion = '{1}', frontend/package.json version = '{2}'" -f $CoreVersion, $WailsVersion, $PackageVersion)
    }

    Write-Host ("Version sync ok: {0} (core/VERSION, wails.json info.productVersion, frontend/package.json)" -f $CoreVersion)
}

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

Assert-VersionSync

if ($VersionOnly) {
    return
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
