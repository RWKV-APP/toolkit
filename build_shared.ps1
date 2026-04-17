param(
    [string]$GoOS = "",
    [string]$GoArch = "",
    [string]$OutputDir = "dist",
    [string]$Name = "llm_toolkit"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$sharedLibDir = Join-Path $repoRoot "cmd/sharedlib"
$resolvedOutputDir = [System.IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDir))

if ([string]::IsNullOrWhiteSpace($GoOS)) {
    $GoOS = $env:GOOS
}
if ([string]::IsNullOrWhiteSpace($GoArch)) {
    $GoArch = $env:GOARCH
}
if ([string]::IsNullOrWhiteSpace($GoOS)) {
    $GoOS = go env GOOS
}
if ([string]::IsNullOrWhiteSpace($GoArch)) {
    $GoArch = go env GOARCH
}

switch ($GoOS) {
    "windows" { $libExt = "dll" }
    "darwin" { $libExt = "dylib" }
    "linux" { $libExt = "so" }
    default { throw "Unsupported GOOS for shared library output: $GoOS" }
}

New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

$outputPath = Join-Path $resolvedOutputDir "$Name.$libExt"
$headerPath = Join-Path $resolvedOutputDir "$Name.h"

Write-Host "Building shared library..."
Write-Host "  GOOS: $GoOS"
Write-Host "  GOARCH: $GoArch"
Write-Host "  Output: $outputPath"

$previousGoOS = $env:GOOS
$previousGoArch = $env:GOARCH

try {
    $env:GOOS = $GoOS
    $env:GOARCH = $GoArch

    go build -buildmode=c-shared -o $outputPath ./cmd/sharedlib
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed with exit code $LASTEXITCODE"
    }
}
finally {
    $env:GOOS = $previousGoOS
    $env:GOARCH = $previousGoArch
}

Write-Host "Shared library created:"
Write-Host "  Library: $outputPath"
Write-Host "  Header:  $headerPath"
