param(
  [string]$Version = "latest",
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\Voidnet"
)

$ErrorActionPreference = "Stop"

$Repo = if ($env:VOIDNET_REPO) { $env:VOIDNET_REPO } else { "JimmyMcBride/voidnet" }
$ReleaseBaseUrl = if ($env:VOIDNET_RELEASE_BASE_URL) { $env:VOIDNET_RELEASE_BASE_URL } else { "https://github.com/$Repo/releases" }

function Write-Status {
  param([string]$Message)
  Write-Host "voidnet install: $Message"
}

function Get-AssetName {
  switch -Regex ("$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)") {
    "X64" { return "voidnet-windows-amd64.zip" }
    default { throw "Unsupported Windows architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
  }
}

function Get-ReleaseUrl {
  param([string]$Asset)

  if ($Version -eq "latest") {
    return "$ReleaseBaseUrl/latest/download/$Asset"
  }
  return "$ReleaseBaseUrl/download/$Version/$Asset"
}

function Ensure-InstallDir {
  if (-not (Test-Path -LiteralPath $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
  }
}

function Verify-Checksum {
  param(
    [string]$ArchivePath,
    [string]$Asset,
    [string]$ChecksumsPath
  )

  $expected = Select-String -Path $ChecksumsPath -Pattern " $([regex]::Escape($Asset))$" | ForEach-Object {
    ($_ -split "\s+")[0]
  } | Select-Object -First 1

  if (-not $expected) {
    throw "Checksum for $Asset not found"
  }

  $actual = (Get-FileHash -Algorithm SHA256 -Path $ArchivePath).Hash.ToLowerInvariant()
  if ($actual -ne $expected.ToLowerInvariant()) {
    throw "Checksum mismatch for $Asset"
  }
}

function Ensure-Path {
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  $entries = @()
  if ($userPath) {
    $entries = $userPath -split ';' | Where-Object { $_ }
  }
  if ($entries -contains $InstallDir) {
    return
  }

  $newPath = @($entries + $InstallDir) -join ';'
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  Write-Status "added $InstallDir to the user PATH"
}

$asset = Get-AssetName
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("voidnet-install-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
  $archivePath = Join-Path $tempDir $asset
  $checksumsPath = Join-Path $tempDir "voidnet-checksums.txt"
  $extractDir = Join-Path $tempDir "extract"

  Write-Status "downloading $asset from $Repo ($Version)"
  Invoke-WebRequest -Uri (Get-ReleaseUrl -Asset $asset) -OutFile $archivePath

  try {
    Invoke-WebRequest -Uri (Get-ReleaseUrl -Asset "voidnet-checksums.txt") -OutFile $checksumsPath
    Verify-Checksum -ArchivePath $archivePath -Asset $asset -ChecksumsPath $checksumsPath
  } catch {
    Write-Warning "Skipping checksum verification: $($_.Exception.Message)"
  }

  Ensure-InstallDir
  New-Item -ItemType Directory -Path $extractDir | Out-Null
  Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
  Copy-Item -Path (Join-Path $extractDir "voidnet.exe") -Destination (Join-Path $InstallDir "voidnet.exe") -Force
  Ensure-Path

  Write-Status "installed voidnet to $InstallDir\voidnet.exe"
  Write-Status "open a new terminal and run 'voidnet -version' or 'voidnet'"
} finally {
  if (Test-Path -LiteralPath $tempDir) {
    Remove-Item -LiteralPath $tempDir -Recurse -Force
  }
}
