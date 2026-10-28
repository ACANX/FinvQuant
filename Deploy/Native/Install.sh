#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 安装脚本（宿主直装，不使用容器）
#
# 适用：Debian 12+ / Ubuntu 22.04+（systemd），root 执行
# 作用：装依赖 → 建用户与目录 → 创建数据库与角色 → 安装二进制、迁移脚本、
#       部署脚本与环境变量文件到统一安装根目录 → 注册并启动 systemd 服务 → 健康检查
# 特性：幂等（重复执行只补齐缺失项，不覆盖已有的 finvquant.env）
#
# 安装根目录：/data/App/FinvQuant（可用 --prefix 或 FINVQ_PREFIX 覆盖），布局：
#   <PREFIX>/bin/finvquant          二进制（另建软链 /usr/local/bin/finvquant）
#   <PREFIX>/Migrations/            数据库迁移脚本（FINV_MIGRATIONS_DIR 指向此目录）
#   <PREFIX>/etc/finvquant.env      核心配置（环境变量，权限 600）
#   <PREFIX>/Deploy/                部署脚本与部署文件（Install/Upgrade/Uninstall、unit、模板、手册）
#   <PREFIX>/Backup/                数据库逻辑备份（升级脚本写入）
#   <PREFIX>/Export/                行情归档导出目录（与容器方式一致）
#   /etc/systemd/system/finvquant.service   systemd unit（由 <PREFIX>/Deploy 的模板渲染后注册）
#
# 用法：
#   sudo ./Install.sh                     # 全量安装
#   sudo ./Install.sh --skip-db           # 跳过 PostgreSQL 安装与建库
#   sudo ./Install.sh --skip-redis        # 跳过 Redis 安装
#   sudo ./Install.sh --binary /path/to/finvquant   # 指定二进制路径
#   sudo ./Install.sh --prefix /data/App/FinvQuant   # 指定安装根目录
#
# 退出码：0 成功；非 0 失败（已在对应步骤打印原因）
# =====================================================================
set -euo pipefail

APP_USER="finvquant"
APP_GROUP="finvquant"

# 安装根目录：默认 /data/App/FinvQuant，可用 --prefix 或环境变量 FINVQ_PREFIX 覆盖
PREFIX="${FINVQ_PREFIX:-/data/App/FinvQuant}"
BIN_DIR="${PREFIX}/bin"
ETC_DIR="${PREFIX}/etc"
MIGRATIONS_DIR="${PREFIX}/Migrations"
BACKUP_DIR="${PREFIX}/Backup"
EXPORT_DIR="${PREFIX}/Export"
DEPLOY_DIR="${PREFIX}/Deploy"
ENV_FILE="${ETC_DIR}/finvquant.env"
BINARY="${BIN_DIR}/finvquant"
BINARY_LINK="/usr/local/bin/finvquant"
SERVICE_FILE="/etc/systemd/system/finvquant.service"
SERVICE="finvquant"

SKIP_DB=0
SKIP_REDIS=0
BINARY_SRC=""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

log()  { printf '\033[1;34m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
ok()   { printf '\033[1;32m  [OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m  [WARN]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m  [ERROR]\033[0m %s\n' "$*" >&2; exit 1; }
step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-db)    SKIP_DB=1; shift ;;
    --skip-redis) SKIP_REDIS=1; shift ;;
    --binary)     BINARY_SRC="${2:-}"; shift 2 ;;
    --prefix)     PREFIX="${2:-}"; shift 2
                  BIN_DIR="${PREFIX}/bin"; ETC_DIR="${PREFIX}/etc"
                  MIGRATIONS_DIR="${PREFIX}/Migrations"; BACKUP_DIR="${PREFIX}/Backup"
                  EXPORT_DIR="${PREFIX}/Export"; DEPLOY_DIR="${PREFIX}/Deploy"
                  ENV_FILE="${ETC_DIR}/finvquant.env"; BINARY="${BIN_DIR}/finvquant" ;;
    -h|--help)    sed -n '2,26p' "$0"; exit 0 ;;
    *)            die "未知参数：$1（--help 查看用法）" ;;
  esac
done

[[ "${PREFIX}" == /* ]] || die "--prefix 必须是绝对路径（当前：${PREFIX}）"

# ---------------------------------------------------------------------
# 0. 前置检查
# ---------------------------------------------------------------------
step "0/9 前置检查"
[[ "$(id -u)" -eq 0 ]] || die "请用 root 执行：sudo $0"
command -v systemctl >/dev/null 2>&1 || die "未检测到 systemd，本脚本仅支持 systemd 系统"
[[ -r /etc/os-release ]] || die "无法识别系统版本（缺少 /etc/os-release）"
# shellcheck disable=SC1091
. /etc/os-release
case "${ID:-}" in
  debian|ubuntu) ok "系统：${PRETTY_NAME}" ;;
  *) die "暂只支持 Debian/Ubuntu（当前：${PRETTY_NAME:-unknown}）；其它发行版请参考 Install.md 手工安装" ;;
esac
ok "安装根目录：${PREFIX}"

# ---------------------------------------------------------------------
# 1. 定位二进制与迁移目录
# ---------------------------------------------------------------------
step "1/9 定位安装产物"
if [[ -z "${BINARY_SRC}" ]]; then
  for candidate in "${REPO_ROOT}/finvquant" "${SCRIPT_DIR}/finvquant" "$(command -v finvquant || true)"; do
    if [[ -n "${candidate}" && -f "${candidate}" ]]; then
      # 跳过已安装位置自身（软链 /usr/local/bin/finvquant → <PREFIX>/bin/finvquant），避免 install 自我覆盖
      if [[ "$(readlink -f "${candidate}")" == "$(readlink -f "${BINARY}" 2>/dev/null || echo -)" ]]; then continue; fi
      BINARY_SRC="${candidate}"; break
    fi
  done
fi
[[ -n "${BINARY_SRC}" && -f "${BINARY_SRC}" ]] || \
  die "找不到 finvquant 二进制；请先构建（go build -o finvquant ./cmd/server）或用 --binary 指定"
ok "二进制：${BINARY_SRC}"

MIGRATIONS_SRC=""
# ① 发行包/安装后布局：<PREFIX>/Migrations（与 Deploy/ 同级）
# ② 源码仓库：<repo>/Deploy/Migrations
# ③ 其它：脚本同级或上一级
for candidate in "${REPO_ROOT}/Migrations" "${REPO_ROOT}/Deploy/Migrations" \
                 "${SCRIPT_DIR}/Migrations" "${SCRIPT_DIR}/../Migrations"; do
  if [[ -d "${candidate}" ]]; then MIGRATIONS_SRC="${candidate}"; break; fi
done
[[ -n "${MIGRATIONS_SRC}" ]] || die "找不到迁移目录（已尝试 <PREFIX>/Migrations、<repo>/Deploy/Migrations、脚本同级与上一级）"
ok "迁移目录：${MIGRATIONS_SRC}（$(ls -1 "${MIGRATIONS_SRC}"/*.sql | wc -l) 个脚本）"

# ---------------------------------------------------------------------
# 2. 基础依赖
# ---------------------------------------------------------------------
step "2/9 安装基础依赖"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg lsb-release sudo >/dev/null
ok "基础依赖已就绪"

# ---------------------------------------------------------------------
# 3. PostgreSQL 18（PGDG 源）
# ---------------------------------------------------------------------
if [[ "${SKIP_DB}" -eq 0 ]]; then
  step "3/9 安装 PostgreSQL 18"
  if command -v psql >/dev/null 2>&1 && psql --version | grep -q ' 18\.'; then
    ok "已安装 $(psql --version)"
  else
    install -d /usr/share/postgresql-common/pgdg
    curl -fsSL -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
      https://www.postgresql.org/media/keys/ACCC4CF8.asc
    echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt ${VERSION_CODENAME}-pgdg main" \
      > /etc/apt/sources.list.d/pgdg.list
    apt-get update -qq
    apt-get install -y -qq postgresql-18 postgresql-client-18 >/dev/null
    ok "已安装 $(psql --version)"
  fi
  systemctl enable --now postgresql >/dev/null 2>&1 || warn "postgresql 服务启动失败，请检查 journalctl -u postgresql"
else
  step "3/9 跳过 PostgreSQL"
fi

# ---------------------------------------------------------------------
# 4. Redis
# ---------------------------------------------------------------------
if [[ "${SKIP_REDIS}" -eq 0 ]]; then
  step "4/9 安装 Redis"
  if command -v redis-server >/dev/null 2>&1; then
    ok "已安装 redis-server $(redis-server --version | awk '{print $3}')"
  else
    apt-get install -y -qq redis-server >/dev/null
    ok "已安装 redis-server $(redis-server --version | awk '{print $3}')"
  fi
  # unit 名优先用 redis-server（Debian/Ubuntu 主 unit）；redis.service 通常只是它的 alias，
  # alias 无法 enable（会报 "no installation config"），不能作为首选。
  REDIS_UNIT="redis-server"
  if ! systemctl list-unit-files 2>/dev/null | grep -qE '^redis-server\.service'; then
    REDIS_UNIT="redis"
  fi
  systemctl enable "${REDIS_UNIT}" >/dev/null 2>&1 || true
  systemctl start "${REDIS_UNIT}" >/dev/null 2>&1 || true
  if systemctl is-active --quiet "${REDIS_UNIT}"; then
    ok "Redis 运行中（${REDIS_UNIT}）"
  else
    warn "Redis 服务未运行（${REDIS_UNIT}）；最近日志："
    journalctl -u "${REDIS_UNIT}" -n 15 --no-pager 2>/dev/null || true
  fi
  REDIS_VER="$(redis-server --version | awk '{print $3}' | cut -d= -f2 | cut -d. -f1)"
  [[ "${REDIS_VER:-0}" -ge 8 ]] || warn "当前 Redis 主版本为 ${REDIS_VER}，项目参考版本为 8；基础 KV 用法兼容，如需严格对齐请用 packages.redis.io 官方源"
else
  step "4/9 跳过 Redis"
fi

# ---------------------------------------------------------------------
# 5. 系统用户与目录（全部落在安装根目录下）
# ---------------------------------------------------------------------
step "5/9 创建用户与目录"
if getent group "${APP_GROUP}" >/dev/null 2>&1; then
  ok "用户组 ${APP_GROUP} 已存在"
else
  groupadd --system "${APP_GROUP}"
  ok "已创建用户组 ${APP_GROUP}"
fi
if id -u "${APP_USER}" >/dev/null 2>&1; then
  ok "用户 ${APP_USER} 已存在"
  # 主组不一致时补正（例如先手工 groupadd 再 useradd 的场景）
  if [[ "$(id -gn "${APP_USER}")" != "${APP_GROUP}" ]]; then
    usermod -g "${APP_GROUP}" "${APP_USER}"
    ok "已将 ${APP_USER} 的主组补正为 ${APP_GROUP}"
  fi
else
  # 必须先 -g 指定已存在的组：useradd 默认会尝试创建同名组，组已存在时直接报错
  # “useradd: group 'finvquant' exists - if you want to add this user to that group, use -g.”
  useradd --system --gid "${APP_GROUP}" --no-create-home --home-dir "${PREFIX}" \
    --shell /usr/sbin/nologin "${APP_USER}"
  ok "已创建系统用户 ${APP_USER}（主组 ${APP_GROUP}）"
fi
install -d -o root -g root -m 0755 "${PREFIX}"
install -d -o root -g root -m 0755 "${BIN_DIR}"
install -d -o root -g "${APP_GROUP}" -m 0755 "${MIGRATIONS_DIR}"
install -d -o root -g "${APP_GROUP}" -m 0750 "${ETC_DIR}"
install -d -o root -g "${APP_GROUP}" -m 0750 "${BACKUP_DIR}"
install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 "${EXPORT_DIR}"
install -d -o root -g root -m 0755 "${DEPLOY_DIR}"
ok "目录：${BIN_DIR} / ${MIGRATIONS_DIR} / ${ETC_DIR} / ${BACKUP_DIR} / ${EXPORT_DIR} / ${DEPLOY_DIR}"

# ---------------------------------------------------------------------
# 6. 安装二进制、迁移脚本与部署文件
# ---------------------------------------------------------------------
step "6/9 安装二进制、迁移脚本与部署文件"
install -o root -g root -m 0755 "${BINARY_SRC}" "${BINARY}"
ln -sfn "${BINARY}" "${BINARY_LINK}"
ok "二进制：${BINARY}（软链 ${BINARY_LINK}）"
if [[ "$(readlink -f "${MIGRATIONS_SRC}")" == "$(readlink -f "${MIGRATIONS_DIR}" 2>/dev/null || echo -)" ]]; then
  ok "迁移目录即安装目录，无需复制：${MIGRATIONS_DIR}"
else
  cp -f "${MIGRATIONS_SRC}"/*.sql "${MIGRATIONS_DIR}/"
  ok "迁移脚本：$(ls -1 "${MIGRATIONS_DIR}"/*.sql | wc -l) 个 → ${MIGRATIONS_DIR}"
fi
chown -R root:"${APP_GROUP}" "${MIGRATIONS_DIR}"
chmod 0644 "${MIGRATIONS_DIR}"/*.sql

# 部署脚本与模板随安装目录留存：后续可直接在 <PREFIX>/Deploy 内升级/卸载
# 注意：若直接在 <PREFIX>/Deploy 内执行本脚本，源与目标为同一文件，必须跳过
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
ok "部署文件：${DEPLOY_DIR}（脚本、unit 模板、env 模板、Nginx 示例、手册）"

# ---------------------------------------------------------------------
# 7. 环境变量文件（存在则不覆盖）
# ---------------------------------------------------------------------
step "7/9 环境变量文件"
DB_PASSWORD=""
if [[ -f "${ENV_FILE}" ]]; then
  ok "已存在 ${ENV_FILE}，保留现有配置（如需重置请先备份后删除）"
  DB_PASSWORD="$(grep -E '^FINV_PG_PASSWORD=' "${ENV_FILE}" | cut -d= -f2- || true)"
else
  DB_PASSWORD="$(openssl rand -hex 16)"
  install -o root -g root -m 0600 "${SCRIPT_DIR}/FinvQuant.env.example" "${ENV_FILE}"
  sed -i "s/^FINV_PG_PASSWORD=.*/FINV_PG_PASSWORD=${DB_PASSWORD}/" "${ENV_FILE}"
  # 模板中的默认路径按实际安装根目录改写
  sed -i "s|^FINV_MIGRATIONS_DIR=.*|FINV_MIGRATIONS_DIR=${MIGRATIONS_DIR}|" "${ENV_FILE}"
  sed -i "s|^FINV_QUOTE_DEDUB_OUTPUT_DIR=.*|FINV_QUOTE_DEDUB_OUTPUT_DIR=${EXPORT_DIR}|" "${ENV_FILE}"
  sed -i "s|^FINV_QUOTE_DEDUB_UPLOAD_DIR=.*|FINV_QUOTE_DEDUB_UPLOAD_DIR=${EXPORT_DIR}/Upload|" "${ENV_FILE}"
  ok "已生成 ${ENV_FILE}（随机数据库密码，权限 600）"
fi
[[ -n "${DB_PASSWORD}" ]] || die "未能读取或生成数据库密码，请检查 ${ENV_FILE}"

# ---------------------------------------------------------------------
# 8. 创建数据库与角色
# ---------------------------------------------------------------------
if [[ "${SKIP_DB}" -eq 0 ]]; then
  step "8/9 创建数据库与角色"
  DB_NAME="$(grep -E '^FINV_PG_DATABASE=' "${ENV_FILE}" | cut -d= -f2- || echo finv)"
  DB_USER="$(grep -E '^FINV_PG_USER=' "${ENV_FILE}" | cut -d= -f2- || echo finv)"
  run_psql() { sudo -u postgres psql -v ON_ERROR_STOP=1 -tAc "$1" >/dev/null; }
  run_psql "DO \$\$ BEGIN
      IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}') THEN
        CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASSWORD}';
      ELSE
        ALTER ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASSWORD}';
      END IF;
    END \$\$;"
  ok "角色 ${DB_USER} 已就绪"
  if sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1; then
    ok "数据库 ${DB_NAME} 已存在"
  else
    sudo -u postgres createdb -O "${DB_USER}" "${DB_NAME}"
    ok "已创建数据库 ${DB_NAME}（owner=${DB_USER}）"
  fi
  # 让应用在同一库上可建 schema（V32/V33 需要 CREATE SCHEMA）
  run_psql "GRANT ALL ON DATABASE ${DB_NAME} TO ${DB_USER};"
  sudo -u postgres psql -d "${DB_NAME}" -v ON_ERROR_STOP=1 -tAc "GRANT ALL ON SCHEMA public TO ${DB_USER};" >/dev/null
  ok "已授权（含 public schema 建表与 CREATE SCHEMA 权限）"
else
  step "8/9 跳过建库（请自行确保数据库与角色已存在）"
fi

# ---------------------------------------------------------------------
# 9. systemd 服务（unit 由模板渲染，同时留存到安装目录）
# ---------------------------------------------------------------------
step "9/9 注册并启动 systemd 服务"
sed "s|@PREFIX@|${PREFIX}|g" "${SCRIPT_DIR}/FinvQuant.service" > "${DEPLOY_DIR}/finvquant.service"
chown root:root "${DEPLOY_DIR}/finvquant.service"
chmod 0644 "${DEPLOY_DIR}/finvquant.service"
install -o root -g root -m 0644 "${DEPLOY_DIR}/finvquant.service" "${SERVICE_FILE}"
ok "unit：${SERVICE_FILE}（副本 ${DEPLOY_DIR}/finvquant.service）"
systemctl daemon-reload
systemctl enable "${SERVICE}" >/dev/null 2>&1
systemctl restart "${SERVICE}"
ok "已注册 ${SERVICE}.service 并启动"

HEALTH_URL="http://127.0.0.1:16001/API/V1/Health/Ready"
for i in $(seq 1 30); do
  if curl -fsS --max-time 2 "${HEALTH_URL}" >/dev/null 2>&1; then
    ok "健康检查通过（${i}s）：${HEALTH_URL}"
    break
  fi
  if [[ "${i}" -eq 30 ]]; then
    warn "健康检查未通过（30s）；请执行：journalctl -u ${SERVICE} -n 80 --no-pager"
    warn "常见原因：数据库连接失败、迁移执行失败、端口被占用"
  fi
  sleep 1
done

cat <<EOF

\033[1;32m安装完成\033[0m（安装根目录：${PREFIX}）
  服务管理：systemctl status|restart|stop ${SERVICE}
  日志查看：journalctl -u ${SERVICE} -f
  二进制：${BINARY}（软链 ${BINARY_LINK}）
  配置文件：${ENV_FILE}（权限 600）
  迁移目录：${MIGRATIONS_DIR}（启动时自动应用未执行版本）
  部署脚本：${DEPLOY_DIR}/Upgrade.sh、${DEPLOY_DIR}/Uninstall.sh
  备份目录：${BACKUP_DIR}
  导出目录：${EXPORT_DIR}
  接口验证：curl http://127.0.0.1:16001/API/V1/Version
  前端地址：http://<服务器IP>:16002（建议用 Nginx 反向代理，见 ${DEPLOY_DIR}/Nginx.conf.example）

后续步骤：
  1) 按需配置 Nginx 反向代理与 TLS（${DEPLOY_DIR}/Nginx.conf.example）
  2) 建立数据库定时备份到 ${BACKUP_DIR}（参考 Upgrade.md）
  3) 防火墙只放行 80/443（16001/16002 建议只绑 127.0.0.1）
EOF
