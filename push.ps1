# Push GSNode to GitHub (via local proxy if needed)
$ErrorActionPreference = 'Stop'
$proxy = $env:GSNODE_PROXY
if (-not $proxy) { $proxy = 'http://127.0.0.1:10809' }
$env:HTTP_PROXY = $proxy
$env:HTTPS_PROXY = $proxy
Set-Location $PSScriptRoot
git -c http.proxy=$proxy -c https.proxy=$proxy push origin main
git -c http.proxy=$proxy -c https.proxy=$proxy push origin --tags
Write-Host 'Done. Tags:' -ForegroundColor Green
git tag -l
