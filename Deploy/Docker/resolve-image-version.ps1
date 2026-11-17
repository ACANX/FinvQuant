# =====================================================================
# FinvQuant 镜像版本识别辅助脚本（resolve-image-version.ps1）
# ---------------------------------------------------------------------
# 功能：给定镜像名（如 ghcr.io/acanx/finvquant）与 VERSION（如 0.1.0），
#   识别 latest 对应的 v{VERSION}-YYYYMMDDHHMM 版本号 与 构建时间。
#
# 原理（docker 无"列出远端 tag"命令，采用 digest 匹配探测）：
#   1) 取 latest 多平台 index 的 manifests digest 列表；
#   2) 由镜像构建时间（Created，UTC）推算北京分钟；
#   3) 探测 v0.1.0-{构建分钟-1/+0/+1} 三个候选 tag，
#      候选 tag 的单平台 manifest digest 命中 latest 的 digest 列表即确认版本号；
#   4) 探测不到则退化为返回构建时间（人可读标识）。
#
# 用法（cmd 调用）：
#   for /f ... in (`powershell -File resolve-image-version.ps1 -Image ghcr.io/acanx/finvquant -Version 0.1.0`)
# 输出：TAG=v0.1.0-202608062124   （识别到的版本号，空=未识别）
#       BJ=2026-08-06 21:25       （构建时间，北京）
# =====================================================================
param(
    [string]$Image = "ghcr.io/acanx/finvquant",
    [string]$Version = "0.1.0",
    [switch]$Friendly
)

$ErrorActionPreference = "SilentlyContinue"

# 1. latest 多平台 index 的 manifests digest 列表
$idxRaw = docker manifest inspect "$Image`:latest" 2>$null | Out-String
$idx = $null
try { $idx = ($idxRaw | ConvertFrom-Json).manifests.digest } catch {}
if (-not $idx) { $idx = @() }

# 2. 镜像构建时间（UTC）
$createdRaw = (docker image inspect "$Image`:latest" --format '{{.Created}}' 2>$null | Out-String).Trim()
$bjDisp = ""
try {
    $created = [datetime]$createdRaw
    $bj = $created.ToUniversalTime().AddHours(8)  # 北京 = UTC+8
    $bjDisp = $bj.ToString("yyyy-MM-dd HH:mm")
} catch { $bjDisp = $createdRaw }

# 3. 探测候选时间戳 tag（分钟 -1/+0/+1，覆盖 CI ResolveImageTags 与构建完成时刻偏差）
$foundTag = ""
if ($idx.Count -gt 0) {
    foreach ($off in @(-1, 0, 1)) {
        $min = $bj.AddMinutes($off).ToString("yyyyMMddHHmm")
        $tag = "v$Version-$min"
        $mRaw = docker manifest inspect -v "$Image`:$tag" 2>$null | Out-String
        if (-not $mRaw) { continue }
        try {
            $m = $mRaw | ConvertFrom-Json
            $dig = $m[0].Descriptor.digest
            if ($idx -contains $dig) { $foundTag = $tag; break }
        } catch {}
    }
}

# 4. 输出结果（供 cmd for /f 解析）
if ($Friendly) {
    # 友好模式：输出单行中文版本信息（upgrade.cmd 直接展示，避免 cmd 解析多行）
    if ($foundTag) {
        Write-Output "版本号: $foundTag  (构建时间: $bjDisp)"
    } else {
        Write-Output "版本号: (未识别到时间戳tag)  构建时间: $bjDisp"
    }
} else {
    Write-Output "TAG=$foundTag"
    Write-Output "BJ=$bjDisp"
}
