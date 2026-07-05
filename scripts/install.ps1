# install.ps1 installs or updates cc-select from GitHub Releases.
# Usage:
#   irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1 | iex
#   iex "& { $(irm https://raw.githubusercontent.com/matiastang/cc-select/main/scripts/install.ps1) } -InstallDir C:\Tools"

param(
    [string]$InstallDir = ""
)

$Repo = "matiastang/cc-select"
$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Write-Info($message) {
    Write-Host $message
}

function Write-ErrorAndExit($message) {
    [Console]::Error.WriteLine("error: $message")
    exit 1
}

function Get-LatestTag {
    try {
        $release = Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing
        $tag = $release.tag_name
        if (-not $tag) {
            Write-ErrorAndExit "failed to fetch latest release tag from $ApiUrl"
        }
        return $tag
    } catch {
        Write-ErrorAndExit "failed to fetch latest release from $ApiUrl"
    }
}

function Get-Architecture {
    $procArch = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture
    switch ($procArch) {
        X64   { return "amd64" }
        Arm64 { Write-ErrorAndExit "unsupported architecture: windows/arm64 release is not yet available" }
        default { Write-ErrorAndExit "unsupported architecture: $procArch (only amd64 is currently supported)" }
    }
}

function Add-ToUserPath {
    param([string]$dir)
    $current = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($current -split ";" | Where-Object { $_ -ieq $dir }) {
        return
    }
    if ([string]::IsNullOrWhiteSpace($current)) {
        $newPath = $dir
    } else {
        $newPath = "$current;$dir"
    }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    # Also update the current session PATH.
    $env:Path = "$env:Path;$dir"
    Write-Info "Added $dir to your user PATH. You may need to restart your terminal."
}

function Resolve-InstallDir {
    if ($InstallDir) {
        return $InstallDir
    }

    $existing = Get-Command cc-select -ErrorAction SilentlyContinue
    if ($existing) {
        $dir = Split-Path -Parent $existing.Source
        Write-Info "Updating existing installation in $dir"
        return $dir
    }

    $localDir = Join-Path $env:LOCALAPPDATA "cc-select"
    return $localDir
}

function Verify-Checksum {
    param(
        [string]$assetPath,
        [string]$checksumsPath,
        [string]$assetName
    )
    $lines = Get-Content $checksumsPath
    $expected = $null
    foreach ($line in $lines) {
        $parts = $line -split "\s+", 2
        if ($parts[1].Trim() -eq $assetName) {
            $expected = $parts[0].Trim()
            break
        }
    }
    if (-not $expected) {
        Write-ErrorAndExit "could not find checksum for $assetName"
    }
    $actual = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLower()
    if ($expected -ne $actual) {
        Write-ErrorAndExit "checksum mismatch for ${assetName}: expected $expected, got $actual"
    }
}

$arch = Get-Architecture
$tag = Get-LatestTag
$version = $tag -replace "^v", ""
$asset = "cc-select_${version}_windows_${arch}.zip"
$downloadUrl = "https://github.com/$Repo/releases/download/$tag/$asset"
$checksumsUrl = "https://github.com/$Repo/releases/download/$tag/checksums.txt"

Write-Info "Installing cc-select $tag for windows/$arch..."

$tmpDir = Join-Path $env:TEMP "cc-select-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
try {
    $assetPath = Join-Path $tmpDir $asset
    $checksumsPath = Join-Path $tmpDir "checksums.txt"

    Write-Info "Downloading $asset..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $assetPath -UseBasicParsing

    Write-Info "Downloading checksums..."
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

    Write-Info "Verifying checksum..."
    Verify-Checksum -assetPath $assetPath -checksumsPath $checksumsPath -assetName $asset

    Write-Info "Extracting..."
    Expand-Archive -Path $assetPath -DestinationPath $tmpDir -Force

    $binaryPath = Join-Path $tmpDir "cc-select.exe"
    if (-not (Test-Path $binaryPath)) {
        Write-ErrorAndExit "archive did not contain cc-select.exe"
    }

    $installDir = Resolve-InstallDir
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    $targetPath = Join-Path $installDir "cc-select.exe"

    # If the binary is running, we cannot overwrite it. Rename the old one first.
    if (Test-Path $targetPath) {
        $backupPath = "$targetPath.old"
        if (Test-Path $backupPath) {
            try {
                Remove-Item $backupPath -Force -ErrorAction Stop
            } catch {
                Write-ErrorAndExit "cannot remove old backup ${backupPath}: $_`nClose any running cc-select processes and try again."
            }
        }
        try {
            Rename-Item -Path $targetPath -NewName $backupPath -Force -ErrorAction Stop
            Write-Info "Backed up existing binary to $backupPath"
        } catch {
            Write-ErrorAndExit "cannot replace ${targetPath}: $_`nClose any running cc-select processes and try again."
        }
    }

    Move-Item -Path $binaryPath -Destination $targetPath -Force -ErrorAction Stop

    # Ensure the install directory is on the user PATH.
    Add-ToUserPath -dir $installDir

    $installedVersion = & "$targetPath" --version 2>$null
    if (-not $installedVersion) {
        $installedVersion = "cc-select"
    }
    Write-Info ""
    Write-Info "$installedVersion installed to $targetPath"
    Write-Info ""
    Write-Info "To enable shell integration, run:"
    Write-Info "  cc-select init | Out-File -Append -Encoding utf8 \$PROFILE"
    Write-Info ""
    Write-Info "Then reload your profile with '. \$PROFILE' or open a new PowerShell window."
} finally {
    Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
