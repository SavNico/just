# ==============================================================================
# just - Windows / PowerShell Installation Script
# ==============================================================================
#
# Usage:
#   irm https://raw.githubusercontent.com/SavNico/just/main/install.ps1 | iex
#   or:
#   powershell -ExecutionPolicy Bypass -File install.ps1
# ==============================================================================

$ErrorActionPreference = 'Stop'

$Owner = "SavNico"
$Repo = "just"

# 1. Detect Architecture
$arch = if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { 'Arm64' } else { 'Amd64' }
} else {
    Write-Error "Error: 32-bit Windows is not supported."
    exit 1
}

Write-Host "Detecting latest release for ${Owner}/${Repo}..." -ForegroundColor Cyan

# 2. Get latest release from GitHub API
$headers = @{ "User-Agent" = "just-installer" }
$apiUrl = "https://api.github.com/repos/${Owner}/${Repo}/releases/latest"

try {
    $release = Invoke-RestMethod -Uri $apiUrl -Headers $headers -UseBasicParsing
    $tag = $release.tag_name
} catch {
    Write-Error "Error: Could not retrieve latest release from GitHub: $_"
    exit 1
}

if (-not $tag) {
    Write-Error "Error: Could not determine latest release tag."
    exit 1
}

Write-Host "Latest release is $tag" -ForegroundColor Green

# 3. Locate Asset URL
$zipName = "just_${tag}_Windows_${arch}.zip"
$downloadUrl = $null
if ($release.assets) {
    $asset = $release.assets | Where-Object { $_.name -eq $zipName } | Select-Object -First 1
    if ($asset) {
        $downloadUrl = $asset.browser_download_url
    }
}

if (-not $downloadUrl) {
    $downloadUrl = "https://github.com/${Owner}/${Repo}/releases/download/${tag}/${zipName}"
}

# 4. Download and Extract
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("just_install_" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
$zipPath = Join-Path $tempDir $zipName

Write-Host "Downloading from ${downloadUrl}..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
} catch {
    Write-Error "Error: Failed to download release asset from ${downloadUrl}: $_"
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}

Write-Host "Extracting..." -ForegroundColor Cyan
Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

$extractedExe = Join-Path $tempDir "just.exe"
if (-not (Test-Path $extractedExe)) {
    # Check inside subfolder if zipped with a folder
    $found = Get-ChildItem -Path $tempDir -Filter "just.exe" -Recurse | Select-Object -First 1
    if ($found) {
        $extractedExe = $found.FullName
    } else {
        Write-Error "Error: 'just.exe' not found in downloaded package."
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        exit 1
    }
}

# 5. Determine installation folder
$installDir = Join-Path $env:LOCALAPPDATA "Programs\just"
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
$targetExe = Join-Path $installDir "just.exe"

Copy-Item -Path $extractedExe -Destination $targetExe -Force
Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue

# 6. Ensure install directory is in User PATH
$userPath = [System.Environment]::GetEnvironmentVariable("Path", [System.EnvironmentVariableTarget]::User)
$pathParts = if ($userPath) { $userPath -split ';' } else { @() }
if ($pathParts -notcontains $installDir) {
    Write-Host "Adding $installDir to user PATH..." -ForegroundColor Yellow
    $newUserPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
    [System.Environment]::SetEnvironmentVariable("Path", $newUserPath, [System.EnvironmentVariableTarget]::User)
    $env:Path = "$env:Path;$installDir"
}

Write-Host "Success! 'just' has been installed to $targetExe" -ForegroundColor Green
& $targetExe -v

Write-Host ""
Write-Host "Configuring shell autocompletion for PowerShell..." -ForegroundColor Cyan
& $targetExe --install-completion

