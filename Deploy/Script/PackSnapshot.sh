#!/usr/bin/env bash
# =====================================================================
# FinvQuant 快照包构建脚本
#
# 唯一打包实现：CI（.github/workflows/CI.yml 的 snapshot-binaries）
# 与本地手工出包都调用本脚本，避免两处逻辑漂移。
#
# 版本标识：v{BASE_VERSION}.{PPPPP}.{yymmddHHmm}
#   BASE_VERSION 取自 VERSION 文件（如 0.1.1）
#   PPPPP        关联 PR 编号，5 位左补零；无 PR 为 00000
#   yymmddHHmm   构建时间戳（Asia/Shanghai）
#
# 用法：
#   bash Deploy/Script/PackSnapshot.sh [选项]
#
#   --version <v>       快照版本（缺省则用 --pr/--timestamp + VERSION 现算）
#   --target <t>        linux/amd64 | linux/arm64 | windows/amd64 | all（默认 all）
#   --pr <n>            关联 PR 编号（默认 00000）
#   --timestamp <ts>    时间戳 yymmddHHmm（默认当前 Asia/Shanghai 时间）
#   --base-version <v>  基础版本（默认读 VERSION 文件）
#   --commit <sha>      提交号（默认 git rev-parse --short HEAD）
#   --out <dir>         输出目录（默认 <仓库根>/dist）
#   --build-web         强制重新构建前端（默认复用 internal/webui/dist）
#   --keep-tmp          保留 pkg/ 临时目录便于检查
#   -h, --help          帮助
#
# 示例：
#   # 本地出全部三个包（无 PR 场景）
#   bash Deploy/Script/PackSnapshot.sh
#
#   # 指定快照版本，只出 Ubuntu arm64
#   bash Deploy/Script/PackSnapshot.sh \
#     --version v0.1.1.00123.2610241530 --target linux/arm64
# =====================================================================
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
cd "${ROOT}"

SNAPSHOT_VERSION="${SNAPSHOT_VERSION:-}"
BASE_VERSION=""
PR=""
TIMESTAMP=""
COMMIT=""
TARGET="all"
OUT_DIR="${ROOT}/dist"
BUILD_WEB=0
KEEP_TMP=0

log()  { printf '[pack] %s\n' "$*"; }
warn() { printf '[pack] 警告：%s\n' "$*" >&2; }
die()  { printf '[pack] 错误：%s\n' "$*" >&2; exit 1; }

usage() {
    sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)      SNAPSHOT_VERSION="$2"; shift 2 ;;
        --target)       TARGET="$2"; shift 2 ;;
        --pr)           PR="$2"; shift 2 ;;
        --timestamp)    TIMESTAMP="$2"; shift 2 ;;
        --base-version) BASE_VERSION="$2"; shift 2 ;;
        --commit)       COMMIT="$2"; shift 2 ;;
        --out)          OUT_DIR="$2"; shift 2 ;;
        --build-web)    BUILD_WEB=1; shift ;;
        --keep-tmp)     KEEP_TMP=1; shift ;;
        -h|--help)      usage ;;
        *)              die "未知参数：$1（用 --help 查看用法）" ;;
    esac
done

# ---------------------------------------------------------------------
# 1. 解析快照版本
# ---------------------------------------------------------------------
[[ -f "${ROOT}/VERSION" ]] || die "缺少 VERSION 文件"
BASE_VERSION="${BASE_VERSION:-$(tr -d '[:space:]' < "${ROOT}/VERSION")}"

if [[ -n "${SNAPSHOT_VERSION}" ]]; then
    [[ "${SNAPSHOT_VERSION}" == v* ]] || SNAPSHOT_VERSION="v${SNAPSHOT_VERSION}"
    if [[ ! "${SNAPSHOT_VERSION}" =~ ^v([0-9]+(\.[0-9]+)+)\.([0-9]{5})\.([0-9]{10})$ ]]; then
        die "版本格式不合法：${SNAPSHOT_VERSION}（应为 v{BASE}.{PPPPP}.{yymmddHHmm}）"
    fi
    BASE_VERSION="${BASH_REMATCH[1]}"
    PR="${BASH_REMATCH[3]}"
    TIMESTAMP="${BASH_REMATCH[4]}"
else
    TIMESTAMP="${TIMESTAMP:-$(TZ=Asia/Shanghai date +%y%m%d%H%M)}"
    [[ "${TIMESTAMP}" =~ ^[0-9]{10}$ ]] || die "时间戳格式不合法：${TIMESTAMP}（应为 yymmddHHmm）"
    if [[ ! "${PR}" =~ ^[0-9]+$ ]]; then
        [[ -n "${PR}" ]] && warn "PR 编号「${PR}」非纯数字，按 00000 处理"
        PR=0
    fi
    PR=$(printf '%05d' "${PR}")
    SNAPSHOT_VERSION="v${BASE_VERSION}.${PR}.${TIMESTAMP}"
fi

COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo dev)}"

log "快照版本   : ${SNAPSHOT_VERSION}"
log "基础版本   : ${BASE_VERSION}"
log "PR 编号    : ${PR}"
log "时间戳     : ${TIMESTAMP} (Asia/Shanghai)"
log "提交       : ${COMMIT}"

# ---------------------------------------------------------------------
# 2. 前端产物：默认复用 internal/webui/dist，必要时显式重建
# ---------------------------------------------------------------------
if [[ "${BUILD_WEB}" == "1" ]]; then
    command -v npm >/dev/null 2>&1 || die "需要 npm 构建前端（或去掉 --build-web 复用已有 dist）"
    log "构建前端（Web/）..."
    ( cd Web && npm ci --no-audit --no-fund && npm run build )
fi
[[ -f "${ROOT}/internal/webui/dist/index.html" ]] \
    || die "缺少前端产物 internal/webui/dist/index.html；请先构建前端或加 --build-web"

# ---------------------------------------------------------------------
# 3. 工具与 Go 环境探测（兼容 WSL 下调用 Windows 版 go）
# ---------------------------------------------------------------------
# WSL 下通常只有 go.exe（无 go 这个无扩展名文件），两者都要探测
GO_CMD=""
for cand in go go.exe; do
    if command -v "${cand}" >/dev/null 2>&1; then GO_CMD="${cand}"; break; fi
done
[[ -n "${GO_CMD}" ]] || die "未找到 go 命令（请安装 Go 1.25+ 并加入 PATH）"

GO_IS_WINDOWS=0
if "${GO_CMD}" version 2>/dev/null | grep -qi 'windows/'; then
    GO_IS_WINDOWS=1
    command -v wslpath >/dev/null 2>&1 || die "检测到 Windows 版 go（${GO_CMD}），但缺少 wslpath（无法转换输出路径）"
    log "Go 环境     : ${GO_CMD}（Windows 版，经 WSLENV 传递 GOOS/GOARCH）"
else
    log "Go 环境     : ${GO_CMD}（原生）"
fi

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"; }

# Windows 版 go 无法直接写 WSL 路径，需转成 Windows 路径
win_path() {
    if [[ "${GO_IS_WINDOWS}" == "1" ]]; then wslpath -w "$1"; else printf '%s' "$1"; fi
}

build_binary() {
    local goos="$1" goarch="$2" out="$3"
    local ver="${SNAPSHOT_VERSION#v}"
    if [[ "${GO_IS_WINDOWS}" == "1" ]]; then
        WSLENV=GOOS/w:GOARCH/w:CGO_ENABLED/w \
        GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
            "${GO_CMD}" build -ldflags "-s -w -X main.version=${ver} -X main.commit=${COMMIT}" \
            -o "$(win_path "${out}")" ./cmd/server
    else
        CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
            "${GO_CMD}" build -ldflags "-s -w -X main.version=${ver} -X main.commit=${COMMIT}" \
            -o "${out}" ./cmd/server
    fi
}

# ---------------------------------------------------------------------
# 4. 目标平台 → 包命名元素
# ---------------------------------------------------------------------
target_meta() {
    local t="$1"
    case "${t}" in
        linux/amd64)   echo "linux amd64 ubuntu2604 '' tar.gz" ;;
        linux/arm64)   echo "linux arm64 ubuntu2604 '' tar.gz" ;;
        windows/amd64) echo "windows amd64 win11 .exe zip" ;;
        *) die "不支持的目标平台：${t}（可选 linux/amd64、linux/arm64、windows/amd64、all）" ;;
    esac
}

if [[ "${TARGET}" == "all" ]]; then
    TARGETS=("linux/amd64" "linux/arm64" "windows/amd64")
else
    TARGETS=("${TARGET}")
fi

mkdir -p "${OUT_DIR}"
OUT_DIR=$(cd "${OUT_DIR}" && pwd)

# ---------------------------------------------------------------------
# 5. 逐平台构建 + 打包
# ---------------------------------------------------------------------
for T in "${TARGETS[@]}"; do
    read -r GOOS_V GOARCH_V OS_LABEL EXT ARCHIVE <<< "$(target_meta "${T}")"
    NAME="finvquant_${SNAPSHOT_VERSION}_${OS_LABEL}_${GOARCH_V}"
    PKG_ROOT="${ROOT}/pkg/${NAME}"

    log "── 构建 ${T} → ${NAME}.${ARCHIVE}"

    rm -rf "${PKG_ROOT}"
    mkdir -p "${PKG_ROOT}/Deploy"

    build_binary "${GOOS_V}" "${GOARCH_V}" "${ROOT}/pkg/finvquant${EXT}"
    cp "${ROOT}/pkg/finvquant${EXT}" "${PKG_ROOT}/"
    rm -f "${ROOT}/pkg/finvquant${EXT}"

    cp -r Deploy/Migrations "${PKG_ROOT}/Deploy/Migrations"
    cp    Deploy/README.md "${PKG_ROOT}/Deploy/README.md"

    if [[ "${GOOS_V}" == "linux" ]]; then
        # Linux：宿主直装（systemd）全套脚本与文档
        mkdir -p "${PKG_ROOT}/Deploy/Native"
        cp Deploy/Native/*.sh Deploy/Native/*.md Deploy/Native/*.service \
           Deploy/Native/*.example "${PKG_ROOT}/Deploy/Native/"
    else
        # Windows：容器部署（compose 拉 GHCR 镜像），不带 Dockerfile
        mkdir -p "${PKG_ROOT}/Deploy/Docker"
        cp -r Deploy/Docker/. "${PKG_ROOT}/Deploy/Docker/"
        rm -f "${PKG_ROOT}/Deploy/Docker/.env" \
              "${PKG_ROOT}/Deploy/Docker/.env.bak" \
              "${PKG_ROOT}/Deploy/Docker/Dockerfile"
    fi

    printf '%s\n' "${SNAPSHOT_VERSION#v}" > "${PKG_ROOT}/VERSION"

    local_sys="Windows 11 (x64)"
    [[ "${GOOS_V}" == "linux" ]] && { [[ "${GOARCH_V}" == "arm64" ]] && local_sys="Ubuntu 26.04 (aarch64)" || local_sys="Ubuntu 26.04 (x86_64)"; }
    cat > "${PKG_ROOT}/BUILD-INFO.txt" <<EOF
FinvQuant 快照包
==================
版本        : ${SNAPSHOT_VERSION}
基础版本    : ${BASE_VERSION}
PR 编号     : ${PR}
构建时间戳  : ${TIMESTAMP} (Asia/Shanghai)
提交        : ${COMMIT}
目标平台    : ${GOOS_V}/${GOARCH_V}
适用系统    : ${local_sys}
构建方式    : Deploy/Script/PackSnapshot.sh${CI:+ (CI run ${CI_RUN_ID:-unknown})}
EOF

    ( cd "${PKG_ROOT}" && find . -type f ! -name SHA256SUMS -print0 | sort -z \
        | xargs -0 sha256sum > SHA256SUMS )

    if [[ "${ARCHIVE}" == "zip" ]]; then
        need_cmd zip
        ( cd "${ROOT}/pkg" && zip -qr "${OUT_DIR}/${NAME}.zip" "${NAME}" )
    else
        ( cd "${ROOT}/pkg" && tar -czf "${OUT_DIR}/${NAME}.tar.gz" "${NAME}" )
    fi
    ( cd "${OUT_DIR}" && sha256sum "${NAME}.${ARCHIVE}" > "${NAME}.${ARCHIVE}.sha256" )

    log "   完成：$(cd "${OUT_DIR}" && ls -lh "${NAME}.${ARCHIVE}" | awk '{print $5}')"
done

[[ "${KEEP_TMP}" == "1" ]] || rm -rf "${ROOT}/pkg"

log "全部完成，产物位于 ${OUT_DIR}"
( cd "${OUT_DIR}" && ls -1 ./*.tar.gz ./*.zip ./*.sha256 2>/dev/null || true )
