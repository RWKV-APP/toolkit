param(
    [string]$OutputDir = "dist/cli",
    [string]$Name = "llm_toolkit_cli",
    [string[]]$Targets = @(
        "windows/amd64",
        "windows/arm64",
        "linux/amd64",
        "linux/arm64",
        "darwin/amd64",
        "darwin/arm64"
    )
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$resolvedOutputDir = [System.IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDir))

New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

$previousGoOS = $env:GOOS
$previousGoArch = $env:GOARCH
$previousCGOEnabled = $env:CGO_ENABLED

try {
    foreach ($target in $Targets) {
        $parts = $target.Split("/")
        if ($parts.Length -ne 2) {
            throw "Invalid target '$target'. Expected format GOOS/GOARCH."
        }

        $goos = $parts[0].Trim()
        $goarch = $parts[1].Trim()
        if ([string]::IsNullOrWhiteSpace($goos) -or [string]::IsNullOrWhiteSpace($goarch)) {
            throw "Invalid target '$target'. GOOS and GOARCH are required."
        }

        $targetDir = Join-Path $resolvedOutputDir "$goos-$goarch"
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

        $fileName = $Name
        if ($goos -eq "windows") {
            $fileName = "$Name.exe"
        }
        $outputPath = Join-Path $targetDir $fileName

        Write-Host "Building CLI..."
        Write-Host "  GOOS: $goos"
        Write-Host "  GOARCH: $goarch"
        Write-Host "  Output: $outputPath"

        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $env:CGO_ENABLED = "0"

        go build -trimpath -o $outputPath ./cmd/cli
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $target with exit code $LASTEXITCODE"
        }
    }
}
finally {
    $env:GOOS = $previousGoOS
    $env:GOARCH = $previousGoArch
    $env:CGO_ENABLED = $previousCGOEnabled
}

Write-Host "CLI artifacts created in $resolvedOutputDir"
