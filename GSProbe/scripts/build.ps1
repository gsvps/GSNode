$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$dist = Join-Path $root 'dist'
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$targets = @(
  @{ OS = 'linux'; Arch = 'amd64'; Name = 'gsprobe-linux-amd64' },
  @{ OS = 'linux'; Arch = 'arm64'; Name = 'gsprobe-linux-arm64' },
  @{ OS = 'windows'; Arch = 'amd64'; Name = 'gsprobe-windows-amd64.exe' }
)
Push-Location $root
try {
  foreach ($target in $targets) {
    $env:GOOS = $target.OS
    $env:GOARCH = $target.Arch
    $env:CGO_ENABLED = '0'
    $outPath = Join-Path $dist $target.Name
    go build -trimpath -ldflags '-s -w' -o $outPath ./cmd/gsprobe
    if ($LASTEXITCODE -ne 0) { throw "build failed: $($target.OS)/$($target.Arch)" }
    $hash = (Get-FileHash -Path $outPath -Algorithm SHA256).Hash.ToLower()
    "$hash  $($target.Name)" | Out-File -FilePath "$outPath.sha256" -Encoding ascii -NoNewline
  }
} finally {
  Pop-Location
}
Get-ChildItem $dist | Select-Object Name, Length, LastWriteTime

