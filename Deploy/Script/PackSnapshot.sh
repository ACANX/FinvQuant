#!/usr/bin/env bash
# =====================================================================
# FinvQuant 快照包构建脚本
#
# 唯一打包实现：CI（.github/workflows/CI.yml 的 snapshot-binaries / pr-binaries /
# release-binaries）与本地手工出包都调用本脚本，避免四处逻辑漂移。
#
# 支持三种包类型（由 --kind 或版本格式自动识别）：
#   snapshot 快照包    v{BASE}.{PPPPP}.{yymmddHHmm}   推送代码到分支时发布
#   pr       构建包    v{BASE}.{PPPPP}               PR 合并时发布（同一 PR 重复合并覆盖重建）
#   release  正式版包  v{BASE}                       代码合并到 main 时发布
#   BASE 取自 VERSION 文件（如 0.1.3）；PPPPP 为关联 PR 编号，5 位左补零（无 PR 为 00000）
#   yymmddHHmm 为构建时间戳（Asia/Shanghai）
#
# 用法：
#   bash Deploy/Script/PackSnapshot.sh [选项]
#
#   --version <v>       包版本（缺省则用 --kind/--pr/--timestamp + VERSION 现算）
#   --kind <k>          snapshot | pr | release（默认按版本格式自动识别）
#   --target <t>        linux/amd64 | linux/arm64 | windows/amd64 | all（默认 all）
#   --pr <n>            关联 PR 编号（默认 00000）
#   --timestamp <ts>    时间戳 yymmddHHmm（默认当前 Asia/Shanghai 时间）
#   --base-version <v>  基础版本（默认读 VERSION 文件）
#   --commit <sha>      提交号（默认 git rev-parse --short HEAD）
#   --out <dir>         输出目录（默认 <仓库根>/Dist）
#   --build-web         强制重新构建前端（默认复用 internal/webui/dist）
#   --keep-tmp          保留 pkg/ 临时目录便于检查
#   -h, --help          帮助
#
# 可选环境变量（CI 使用，本地留空即可）：
#   CI_RUN_ID       流水线运行号，写入 BUILD-INFO.txt
#   BUILD_REPO      owner/repo，写入 BUILD-INFO.txt 的仓库链接
#   BUILD_RUN_URL   流水线地址，写入 BUILD-INFO.txt
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
KIND=""
BASE_VERSION=""
PR=""
TIMESTAMP=""
COMMIT=""
TARGET="all"
OUT_DIR="${ROOT}/Dist"
BUILD_WEB=0
KEEP_TMP=0

log()  { printf '[pack] %s\n' "$*"; }
warn() { printf '[pack] 警告：%s\n' "$*" >&2; }
die()  { printf '[pack] 错误：%s\n' "$*" >&2; exit 1; }

usage() {
    sed -n '2,42p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)      SNAPSHOT_VERSION="$2"; shift 2 ;;
        --kind)         KIND="$2"; shift 2 ;;
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
    if [[ "${SNAPSHOT_VERSION}" =~ ^v([0-9]+(\.[0-9]+)+)\.([0-9]{5})\.([0-9]{10})$ ]]; then
        DETECTED_KIND="snapshot"
        BASE_VERSION="${BASH_REMATCH[1]}"
        PR="${BASH_REMATCH[3]}"
        TIMESTAMP="${BASH_REMATCH[4]}"
    elif [[ "${SNAPSHOT_VERSION}" =~ ^v([0-9]+(\.[0-9]+)+)\.([0-9]{5})$ ]]; then
        DETECTED_KIND="pr"
        BASE_VERSION="${BASH_REMATCH[1]}"
        PR="${BASH_REMATCH[3]}"
        TIMESTAMP=""
    elif [[ "${SNAPSHOT_VERSION}" =~ ^v([0-9]+(\.[0-9]+)+)$ ]]; then
        DETECTED_KIND="release"
        BASE_VERSION="${BASH_REMATCH[1]}"
        PR=""
        TIMESTAMP=""
    else
        die "版本格式不合法：${SNAPSHOT_VERSION}"
    fi
    if [[ -n "${KIND}" && "${KIND}" != "${DETECTED_KIND}" ]]; then
        warn "--kind=${KIND} 与版本格式不符，按 ${DETECTED_KIND} 处理"
    fi
    KIND="${DETECTED_KIND}"
else
    KIND="${KIND:-snapshot}"
    case "${KIND}" in
        snapshot|pr|release) ;;
        *) die "--kind 取值不合法：${KIND}（应为 snapshot / pr / release）" ;;
    esac
    TIMESTAMP="${TIMESTAMP:-$(TZ=Asia/Shanghai date +%y%m%d%H%M)}"
    if [[ "${KIND}" != "release" ]]; then
        if [[ ! "${PR}" =~ ^[0-9]+$ ]]; then
            [[ -n "${PR}" ]] && warn "PR 编号「${PR}」非纯数字，按 00000 处理"
            PR=0
        fi
        PR=$(printf '%05d' "${PR}")
    fi
    case "${KIND}" in
        snapshot) SNAPSHOT_VERSION="v${BASE_VERSION}.${PR}.${TIMESTAMP}" ;;
        pr)       SNAPSHOT_VERSION="v${BASE_VERSION}.${PR}"; TIMESTAMP="" ;;
        release)  SNAPSHOT_VERSION="v${BASE_VERSION}"; PR=""; TIMESTAMP="" ;;
    esac
fi

COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo dev)}"

log "包类型     : ${KIND}"
log "包版本     : ${SNAPSHOT_VERSION}"
log "基础版本   : ${BASE_VERSION}"
log "PR 编号    : ${PR:-(不适用)}"
log "时间戳     : ${TIMESTAMP:-(不适用)}"
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
# 运行环境：native（Linux/macOS）/ wsl（WSL）/ msys（Git Bash、MSYS2、Cygwin）
ENV_KIND="native"
case "$(uname -s 2>/dev/null || echo unknown)" in
    MSYS*|MINGW*|CYGWIN*) ENV_KIND="msys" ;;
    *)
        if [[ -n "${WSL_DISTRO_NAME:-}" ]] || grep -qi microsoft /proc/version 2>/dev/null; then
            ENV_KIND="wsl"
        fi
        ;;
esac

# WSL / Git Bash 下通常只有 go.exe（无无扩展名的 go），两者都探测
GO_CMD=""
for cand in go go.exe; do
    if command -v "${cand}" >/dev/null 2>&1; then GO_CMD="${cand}"; break; fi
done
[[ -n "${GO_CMD}" ]] || die "未找到 go 命令（请安装 Go 1.25+ 并加入 PATH）"

GO_IS_WINDOWS=0
if "${GO_CMD}" version 2>/dev/null | grep -qi 'windows/'; then
    GO_IS_WINDOWS=1
fi

# Windows 版 go 认 Windows 路径：WSL 用 wslpath、Git Bash 用 cygpath 转换
to_win_path() {
    local p="$1"
    case "${ENV_KIND}" in
        wsl)  command -v wslpath >/dev/null 2>&1 && wslpath -w "${p}" || printf '%s' "${p}" ;;
        msys) command -v cygpath >/dev/null 2>&1 && cygpath -w "${p}" || printf '%s' "${p}" ;;
        *)    printf '%s' "${p}" ;;
    esac
}

if [[ "${GO_IS_WINDOWS}" == "1" && "${ENV_KIND}" == "wsl" ]]; then
    command -v wslpath >/dev/null 2>&1 || die "检测到 Windows 版 go（${GO_CMD}），但缺少 wslpath（无法转换输出路径）"
fi
if [[ "${GO_IS_WINDOWS}" == "1" && "${ENV_KIND}" == "msys" ]]; then
    command -v cygpath >/dev/null 2>&1 || die "检测到 Windows 版 go（${GO_CMD}），但缺少 cygpath（无法转换输出路径）"
fi

case "${ENV_KIND}" in
    wsl)  log "运行环境    : WSL（GOOS/GOARCH 经 WSLENV 传递，路径用 wslpath 转换）" ;;
    msys) log "运行环境    : Git Bash / MSYS（路径用 cygpath 转换）" ;;
    *)    log "运行环境    : 原生（Linux/macOS）" ;;
esac
log "Go 环境     : ${GO_CMD}$([[ "${GO_IS_WINDOWS}" == "1" ]] && echo "（Windows 版）" || echo "（原生）")"

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"; }

# 定位 Windows 自带的 bsdtar（Win10 1803+）。它生成的 zip 用正斜杠，
# 比 PowerShell Compress-Archive（历史 bug：写入反斜杠，跨平台解压会错乱）标准。
find_win_tar() {
    local c=""
    case "${ENV_KIND}" in
        msys) c="/c/Windows/System32/tar.exe" ;;
        wsl)  c="/mnt/c/Windows/System32/tar.exe" ;;
        *)    return 1 ;;
    esac
    [[ -x "${c}" ]] && { printf '%s' "${c}"; return 0; }
    return 1
}

# 打 zip 包：按可用性依次尝试 zip → Windows bsdtar → PowerShell → python3
make_zip() {
    local parent="$1" name="$2" out="$3"
    local win_tar=""
    if command -v zip >/dev/null 2>&1; then
        ( cd "${parent}" && zip -qr "${out}" "${name}" )
    elif win_tar="$(find_win_tar)"; then
        # -a：按输出扩展名自动选择 zip 格式
        ( cd "${parent}" && "${win_tar}" -a -c -f "$(to_win_path "${out}")" "${name}" )
    elif command -v powershell.exe >/dev/null 2>&1; then
        local src_win out_win
        src_win="$(to_win_path "${parent}/${name}")"
        out_win="$(to_win_path "${out}")"
        powershell.exe -NoProfile -NonInteractive -Command \
            "Compress-Archive -LiteralPath '${src_win}' -DestinationPath '${out_win}' -Force" >/dev/null
    elif command -v python3 >/dev/null 2>&1; then
        python3 - "${parent}" "${name}" "${out}" <<'PYPACK'
import os, sys, zipfile
parent, name, out = sys.argv[1], sys.argv[2], sys.argv[3]
root = os.path.join(parent, name)
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    for dirpath, dirnames, filenames in os.walk(root):
        for d in dirnames:
            full = os.path.join(dirpath, d)
            z.write(full, os.path.relpath(full, parent) + "/")
        for f in filenames:
            full = os.path.join(dirpath, f)
            z.write(full, os.path.relpath(full, parent))
PYPACK
    else
        die "打包 zip 需要以下之一：zip、Windows 自带 tar.exe（Win10+）、powershell.exe、python3"
    fi
}

build_binary() {
    local goos="$1" goarch="$2" out="$3"
    local ver="${SNAPSHOT_VERSION#v}"
    local out_arg="${out}"
    [[ "${GO_IS_WINDOWS}" == "1" ]] && out_arg="$(to_win_path "${out}")"

    if [[ "${ENV_KIND}" == "wsl" ]]; then
        # WSL 调用 Windows go.exe：环境变量必须经 WSLENV 才可见
        WSLENV=GOOS/w:GOARCH/w:CGO_ENABLED/w \
        GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
            "${GO_CMD}" build -ldflags "-s -w -X main.version=${ver} -X main.commit=${COMMIT}" \
            -o "${out_arg}" ./cmd/server
    else
        # 原生 / Git Bash：直接传环境变量即可
        GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
            "${GO_CMD}" build -ldflags "-s -w -X main.version=${ver} -X main.commit=${COMMIT}" \
            -o "${out_arg}" ./cmd/server
    fi
}

# ---------------------------------------------------------------------
# 4. 目标平台 → 包命名元素
# ---------------------------------------------------------------------
# 用 | 分隔输出：空字段（Linux 无 .exe 后缀）在空格分隔下会被 read 丢弃，
# 导致 EXT 错位拿到 ARCHIVE 的值。
target_meta() {
    local t="$1"
    case "${t}" in
        linux/amd64)   echo "linux|amd64|ubuntu||tar.gz" ;;
        linux/arm64)   echo "linux|arm64|ubuntu||tar.gz" ;;
        windows/amd64) echo "windows|amd64|windows|.exe|zip" ;;
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
    IFS='|' read -r GOOS_V GOARCH_V OS_LABEL EXT ARCHIVE <<< "$(target_meta "${T}")"
    NAME="FinvQuant-${SNAPSHOT_VERSION}-${OS_LABEL}-${GOARCH_V}"
    PKG_ROOT="${ROOT}/pkg/${NAME}"

    log "── 构建 ${T} → ${NAME}.${ARCHIVE}"

    rm -rf "${PKG_ROOT}"
    mkdir -p "${PKG_ROOT}/Deploy"

    build_binary "${GOOS_V}" "${GOARCH_V}" "${ROOT}/pkg/finvquant${EXT}"
    cp "${ROOT}/pkg/finvquant${EXT}" "${PKG_ROOT}/"
    rm -f "${ROOT}/pkg/finvquant${EXT}"

    # 迁移脚本放在 Deploy/ 下（与仓库结构、安装后的 FINV_MIGRATIONS_DIR=<PREFIX>/Deploy/Migrations 一致）
    cp -r Deploy/Migrations "${PKG_ROOT}/Deploy/Migrations"
    cp    Deploy/README.md "${PKG_ROOT}/Deploy/README.md"

    # 一次性运维脚本（如 public→cdeu 历史数据迁移）随包分发：
    # 安装后在 <PREFIX>/Deploy/Script/，不进入 schema_version 迁移链
    if compgen -G "Deploy/Script/*.sql" > /dev/null; then
        mkdir -p "${PKG_ROOT}/Deploy/Script"
        cp Deploy/Script/*.sql "${PKG_ROOT}/Deploy/Script/"
        if [[ -f Deploy/Script/README.md ]]; then
            cp Deploy/Script/README.md "${PKG_ROOT}/Deploy/Script/"
        fi
        chmod 0644 "${PKG_ROOT}/Deploy/Script/"*
    fi

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
    case "${KIND}" in
        snapshot) KIND_LABEL="快照包（分支推送）" ;;
        pr)       KIND_LABEL="构建包（PR 合并）" ;;
        release)  KIND_LABEL="正式版包（main 合并）" ;;
        *)        KIND_LABEL="${KIND}" ;;
    esac
    {
        echo "FinvQuant ${KIND_LABEL}"
        echo "=================="
        echo "包类型      : ${KIND}（${KIND_LABEL}）"
        echo "版本        : ${SNAPSHOT_VERSION}"
        echo "基础版本    : ${BASE_VERSION}"
        echo "PR 编号     : ${PR:-(不适用)}"
        echo "构建时间戳  : ${TIMESTAMP:-(不适用)}"
        echo "提交        : ${COMMIT}"
        echo "目标平台    : ${GOOS_V}/${GOARCH_V}"
        echo "适用系统    : ${local_sys}"
        if [[ -n "${BUILD_REPO:-}" ]]; then
            echo "仓库        : https://github.com/${BUILD_REPO}"
        fi
        if [[ -n "${BUILD_RUN_URL:-}" ]]; then
            echo "构建流水线  : ${BUILD_RUN_URL}"
        fi
        echo "构建方式    : Deploy/Script/PackSnapshot.sh${CI_RUN_ID:+（CI run ${CI_RUN_ID}）}"
    } > "${PKG_ROOT}/BUILD-INFO.txt"

    ( cd "${PKG_ROOT}" && find . -type f ! -name SHA256SUMS -print0 | sort -z \
        | xargs -0 sha256sum > SHA256SUMS )

    if [[ "${ARCHIVE}" == "zip" ]]; then
        make_zip "${ROOT}/pkg" "${NAME}" "${OUT_DIR}/${NAME}.zip"
    else
        ( cd "${ROOT}/pkg" && tar -czf "${OUT_DIR}/${NAME}.tar.gz" "${NAME}" )
    fi
    ( cd "${OUT_DIR}" && sha256sum "${NAME}.${ARCHIVE}" > "${NAME}.${ARCHIVE}.sha256" )

    log "   完成：$(cd "${OUT_DIR}" && ls -lh "${NAME}.${ARCHIVE}" | awk '{print $5}')"
done

[[ "${KEEP_TMP}" == "1" ]] || rm -rf "${ROOT}/pkg"

log "全部完成，产物位于 ${OUT_DIR}"
( cd "${OUT_DIR}" && ls -1 ./*.tar.gz ./*.zip ./*.sha256 2>/dev/null || true )
