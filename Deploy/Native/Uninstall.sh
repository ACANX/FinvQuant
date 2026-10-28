#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 卸载脚本
#
# 安装根目录：/data/App/FinvQuant（由 systemd unit 的 ExecStart 推断，
#             可用 --prefix 或环境变量 FINVQ_PREFIX 覆盖），布局见 Install.sh。
#
# 默认：只停服务、禁用开机自启、删除 systemd unit 与二进制；
#       **保留** 配置（<PREFIX>/etc）、迁移（<PREFIX>/Migrations）、部署文件（<PREFIX>/Deploy）
#       与数据（<PREFIX>/Backup、<PREFIX>/Export、PostgreSQL 库）。
# --purge：额外删除 <PREFIX> 下的二进制目录、配置、迁移与部署文件（**不删** Backup/Export 数据）。
# --purge-data：额外提示手工删除数据库与清理备份/导出目录（本脚本不自动删）。
#
# 用法：
#   sudo ./Uninstall.sh
#   sudo ./Uninstall.sh --purge
#   sudo ./Uninstall.sh --purge --purge-data
#   sudo ./Uninstall.sh --prefix /data/App/FinvQuant
# =====================================================================
set -euo pipefail

SERVICE="finvquant"
SERVICE_FILE="/etc/systemd/system/${SERVICE}.service"
BINARY_LINK="/usr/local/bin/finvquant"

PREFIX_EXPLICIT=0
infer_prefix() {
  local detected=""
  if [[ "${PREFIX_EXPLICIT}" -eq 0 && -f "${SERVICE_FILE}" ]]; then
    detected="$(sed -n 's|^ExecStart=\(.*\)/bin/finvquant$|\1|p' "${SERVICE_FILE}" | head -1)"
  fi
  printf '%s' "${detected:-/data/App/FinvQuant}"
}
PREFIX="${FINVQ_PREFIX:-$(infer_prefix)}"

PURGE=0
PURGE_DATA=0

log()  { printf '\033[1;34m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
ok()   { printf '\033[1;32m  [OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m  [WARN]\033[0m %s\n' "$*"; }
step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31m  [ERROR]\033[0m %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --purge)      PURGE=1; shift ;;
    --purge-data) PURGE_DATA=1; shift ;;
    --prefix)     PREFIX_EXPLICIT=1; PREFIX="${2:-}"; shift 2 ;;
    -h|--help)    sed -n '2,18p' "$0"; exit 0 ;;
    *)            die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[[ "${PREFIX}" == /* ]] || die "--prefix 必须是绝对路径（当前：${PREFIX}）"
BIN_DIR="${PREFIX}/bin"
ETC_DIR="${PREFIX}/etc"
MIGRATIONS_DIR="${PREFIX}/Migrations"
DEPLOY_DIR="${PREFIX}/Deploy"
BACKUP_DIR="${PREFIX}/Backup"
EXPORT_DIR="${PREFIX}/Export"
BINARY="${BIN_DIR}/finvquant"
BINARY_PREV="${BIN_DIR}/finvquant.prev"
ENV_FILE="${ETC_DIR}/finvquant.env"

[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行：sudo $0"
ok "安装根目录：${PREFIX}"

step "1/4 停止并禁用服务"
if systemctl list-unit-files | grep -q "^${SERVICE}.service"; then
  systemctl stop "${SERVICE}" 2>/dev/null || true
  systemctl disable "${SERVICE}" >/dev/null 2>&1 || true
  ok "已停止并禁用 ${SERVICE}"
else
  warn "未找到 ${SERVICE}.service（可能已卸载）"
fi

step "2/4 删除 systemd unit 与二进制"
rm -f "${SERVICE_FILE}"
systemctl daemon-reload
ok "已删除 ${SERVICE_FILE}"
rm -f "${BINARY_LINK}" "${BINARY}" "${BINARY_PREV}"
ok "已删除二进制与软链（${BIN_DIR} 目录保留，可用 --purge 清理）"

step "3/4 处理配置、迁移与部署文件"
if [[ "${PURGE}" -eq 1 ]]; then
  rm -rf "${ETC_DIR}" "${MIGRATIONS_DIR}" "${DEPLOY_DIR}" "${BIN_DIR}"
  ok "已删除 ${ETC_DIR} / ${MIGRATIONS_DIR} / ${DEPLOY_DIR} / ${BIN_DIR}"
  warn "保留数据目录：${BACKUP_DIR}、${EXPORT_DIR}（如需清理见 --purge-data）"
else
  warn "保留 ${ETC_DIR}（含 finvquant.env）、${MIGRATIONS_DIR}、${DEPLOY_DIR}（如需删除请加 --purge）"
fi

step "4/4 数据处置"
if [[ "${PURGE_DATA}" -eq 1 ]]; then
  warn "本脚本不自动删库、不自动删备份/导出数据；如需彻底清理请手工执行（确认已备份）："
  cat <<EOF
    sudo -u postgres dropdb --if-exists <数据库名>
    sudo -u postgres psql -c "DROP ROLE IF EXISTS <角色名>;"
    # 备份与导出（确认无用后再删）：
    sudo rm -rf ${BACKUP_DIR} ${EXPORT_DIR}
    # 如已安装的 PostgreSQL/Redis 不再需要：
    sudo systemctl disable --now postgresql redis-server
    sudo apt-get purge postgresql-18 redis-server
EOF
else
  warn "数据库与角色保持不动（如需清理请加 --purge-data 查看指引）"
fi

cat <<EOF

\033[1;32m卸载完成\033[0m
  剩余内容：
    - 数据目录：PostgreSQL / Redis 自身数据目录（不由本脚本管理）
    - 备份目录：${BACKUP_DIR}（未删除，请自行确认是否保留）
    - 导出目录：${EXPORT_DIR}（未删除，请自行确认是否保留）
    - 安装根目录：${PREFIX}（$( [[ "${PURGE}" -eq 1 ]] && echo "已清理程序/配置/迁移/部署文件" || echo "配置、迁移与部署文件保留" )）
EOF
