$ErrorActionPreference = 'Stop'
$probeRoot = Split-Path -Parent $PSScriptRoot
$gsnodeRoot = Join-Path (Split-Path -Parent $probeRoot) 'GSNode'
$binDir = Join-Path $gsnodeRoot 'bin'
New-Item -ItemType Directory -Force -Path $binDir | Out-Null

$targets = @(
  @{ OS = 'linux'; Arch = 'amd64'; Name = 'gsnode-linux-amd64' },
  @{ OS = 'linux'; Arch = 'arm64'; Name = 'gsnode-linux-arm64' },
  @{ OS = 'linux'; Arch = '386'; Name = 'gsnode-linux-386' },
  @{ OS = 'darwin'; Arch = 'amd64'; Name = 'gsnode-darwin-amd64' },
  @{ OS = 'darwin'; Arch = 'arm64'; Name = 'gsnode-darwin-arm64' },
  @{ OS = 'windows'; Arch = 'amd64'; Name = 'gsnode-windows-amd64.exe' }
)

Push-Location $probeRoot
try {
  foreach ($target in $targets) {
    $env:GOOS = $target.OS
    $env:GOARCH = $target.Arch
    $env:CGO_ENABLED = '0'
    $outNode = Join-Path $binDir $target.Name
    go build -trimpath -ldflags '-s -w' -o $outNode ./cmd/gsprobe
    if ($LASTEXITCODE -ne 0) { throw "build failed: $($target.OS)/$($target.Arch)" }
    $hash = (Get-FileHash -Path $outNode -Algorithm SHA256).Hash.ToLower()
    "$hash  $($target.Name)" | Out-File -FilePath "$outNode.sha256" -Encoding ascii -NoNewline
    Write-Host "built $($target.Name)"
  }
} finally {
  Pop-Location
}
