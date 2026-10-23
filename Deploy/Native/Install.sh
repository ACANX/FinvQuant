#!/usr/bin/env bash
# =====================================================================
# FinvQuant Native 安装脚本（宿主直装，不使用容器）
#
# 适用：Debian 12+ / Ubuntu 22.04+（systemd），root 执行
# 作用：装依赖 → 建用户与目录 → 创建数据库与角色 → 安装二进制与迁移
#       → 写入环境变量 → 注册并启动 systemd 服务 → 健康检查
# 特性：幂等（重复执行只补齐缺失项，不覆盖已有的 /etc/finvquant/finvquant.env）
#
# 用法：
#   sudo ./Install.sh                     # 全量安装
#   sudo ./Install.sh --skip-db           # 跳过 PostgreSQL 安装与建库
#   sudo ./Install.sh --skip-redis        # 跳过 Redis 安装
#   sudo ./Install.sh --binary /path/to/finvquant   # 指定二进制路径
#
# 退出码：0 成功；非 0 失败（已在对应步骤打印原因）
# =====================================================================
set -euo pipefail

APP_USER="finvquant"
APP_GROUP="finvquant"
APP_DIR="/opt/finvquant"
ETC_DIR="/etc/finvquant"
MIGRATIONS_DIR="${ETC_DIR}/Migrations"
SERVICE_FILE="/etc/systemd/system/finvquant.service"
BINARY_DST="/usr/local/bin/finvquant"

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
    -h|--help)    sed -n '2,20p' "$0"; exit 0 ;;
    *)            die "未知参数：$1（--help 查看用法）" ;;
  esac
done

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

# ---------------------------------------------------------------------
# 1. 定位二进制与迁移目录
# ---------------------------------------------------------------------
step "1/9 定位安装产物"
if [[ -z "${BINARY_SRC}" ]]; then
  for candidate in "${REPO_ROOT}/finvquant" "${SCRIPT_DIR}/finvquant" "$(command -v finvquant || true)"; do
    if [[ -n "${candidate}" && -f "${candidate}" ]]; then BINARY_SRC="${candidate}"; break; fi
  done
fi
[[ -n "${BINARY_SRC}" && -f "${BINARY_SRC}" ]] || \
  die "找不到 finvquant 二进制；请先构建（go build -o finvquant ./cmd/server）或用 --binary 指定"
ok "二进制：${BINARY_SRC}"

MIGRATIONS_SRC=""
for candidate in "${REPO_ROOT}/Deploy/Migrations" "${SCRIPT_DIR}/Migrations"; do
  if [[ -d "${candidate}" ]]; then MIGRATIONS_SRC="${candidate}"; break; fi
done
[[ -n "${MIGRATIONS_SRC}" ]] || die "找不到迁移目录 Deploy/Migrations"
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
  REDIS_UNIT="redis-server"
  systemctl list-unit-files | grep -q '^redis.service' && REDIS_UNIT="redis"
  systemctl enable --now "${REDIS_UNIT}" >/dev/null 2>&1 || warn "Redis 服务启动失败，请检查 journalctl -u ${REDIS_UNIT}"
  REDIS_VER="$(redis-server --version | awk '{print $3}' | cut -d= -f2 | cut -d. -f1)"
  [[ "${REDIS_VER:-0}" -ge 8 ]] || warn "当前 Redis 主版本为 ${REDIS_VER}，项目参考版本为 8；基础 KV 用法兼容，如需严格对齐请用 packages.redis.io 官方源"
else
  step "4/9 跳过 Redis"
fi

# ---------------------------------------------------------------------
# 5. 系统用户与目录
# ---------------------------------------------------------------------
step "5/9 创建用户与目录"
if id -u "${APP_USER}" >/dev/null 2>&1; then
  ok "用户 ${APP_USER} 已存在"
else
  useradd --system --create-home --home-dir "${APP_DIR}" --shell /usr/sbin/nologin "${APP_USER}"
  ok "已创建系统用户 ${APP_USER}"
fi
install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 "${APP_DIR}"
install -d -o root -g "${APP_GROUP}" -m 0750 "${ETC_DIR}"
install -d -o root -g "${APP_GROUP}" -m 0755 "${MIGRATIONS_DIR}"
ok "目录：${APP_DIR} / ${ETC_DIR} / ${MIGRATIONS_DIR}"

# ---------------------------------------------------------------------
# 6. 安装二进制与迁移脚本
# ---------------------------------------------------------------------
step "6/9 安装二进制与迁移脚本"
install -o root -g root -m 0755 "${BINARY_SRC}" "${APP_DIR}/finvquant"
ln -sfn "${APP_DIR}/finvquant" "${BINARY_DST}"
ok "二进制：${APP_DIR}/finvquant（软链 ${BINARY_DST}）"
cp -f "${MIGRATIONS_SRC}"/*.sql "${MIGRATIONS_DIR}/"
chown -R root:"${APP_GROUP}" "${MIGRATIONS_DIR}"
chmod 0644 "${MIGRATIONS_DIR}"/*.sql
ok "迁移脚本：$(ls -1 "${MIGRATIONS_DIR}"/*.sql | wc -l) 个"

# ---------------------------------------------------------------------
# 7. 环境变量文件（存在则不覆盖）
# ---------------------------------------------------------------------
step "7/9 环境变量文件"
ENV_FILE="${ETC_DIR}/finvquant.env"
DB_PASSWORD=""
if [[ -f "${ENV_FILE}" ]]; then
  ok "已存在 ${ENV_FILE}，保留现有配置（如需重置请先备份后删除）"
  DB_PASSWORD="$(grep -E '^FINV_PG_PASSWORD=' "${ENV_FILE}" | cut -d= -f2- || true)"
else
  DB_PASSWORD="$(openssl rand -hex 16)"
  install -o root -g root -m 0600 "${SCRIPT_DIR}/FinvQuant.env.example" "${ENV_FILE}"
  sed -i "s/^FINV_PG_PASSWORD=.*/FINV_PG_PASSWORD=${DB_PASSWORD}/" "${ENV_FILE}"
  ok "已生成 ${ENV_FILE}（随机数据库密码，权限 600）"
fi
[[ -n "${DB_PASSWORD}" ]] || die "未能读取或生成数据库密码，请检查 ${ENV_FILE}"

# ---------------------------------------------------------------------
# 8. 创建数据库与角色
# ---------------------------------------------------------------------
if [[ "${SKIP_DB}" -eq 0 ]]; then
  step "8/9 创建数据库与角色"
  DB_NAME="$(grep -E '^FINV_PG_DATABASE=' "${ENV_FILE}" | cut -d= -f2- || echo finvquant)"
  DB_USER="$(grep -E '^FINV_PG_USER=' "${ENV_FILE}" | cut -d= -f2- || echo finvquant)"
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
# 9. systemd 服务
# ---------------------------------------------------------------------
step "9/9 注册并启动 systemd 服务"
install -o root -g root -m 0644 "${SCRIPT_DIR}/FinvQuant.service" "${SERVICE_FILE}"
systemctl daemon-reload
systemctl enable finvquant >/dev/null 2>&1
systemctl restart finvquant
ok "已注册 finvquant.service 并启动"

HEALTH_URL="http://127.0.0.1:16001/API/V1/Health/Ready"
for i in $(seq 1 30); do
  if curl -fsS --max-time 2 "${HEALTH_URL}" >/dev/null 2>&1; then
    ok "健康检查通过（${i}s）：${HEALTH_URL}"
    break
  fi
  if [[ "${i}" -eq 30 ]]; then
    warn "健康检查未通过（30s）；请执行：journalctl -u finvquant -n 80 --no-pager"
    warn "常见原因：数据库连接失败、迁移执行失败、端口被占用"
  fi
  sleep 1
done

cat <<EOF

\033[1;32m安装完成\033[0m
  服务管理：systemctl status|restart|stop finvquant
  日志查看：journalctl -u finvquant -f
  配置文件：${ENV_FILE}（权限 600）
  迁移目录：${MIGRATIONS_DIR}（启动时自动应用未执行版本）
  接口验证：curl http://127.0.0.1:16001/API/V1/Version
  前端地址：http://<服务器IP>:16002（建议用 Nginx 反向代理，见 Nginx.conf.example）

后续步骤：
  1) 按需配置 Nginx 反向代理与 TLS（Nginx.conf.example）
  2) 建立数据库定时备份（参考 Upgrade.md 的备份命令）
  3) 防火墙只放行 80/443（16001/16002 建议只绑 127.0.0.1）
EOF
