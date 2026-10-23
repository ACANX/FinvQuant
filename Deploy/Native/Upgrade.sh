#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 升级脚本（宿主直装，systemd 守护）
#
# 流程：备份数据库 → 停服务 → 备份旧二进制 → 替换二进制与迁移
#       → 启动服务 → 健康检查（失败时给出一键回滚命令）
#
# 用法：
#   sudo ./Upgrade.sh                          # 使用仓库/脚本目录下的 finvquant 升级
#   sudo ./Upgrade.sh --binary /tmp/finvquant  # 指定新二进制
#   sudo ./Upgrade.sh --skip-backup            # 跳过数据库备份（仅应急）
#
# 回滚：sudo ./Upgrade.sh --rollback
# 说明：数据库迁移由新二进制启动时自动执行，**不可自动回滚**；如需回退数据库，
#       只能从本次备份恢复（见 Upgrade.md）。
# =====================================================================
set -euo pipefail

APP_DIR="/opt/finvquant"
ETC_DIR="/etc/finvquant"
MIGRATIONS_DIR="${ETC_DIR}/Migrations"
BACKUP_DIR="/var/backups/finvquant"
BINARY="${APP_DIR}/finvquant"
BINARY_PREV="${APP_DIR}/finvquant.prev"
ENV_FILE="${ETC_DIR}/finvquant.env"
SERVICE="finvquant"

BINARY_SRC=""
SKIP_BACKUP=0
ROLLBACK=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

log()  { printf '\033[1;34m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
ok()   { printf '\033[1;32m  [OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m  [WARN]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m  [ERROR]\033[0m %s\n' "$*" >&2; exit 1; }
step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)      BINARY_SRC="${2:-}"; shift 2 ;;
    --skip-backup) SKIP_BACKUP=1; shift ;;
    --rollback)    ROLLBACK=1; shift ;;
    -h|--help)     sed -n '2,18p' "$0"; exit 0 ;;
    *)             die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行：sudo $0"
[[ -f "${ENV_FILE}" ]] || die "未找到 ${ENV_FILE}，请先执行 Install.sh"

# ---------------------------------------------------------------------
# 回滚模式：换回 .prev 二进制
# ---------------------------------------------------------------------
if [[ "${ROLLBACK}" -eq 1 ]]; then
  step "回滚到上一版本二进制"
  [[ -f "${BINARY_PREV}" ]] || die "未找到备份二进制 ${BINARY_PREV}，无法回滚"
  systemctl stop "${SERVICE}"
  install -o root -g root -m 0755 "${BINARY_PREV}" "${BINARY}"
  systemctl start "${SERVICE}"
  ok "已回滚并启动；请执行 journalctl -u ${SERVICE} -n 50 确认"
  warn "注意：数据库结构不会随二进制回退（迁移已应用）。若新版本含破坏性迁移（如 V29～V33 列/schema 变更），必须从升级前备份恢复数据库。"
  exit 0
fi

# ---------------------------------------------------------------------
# 1. 定位新二进制
# ---------------------------------------------------------------------
step "1/6 定位新二进制"
if [[ -z "${BINARY_SRC}" ]]; then
  for candidate in "${REPO_ROOT}/finvquant" "${SCRIPT_DIR}/finvquant"; do
    if [[ -n "${candidate}" && -f "${candidate}" ]]; then BINARY_SRC="${candidate}"; break; fi
  done
fi
[[ -n "${BINARY_SRC}" && -f "${BINARY_SRC}" ]] || die "找不到新二进制；请用 --binary 指定"
ok "新二进制：${BINARY_SRC}"
NEW_VER="$("${BINARY_SRC}" --version 2>/dev/null || true)"
[[ -n "${NEW_VER}" ]] && ok "版本：${NEW_VER}"

MIGRATIONS_SRC=""
for candidate in "${REPO_ROOT}/Deploy/Migrations" "${SCRIPT_DIR}/Migrations"; do
  if [[ -d "${candidate}" ]]; then MIGRATIONS_SRC="${candidate}"; break; fi
done
[[ -n "${MIGRATIONS_SRC}" ]] || die "找不到迁移目录 Deploy/Migrations"
ok "迁移目录：${MIGRATIONS_SRC}"

# ---------------------------------------------------------------------
# 2. 备份数据库
# ---------------------------------------------------------------------
if [[ "${SKIP_BACKUP}" -eq 0 ]]; then
  step "2/6 备份数据库"
  mkdir -p "${BACKUP_DIR}"
  DB_NAME="$(grep -E '^FINV_PG_DATABASE=' "${ENV_FILE}" | cut -d= -f2- || echo finvquant)"
  BK_FILE="${BACKUP_DIR}/finvquant_$(date +%Y%m%d_%H%M%S).dump"
  if sudo -u postgres pg_dump -Fc "${DB_NAME}" -f "${BK_FILE}"; then
    chmod 0600 "${BK_FILE}"
    ok "备份完成：${BK_FILE}（$(du -h "${BK_FILE}" | cut -f1)）"
  else
    die "数据库备份失败；如确认可跳过请加 --skip-backup（不建议）"
  fi
else
  step "2/6 跳过数据库备份（应急模式）"
fi

# ---------------------------------------------------------------------
# 3. 停服务
# ---------------------------------------------------------------------
step "3/6 停止服务"
systemctl stop "${SERVICE}"
ok "服务已停止（避免新旧二进制同时连接数据库）"

# ---------------------------------------------------------------------
# 4. 替换二进制与迁移
# ---------------------------------------------------------------------
step "4/6 替换二进制与迁移脚本"
if [[ -f "${BINARY}" ]]; then
  install -o root -g root -m 0755 "${BINARY}" "${BINARY_PREV}"
  ok "旧二进制已备份为 ${BINARY_PREV}（可用 --rollback 回退程序）"
fi
install -o root -g root -m 0755 "${BINARY_SRC}" "${BINARY}"
ok "二进制已替换"
cp -f "${MIGRATIONS_SRC}"/*.sql "${MIGRATIONS_DIR}/"
chown -R root:finvquant "${MIGRATIONS_DIR}"
chmod 0644 "${MIGRATIONS_DIR}"/*.sql
ok "迁移脚本已同步（$(ls -1 "${MIGRATIONS_DIR}"/*.sql | wc -l) 个）"

# ---------------------------------------------------------------------
# 5. 启动
# ---------------------------------------------------------------------
step "5/6 启动服务"
systemctl start "${SERVICE}"
ok "已启动"

# ---------------------------------------------------------------------
# 6. 健康检查
# ---------------------------------------------------------------------
step "6/6 健康检查"
HEALTH_URL="http://127.0.0.1:16001/API/V1/Health/Ready"
HEALTHY=0
for i in $(seq 1 30); do
  if curl -fsS --max-time 2 "${HEALTH_URL}" >/dev/null 2>&1; then HEALTHY=1; ok "健康检查通过（${i}s）"; break; fi
  sleep 1
done

if [[ "${HEALTHY}" -eq 0 ]]; then
  warn "健康检查未通过，最近日志："
  journalctl -u "${SERVICE}" -n 40 --no-pager || true
  cat <<EOF

\033[1;31m升级未通过\033[0m
  回退程序：sudo $0 --rollback
  回退数据库（若新版本执行了破坏性迁移）：停止服务后
    sudo -u postgres pg_restore -d <DB> --clean --if-exists ${BACKUP_DIR}/<本次备份文件>
  排查：journalctl -u ${SERVICE} -n 100 --no-pager
EOF
  exit 1
fi

cat <<EOF

\033[1;32m升级完成\033[0m
  版本验证：curl http://127.0.0.1:16001/API/V1/Version
  迁移记录：sudo -u postgres psql -d <DB> -c "select version, success from schema_version order by version::int desc limit 5;"
  回滚程序：sudo $0 --rollback（仅程序；数据库需另行恢复）
  备份位置：${BACKUP_DIR}
EOF
