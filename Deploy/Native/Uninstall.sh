#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 卸载脚本
#
# 默认：只停服务、禁用开机自启、删除 systemd unit 与二进制；
#       **保留** 配置（/etc/finvquant）与数据（PostgreSQL 库）。
# --purge：额外删除 /etc/finvquant（配置与迁移副本），仍不删除数据库。
# --purge-data：额外提示手工删除数据库（本脚本不自动删库，避免误操作）。
#
# 用法：
#   sudo ./Uninstall.sh
#   sudo ./Uninstall.sh --purge
#   sudo ./Uninstall.sh --purge --purge-data
# =====================================================================
set -euo pipefail

APP_DIR="/opt/finvquant"
ETC_DIR="/etc/finvquant"
SERVICE_FILE="/etc/systemd/system/finvquant.service"
BINARY_LINK="/usr/local/bin/finvquant"
SERVICE="finvquant"

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
    -h|--help)    sed -n '2,15p' "$0"; exit 0 ;;
    *)            die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行：sudo $0"

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
rm -f "${BINARY_LINK}" "${APP_DIR}/finvquant" "${APP_DIR}/finvquant.prev"
ok "已删除二进制（${APP_DIR} 目录保留）"

step "3/4 处理配置"
if [[ "${PURGE}" -eq 1 ]]; then
  rm -rf "${ETC_DIR}"
  ok "已删除 ${ETC_DIR}（含 finvquant.env 与迁移副本）"
else
  warn "保留 ${ETC_DIR}（如需删除请加 --purge）"
fi

step "4/4 数据处置"
if [[ "${PURGE_DATA}" -eq 1 ]]; then
  warn "本脚本不自动删库；如需彻底清理请手工执行（确认已备份）："
  cat <<'EOF'
    sudo -u postgres dropdb --if-exists <数据库名>
    sudo -u postgres psql -c "DROP ROLE IF EXISTS <角色名>;"
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
    - 数据目录：/var/lib/postgresql/18（或发行版默认）+ Redis 数据
    - 配置目录：$( [[ "${PURGE}" -eq 1 ]] && echo "已删除" || echo "${ETC_DIR}" )
    - 备份目录：/var/backups/finvquant（未删除，请自行确认是否保留）
EOF
