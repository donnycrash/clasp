#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$Repo = "donnycrash/clasp"
$InstallDir = "$env:LOCALAPPDATA\clasp"

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    $cpu = (Get-CimInstance Win32_Processor).Architecture
    # Architecture: 12 = ARM64, 9 = AMD64
    if ($cpu -eq 12) { "arm64" } else { "amd64" }
} else {
    Write-Error "Unsupported: 32-bit operating systems are not supported."
    exit 1
}

# Fetch latest release tag if not specified
$Tag = if ($args.Count -ge 2 -and $args[0] -eq "--version") { $args[1] } else { "" }

if (-not $Tag) {
    Write-Host "Fetching latest release..."
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Tag = $release.tag_name
}

Write-Host "Fetching release $Tag..."
$release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/tags/$Tag"

$asset = $release.assets | Where-Object { $_.name -match "windows_$arch\.zip$" } | Select-Object -First 1
if (-not $asset) {
    Write-Error "Could not find asset for windows/$arch in release $Tag"
    exit 1
}

$url = $asset.browser_download_url
$fileName = $asset.name

# Download and extract
$tmpDir = Join-Path $env:TEMP "clasp-install"
if (Test-Path $tmpDir) { Remove-Item -Recurse -Force $tmpDir }
New-Item -ItemType Directory -Path $tmpDir | Out-Null

$archivePath = Join-Path $tmpDir $fileName
Write-Host "Downloading clasp for windows/$arch..."
Invoke-WebRequest -Uri $url -OutFile $archivePath

Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

# Install binary
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

Copy-Item (Join-Path $tmpDir "clasp.exe") (Join-Path $InstallDir "clasp.exe") -Force

# Clean up
Remove-Item -Recurse -Force $tmpDir

Write-Host "clasp installed to $InstallDir\clasp.exe"

# Add to PATH if not present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "Added $InstallDir to user PATH."
}

# Set up scheduled task
Write-Host "Setting up scheduled task..."
& "$InstallDir\clasp.exe" install

Write-Host ""
Write-Host "Next steps:"
Write-Host "  clasp auth login"
