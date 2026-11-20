# BRS 数据备份服务（首期：PostgreSQL 表级备份）

> 源码：`internal/meta/finv/brs`（调度/执行）、`internal/meta/base/cron`（6 段 cron）、`internal/meta/base/oss`（对象存储抽象，二期）
> 配置：`sys.sys_config` **39001**（每日备份 cron）
> 登记表：`brs.brs_bak_rdb_table_full`
> 备份实现：`pg_dump --table=<schema>.<table> -Fc --compress=<算法>`

## 1. 功能

BRS 有**两套相互独立的备份策略**，各自一张登记表、各自一条执行链：

| 策略 | 适用对象 | 登记表 | 产物 | 触发 |
|------|----------|--------|------|------|
| 小表全量 | 配置/字典/元数据等小表（24 张） | `brs.brs_bak_rdb_table_full` | 整表 1 个 `.dump`（`pg_dump -Fc`） | 39001 cron + 页面手动 |
| 大表增量（分片） | 体量大、按天/周/月增长的表（如 K 线） | `brs.brs_bak_rdb_table_incr` | 按分片键 1 片 1 个 `.mvsv[.gz]`（MVSV-2） | 页面手动（本期） |

- 小表全量：每天按 cron 自动执行一次，备份范围由登记表控制（`priority` 唯一，倒序执行）；
- 备份文件保存在 `$FINV_BRS_BACKUP_DIR/Full/<yyyyMMdd>/`（`Full/` 与大表增量的 `Incr/` 对称）；Native 与容器**统一**为 `/data/App/FinvQuant/BRS`（Native 由 `Install.sh`/`Upgrade.sh` 预建该目录与 `Full/`、`Incr/` 子目录并补齐 env 项；容器由 compose volume 映射到宿主 `FINV_BRS_HOST_DIR`）；文件名 `<db>.<schema>.<table>.dump`（库名前缀取登记 `db`），不含时间戳；
- 大表分片：按分片维度（日/周/月/列值/自定义条件）把表切成多个文件，**同一分片键覆盖同一文件**；已存在的分片按**内容指纹**比对，数据未变才跳过，变了就重做（见 §1.1）；
- 过程只写服务日志（`Log/FinvQuant.log`，前缀 `[brs]`），运行状态与进度由页面内存展示，暂不落库。

### 1.1 大表增量（分片）备份

配置入口：页面「备份与恢复 → 备份管理 → 大表增量备份」（接口见 [`../API/BRS/Slice.md`](../API/BRS/Slice.md)）。

| 项 | 说明 |
|------|------|
| 登记表 | `brs.brs_bak_rdb_table_incr`（迁移 `V300088` 建表、`V300090` 由 `_slice` 改名为 `_incr`；首期种子：`cdeu.finv_quote_secu_kline_min` 按自然日） |
| 切分依据 | `slice_mode`（`date`/`week`/`month`/`column`/`expr`）+ `slice_where_tpl`（占位 `{value}`/`{start}`/`{end}`） |
| 导出方式 | 按行读取分片数据后写 **MVSV-2**（复用 `mvsv.WriteV2`，与 `qiom export` 同格式）；`pg_dump` 无行级过滤（无 `--where`），故不用它 |
| 本地产物 | `<备份目录>/Incr/<db>.<schema>.<表>/<分片键>.mvsv.gz`（`compression=none` 时 `.mvsv`） |
| 远端 key | `<prefix>/<env>/Incr/<db>.<schema>.<表>/<分片键>.mvsv.gz`（`Incr/` 与本地目录对应） |
| 增量判定 | 分片不存在 → 备份；已存在 → **比对内容指纹**（行数 + 全行 `hashtextextended` 哈希和）：相同跳过、不同重做；指纹存于本地 `<分片文件>.fp` 与远端 `<key>.fp` |
| 旧产物自愈 | 缺指纹（10916 之前）→ **重做一次建立基线**并写回指纹；`redo_slices=0` 时改为跳过并 `warn`（只补缺失分片） |
| 指纹成本 | 每个分片一次范围扫描（不导出、不压缩、不上传），4 万行分片约数十毫秒 |
| 粒度保护 | `min_rows`（低于则跳过）、`max_slices`（超过则拒绝执行，防误配） |
| 目录预检 | 启动时与每轮执行前各预检一次 `<备份目录>/Incr` 是否可写；不可写时**整轮一次性失败**并在页面顶部红色告警（`backup_dir_ok=false` + `backup_dir_error`），不逐分片重复报错 |
| 目录创建 | Native：`Install.sh`/`Upgrade.sh` 预建 `<PREFIX>/BRS` 与 `<PREFIX>/BRS/Incr`（属主运行用户，0750）；容器：`FINV_BRS_BACKUP_DIR` 指向的 volume 需可写 |
| 恢复 | 目标库先由迁移建好表结构，再 `qiom import <分片>.mvsv.gz`（默认 upsert，可按需 `qiom verify` 先校验） |

```bash
# 本机实测（10 个自然日、27.7 万行）
# 首轮：10 个分片全部导出并上传，约 17 s（同时写入指纹）
# 第二轮：指纹一致 → 全部跳过（done=0）；数据有变化的分片才重做
# 备份成功日志会带本地产物路径：…（37355 行，389.7 KiB，耗时 656ms）→ <备份目录>/Incr/<表>/<分片键>.mvsv.gz
# 产物 20260930.mvsv.gz：qiom verify 校验通过，Count=37355（与源表该日行数一致）
```

> 本期只支持手动触发；要不要接入定时（跑完小表后接着跑）待验收后再定。

### 1.2 本地空间与清理

| 策略 | 本地增长特征 | 清理方式 |
|------|--------------|----------|
| 小表全量 | **每天一个新日期目录**（`<备份目录>/Full/<yyyyMMdd>/`，每表一个 dump），不清理会持续累积 | 每轮结束自动清理：删除超过 `FINV_BRS_LOCAL_KEEP_DAYS`（默认 7 天）**且远端已存在**的 dump，空目录一并删除 |
| 大表增量 | 文件数 = 分片数（按分片键覆盖），**不随时间增长** | 不自动删除（分片本身就是全量集合）；需要释放时可手工删除 `<备份目录>/Incr/<表>/` 下的旧分片，下一轮会自动重新导出缺失分片 |

### 1.3 磁盘水位告警

| 项 | 说明 |
|------|------|
| 检测点 | 备份目录所在文件系统（目录尚未创建时沿父目录向上取最近的已存在目录） |
| 检测时机 | 每轮备份开始前与结束后各一次；运行器启动时也取一次初值（页面打开即可见） |
| 阈值 | `FINV_BRS_DISK_MIN_FREE_GB`（默认 **5 GB**，`<=0` 关闭告警） |
| 告警方式 | 日志 `warn`（含可用/阈值/路径与清理建议）+ 页面顶部黄色告警条 + 状态行「磁盘剩余 / 阈值」 |
| 平台 | Linux/macOS 走 `syscall.Statfs`；Windows 返回「未知」（不影响 Linux 生产环境） |

```bash
# 无告警时状态接口字段
curl -s '.../Run/Status' -H "Authorization: Bearer $T" | jq '.data | {disk_path,disk_free_bytes,disk_min_free_bytes,disk_low}'
```

清理的安全边界（宁可不删，不可误删）：

| 场景 | 行为 |
|------|------|
| 对象存储未启用 / 运行环境标识（1002）缺失或非法 | **完全跳过清理**并 `warn`（本地是唯一副本） |
| 某个 dump 在远端不存在（上传曾失败） | **保留**该文件，日志计入「因远端不存在而保留」 |
| `FINV_BRS_LOCAL_KEEP_DAYS<=0` | 不清理，保留全部本地产物（`info` 提示） |
| `Incr/` 分片目录、非 `yyyyMMdd` 目录 | 不参与清理；10915 之前的旧位置 `<备份目录>/<yyyyMMdd>/` 仍按同一规则清理 |

```bash
# 查看当前占用
du -sh /data/App/FinvQuant/BRS /data/App/FinvQuant/BRS/Incr/*

# 日志（每轮结束）
grep '本地清理完成' /data/App/FinvQuant/Log/FinvQuant.log | tail -3
# [brs] 本地清理完成：删除 24 个文件（释放 512.0 KB）、1 个空目录；0 个文件因远端不存在而保留（保留期 7 天）
```

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

`brs.brs_bak_rdb_table_full`：`idn`、`rdb_type`、`schema_name`、`table_name`、
`flag_enable`（`'1'/'0'`）、`priority`（**必填且唯一**）、`bak_scope`（both/schema/data）、
`bak_method`（pg_dump）、`compression`（none/gzip/zstd）、`where_clause`、`remark`、
`creator`（uid）、`gmt_create`、`gmt_update`。

### 2.3 备份目录

```env
FINV_BRS_BACKUP_DIR=/data/App/FinvQuant/BRS
```

目录由服务自动创建（`0750`）。

## 3. 备份产物

```
<FINV_BRS_BACKUP_DIR>/
├── Full/                     小表全量（与大表增量的 Incr/ 对称）
│   └── 20261029/
│       ├── finv.sys.sys_user.dump
│       ├── finv.sys.sys_config.dump
│       ├── finv.cdeu.finv_quote_cdeu_workflow.dump
│       └── ...
└── Incr/                     大表增量分片
    └── finv.cdeu.finv_quote_secu_kline_min/
        └── 20260930.mvsv.gz
```

> 10915 之前的本地产物在 `<FINV_BRS_BACKUP_DIR>/<yyyyMMdd>/`（无 `Full/` 段），清理逻辑仍会处理；如需手工归位可 `mkdir -p Full && mv <yyyyMMdd> Full/`。

- `-Fc` 自定义格式（`pg_restore` 可选择性恢复）；
- 压缩在文件内部（`--compress=zstd:3`），无需再 `gzip`；
- 文件名**不带时间戳**：同一张表当天重复备份会覆盖上一次（以最后一次为准）。

## 4. 查看与恢复

```bash
grep '\[brs\]' /data/App/FinvQuant/Log/FinvQuant.log | tail -20
journalctl -u finvquant | grep '\[brs\]' | tail -20

pg_restore -l /data/App/FinvQuant/BRS/20261029/finv.sys.sys_config.dump

pg_restore -h 127.0.0.1 -U finv -d finv \
  --table=sys.sys_config --clean --if-exists \
  /data/App/FinvQuant/BRS/20261029/finv.sys.sys_config.dump
```

## 5. 预置登记（种子，共 24 张）

| 来源 | 表 | priority |
|------|------|----------|
| `sys`（`V300060`） | `sys_user`/`sys_config`/`sys_dict`/`sys_dict_sub` | 1000/990/980/970 |
| `cdeu`（`V300060`） | `finv_quote_cdeu_workflow`/`phase`/`process`/`job`/`job_step`/`usc`/`trade_calendar`/`daily_budget` | 900～830 |
| `futu`（`V300083`） | `finv_futu_mapping_cs_market`/`mapping_exchange`/`mapping_market_code`/`mapping_region`/`mapping_security`、`finv_futu_quote_collect` | 800～750 |
| `public`（`V300083`） | `finv_currency`/`finv_exchange`/`finv_market`/`finv_region`/`finv_security`、`schema_version` | 740～690 |

**未包含** `cdeu.finv_quote_secu_kline_min`（大表后续单独处理）。

> `public.schema_version` 是迁移元数据，按「public schema 全部登记」一并纳入；
> 恢复时请用 `pg_restore` 选择表，**勿整库覆盖**（否则会用备份里的版本记录覆盖目标库实际状态）。
> **列名变更提示**：`paocd` 已于需求 10902 改名为 `vwap`（迁移 `V300087`）。恢复 2026-11 之前、
> 含 `quote` / `cdeu` K 线表的备份后，表结构会带回旧列名 `paocd`，需手工补一次改名：
> `ALTER TABLE <表> RENAME COLUMN paocd TO vwap;`（三张表：`quote.finv_quote_secu_kline_min`、
> `cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_secu_kline_min_export`）。
> 原 `public.schema_version_plan_shift`（号段平移 marker）已随需求 10895 删除，不再备份。

## 6. 对象存储上传（已实现）

- 抽象层：`internal/meta/base/oss`（`Store` 接口 + `Config` + 注册式 `New`），业务只依赖接口；
- 适配器：`oss/local`（单测/离线）、`oss/s3`（`aws-sdk-go-v2/service/s3`）；
- **不使用 `minio-go` 及任何 MinIO 官方库**；切换客户端只替换适配器；
- 配置 `sys.sys_config 39002`（JSON）：

```json
{
  "enable": false,
  "protocol": "s3",
  "provider": "cloudflare-r2",
  "endpoint": "https://<account>.r2.cloudflarestorage.com",
  "bucket": "finvquant-backup",
  "region": "auto",
  "ak": "...",
  "sk": "...",
  "prefix": "brs/"
}
```

> 字段语义：`enable` 启用上传；`protocol` 是协议（`s3` / `local`）；`provider` 是厂商标签（`cloudflare-r2` / `aws-s3` / 自定义），S3 兼容厂商统一走 `s3` 适配器。
> 兼容：旧键 `enabled`、误拼 `protocal`、以及只有协议的 `"provider":"s3"` 都能读；**保存时一律写上面这套新键**（详见 [`../API/BRS/OSSConfig.md`](../API/BRS/OSSConfig.md) §0）。

| 项 | 说明 |
|---|---|
| 全局开关 | 39002 `enable`（旧键 `enabled` 兼容读取；默认 `false`） |
| 表级开关 | `brs_bak_rdb_table_full.flag_upload`（`'1'` 上传 / `'0'` 只留本地，默认 `'1'`） |
| 上传 key | `<prefix>/<env>/Full/<yyyyMMdd>/<db>.<schema>.<table>.dump`（`Full/` 与大表增量的 `Incr/` 对称；`env` = `sys_config` 1002 的运行环境标识；缺失或非法则跳过上传） |
| 凭证 | `access_key`/`secret_key` 存 `sys_config`（`access_level` 限制），不写日志 |
| 失败 | 保留本地文件并记日志，不影响下一张表 |
| 下载 | `aws s3 cp` / `rclone` 等任意 S3 兼容工具 |

### 6.1 配置步骤（Cloudflare R2 示例）

1. 在 Cloudflare Dashboard → R2 创建桶（如 `finvquant-backup`）；
2. 进入 **Manage R2 API Tokens** → **Create API Token**，权限选 **Object Read & Write**；
3. 记录 **Access Key ID** 与 **Secret Access Key**（Secret 只显示一次）；
4. 更新 39002：

```sql
UPDATE sys.sys_config
   SET json_value = '{
     "enable": true,
     "protocol": "s3",
     "provider": "cloudflare-r2",
     "endpoint": "https://<account_id>.r2.cloudflarestorage.com",
     "bucket": "finvquant-backup",
     "region": "auto",
     "ak": "<ACCESS_KEY>",
     "sk": "<SECRET_KEY>",
     "prefix": "brs/"
   }'::json,
       gmt_modify = now()
 WHERE config_no = 39002;
```

5. 下一轮备份自动上传；可用 `aws s3 ls s3://<bucket>/brs/ --endpoint-url <endpoint>` 或 `rclone` 验证。

> 39002 的 `remark` 已写入完整字段说明与 R2/AWS/MinIO 配置差异，可直接查询：
> `select remark from sys.sys_config where config_no = 39002;`

**注意**：39002 修改后下一轮备份生效；上传失败不影响本地备份结果。

## 7. 手动验证

临时把 39001 改为每秒触发（测试后务必改回）：

```sql
UPDATE sys.sys_config SET str_value = '* * * * * *', gmt_modify = now() WHERE config_no = 39001;
-- 观察日志与 /data/App/FinvQuant/BRS/<今天>/ 下的 dump
UPDATE sys.sys_config SET str_value = '0 0 3 * * *', gmt_modify = now() WHERE config_no = 39001;
```

## 8. 注意事项

- `pg_dump` 需在服务进程 `PATH` 中（随 PostgreSQL 客户端安装）；
- 密码通过子进程 `PGPASSWORD` 传递，不落命令行；
- 单表备份超时 30 分钟；失败继续下一张表，最后汇总；
- `where_clause` 首期仅登记、不生效（`pg_dump` 不支持行过滤）；
- 大表建议后续单独方案，不要直接登记。
