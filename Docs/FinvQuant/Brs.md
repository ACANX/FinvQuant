# BRS 数据备份服务（首期：PostgreSQL 表级备份）

> 源码：`internal/brs`（调度/执行）、`internal/meta/base/cron`（6 段 cron）、`internal/meta/base/oss`（对象存储抽象，二期）
> 配置：`sys.sys_config` **39001**（每日备份 cron）
> 登记表：`brs.brs_bak_rdb_table`
> 备份实现：`pg_dump --table=<schema>.<table> -Fc --compress=<算法>`

## 1. 功能

- 每天按 cron 自动执行一次表级备份；
- 备份范围由登记表控制（`priority` 唯一，按倒序执行）；
- 备份文件保存在 `<PREFIX>/Backup/BRS/<yyyyMMdd>/`；
- 过程只写服务日志（`Log/FinvQuant.log`，前缀 `[brs]`），暂不落库。

## 2. 配置

### 2.1 定时 cron（39001）

`sys.sys_config.config_no = 39001`，`str_value` 为 **6 段** cron：

```text
秒 分 时 日 月 周
0  0  3  *  *  *     # 每天 03:00:00（默认）
```

- 周：`0-6`，`0=周日`；
- 支持 `*`、`?`（仅日/周）、数字、`a-b`、`*/n`、`a-b/n`、列表；
- 修改后最多 **60 秒**生效（服务端缓存）；
- 非法表达式只记日志并跳过，不影响主服务。

### 2.2 登记表

`brs.brs_bak_rdb_table`：`idn`、`rdb_type`、`schema_name`、`table_name`、
`flag_enable`（`'1'/'0'`）、`priority`（**必填且唯一**）、`bak_scope`（both/schema/data）、
`bak_method`（pg_dump）、`compression`（none/gzip/zstd）、`where_clause`、`remark`、
`creator`（uid）、`gmt_create`、`gmt_update`。

### 2.3 备份目录

```env
FINV_BRS_BACKUP_DIR=/data/App/FinvQuant/Backup/BRS
```

目录由服务自动创建（`0750`）。

## 3. 备份产物

```
Backup/BRS/20261029/
├── sys.sys_user_030000.dump
├── sys.sys_config_030001.dump
├── cdeu.finv_quote_cdeu_workflow_030002.dump
└── ...
```

- `-Fc` 自定义格式（`pg_restore` 可选择性恢复）；
- 压缩在文件内部（`--compress=zstd:3`），无需再 `gzip`。

## 4. 查看与恢复

```bash
grep '\[brs\]' /data/App/FinvQuant/Log/FinvQuant.log | tail -20
journalctl -u finvquant | grep '\[brs\]' | tail -20

pg_restore -l Backup/BRS/20261029/sys.sys_config_030001.dump

pg_restore -h 127.0.0.1 -U finv -d finv \
  --table=sys.sys_config --clean --if-exists \
  Backup/BRS/20261029/sys.sys_config_030001.dump
```

## 5. 预置登记（种子）

`sys.sys_user/sys_config/sys_dict/sys_dict_sub` 与 `cdeu` 配置表（workflow/phase/process/job/job_step/usc/trade_calendar/daily_budget）。

**未包含** `cdeu.finv_quote_secu_kline_min`（大表后续单独处理）。

## 6. 对象存储（二期）

- 抽象层：`internal/meta/base/oss`（`Store` 接口 + `Config` + `New`），业务只依赖接口；
- 适配器：`oss/local`（单测/离线）、`oss/s3`（二期，`aws-sdk-go-v2/service/s3`）；
- **不使用 `minio-go` 及任何 MinIO 官方库**；
- 配置 `sys.sys_config 39002`（JSON）：`enabled/provider/endpoint/bucket/region/access_key/secret_key/prefix`；
- 上传 key：`<prefix>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump`；
- 失败保留本地文件并重试，不影响下一张表。

## 7. 手动验证

临时把 39001 改为每秒触发（测试后务必改回）：

```sql
UPDATE sys.sys_config SET str_value = '* * * * * *', gmt_modify = now() WHERE config_no = 39001;
-- 观察日志与 Backup/BRS/<今天>/ 下的 dump
UPDATE sys.sys_config SET str_value = '0 0 3 * * *', gmt_modify = now() WHERE config_no = 39001;
```

## 8. 注意事项

- `pg_dump` 需在服务进程 `PATH` 中（随 PostgreSQL 客户端安装）；
- 密码通过子进程 `PGPASSWORD` 传递，不落命令行；
- 单表备份超时 30 分钟；失败继续下一张表，最后汇总；
- `where_clause` 首期仅登记、不生效（`pg_dump` 不支持行过滤）；
- 大表建议后续单独方案，不要直接登记。
