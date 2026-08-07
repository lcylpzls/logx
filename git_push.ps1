# logx 自动发版脚本（PowerShell 版，与 git_push.sh 等价）
$ErrorActionPreference = "Stop"

$latestTag = git describe --tags --abbrev=0 2>$null
if (-not $latestTag) {
    $latestTag = "无"
}

Write-Host "=========================================="
Write-Host "  logx 自动发版脚本"
Write-Host "=========================================="
Write-Host ""
Write-Host "  当前最新 tag: $latestTag"
Write-Host ""

$version = Read-Host "  请输入即将发布的版本号 (例如 0.10.0)"
if ($version -notmatch '^\d+\.\d+\.\d+$') {
    Write-Host "错误: 版本号格式不正确，请使用 x.y.z 格式"
    exit 1
}

$tag = "v$version"
git rev-parse --verify "$tag" *> $null
if ($LASTEXITCODE -eq 0) {
    Write-Host "错误: tag ${tag} 已存在，请使用其他版本号"
    exit 1
}

Write-Host ""
Write-Host "  即将执行以下操作:"
Write-Host "    1. git push origin main"
Write-Host "    2. git tag ${tag}"
Write-Host "    3. git push origin ${tag}"
Write-Host ""

$confirm = Read-Host "  确认继续? (y/n)"
if ($confirm -notin @("y", "Y")) {
    Write-Host "已取消"
    exit 0
}

Write-Host ">>> 推送代码到 origin/main ..."
git push origin main

Write-Host ">>> 创建 tag ${tag} ..."
git tag "${tag}"

Write-Host ">>> 推送 tag ${tag} 到 origin ..."
git push origin "${tag}"

Write-Host ""
Write-Host "=========================================="
Write-Host "  发版完成！"
Write-Host ""
Write-Host "  用户可通过以下命令使用此版本:"
Write-Host "    go get github.com/lcylpzls/logx@${tag}"
Write-Host "=========================================="
