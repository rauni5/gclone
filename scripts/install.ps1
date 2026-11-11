<#
.SYNOPSIS
  Installs gclone with `go install` and adds Go's bin folder to your user PATH.

.EXAMPLE
  .\scripts\install.ps1
  .\scripts\install.ps1 -Version v0.1.0

.NOTES
  Needs Go 1.24 or newer (https://go.dev/dl/). Only your *user* PATH is changed, no admin
  rights are needed. Open a new terminal afterwards so it picks up the change.
#>
param(
  [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "Go is not installed or not on PATH. Install it from https://go.dev/dl/ and run this again."
}

Write-Host "Installing github.com/rauni5/gclone@$Version ..."
& go install "github.com/rauni5/gclone@$Version"
if ($LASTEXITCODE -ne 0) {
  throw "go install failed (exit code $LASTEXITCODE)."
}

# Where go install puts binaries: GOBIN if set, otherwise the first GOPATH entry + \bin.
$bin = (& go env GOBIN).Trim()
if (-not $bin) {
  $gopath = ((& go env GOPATH).Trim() -split ";")[0]
  $bin = Join-Path $gopath "bin"
}

# Read the user PATH (not the merged one) so we only ever change the user's own value.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $userPath) { $userPath = "" }
$entries = @($userPath -split ";" | Where-Object { $_ })

$normalized = $bin.TrimEnd("\")
$already = $entries | Where-Object { $_.TrimEnd("\") -ieq $normalized }

if ($already) {
  Write-Host "$bin is already on your user PATH."
} else {
  $newPath = (@($entries) + $bin) -join ";"
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  Write-Host "Added $bin to your user PATH."
}

# Make it work in this window too, without reopening the terminal.
if (-not (($env:Path -split ";") | Where-Object { $_.TrimEnd("\") -ieq $normalized })) {
  $env:Path = "$env:Path;$bin"
}

Write-Host ""
if (Get-Command gclone -ErrorAction SilentlyContinue) {
  Write-Host "Done. Try:  gclone --help"
} else {
  Write-Host "Installed to $bin, but gclone is not found yet. Open a new terminal and try again."
}
