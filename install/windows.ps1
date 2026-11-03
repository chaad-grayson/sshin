# SSHIN Windows Installation Script
# Usage: 
#   Online:  irm https://raw.githubusercontent.com/chaad-grayson/sshin/main/install/windows.ps1 | iex
#   Local:   .\install\windows.ps1 -LocalBinary ".\sshin.exe"

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\sshin",
    [switch]$Force = $false,
    [string]$LocalBinary = ""
)

$ErrorActionPreference = "Stop"

# Colors for output
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Info { Write-ColorOutput Green $args }
function Write-Warning { Write-ColorOutput Yellow $args }
function Write-Error { Write-ColorOutput Red $args }

Write-Info "🚀 Installing SSHIN - SSH Manager"
Write-Info ""

# Check if SSHIN is already installed
$existingSSHIN = Get-Command sshin -ErrorAction SilentlyContinue
if ($existingSSHIN -and -not $Force) {
    $currentVersion = & sshin --version 2>$null | Select-String "version" | ForEach-Object { $_.ToString().Split()[-1] }
    Write-Warning "SSHIN is already installed (version: $currentVersion)"
    $response = Read-Host "Do you want to continue with the installation? (y/N)"
    if ($response -ne "y" -and $response -ne "Y") {
        Write-Info "Installation cancelled."
        exit 0
    }
}

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Info "Detected platform: Windows ($arch)"

# Check if using local binary
if ($LocalBinary -ne "") {
    if (-not (Test-Path $LocalBinary)) {
        Write-Error "Local binary not found: $LocalBinary"
        exit 1
    }
    
    Write-Info "Using local binary: $LocalBinary"
    $targetPath = "$InstallDir\sshin.exe"
    
    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        Write-Info "Creating installation directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Copy local binary
    Write-Info "Installing binary to: $targetPath"
    Copy-Item -Path $LocalBinary -Destination $targetPath -Force
    
} else {
    # Online installation
    Write-Info "Starting online installation..."
    
    # Get latest version
    Write-Info "Fetching latest version..."
    try {
        $latestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/chaad-grayson/sshin/releases/latest"
        $latestVersion = $latestRelease.tag_name
        Write-Info "Target version: $latestVersion"
    } catch {
        Write-Error "Failed to fetch version information"
        exit 1
    }

    # Download binary
    # Map architecture to match GoReleaser format
    $goreleaserArch = if ($arch -eq "amd64") { "x86_64" } else { "i386" }
    
    # GoReleaser format: sshin_Windows_x86_64.zip
    $fileName = "sshin_Windows_$goreleaserArch.zip"
    $downloadUrl = "https://github.com/chaad-grayson/sshin/releases/download/$latestVersion/$fileName"
    $tempFile = "$env:TEMP\$fileName"

    Write-Info "Downloading $fileName..."
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile
    } catch {
        Write-Error "Download failed"
        exit 1
    }

    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Extract archive
    Write-Info "Extracting..."
    try {
        Expand-Archive -Path $tempFile -DestinationPath $env:TEMP -Force
        # GoReleaser extracts the binary as just "sshin.exe", not with platform suffix
        $extractedBinary = "$env:TEMP\sshin.exe"
        $targetPath = "$InstallDir\sshin.exe"
        
        Move-Item -Path $extractedBinary -Destination $targetPath -Force
    } catch {
        Write-Error "Extraction failed"
        exit 1
    }

    # Clean up
    Remove-Item $tempFile -Force
}

# Check PATH
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    Write-Warning "The directory $InstallDir is not in your PATH."
    Write-Info "Adding to user PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Write-Info "Please restart your terminal to use the 'sshin' command."
}

Write-Info ""
Write-Info "✅ SSHIN successfully installed to: $targetPath"
Write-Info "You can now use the 'sshin' command!"

# Verify installation
if (Test-Path $targetPath) {
    Write-Info ""
    Write-Info "Verifying installation..."
    & $targetPath --version
}
