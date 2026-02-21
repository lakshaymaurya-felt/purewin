# PureWin Installer
# Usage: irm https://raw.githubusercontent.com/lakshaymdev/purewin/main/scripts/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo = "lakshaymdev/purewin"
$installDir = "$env:LOCALAPPDATA\PureWin"
$exe = "pw.exe"

Write-Host ""
Write-Host "  PureWin Installer" -ForegroundColor Magenta
Write-Host "  =================" -ForegroundColor DarkGray
Write-Host ""

# Get latest release
Write-Host "  Fetching latest release..." -ForegroundColor Cyan
try {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "PureWin-Installer" }
} catch {
    Write-Host "  ERROR: Could not reach GitHub. Check your internet connection." -ForegroundColor Red
    exit 1
}

$version = $release.tag_name
$asset = $release.assets | Where-Object { $_.name -eq $exe }

if (-not $asset) {
    Write-Host "  ERROR: Could not find $exe in release $version" -ForegroundColor Red
    exit 1
}

Write-Host "  Found PureWin $version" -ForegroundColor Green

# Create install directory
if (-not (Test-Path $installDir)) {
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

# Download
$downloadUrl = $asset.browser_download_url
$dest = Join-Path $installDir $exe
Write-Host "  Downloading $exe..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $dest -UseBasicParsing
} catch {
    Write-Host "  ERROR: Download failed. Check your internet connection." -ForegroundColor Red
    exit 1
}

# Verify download succeeded
if (-not (Test-Path $dest) -or (Get-Item $dest).Length -eq 0) {
    Write-Host "  ERROR: Downloaded file is missing or empty." -ForegroundColor Red
    exit 1
}

Write-Host "  Installed to $dest" -ForegroundColor Green

# Add to user PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ([string]::IsNullOrEmpty($userPath)) {
    $newPath = $installDir
} elseif ($userPath -notlike "*$installDir*") {
    $newPath = "$userPath;$installDir"
} else {
    $newPath = $null
    Write-Host "  $installDir already in PATH" -ForegroundColor DarkGray
}

if ($newPath) {
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "  Added $installDir to PATH" -ForegroundColor Green
    Write-Host ""
    Write-Host "  NOTE: Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "  PureWin $version installed successfully!" -ForegroundColor Green
Write-Host "  Run 'pw' to get started." -ForegroundColor Cyan
Write-Host ""
