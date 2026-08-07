# GSNode 纯净一键检测 (Windows)：临时下载 -> 完整检测 -> 上传 GSVPS -> 终端输出 -> 自动清理
# Usage:
#   irm https://dl.gsvps.com/install.ps1 | iex
# 备用地址:
#   irm https://github.com/gsvps/GSNode/raw/main/install.ps1 | iex
#   irm https://cdn.jsdelivr.net/gh/gsvps/GSNode@main/install.ps1 | iex

[CmdletBinding()]
param(
    [switch]$Help
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Get-EnvOrDefault {
    param([string]$Name, [string]$Default)
    $v = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrEmpty($v)) { return $Default }
    return $v
}

$Version         = Get-EnvOrDefault 'GSNODE_VERSION' '0.1.29'
$Repo            = Get-EnvOrDefault 'GSNODE_REPO' 'https://github.com/gsvps/GSNode'
$DataPrimary     = Get-EnvOrDefault 'GSNODE_DATA_PRIMARY' 'https://dl.gsvps.com'
$BaseUrl         = Get-EnvOrDefault 'GSNODE_BASE_URL' "$Repo/raw/v$Version/bin"
$InstallDir      = Get-EnvOrDefault 'GSNODE_INSTALL_DIR' (Join-Path $env:LOCALAPPDATA 'GSNode')
$GsvpsUploadUrl  = Get-EnvOrDefault 'GSVPS_UPLOAD_URL' 'https://www.gsvps.com/api/reports/upload'
$GsvpsSiteUrl    = Get-EnvOrDefault 'GSVPS_SITE_URL' 'https://www.gsvps.com'
$DownloadRetries = [int](Get-EnvOrDefault 'GSNODE_DOWNLOAD_RETRIES' '3')
$RetryDelay      = [int](Get-EnvOrDefault 'GSNODE_DOWNLOAD_RETRY_DELAY' '2')

$PureMode = $true
if ((Get-EnvOrDefault 'GSNODE_KEEP' '0') -eq '1' -or (Get-EnvOrDefault 'GSNODE_INSTALL_ONLY' '0') -eq '1') {
    $PureMode = $false
}

$helpText = @"
GSNode 纯净一键检测 (Windows)

用法:
  irm https://dl.gsvps.com/install.ps1 | iex

备用地址:
  irm https://github.com/gsvps/GSNode/raw/main/install.ps1 | iex
  irm https://cdn.jsdelivr.net/gh/gsvps/GSNode@main/install.ps1 | iex

默认行为（纯净模式）:
  临时下载检测程序 -> 完整检测 -> 上传 GSVPS -> 终端显示结果 -> 自动清理所有本地文件
  不会在系统中永久安装 gsnode，也不会留下报告缓存。

环境变量:
  GSNODE_VERSION        版本号 (默认 $Version)
  GSVPS_UPLOAD_URL      上传 API (默认 $GsvpsUploadUrl)
  GSVPS_SITE_URL        站点 URL (默认 $GsvpsSiteUrl)
  GSVPS_UPLOAD=0        禁用上传
  GSNODE_KEEP=1         检测后保留安装到 $InstallDir\gsnode.exe
  GSNODE_INSTALL_ONLY=1 仅安装，不检测（需配合 GSNODE_KEEP=1）
  GSNODE_BIN            使用已有二进制路径
  GSNODE_DATA           自定义报告缓存目录
  GSNODE_HOST_PROVIDER  主机商名称（非交互式安装时预设，跳过交互提问）
  GSNODE_DATA_PRIMARY   数据/二进制主下载源 (默认 https://dl.gsvps.com)
  GSNODE_DOWNLOAD_RETRIES      每个下载源重试次数 (默认 3)
  GSNODE_DOWNLOAD_RETRY_DELAY  重试间隔秒数 (默认 2)

参考数据 (ping_targets / dnsbl) 由探针自动切换:
  主源 $DataPrimary -> 备用 GitHub -> jsDelivr CDN
"@

if ($Help) {
    Write-Host $helpText
    return
}

$WorkDir = $null
$DataDir = $null
$TargetBin = $null
$DownloadedBin = $null
$InstalledThisRun = $false

function Invoke-Cleanup {
    if ($DataDir -and (Test-Path $DataDir) -and $PureMode) {
        Remove-Item -Recurse -Force $DataDir -ErrorAction SilentlyContinue
    }
    if ($PureMode) {
        if ($DownloadedBin -and (Test-Path $DownloadedBin)) {
            Remove-Item -Force $DownloadedBin -ErrorAction SilentlyContinue
        }
        if ($WorkDir -and (Test-Path $WorkDir)) {
            Remove-Item -Recurse -Force $WorkDir -ErrorAction SilentlyContinue
        }
        $diskTmp = Join-Path $env:TEMP 'gsprobe-disk.tmp'
        if (Test-Path $diskTmp) { Remove-Item -Force $diskTmp -ErrorAction SilentlyContinue }
        Write-Host ""
        Write-Host "-> 已清理本地检测临时文件（未在系统中安装 gsnode）"
    }
}

function Get-Sha256 {
    param([string]$Path)
    (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Test-Checksum {
    param([string]$BinPath, [string]$SumUrl)
    try {
        $raw = (Invoke-WebRequest -Uri $SumUrl -UseBasicParsing -TimeoutSec 30).Content
    } catch {
        Write-Host "-> 未找到校验和文件 ($SumUrl)，拒绝安装" -ForegroundColor Red
        return $false
    }
    # .sha256 sidecars are served as application/octet-stream; older Invoke-WebRequest
    # returns .Content as byte[] (not string) for such responses.
    if ($raw -is [byte[]]) {
        $sumText = [System.Text.Encoding]::UTF8.GetString($raw)
    } else {
        $sumText = [string]$raw
    }
    $expected = ($sumText.Trim() -split '\s+')[0].Trim().ToLowerInvariant()
    if (-not $expected) {
        Write-Host "-> 校验和文件为空 ($SumUrl)，拒绝安装" -ForegroundColor Red
        return $false
    }
    $actual = Get-Sha256 -Path $BinPath
    if ($expected -ne $actual) {
        Write-Host "-> 校验和不匹配: 期望 $expected，实际 $actual" -ForegroundColor Red
        return $false
    }
    return $true
}

function Get-BinaryTo {
    param([string]$Dest, [string]$BinName)
    $urlPrimary = "$DataPrimary/bin/$BinName"
    $urlFallback = "$BaseUrl/$BinName"

    $sources = @(
        @{ Url = $urlPrimary; Label = "主源 ($DataPrimary)" },
        @{ Url = $urlFallback; Label = "备用源 (GitHub)" }
    )

    foreach ($src in $sources) {
        if ($src -ne $sources[0]) {
            Write-Host "-> 主源不可用，切换至$($src.Label) ..."
        }
        for ($retry = 1; $retry -le $DownloadRetries; $retry++) {
            if ($retry -gt 1) {
                Write-Host "-> 下载失败，${RetryDelay}s 后重试 ($retry/$DownloadRetries) [$($src.Label)] ..."
                Start-Sleep -Seconds $RetryDelay
            } else {
                Write-Host "-> 正在下载 GSNode [$($src.Label)] ($retry/$DownloadRetries) ..."
            }

            Remove-Item -Force $Dest -ErrorAction SilentlyContinue

            try {
                Invoke-WebRequest -Uri $src.Url -OutFile $Dest -UseBasicParsing -TimeoutSec 600
                $ok = (Test-Path $Dest) -and ((Get-Item $Dest).Length -gt 0)
            } catch {
                $ok = $false
            }

            if ($ok) {
                if (Test-Checksum -BinPath $Dest -SumUrl "$($src.Url).sha256") {
                    Write-Host "-> 下载完成，校验和通过"
                    return
                }
                Write-Host "-> 校验和验证未通过，丢弃本次下载" -ForegroundColor Yellow
            }
            Remove-Item -Force $Dest -ErrorAction SilentlyContinue
        }
    }

    throw "无法从 $DataPrimary/bin 或 GitHub 下载并校验 $BinName（各源已重试 $DownloadRetries 次）"
}

try {
    $arch = 'amd64'
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -and -not $env:PROCESSOR_ARCHITEW6432) {
        Write-Host "-> 提示: 未提供原生 Windows ARM64 二进制，将通过 x64 模拟运行 amd64 版本" -ForegroundColor Yellow
    }
    $binName = "gsnode-windows-$arch.exe"

    Write-Host "===================================="
    Write-Host " GSNode 一键检测 (Windows)"
    Write-Host "===================================="
    Write-Host "版本     : v$Version"
    Write-Host "平台     : windows/$arch"
    $envGsnodeBin = Get-EnvOrDefault 'GSNODE_BIN' ''
    if ($PureMode -and -not $envGsnodeBin) {
        Write-Host "模式     : 纯净检测（完成后自动清理）"
    } elseif (-not $PureMode) {
        Write-Host "模式     : 保留安装"
    }
    Write-Host ""

    if ($envGsnodeBin -and (Test-Path $envGsnodeBin)) {
        $TargetBin = $envGsnodeBin
        $DataDir = Get-EnvOrDefault 'GSNODE_DATA' (Join-Path $env:TEMP "gsnode-$PID")
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
        Write-Host "-> 使用已有二进制: $TargetBin"
    } elseif (-not $PureMode) {
        $TargetBin = Join-Path $InstallDir 'gsnode.exe'
        if (Test-Path $TargetBin) {
            Write-Host "-> 使用已安装: $TargetBin"
        } else {
            New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
            $tmpDl = Join-Path $env:TEMP "gsnode-dl-$PID.exe"
            Get-BinaryTo -Dest $tmpDl -BinName $binName
            Move-Item -Force $tmpDl $TargetBin
            $InstalledThisRun = $true
            Write-Host "-> 已安装: $TargetBin"
        }
        $DataDir = Get-EnvOrDefault 'GSNODE_DATA' (Join-Path $env:TEMP "gsnode-$PID")
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
    } else {
        $WorkDir = Join-Path $env:TEMP "gsnode.$([guid]::NewGuid().ToString('N').Substring(0,8))"
        New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
        $DownloadedBin = Join-Path $WorkDir 'gsnode.exe'
        $DataDir = Join-Path $WorkDir 'data'
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
        $TargetBin = $DownloadedBin

        Get-BinaryTo -Dest $DownloadedBin -BinName $binName
        Write-Host "-> 已就绪: 临时二进制（检测后自动删除）"
        Write-Host ""
    }

    if ((Get-EnvOrDefault 'GSNODE_INSTALL_ONLY' '0') -eq '1') {
        Write-Host "安装完成。运行完整检测:"
        Write-Host "  & `"$TargetBin`" -run"
        return
    }

    $env:GSVPS_UPLOAD_URL = $GsvpsUploadUrl
    $env:GSVPS_SITE_URL = $GsvpsSiteUrl

    $provider = Get-EnvOrDefault 'GSNODE_HOST_PROVIDER' (Get-EnvOrDefault 'GSPROBE_HOST_PROVIDER' '')
    if (-not $provider) {
        try {
            $provider = Read-Host "-> 主机商名称（可选，直接回车跳过）"
        } catch {
            $provider = ''
        }
    }
    if ($provider) {
        Write-Host "-> 主机商: $provider（将随报告一并上传）"
    }

    Write-Host "-> 参考数据: $DataPrimary (失败自动切换 GitHub / jsDelivr)"
    Write-Host "-> 开始完整检测（约 3-8 分钟，进度见下方日志）"
    if ((Get-EnvOrDefault 'GSVPS_UPLOAD' '1') -ne '0') {
        Write-Host "-> 检测完成后将上传至 GSVPS 并显示在线报告链接"
    }
    if ($PureMode) {
        Write-Host "-> 检测结束后将自动清理所有本地文件"
    }
    Write-Host ""

    if ($provider) {
        & $TargetBin -run -data $DataDir -provider $provider
    } else {
        & $TargetBin -run -data $DataDir
    }
}
finally {
    Invoke-Cleanup
}
