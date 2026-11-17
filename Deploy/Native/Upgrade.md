# Native 升级与回滚手册（宿主直装 + systemd）

> 适用：已按 [`Install.md`](Install.md) 完成部署的 Debian / Ubuntu 主机。
> 配套脚本：`Upgrade.sh`（升级/回滚）、`Uninstall.sh`（卸载）。
> **核心约束：升级 = 停服务 → 备份 → 换二进制 → 启服务；数据库迁移在新版本启动时自动执行，且不可自动回滚。**

## 0. 升级前必读

| 事项 | 说明 |
|---|---|
| 停机 | Native 升级**不是零停机**，需安排窗口（小库通常 < 1 分钟，`V29`～`V33` 等结构迁移视数据量而定） |
| 备份 | 升级脚本默认先 `pg_dump -Fc` 到 `/var/backups/finvquant/`；无备份不回滚数据库 |
| 禁止并用 | 新旧二进制**不得**同时连接同一数据库（列名/schema 已变） |
| 破坏性版本 | 含 `V29`（列改名）、`V32`/`V33`（schema 迁移）的版本升级后**无法靠换回旧二进制恢复数据**，必须用备份恢复 |
| 迁移来源 | 迁移脚本在 `/etc/finvquant/Migrations/`（由 `FINV_MIGRATIONS_DIR` 指定），升级脚本会同步新版本脚本 |

## 1. 标准升级流程（脚本）

```bash
# 1) 上传/构建新版本二进制与源码（迁移脚本随仓库提供）
#    假设目录结构：/root/finvquant-src/{finvquant, Deploy/Migrations, Deploy/Native}

# 2) 执行升级（默认会先备份数据库）
cd /root/finvquant-src
sudo bash Deploy/Native/Upgrade.sh

# 3) 或指定二进制路径
sudo bash Deploy/Native/Upgrade.sh --binary /tmp/finvquant

# 4) 应急跳过备份（不建议）
sudo bash Deploy/Native/Upgrade.sh --skip-backup
```

脚本流程与成功标志：

```
[OK] 新二进制：/root/finvquant-src/finvquant
[OK] 备份完成：/var/backups/finvquant/finvquant_20261024_012329.dump（68M）
[OK] 服务已停止
[OK] 旧二进制已备份为 /opt/finvquant/finvquant.prev
[OK] 二进制已替换
[OK] 迁移脚本已同步（57 个）
[OK] 已启动
[OK] 健康检查通过（3s）
升级完成
```

## 2. 升级后验证

```bash
curl -s http://127.0.0.1:16001/API/V1/Version          # version 与发布号一致
curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:16001/API/V1/Health/Ready   # 200
journalctl -u finvquant -n 80 --no-pager | grep -Ei 'migrat|迁移|error|panic'      # 迁移结果、无 ERROR

# 迁移记录（确认新版本号已应用）
sudo -u postgres psql -d finv -c \
  "select version, success from schema_version order by version::int desc limit 6;"

# 数据回归（示例：行情主表行数与时间范围）
sudo -u postgres psql -d finv -c \
  "select count(*), min(ts), max(ts) from quote.finv_quote_secu_kline_min;"
```

## 3. 回滚

### 3.1 只回退程序（数据库未发生破坏性变更）

```bash
sudo bash Deploy/Native/Upgrade.sh --rollback
```

脚本会把 `/opt/finvquant/finvquant.prev` 换回主二进制并重启。

> 适用前提：新版本只做了增量迁移（如新增可空列）。若新版本执行过**列改名 / 删列 / schema 迁移**，旧二进制无法读取新结构，必须走 3.2。

### 3.2 回退数据库（破坏性变更后）

```bash
sudo systemctl stop finvquant
sudo -u postgres pg_restore -d finv --clean --if-exists \
  /var/backups/finvquant/finvquant_20261024_012329.dump
sudo -u postgres psql -d finv -c "select max(version::int) from schema_version;"
sudo systemctl start finvquant
```

注意：`--clean --if-exists` 会先删除现有对象再重建，**必须在服务停止时执行**，且确认备份文件完整（`pg_restore --list <file>` 可列出内容）。

### 3.3 数据库恢复后的清理

`schema_version` 会随备份一起回退到旧状态；服务启动时会重新应用「备份时间点之后」的迁移，这是预期行为。

## 4. 停机窗口外的准备工作

| 步骤 | 命令 |
|---|---|
| 预留磁盘（备份 + WAL） | `df -h /var/backups /var/lib/postgresql` |
| 校验备份可用性 | `pg_restore --list /var/backups/finvquant/<file>.dump \| head` |
| 记录当前版本与迁移版本 | `curl -s .../Version`、`select max(version::int) from schema_version;` |
| 通知使用方 | 升级期间 API/Web 不可用 |

## 5. 定时备份（建议）

```bash
# /etc/cron.d/finvquant-backup
# 每日 03:10 备份，保留 14 天
10 3 * * * postgres pg_dump -Fc finvquant -f /var/backups/finvquant/finvquant_$(date +\%Y\%m\%d_\%H\%M\%S).dump
20 3 * * * root find /var/backups/finvquant -name 'finvquant_*.dump' -mtime +14 -delete
```

> 文件名必须为 `finvquant_YYYYMMDD_HHMMSS.dump`（纯 ASCII、可排序），见 `Docs/DevSpec/FileNamingSpec.md` 第 5 节。

## 6. 常见问题

| 现象 | 处理 |
|---|---|
| 升级后 `Health/Ready` 一直 503 | `journalctl -u finvquant -n 100`；多为迁移失败或数据库连接配置错误 |
| 日志 `迁移 V<n> 失败` | 迁移单事务执行已回滚；常见为权限不足（需对目标 schema 有 `CREATE` 权限） |
| 回滚后仍报列不存在 | 数据库已被迁移改动，需按 3.2 从备份恢复 |
| 备份文件为 0 字节 | `pg_dump` 失败（磁盘满/权限），检查后重新备份 |
| 服务反复重启 | `systemctl status finvquant` + `journalctl`；确认 `FINV_PG_*` 与 `pg_hba.conf` |
