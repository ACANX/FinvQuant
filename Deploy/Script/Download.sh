#!/usr/bin/env bash
# =====================================================================
# FinvQuant 一键下载脚本（按操作系统/架构自动选包，支持正式版与快照版）
#
# 作用：检测本机 OS 与 CPU 架构 → 从 GitHub Releases 选择目标版本
#       （默认取最新，含快照/构建包）→ 下载对应资产 + SHA256 → 校验 →
#       解压到指定目录（默认 /data/App/FinvQuant，与 Native 安装根目录一致）。
#
# 资产命名（由 Deploy/Script/PackSnapshot.sh 产出）：
#   FinvQuant-<版本>-<ubuntu|windows>-<amd64|arm64>.{tar.gz|zip}
#   附带同名 .sha256 资产
# 版本形态：
#   v0.1.3                 正式版（release，main 合并）
#   v0.1.3.00357           PR 构建包（pr）
#   v0.1.3.00357.2610281256 快照包（snapshot，分支推送）
#
# 用法：
#   bash Deploy/Script/Download.sh                        # 最新（含快照）→ /data/App/FinvQuant
#   bash Deploy/Script/Download.sh -d /opt/FinvQuant      # 指定解压目录
#   bash Deploy/Script/Download.sh -c release             # 只要正式版
#   bash Deploy/Script/Download.sh -c snapshot            # 只要最新快照/构建包
#   bash Deploy/Script/Download.sh -v v0.1.3.00357.2610281256
#   bash Deploy/Script/Download.sh --list                 # 列出最近 10 个版本后退出
#   bash Deploy/Script/Download.sh --no-extract -o ./pkg  # 只下载到 ./pkg
#
# 选项：
#   -d, --dir <目录>      解压目标目录（Linux 默认 /data/App/FinvQuant；
#                         Windows 默认 <当前目录>/FinvQuant）
#   -c, --channel <渠道>  auto（默认，最新且含快照）/ release（正式版）/
#                         snapshot（快照与 PR 构建包）/ pr（仅 PR 构建包）
#   -v, --version <版本>  指定 tag（可省略前导 v），跳过渠道选择
#   -r, --repo <o/r>      GitHub 仓库（默认 ACANX/FinvQuant）
#   -t, --token <token>   访问令牌（默认取 GH_TOKEN / GITHUB_TOKEN 环境变量）
#   -o, --out <目录>      下载缓存目录（默认临时目录，解压后清理）
#       --base-url <url>  下载源根地址（默认 https://github.com，可指向镜像站）
#       --keep-top        保留包内顶层目录 FinvQuant-<版本>-<os>-<arch>/
#       --no-extract      只下载、不解压
#       --force           目标目录非空时继续（覆盖同名文件，不删除多余文件）
#       --no-verify       跳过 SHA256 校验（不建议）
#       --dry-run         只解析并打印将要下载的 URL / 目标目录，不下载
#       --list            列出最近 10 个版本（含类型）后退出
#   -h, --help            显示本帮助
#
# 环境变量：
#   GH_TOKEN / GITHUB_TOKEN   同 --token（私有仓库或规避 API 限流时使用）
#   FINVQ_REPO                同 --repo
#   FINVQ_BASE_URL            同 --base-url
#
# 退出码：0 成功；非 0 失败（已打印原因）
# =====================================================================
set -euo pipefail

DEFAULT_REPO="ACANX/FinvQuant"
DEFAULT_DIR_LINUX="/data/App/FinvQuant"

REPO="${FINVQ_REPO:-${DEFAULT_REPO}}"
BASE_URL="${FINVQ_BASE_URL:-https://github.com}"
BASE_URL="${BASE_URL%/}"
CHANNEL="auto"
TARGET_VERSION=""
DIR=""
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
OUT_DIR=""
KEEP_TOP=0
DO_EXTRACT=1
FORCE=0
VERIFY=1
DRY_RUN=0
LIST_ONLY=0

log()  { printf '\033[1;34m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
ok()   { printf '\033[1;32m  [OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m  [WARN]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m  [ERROR]\033[0m %s\n' "$*" >&2; exit 1; }
step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

usage() { sed -n '2,49p' "$0"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    -d|--dir)      DIR="${2:-}"; shift 2 ;;
    -c|--channel)  CHANNEL="${2:-}"; shift 2 ;;
    -v|--version)  TARGET_VERSION="${2:-}"; shift 2 ;;
    -r|--repo)     REPO="${2:-}"; shift 2 ;;
    -t|--token)    TOKEN="${2:-}"; shift 2 ;;
    -o|--out)      OUT_DIR="${2:-}"; shift 2 ;;
    --base-url)    BASE_URL="${2:-}"; shift 2 ;;
    --keep-top)    KEEP_TOP=1; shift ;;
    --no-extract)  DO_EXTRACT=0; shift ;;
    --force)       FORCE=1; shift ;;
    --no-verify)   VERIFY=0; shift ;;
    --dry-run)     DRY_RUN=1; shift ;;
    --list)        LIST_ONLY=1; shift ;;
    -h|--help)     usage; exit 0 ;;
    *)             die "未知参数：$1（--help 查看用法）" ;;
  esac
done

case "${CHANNEL}" in
  auto|release|snapshot|prerelease|pr) ;;
  *) die "未知渠道：${CHANNEL}（可选 auto / release / snapshot / pr）" ;;
esac

# ---------------------------------------------------------------------
# 0. 依赖与平台检测
# ---------------------------------------------------------------------
step "0/6 检测运行环境"
command -v curl >/dev/null 2>&1 || die "缺少 curl，请先安装（apt-get install -y curl / Git for Windows 自带）"

# Git Bash 下把 Windows 路径转成 unix 形式，便于 mv / tar 使用
normalize_path() {
  local p="$1"
  if command -v cygpath >/dev/null 2>&1; then cygpath -u "$p"; else printf '%s\n' "$p"; fi
}

PLATFORM_OS=""
OS_LABEL=""
EXT=""
case "$(uname -s)" in
  Linux)   PLATFORM_OS="linux";   OS_LABEL="ubuntu";  EXT="tar.gz" ;;
  Darwin)  die "官方暂未发布 macOS 包（仅 ubuntu amd64/arm64、windows amd64）；可在 macOS 上从源码构建" ;;
  MINGW*|MSYS*|CYGWIN*) PLATFORM_OS="windows"; OS_LABEL="windows"; EXT="zip" ;;
  *) die "不支持的操作系统：$(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "不支持的 CPU 架构：$(uname -m)（官方仅提供 amd64 / arm64）" ;;
esac

if [[ "${PLATFORM_OS}" == "windows" && "${ARCH}" == "arm64" ]]; then
  die "官方暂未发布 windows/arm64 包（仅提供 windows/amd64）；可在 x64 上运行或从源码构建"
fi

if [[ "${PLATFORM_OS}" == "linux" ]]; then
  if [[ -r /etc/os-release ]]; then
    # 用子 shell 读取，避免 os-release 的 VERSION / NAME 等变量污染本脚本
    OS_PRETTY="$( . /etc/os-release 2>/dev/null; printf '%s' "${PRETTY_NAME:-unknown}" )"
    OS_ID="$( . /etc/os-release 2>/dev/null; printf '%s' "${ID:-}" )"
    ok "系统：${OS_PRETTY}（$(uname -m)）"
    case "${OS_ID}" in
      debian|ubuntu) ;;
      *) warn "非 Debian/Ubuntu：二进制为静态链接可直接运行，但 Deploy/Native 安装脚本仅支持 Debian/Ubuntu" ;;
    esac
  else
    ok "系统：Linux（$(uname -m)）"
  fi
else
  ok "系统：Windows（$(uname -m)）"
fi
ok "匹配资产：${OS_LABEL}-${ARCH}.${EXT}"

if [[ -z "${DIR}" ]]; then
  if [[ "${PLATFORM_OS}" == "linux" ]]; then DIR="${DEFAULT_DIR_LINUX}"; else DIR="${PWD}/FinvQuant"; fi
fi
DIR="$(normalize_path "${DIR}")"
[[ "${DIR}" == /* ]] || die "解压目录必须是绝对路径（当前：${DIR}）"
ok "解压目录：${DIR}"

# ---------------------------------------------------------------------
# 1. 解析可用版本
# ---------------------------------------------------------------------
step "1/6 解析版本"

# 输出：<tag>\t<draft>\t<prerelease>
fetch_release_entries() {
  local out=""

  # 1) gh CLI（复用 gh 登录态，走 api.github.com，国内链路比 github.com 稳）
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    out="$(gh release list -R "${REPO}" -L 100 --json tagName,isDraft,isPrerelease \
      --jq '.[] | [.tagName, (.isDraft|tostring), (.isPrerelease|tostring)] | @tsv' 2>/dev/null || true)"
    if [[ -n "${out}" ]]; then printf '%s\n' "${out}"; return 0; fi
  fi

  # 2) 显式 token → Releases API
  if [[ -n "${TOKEN}" ]]; then
    out="$(curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 20 \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "https://api.github.com/repos/${REPO}/releases?per_page=100" 2>/dev/null || true)"
    [[ -n "${out}" ]] || return 1
    printf '%s\n' "${out}" | awk '
        /"tag_name":/ {
          line = $0
          sub(/.*"tag_name": *"/, "", line); sub(/".*/, "", line); tag = line
        }
        /"draft":/ {
          line = $0; sub(/.*"draft": */, "", line); sub(/,.*/, "", line); draft = line
        }
        /"prerelease":/ {
          line = $0; sub(/.*"prerelease": */, "", line); sub(/,.*/, "", line)
          if (tag != "") { printf "%s\t%s\t%s\n", tag, draft, line; tag = "" }
        }
      '
    return 0
  fi

  # 3) 公开仓库 → releases.atom（按发布时间倒序，含 prerelease，不含 draft）
  out="$(curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 20 \
    "https://github.com/${REPO}/releases.atom" 2>/dev/null || true)"
  [[ -n "${out}" ]] || return 1
  printf '%s\n' "${out}" | grep -o 'releases/tag/[^"]*' | sed 's|releases/tag/||' \
    | awk '!seen[$0]++ {
        pre = ($0 ~ /^v[0-9]+\.[0-9]+\.[0-9]+$/) ? "false" : "true"
        printf "%s\tfalse\t%s\n", $0, pre
      }' || true
  return 0
}

tag_kind() {
  local t="$1"
  if [[ "${t}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "正式版"
  elif [[ "${t}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "PR 构建包"
  elif [[ "${t}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "快照包"
  else
    echo "其它"
  fi
}

ENTRIES=""
# 指定了 --version 且只是下载时不联网拉列表（离线/镜像场景友好）；--list 与渠道选择才需要
if [[ "${LIST_ONLY}" -eq 1 || -z "${TARGET_VERSION}" ]]; then
  [[ -n "${TOKEN}" ]] || warn "未提供 token：按 gh CLI → releases.atom 顺序取列表（私有仓库请用 -t 或 GH_TOKEN）"
  if ! ENTRIES="$(fetch_release_entries 2>/dev/null)"; then
    ENTRIES=""
    die "获取版本列表失败（检查网络 / 仓库名 / token）"
  fi
fi

if [[ "${LIST_ONLY}" -eq 1 ]]; then
  [[ -n "${ENTRIES}" ]] || die "仓库 ${REPO} 没有可用 release"
  printf '\n最近版本（仓库：%s）\n' "${REPO}"
  printf '  %-28s %s\n' "版本" "类型"
  printf '%s\n' "${ENTRIES}" | head -10 | while IFS=$'\t' read -r tag draft pre; do
    [[ -z "${tag}" ]] && continue
    [[ "${draft}" == "true" ]] && continue
    printf '  %-28s %s\n' "${tag}" "$(tag_kind "${tag}")"
  done
  exit 0
fi

select_tag() {
  local channel="$1" tag draft pre
  while IFS=$'\t' read -r tag draft pre; do
    [[ -z "${tag}" ]] && continue
    [[ "${draft}" == "true" ]] && continue
    case "${channel}" in
      auto)
        printf '%s\n' "${tag}"; return 0 ;;
      release)
        if [[ "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then printf '%s\n' "${tag}"; return 0; fi ;;
      snapshot|prerelease)
        if [[ "${pre}" == "true" ]]; then printf '%s\n' "${tag}"; return 0; fi ;;
      pr)
        if [[ "${pre}" == "true" && "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
          printf '%s\n' "${tag}"; return 0
        fi ;;
    esac
  done <<< "${ENTRIES}"
  return 1
}

if [[ -n "${TARGET_VERSION}" ]]; then
  [[ "${TARGET_VERSION}" == v* ]] || TARGET_VERSION="v${TARGET_VERSION}"
  # 指定版本不强制要求出现在最近 100 条中（便于下载镜像/历史版本）；下载 404 时再报错
  if [[ -n "${ENTRIES}" ]] && ! printf '%s\n' "${ENTRIES}" | cut -f1 | grep -Fxq "${TARGET_VERSION}"; then
    warn "版本 ${TARGET_VERSION} 不在最近 100 条 release 中，仍尝试下载"
  fi
  ok "指定版本：${TARGET_VERSION}"
else
  [[ -n "${ENTRIES}" ]] || die "无法获取版本列表（请联网或显式指定 --version）"
  TARGET_VERSION="$(select_tag "${CHANNEL}")" || die "渠道 ${CHANNEL} 下没有可用版本（可用 --list 查看；正式版需 main 合并后才存在）"
  ok "最新${CHANNEL}版本：${TARGET_VERSION}（$(tag_kind "${TARGET_VERSION}")）"
fi

if [[ "$(tag_kind "${TARGET_VERSION}")" != "正式版" ]]; then
  warn "所选为$(tag_kind "${TARGET_VERSION}")：为分支/PR 的集成验证产物，不保证稳定，勿用于生产"
fi

ASSET="FinvQuant-${TARGET_VERSION}-${OS_LABEL}-${ARCH}.${EXT}"
TOP_DIR="FinvQuant-${TARGET_VERSION}-${OS_LABEL}-${ARCH}"
URL="${BASE_URL}/${REPO}/releases/download/${TARGET_VERSION}/${ASSET}"
SHA_URL="${URL}.sha256"

# ---------------------------------------------------------------------
# 2. 准备下载目录
# ---------------------------------------------------------------------
step "2/6 准备下载"
if [[ -n "${OUT_DIR}" ]]; then
  WORK_DIR="$(normalize_path "${OUT_DIR}")"
  mkdir -p "${WORK_DIR}"
  TEMP_WORK=0
  ok "下载目录：${WORK_DIR}（保留）"
else
  WORK_DIR="$(mktemp -d)"
  TEMP_WORK=1
  ok "下载目录：${WORK_DIR}（临时，完成后清理）"
fi
PKG="${WORK_DIR}/${ASSET}"

cleanup() {
  if [[ "${TEMP_WORK}" -eq 1 && -d "${WORK_DIR}" ]]; then rm -rf "${WORK_DIR}"; fi
}
trap cleanup EXIT

ok "资产：${ASSET}"
ok "URL ：${URL}"

if [[ "${DRY_RUN}" -eq 1 ]]; then
  step "3/6 试运行（不下载）"
  echo "  将下载：${URL}"
  echo "  校验文件：${SHA_URL}"
  echo "  解压到：${DIR}$( [[ "${KEEP_TOP}" -eq 1 ]] && echo "（保留顶层目录 ${TOP_DIR}/）" )"
  exit 0
fi

# ---------------------------------------------------------------------
# 3. 下载（含 .sha256）
# ---------------------------------------------------------------------
step "3/6 下载资产"
CURL_AUTH=()
if [[ -n "${TOKEN}" ]]; then CURL_AUTH=(-H "Authorization: Bearer ${TOKEN}"); fi

if [[ -f "${PKG}" ]]; then
  warn "已存在同名文件，跳过下载：${PKG}"
else
  # 终端下显示进度条，非终端（CI/重定向）保持安静，仅输出错误
  if [[ -t 2 ]]; then CURL_PROGRESS=(--progress-bar); else CURL_PROGRESS=(-sS); fi
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 "${CURL_PROGRESS[@]}" \
    "${CURL_AUTH[@]}" -o "${PKG}" "${URL}" || die "下载失败：${URL}"
fi
ok "已下载：$(du -h "${PKG}" | cut -f1)"

SHA_OK=0
if curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 20 \
     "${CURL_AUTH[@]}" -o "${PKG}.sha256" "${SHA_URL}"; then
  SHA_OK=1
  ok "已获取校验文件：${ASSET}.sha256"
else
  warn "未获取到 .sha256（该资产可能未附带校验文件）"
fi

# ---------------------------------------------------------------------
# 4. 校验 SHA256
# ---------------------------------------------------------------------
step "4/6 校验完整性"
if [[ "${VERIFY}" -eq 0 ]]; then
  warn "已按 --no-verify 跳过校验"
elif [[ "${SHA_OK}" -eq 1 ]]; then
  if command -v sha256sum >/dev/null 2>&1; then
    ( cd "${WORK_DIR}" && sha256sum -c "${ASSET}.sha256" ) || die "SHA256 校验失败：${PKG}（请重新下载）"
  elif command -v shasum >/dev/null 2>&1; then
    expected="$(cut -d' ' -f1 < "${PKG}.sha256")"
    actual="$(shasum -a 256 "${PKG}" | cut -d' ' -f1)"
    [[ "${expected}" == "${actual}" ]] || die "SHA256 校验失败：${PKG}"
  else
    warn "未找到 sha256sum/shasum，跳过校验"
  fi
  ok "SHA256 校验通过"
else
  warn "缺少 .sha256，未校验（可用 --no-verify 明确跳过）"
fi

if [[ "${DO_EXTRACT}" -eq 0 ]]; then
  step "5/6 跳过解压"
  ok "仅下载模式；产物：${PKG}"
  exit 0
fi

# ---------------------------------------------------------------------
# 5. 解压到临时目录并安全校验
# ---------------------------------------------------------------------
step "5/6 解压"
command -v tar >/dev/null 2>&1 || die "缺少 tar"
EXTRACT_DIR="${WORK_DIR}/extract"
rm -rf "${EXTRACT_DIR}"
mkdir -p "${EXTRACT_DIR}"

case "${EXT}" in
  tar.gz)
    tar_list="$(tar -tzf "${PKG}")"
    if printf '%s\n' "${tar_list}" | grep -qE '(^/|(^|/)\.\./)'; then
      die "压缩包内含绝对路径或 .. 路径，拒绝解压：${PKG}"
    fi
    tar -xzf "${PKG}" -C "${EXTRACT_DIR}" || die "解压失败：${PKG}"
    ;;
  zip)
    if command -v unzip >/dev/null 2>&1; then
      zip_list="$(unzip -Z1 "${PKG}" 2>/dev/null || true)"
      if printf '%s\n' "${zip_list}" | grep -qE '(^/|(^|/)\.\./)'; then
        die "压缩包内含绝对路径或 .. 路径，拒绝解压：${PKG}"
      fi
      unzip -q "${PKG}" -d "${EXTRACT_DIR}" || die "解压失败：${PKG}"
    elif command -v powershell.exe >/dev/null 2>&1; then
      win_pkg="$(cygpath -w "${PKG}" 2>/dev/null || printf '%s' "${PKG}")"
      win_dst="$(cygpath -w "${EXTRACT_DIR}" 2>/dev/null || printf '%s' "${EXTRACT_DIR}")"
      powershell.exe -NoProfile -Command \
        "Expand-Archive -LiteralPath '${win_pkg}' -DestinationPath '${win_dst}' -Force" \
        || die "解压失败（PowerShell）：${PKG}"
    else
      die "缺少解压工具：请安装 unzip（Windows 可用 Git Bash 自带 unzip）"
    fi
    ;;
esac

[[ -d "${EXTRACT_DIR}/${TOP_DIR}" ]] || die "包内未找到顶层目录 ${TOP_DIR}（包结构异常）"
ok "已解压到临时目录"

# 包内 SHA256SUMS 校验
if [[ "${VERIFY}" -eq 1 && -f "${EXTRACT_DIR}/${TOP_DIR}/SHA256SUMS" ]] && command -v sha256sum >/dev/null 2>&1; then
  ( cd "${EXTRACT_DIR}/${TOP_DIR}" && sha256sum -c SHA256SUMS >/dev/null ) \
    && ok "包内 SHA256SUMS 校验通过" \
    || die "包内文件校验失败：${TOP_DIR}/SHA256SUMS"
fi

# ---------------------------------------------------------------------
# 6. 落盘到目标目录
# ---------------------------------------------------------------------
step "6/6 安装到目标目录"
if [[ -d "${DIR}" && -n "$(ls -A "${DIR}" 2>/dev/null || true)" && "${FORCE}" -eq 0 ]]; then
  die "目标目录非空：${DIR}
       请改用 -d 指定新目录，或加 --force 覆盖同名文件（不会删除多余文件）"
fi
mkdir -p "${DIR}"

if [[ "${KEEP_TOP}" -eq 1 ]]; then
  if [[ -e "${DIR}/${TOP_DIR}" ]]; then
    rm -rf "${DIR:?}/${TOP_DIR}"
  fi
  mv "${EXTRACT_DIR}/${TOP_DIR}" "${DIR}/${TOP_DIR}"
  ok "已保留包目录：${DIR}/${TOP_DIR}"
else
  # 默认铺平：把包内顶层目录的内容直接放进目标目录（含隐藏文件）
  # --force 时：同名文件覆盖；同名目录合并（不删除目标目录里现有的多余文件）
  shopt -s dotglob nullglob
  for item in "${EXTRACT_DIR}/${TOP_DIR}"/*; do
    base="$(basename "${item}")"
    if [[ -d "${item}" && -d "${DIR}/${base}" ]]; then
      cp -a "${item}/." "${DIR}/${base}/"
    else
      mv -f "${item}" "${DIR}/"
    fi
  done
  shopt -u dotglob nullglob
  ok "已铺平到：${DIR}"
fi

log "内容清单"
( cd "${DIR}" && ls -1A | head -20 )

VERIFY_TEXT="未完整校验"
if [[ "${VERIFY}" -eq 1 && "${SHA_OK}" -eq 1 ]]; then VERIFY_TEXT="SHA256 通过（资产 + 包内）"; fi
printf '\n\033[1;32m下载完成\033[0m\n'
printf '  版本  ：%s（%s）\n' "${TARGET_VERSION}" "$(tag_kind "${TARGET_VERSION}")"
printf '  平台  ：%s/%s\n' "${OS_LABEL}" "${ARCH}"
printf '  目录  ：%s\n' "${DIR}"
printf '  校验  ：%s\n' "${VERIFY_TEXT}"

if [[ "${PLATFORM_OS}" == "linux" ]]; then
  printf '\n下一步（Linux 宿主直装）：\n'
  printf '  sudo bash %s/Deploy/Native/Install.sh --prefix %s\n' "${DIR}" "${DIR}"
  printf '  说明：包内已含 finvquant 二进制、Deploy/Migrations 与 Deploy/Native 全套脚本，\n'
  printf '        Install.sh 会补齐用户/目录/数据库/Redis/systemd 并启动服务。\n'
else
  win_dir="$(cygpath -w "${DIR}" 2>/dev/null || printf '%s' "${DIR}")"
  printf '\n下一步（Windows 容器方式）：\n'
  printf '  cd /d %s\\Deploy\\Docker\n' "${win_dir}"
  printf '  deploy.cmd\n'
  printf '  说明：Windows 包内为 Docker 部署编排（compose 拉取 GHCR 镜像），不含 Dockerfile。\n'
fi
