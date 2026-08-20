# GSNode 纯净一键检测 (Windows)：临时下载 -> 完整检测 -> 上传 GSVPS -> 终端输出 -> 自动清理
# GSNode pure one-line scan (Windows): temp download -> full scan -> upload to GSVPS -> print -> auto-clean
# Usage:
#   irm https://dl.gsvps.com/install.ps1 | iex
# 备用地址 / Fallback URLs:
#   irm https://github.com/gsvps/GSNode/raw/main/install.ps1 | iex
#   irm https://cdn.jsdelivr.net/gh/gsvps/GSNode@main/install.ps1 | iex

[CmdletBinding()]
param(
    [switch]$Help
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# Non-UTF-8 console codepages (common on non-Chinese Windows locales) can mangle
# the Chinese status text; switching the console to UTF-8 fixes it where the
# console font has CJK glyphs. Where it doesn't, output falls back to English
# below (see $Lang detection) rather than showing tofu boxes.
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
} catch {}

function Get-EnvOrDefault {
    param([string]$Name, [string]$Default)
    $v = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrEmpty($v)) { return $Default }
    return $v
}

# Language: GSNODE_LANG overrides; otherwise follow the OS display language.
# Non-Chinese Windows consoles usually lack CJK glyphs, so default to English there.
$Lang = 'zh'
$langOverride = Get-EnvOrDefault 'GSNODE_LANG' ''
if ($langOverride) {
    $Lang = if ($langOverride -like 'zh*') { 'zh' } else { 'en' }
} else {
    try {
        $uiName = [System.Globalization.CultureInfo]::CurrentUICulture.Name
        if ($uiName -notlike 'zh*') { $Lang = 'en' }
    } catch {}
}

function Say {
    param([string]$Zh, [string]$En, [string]$Color)
    $msg = if ($Lang -eq 'zh') { $Zh } else { $En }
    if ($Color) { Write-Host $msg -ForegroundColor $Color } else { Write-Host $msg }
}

$Version         = Get-EnvOrDefault 'GSNODE_VERSION' '0.1.30'
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

$helpTextZh = @"
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
  GSNODE_LANG            输出语言 zh/en（默认跟随系统显示语言）

参考数据 (ping_targets / dnsbl) 由探针自动切换:
  主源 $DataPrimary -> 备用 GitHub -> jsDelivr CDN
"@

$helpTextEn = @"
GSNode pure one-line scan (Windows)

Usage:
  irm https://dl.gsvps.com/install.ps1 | iex

Fallback URLs:
  irm https://github.com/gsvps/GSNode/raw/main/install.ps1 | iex
  irm https://cdn.jsdelivr.net/gh/gsvps/GSNode@main/install.ps1 | iex

Default behavior (pure mode):
  Temporarily download the probe -> full scan -> upload to GSVPS -> print results -> auto-clean all local files
  gsnode is never permanently installed, and no report cache is left behind.

Environment variables:
  GSNODE_VERSION        Version (default $Version)
  GSVPS_UPLOAD_URL      Upload API (default $GsvpsUploadUrl)
  GSVPS_SITE_URL        Site URL (default $GsvpsSiteUrl)
  GSVPS_UPLOAD=0        Disable upload
  GSNODE_KEEP=1         Keep gsnode installed at $InstallDir\gsnode.exe after the scan
  GSNODE_INSTALL_ONLY=1 Install only, skip the scan (use with GSNODE_KEEP=1)
  GSNODE_BIN            Use an existing binary path
  GSNODE_DATA           Custom report cache directory
  GSNODE_HOST_PROVIDER  Host provider name (preset for non-interactive installs, skips the prompt)
  GSNODE_DATA_PRIMARY   Primary data/binary download source (default https://dl.gsvps.com)
  GSNODE_DOWNLOAD_RETRIES      Retries per download source (default 3)
  GSNODE_DOWNLOAD_RETRY_DELAY  Seconds between retries (default 2)
  GSNODE_LANG            Output language zh/en (default follows OS display language)

Reference data (ping_targets / dnsbl) auto-fallback inside the probe:
  Primary $DataPrimary -> fallback GitHub -> jsDelivr CDN
"@

if ($Help) {
    Write-Host $(if ($Lang -eq 'zh') { $helpTextZh } else { $helpTextEn })
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
        Say '-> 已清理本地检测临时文件（未在系统中安装 gsnode）' '-> Cleaned up local temp files (gsnode was not installed on this system)'
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
        Say "-> 未找到校验和文件 ($SumUrl)，拒绝安装" "-> Checksum file not found ($SumUrl), refusing to install" 'Red'
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
        Say "-> 校验和文件为空 ($SumUrl)，拒绝安装" "-> Checksum file is empty ($SumUrl), refusing to install" 'Red'
        return $false
    }
    $actual = Get-Sha256 -Path $BinPath
    if ($expected -ne $actual) {
        Say "-> 校验和不匹配: 期望 $expected，实际 $actual" "-> Checksum mismatch: expected $expected, got $actual" 'Red'
        return $false
    }
    return $true
}

function Get-BinaryTo {
    param([string]$Dest, [string]$BinName)
    $urlPrimary = "$DataPrimary/bin/$BinName"
    $urlFallback = "$BaseUrl/$BinName"

    $primaryLabel = if ($Lang -eq 'zh') { "主源 ($DataPrimary)" } else { "primary ($DataPrimary)" }
    $fallbackLabel = if ($Lang -eq 'zh') { "备用源 (GitHub)" } else { "fallback (GitHub)" }
    $sources = @(
        @{ Url = $urlPrimary; Label = $primaryLabel },
        @{ Url = $urlFallback; Label = $fallbackLabel }
    )

    foreach ($src in $sources) {
        if ($src -ne $sources[0]) {
            Say "-> 主源不可用，切换至$($src.Label) ..." "-> Primary source unavailable, switching to $($src.Label) ..."
        }
        for ($retry = 1; $retry -le $DownloadRetries; $retry++) {
            if ($retry -gt 1) {
                Say "-> 下载失败，${RetryDelay}s 后重试 ($retry/$DownloadRetries) [$($src.Label)] ..." "-> Download failed, retrying in ${RetryDelay}s ($retry/$DownloadRetries) [$($src.Label)] ..."
                Start-Sleep -Seconds $RetryDelay
            } else {
                Say "-> 正在下载 GSNode [$($src.Label)] ($retry/$DownloadRetries) ..." "-> Downloading GSNode [$($src.Label)] ($retry/$DownloadRetries) ..."
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
                    Say '-> 下载完成，校验和通过' '-> Download complete, checksum verified'
                    return
                }
                Say '-> 校验和验证未通过，丢弃本次下载' '-> Checksum verification failed, discarding this download' 'Yellow'
            }
            Remove-Item -Force $Dest -ErrorAction SilentlyContinue
        }
    }

    $msg = if ($Lang -eq 'zh') {
        "无法从 $DataPrimary/bin 或 GitHub 下载并校验 $BinName（各源已重试 $DownloadRetries 次）"
    } else {
        "Unable to download and verify $BinName from $DataPrimary/bin or GitHub (each source retried $DownloadRetries times)"
    }
    throw $msg
}

try {
    $arch = 'amd64'
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -and -not $env:PROCESSOR_ARCHITEW6432) {
        Say '-> 提示: 未提供原生 Windows ARM64 二进制，将通过 x64 模拟运行 amd64 版本' '-> Note: no native Windows ARM64 binary yet; running the amd64 build via x64 emulation' 'Yellow'
    }
    $binName = "gsnode-windows-$arch.exe"

    Write-Host "===================================="
    Say ' GSNode 一键检测 (Windows)' ' GSNode one-line scan (Windows)'
    Write-Host "===================================="
    Say "版本     : v$Version" "Version  : v$Version"
    Say "平台     : windows/$arch" "Platform : windows/$arch"
    $envGsnodeBin = Get-EnvOrDefault 'GSNODE_BIN' ''
    if ($PureMode -and -not $envGsnodeBin) {
        Say '模式     : 纯净检测（完成后自动清理）' 'Mode     : pure scan (auto-clean when done)'
    } elseif (-not $PureMode) {
        Say '模式     : 保留安装' 'Mode     : keep install'
    }
    Write-Host ""

    if ($envGsnodeBin -and (Test-Path $envGsnodeBin)) {
        $TargetBin = $envGsnodeBin
        $DataDir = Get-EnvOrDefault 'GSNODE_DATA' (Join-Path $env:TEMP "gsnode-$PID")
        New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
        Say "-> 使用已有二进制: $TargetBin" "-> Using existing binary: $TargetBin"
    } elseif (-not $PureMode) {
        $TargetBin = Join-Path $InstallDir 'gsnode.exe'
        if (Test-Path $TargetBin) {
            Say "-> 使用已安装: $TargetBin" "-> Using installed: $TargetBin"
        } else {
            New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
            $tmpDl = Join-Path $env:TEMP "gsnode-dl-$PID.exe"
            Get-BinaryTo -Dest $tmpDl -BinName $binName
            Move-Item -Force $tmpDl $TargetBin
            $InstalledThisRun = $true
            Say "-> 已安装: $TargetBin" "-> Installed: $TargetBin"
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
        Say '-> 已就绪: 临时二进制（检测后自动删除）' '-> Ready: temporary binary (auto-deleted after the scan)'
        Write-Host ""
    }

    if ((Get-EnvOrDefault 'GSNODE_INSTALL_ONLY' '0') -eq '1') {
        Say '安装完成。运行完整检测:' 'Install complete. Run a full scan:'
        Write-Host "  & `"$TargetBin`" -run"
        return
    }

    $env:GSVPS_UPLOAD_URL = $GsvpsUploadUrl
    $env:GSVPS_SITE_URL = $GsvpsSiteUrl

    $provider = Get-EnvOrDefault 'GSNODE_HOST_PROVIDER' (Get-EnvOrDefault 'GSPROBE_HOST_PROVIDER' '')
    if (-not $provider) {
        try {
            $promptText = if ($Lang -eq 'zh') { '-> 主机商名称（可选，直接回车跳过）' } else { '-> Host provider name (optional, press Enter to skip)' }
            $provider = Read-Host $promptText
        } catch {
            $provider = ''
        }
    }
    if ($provider) {
        Say "-> 主机商: $provider（将随报告一并上传）" "-> Host provider: $provider (included in the uploaded report)"
    }

    Say "-> 参考数据: $DataPrimary (失败自动切换 GitHub / jsDelivr)" "-> Reference data: $DataPrimary (auto-fallback to GitHub / jsDelivr)"
    Say '-> 开始完整检测（约 3-8 分钟，进度见下方日志）' '-> Starting full scan (about 3-8 minutes, see log below)'
    if ((Get-EnvOrDefault 'GSVPS_UPLOAD' '1') -ne '0') {
        Say '-> 检测完成后将上传至 GSVPS 并显示在线报告链接' '-> Report will be uploaded to GSVPS and the online link printed when done'
    }
    if ($PureMode) {
        Say '-> 检测结束后将自动清理所有本地文件' '-> All local files will be auto-cleaned when the scan finishes'
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
