#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 升级脚本（宿主直装，systemd 守护）
#
# 安装根目录：/data/App/FinvQuant（由 systemd unit 的 ExecStart 推断，
#             可用 --prefix 或环境变量 FINVQ_PREFIX 覆盖）
#   <PREFIX>/bin/finvquant        当前二进制（升级备份为 finvquant.prev）
#   <PREFIX>/Migrations/          数据库迁移脚本
#   <PREFIX>/etc/finvquant.env    核心配置
#   <PREFIX>/Deploy/              部署脚本与部署文件（升级时一并刷新）
#   <PREFIX>/Backup/              数据库逻辑备份（本脚本写入）
#
# 流程：备份数据库 → 停服务 → 备份旧二进制 → 替换二进制/迁移/部署文件
#       → 启动服务 → 健康检查（失败时给出一键回滚命令）
#
# 用法：
#   sudo ./Upgrade.sh                          # 使用仓库/脚本目录下的 finvquant 升级
#   sudo ./Upgrade.sh --binary /tmp/finvquant  # 指定新二进制
#   sudo ./Upgrade.sh --skip-backup            # 跳过数据库备份（仅应急）
#   sudo ./Upgrade.sh --prefix /data/App/FinvQuant
#
# 回滚：sudo ./Upgrade.sh --rollback
# 说明：数据库迁移由新二进制启动时自动执行，**不可自动回滚**；如需回退数据库，
#       只能从本次备份恢复（见 Upgrade.md）。
# =====================================================================
set -euo pipefail

APP_USER="finvquant"
APP_GROUP="finvquant"
SERVICE="finvquant"
SERVICE_FILE="/etc/systemd/system/${SERVICE}.service"
BINARY_LINK="/usr/local/bin/finvquant"

PREFIX_EXPLICIT=0
# 安装根目录：优先 --prefix / FINVQ_PREFIX，其次由 unit 的 ExecStart 推断，最后回退默认值
infer_prefix() {
  local detected=""
  if [[ "${PREFIX_EXPLICIT}" -eq 0 && -f "${SERVICE_FILE}" ]]; then
    detected="$(sed -n 's|^ExecStart=\(.*\)/bin/finvquant$|\1|p' "${SERVICE_FILE}" | head -1)"
  fi
  printf '%s' "${detected:-/data/App/FinvQuant}"
}
PREFIX="${FINVQ_PREFIX:-$(infer_prefix)}"

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

set_prefix() {
  PREFIX="$1"
  BIN_DIR="${PREFIX}/bin"
  MIGRATIONS_DIR="${PREFIX}/Migrations"
  ETC_DIR="${PREFIX}/etc"
  DEPLOY_DIR="${PREFIX}/Deploy"
  BACKUP_DIR="${PREFIX}/Backup"
  ENV_FILE="${ETC_DIR}/finvquant.env"
  BINARY="${BIN_DIR}/finvquant"
  BINARY_PREV="${BIN_DIR}/finvquant.prev"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)      BINARY_SRC="${2:-}"; shift 2 ;;
    --prefix)      PREFIX_EXPLICIT=1; PREFIX="${2:-}"; shift 2 ;;
    --skip-backup) SKIP_BACKUP=1; shift ;;
    --rollback)    ROLLBACK=1; shift ;;
    -h|--help)     sed -n '2,25p' "$0"; exit 0 ;;
    *)             die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[[ "${PREFIX}" == /* ]] || die "--prefix 必须是绝对路径（当前：${PREFIX}）"
set_prefix "${PREFIX}"

[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行：sudo $0"
[[ -f "${ENV_FILE}" ]] || die "未找到 ${ENV_FILE}，请先执行 Install.sh（或用 --prefix 指定安装根目录）"
ok "安装根目录：${PREFIX}"

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
# 1. 定位新二进制与迁移目录
# ---------------------------------------------------------------------
step "1/6 定位新二进制"
if [[ -z "${BINARY_SRC}" ]]; then
  for candidate in "${REPO_ROOT}/finvquant" "${SCRIPT_DIR}/finvquant"; do
    if [[ -n "${candidate}" && -f "${candidate}" ]]; then BINARY_SRC="${candidate}"; break; fi
  done
fi
[[ -n "${BINARY_SRC}" && -f "${BINARY_SRC}" ]] || die "找不到新二进制；请用 --binary 指定"
if [[ "$(readlink -f "${BINARY_SRC}")" == "$(readlink -f "${BINARY}" 2>/dev/null || echo -)" ]]; then
  die "新二进制与已安装二进制为同一文件（${BINARY}）；请用 --binary 指定真正的升级包"
fi
ok "新二进制：${BINARY_SRC}"
NEW_VER="$("${BINARY_SRC}" --version 2>/dev/null || true)"
if [[ -n "${NEW_VER}" ]]; then ok "版本：${NEW_VER}"; fi

MIGRATIONS_SRC=""
for candidate in "${REPO_ROOT}/Deploy/Migrations" "${SCRIPT_DIR}/Migrations"; do
  if [[ -d "${candidate}" ]]; then MIGRATIONS_SRC="${candidate}"; break; fi
done
[[ -n "${MIGRATIONS_SRC}" ]] || die "找不到迁移目录 Deploy/Migrations"
ok "迁移目录：${MIGRATIONS_SRC}"

# ---------------------------------------------------------------------
# 2. 备份数据库（写入安装目录下的 Backup/）
# ---------------------------------------------------------------------
if [[ "${SKIP_BACKUP}" -eq 0 ]]; then
  step "2/6 备份数据库"
  install -d -o root -g "${APP_GROUP}" -m 0750 "${BACKUP_DIR}"
  DB_NAME="$(grep -E '^FINV_PG_DATABASE=' "${ENV_FILE}" | cut -d= -f2- || echo finv)"
  BK_FILE="${BACKUP_DIR}/finvquant_$(date +%Y%m%d_%H%M%S).dump"
  # 由 root 重定向写入（目录属主 root，postgres 无权写入）
  if sudo -u postgres pg_dump -Fc "${DB_NAME}" > "${BK_FILE}"; then
    chmod 0600 "${BK_FILE}"
    ok "备份完成：${BK_FILE}（$(du -h "${BK_FILE}" | cut -f1)）"
  else
    rm -f "${BK_FILE}"
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
# 4. 替换二进制、迁移与部署文件
# ---------------------------------------------------------------------
step "4/6 替换二进制、迁移脚本与部署文件"
if [[ -f "${BINARY}" ]]; then
  install -o root -g root -m 0755 "${BINARY}" "${BINARY_PREV}"
  ok "旧二进制已备份为 ${BINARY_PREV}（可用 --rollback 回退程序）"
fi
install -o root -g root -m 0755 "${BINARY_SRC}" "${BINARY}"
ln -sfn "${BINARY}" "${BINARY_LINK}"
ok "二进制已替换：${BINARY}"
cp -f "${MIGRATIONS_SRC}"/*.sql "${MIGRATIONS_DIR}/"
chown -R root:"${APP_GROUP}" "${MIGRATIONS_DIR}"
chmod 0644 "${MIGRATIONS_DIR}"/*.sql
ok "迁移脚本已同步（$(ls -1 "${MIGRATIONS_DIR}"/*.sql | wc -l) 个）"

# 刷新部署脚本与模板（脚本自身也在此目录，源与目标相同时跳过，避免 cp 自我复制失败）
install -d -o root -g root -m 0755 "${DEPLOY_DIR}"
copy_deploy_file() {
  local src="$1" dst="$2"
  if [[ "$(readlink -f "${src}")" != "$(readlink -f "${dst}" 2>/dev/null || echo -)" ]]; then
    cp -f "${src}" "${dst}"
  fi
}
for f in Install.sh Upgrade.sh Uninstall.sh FinvQuant.service FinvQuant.env.example Nginx.conf.example; do
  copy_deploy_file "${SCRIPT_DIR}/${f}" "${DEPLOY_DIR}/${f}"
done
for md in "${SCRIPT_DIR}"/*.md; do
  [[ -f "${md}" ]] || continue
  copy_deploy_file "${md}" "${DEPLOY_DIR}/$(basename "${md}")"
done
chown -R root:root "${DEPLOY_DIR}"
chmod 0755 "${DEPLOY_DIR}"/*.sh
for f in "${DEPLOY_DIR}"/*.md "${DEPLOY_DIR}"/*.example "${DEPLOY_DIR}"/*.service; do
  if [[ -f "${f}" ]]; then chmod 0644 "${f}"; fi
done
sed "s|@PREFIX@|${PREFIX}|g" "${DEPLOY_DIR}/FinvQuant.service" > "${DEPLOY_DIR}/finvquant.service"
chown root:root "${DEPLOY_DIR}/finvquant.service"
chmod 0644 "${DEPLOY_DIR}/finvquant.service"
install -o root -g root -m 0644 "${DEPLOY_DIR}/finvquant.service" "${SERVICE_FILE}"
systemctl daemon-reload
ok "部署文件与 unit 已刷新：${DEPLOY_DIR}"

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
