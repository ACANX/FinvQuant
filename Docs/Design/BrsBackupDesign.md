# BRS 表备份与对象存储上传设计

**文档版本**：v1.0.0
**状态**：生效
**生效日期**：2026-10-29
**最后更新**：2026-10-29
**负责人**：@ACANX
**适用版本**：FinvQuant 0.1.4

---

## 变更日志

| 版本 | 日期 | 变更内容 | 变更人 |
|---|---|---|---|
| v1.0.0 | 2026-10-29 | 初始版本：BRS 登记驱动备份、6 段 cron 调度、pg_dump 表级导出、S3 兼容对象存储上传（oss 抽象与适配器）、配置与运维约束 | @ACANX |

---

## 0. 定位与读者

本文描述 **BRS（Backup & Restore Service）** 的设计与实现：按登记表对指定的库表做每日 `pg_dump` 表级备份，并把备份文件上传到 S3 协议兼容的对象存储（AWS S3 / Cloudflare R2 / MinIO / B2 / OSS 等）。

读者：

- 需要判断「某张表为什么没备份 / 备份文件为什么没上传」的运维；
- 需要新增/调整备份表、改 cron、换对象存储的运维与开发；
- 需要扩展对象存储适配器（新增厂商）或改造备份方式的开发。

配套文档：

- 使用手册（配置步骤、恢复命令、R2 示例）：[Brs.md](../FinvQuant/Brs.md)
- 设计前身（方案与取舍）：[2610-011-BrsBackup.md](../Refactor/2610-011-BrsBackup.md)
- schema 隔离与改库红线：[DbSchemaSpec.md](../DevSpec/DbSchemaSpec.md)
- 相邻的归档链路（行情文件上传，与本文不是同一套）：[CdeuWorkflowDesign.md](CdeuWorkflowDesign.md)

## 1. 目标与边界

| 目标 | 说明 |
|---|---|
| 登记驱动 | 备份哪些表由 `brs.brs_bak_rdb_table` 决定，改配置即可增减，不改代码、不重启 |
| 优先级可控 | `priority` 越大越先备份，保证核心表（用户、配置、字典）先落地 |
| 在线可备 | 用 `pg_dump` 一致性快照导出，不需要停服；单表 30 分钟超时 |
| 失败隔离 | 单表失败只记日志并继续下一张，一轮结束后汇总失败数 |
| 远端留存 | 备份完成后按表级开关上传 S3 兼容对象存储；上传失败**保留本地文件** |
| 零侵入扩展 | 业务只依赖 `oss.Store` 接口，换厂商只替换适配器 |

**不在范围内**：

- **恢复流程自动化**：本文只负责「产出并留存」，恢复用 `pg_restore` 手工执行（见 `Brs.md` §4）。
- **增量/差异备份**：每轮都是整表 `pg_dump`，不做 WAL 归档，也不做 PITR。
- **备份文件生命周期管理**：本地目录与远端桶的保留期、清理策略不在代码内，由运维侧策略或桶生命周期规则处理。
- **备份结果落库**：当前只写服务日志，不写「备份历史表」（见 §9）。
- **跨数据库类型**：`rdb_type` 当前只有 `postgresql`；`bak_method` 约束里保留了 `copy`，但实现只有 `pg_dump`。

## 2. 术语

| 术语 | 含义 |
|---|---|
| BRS | Backup & Restore Service，本备份子系统的简称 |
| 登记表 | `brs.brs_bak_rdb_table`，一行 = 一张要备份的表 |
| `bak_scope` | 备份范围：`both`（结构+数据）/ `schema`（`--schema-only`）/ `data`（`--data-only`） |
| `compression` | 压缩方式：`none` / `gzip` / `zstd`，映射到 `pg_dump --compress` |
| `flag_upload` | 表级上传开关：`1` 上传远端，`0` 只留本地 |
| 39001 | `sys.sys_config` 中的每日备份 cron（6 段：秒 分 时 日 月 周） |
| 39002 | `sys.sys_config` 中的对象存储配置（JSON） |
| path-style | S3 兼容存储的寻址方式（`endpoint/bucket/key`），R2 / MinIO 必需 |
| 适配器 | `oss/local`、`oss/s3` 等实现，向 `oss.Store` 注册 |

## 3. 总体架构

```
sys.sys_config 39001（6 段 cron）
        │  每秒 tick，表达式 60 s 缓存
        ▼
brs.Start(ctx, pool, opts)  ── 命中且无进行中轮次 → go RunOnce(...)
        │
        ▼
RunOnce
  ├─ LoadTables：brs.brs_bak_rdb_table（flag_enable='1'，priority DESC, idn ASC）
  ├─ 建本地目录：<BackupDir>/<yyyyMMdd>/（0750）
  ├─ 解析 39002 → oss.New(cfg) 得到 Store（未启用/失败 → 本轮不上传）
  └─ 逐表（priority 倒序）
        ├─ pg_dump --table <schema>.<table> -Fc --compress <…> --file <…>
        │     └─ 失败：记日志，继续下一张
        └─ flag_upload='1' 且 Store 可用
              └─ Put(key = <yyyyMMdd>/<basename>)  → 加 prefix 后为 <prefix>/<yyyyMMdd>/<basename>
                    └─ 失败：记日志，本地文件保留
```

代码分层：

| 层 | 位置 | 职责 |
|---|---|---|
| 调度与执行 | `internal/meta/finv/brs/brs.go` | cron 读取与命中判断、轮次互斥、逐表 dump、上传编排 |
| cron 解析 | `internal/meta/base/cron` | 6 段表达式的解析与 `Matches(now)`（仅标准库实现） |
| 存储抽象 | `internal/meta/base/oss/oss.go` | `Store` 接口（`Put` / `Get` / `Stat` / `Delete`）、`Config`、注册式工厂 |
| 适配器 | `internal/meta/base/oss/s3`、`oss/local` | S3 兼容实现（`aws-sdk-go-v2`）、本地目录实现（单测/离线） |
| 接入点 | `cmd/server/main.go` | 启动时 `go brs.Start(...)`，连接参数来自 `FINV_PG_*` 与 `FINV_BRS_BACKUP_DIR` |
| 配置读取 | `internal/meta/finv/sys` | `sys_config` 39001/39002 带 TTL 缓存读取 |

## 4. 调度设计

| 项 | 实现 | 为什么 |
|---|---|---|
| 触发方式 | 每秒 `ticker` 检查 cron 是否命中 | 6 段 cron 支持到「秒」，不能用 1 分钟粒度 |
| 表达式来源 | `sys.sys_config` 39001（`str_value`） | 运维改配置即生效，不用重启进程 |
| 表达式缓存 | 最多 **60 秒**重读一次 | 兼顾「改了能快速生效」与「不每秒打库」 |
| 表达式非法 | 只记日志并沿用上一个有效表达式 | 配置写错不应该让备份彻底停摆 |
| 并发控制 | `running` 标志互斥 | 上一轮没跑完时不再叠加新一轮 |
| 执行方式 | 命中后 `go RunOnce(...)` | 备份是长任务，不能阻塞 ticker |
| 停止 | `ctx.Done()` 退出循环 | 与进程生命周期一致 |

**默认值**：迁移种子把 39001 设为 `0 0 3 * * *`（每日 03:00）；`Start` 的兜底备份目录为 `/data/App/FinvQuant/Backup/BRS`，实际以 `FINV_BRS_BACKUP_DIR` 为准。

## 5. 单表备份

| 项 | 内容 |
|---|---|
| 输入 | 登记行（schema、表名、范围、压缩）、连接参数（host/port/user/password/db） |
| 处理 | 拼装 `pg_dump` 参数 → `exec.CommandContext`（30 分钟超时）→ 密码经 `PGPASSWORD` 环境变量传递 |
| 输出 | `<BackupDir>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump`（`-Fc` 自定义格式，可用 `pg_restore` 选择性恢复） |
| 失败 | 记录 `pg_dump` 输出（截断 500 字符）后继续下一张表；本轮结束汇总 `N/M 张表备份失败` |

`pg_dump` 参数映射：

| 登记字段 | 取值 | 落到参数 |
|---|---|---|
| `bak_scope` | `both` / `schema` / `data` | 无 / `--schema-only` / `--data-only` |
| `compression` | `none` / `gzip` / `zstd`（空值） | `--compress none` / `gzip:6` / `zstd:3` |

**为什么用 `-Fc`（custom 格式）**：支持 `pg_restore` 的按表/按对象选择性恢复，也能与 `--compress` 协同；纯 SQL 文本虽然可读，但体积大且无法选择性恢复。

**文件名含时间戳**：同一张表一天内多次备份不会互相覆盖（`<schema>.<table>_<HHmmss>.dump`）。

## 6. 对象存储上传

### 6.1 抽象与适配器

```go
type Store interface {
	Put(ctx, key string, body io.Reader, size int64, contentType string) error
	Get(ctx, key string) (io.ReadCloser, error)
	Stat(ctx, key string) (ObjectInfo, error)
	Delete(ctx, key string) error
}
```

| 适配器 | 注册名 | 用途 | 关键实现 |
|---|---|---|---|
| `oss/s3` | `s3` | AWS S3 / R2 / MinIO / B2 / OSS 等 S3 兼容 | `aws-sdk-go-v2/service/s3`；`BaseEndpoint` 可配；**强制 `UsePathStyle=true`**；`access_key`/`secret_key` 非空时用静态凭证，否则走 SDK 默认凭证链 |
| `oss/local` | `local` | 单测与离线环境 | 落本地目录，`Endpoint` 作根目录 |

**为什么强制 path-style**：R2、MinIO 等兼容存储普遍不支持虚拟主机式域名（`bucket.endpoint`），开启 path-style 后统一为 `endpoint/bucket/key`，一套配置适配多数厂商。

**为什么不用 MinIO 官方库**：只需要 S3 协议，`aws-sdk-go-v2` 覆盖面更广、维护更稳；换成任何客户端都只改适配器，不动业务代码。

### 6.2 配置（39002）

```json
{
  "enabled": false,
  "provider": "s3",
  "endpoint": "https://<account>.r2.cloudflarestorage.com",
  "bucket": "finvquant-backup",
  "region": "auto",
  "access_key": "...",
  "secret_key": "...",
  "prefix": "brs/"
}
```

| 字段 | 默认 | 说明 |
|---|---|---|
| `enabled` | `false` | 总开关；`false` 时本轮不上传但照常本地备份 |
| `provider` | `s3` | 适配器名；空值回落 `s3` |
| `endpoint` | 空 | 空则用 AWS 默认地址；R2/MinIO 必须填 |
| `bucket` | 空 | 必填，否则适配器构造失败 |
| `region` | `auto` | 空值回落 `auto`（R2 语义） |
| `access_key` / `secret_key` | 空 | 非空则静态凭证；都为空则走 SDK 默认链（环境变量/实例角色） |
| `prefix` | `brs/` | 对象 key 前缀，首尾斜杠会被规整 |

### 6.3 上传语义

| 项 | 行为 |
|---|---|
| 上传条件 | 39002 `enabled=true` 且能构造出 `Store`；且该表 `flag_upload='1'` |
| 对象 key | `<prefix>/<yyyyMMdd>/<schema>.<table>_<HHmmss>.dump` |
| Content-Type | `application/octet-stream` |
| 大小 | 取本地文件 `Stat` 的字节数，作为 `ContentLength` 传给 SDK |
| 失败处理 | 记日志「上传失败 …（本地文件保留）」，**不影响其它表**，也不影响本轮结论 |
| 配置缺失/非法 | 记日志「对象存储配置无效，跳过上传」，本地备份照常完成 |
| 幂等 | `PutObject` 覆盖同名 key；同名文件只可能在同一秒重复备份时出现 |

## 7. 数据契约

### 7.1 登记表 `brs.brs_bak_rdb_table`

| 字段 | 类型 | 约束/默认 | 说明 |
|---|---|---|---|
| `idn` | bigint identity | PK | 主键 |
| `rdb_type` | varchar(32) | 默认 `postgresql` | 数据库类型 |
| `schema_name` / `table_name` | varchar | 与 `rdb_type` 组合唯一 | 目标表 |
| `flag_enable` | char(1) | 默认 `1`，`IN ('0','1')` | 备份开关 |
| `priority` | integer | **唯一**、`>= 0` | 越大越先备份 |
| `bak_scope` | varchar(16) | 默认 `both`，`IN ('both','schema','data')` | 备份范围 |
| `bak_method` | varchar(16) | 默认 `pg_dump`，`IN ('pg_dump','copy')` | 首期只实现 `pg_dump` |
| `compression` | varchar(16) | 默认 `zstd`，`IN ('none','gzip','zstd')` | 压缩方式 |
| `where_clause` | text | 可空 | **预留**，当前未生效（见 §9） |
| `flag_upload` | char(1) | 默认 `1`，`IN ('0','1')` | 表级上传开关（V200061 新增） |
| `remark` / `creator` / `gmt_create` / `gmt_update` | — | — | 备注、创建人、时间戳 |

### 7.2 预置登记（12 行）

| priority | 表 | 说明 |
|---|---|---|
| 1000 | `sys.sys_user` | 用户 |
| 990 | `sys.sys_config` | 系统配置（含 39001/39002 本身） |
| 980 / 970 | `sys.sys_dict`、`sys.sys_dict_sub` | 字典主表/子表 |
| 900–860 | `cdeu.finv_quote_cdeu_workflow`、`_phase`、`_process`、`_job`、`_job_step` | CDEU 工作流定义 |
| 850 / 840 / 830 | `cdeu.finv_quote_cdeu_usc`、`_trade_calendar`、`_daily_budget` | 证券字典、交易日历、导出预算 |

**刻意不含** `cdeu.finv_quote_secu_kline_min`：该表体量大且已有「导出 → 上传」的专用归档链路（见 `CdeuWorkflowDesign.md`），用 `pg_dump` 备份既慢又重复。

### 7.3 配置项

| 编号 | 名称 | 默认 | 说明 |
|---|---|---|---|
| 39001 | 每日备份 cron | `0 0 3 * * *` | 6 段：秒 分 时 日 月 周 |
| 39002 | 对象存储配置 | `enabled=false` | JSON，字段见 §6.2 |

## 8. 运维

### 8.1 常见操作

| 操作 | 做法 |
|---|---|
| 改备份时间 | 更新 `sys.sys_config` 39001 的 `str_value`，**最多 60 秒**生效 |
| 新增备份表 | 向 `brs_bak_rdb_table` 插入一行（`priority` 必须唯一），下一轮自动纳入 |
| 临时停某表 | 把 `flag_enable` 置 `'0'`（保留登记） |
| 只留本地不上传 | 该表 `flag_upload='0'`，或 39002 `enabled=false` |
| 立即跑一轮 | 不改 cron，直接调用 `brs.RunOnce`（当前无 CLI 入口，见 §9） |
| 校验备份文件 | `pg_restore --list <file>` 查看目录；`pg_restore -d <db> --table=<t> <file>` 恢复 |

### 8.2 常见问题

| 现象 | 原因 | 处置 |
|---|---|---|
| 日志「读取 39001 cron 失败」 | `sys_config` 无该行或 `str_value` 为空 | `select * from sys.sys_config where config_no=39001` 检查 |
| 日志「39001 cron 非法」 | 段数不对（必须是 6 段）或取值范围错 | 用 `0 0 3 * * *` 格式；非法时沿用上一个有效表达式 |
| 备份失败：`pg_dump: command not found` | 镜像/宿主机没有 `pg_dump` | 安装 `postgresql-client`（版本需 ≥ 服务端） |
| 备份失败：`server version mismatch` | `pg_dump` 版本低于服务端 | 使用与服务端一致的 `pg_dump` |
| 日志「对象存储配置无效，跳过上传」 | 39002 缺 `bucket` / JSON 解析失败 / provider 未注册 | 检查 JSON 与 `bucket`；确认 `provider` 为 `s3` 或 `local` |
| 上传报 S3 域名/签名错 | `endpoint` 或 `region` 不对，或存储不支持虚拟主机寻址 | 填完整 `endpoint`（含 `https://`）；R2 用 `region=auto`（适配器已强制 path-style） |
| 上传 403 | 凭证无写权限或桶名错 | 重新签发具备 `Object Read & Write` 的令牌，核对 `bucket` |
| 某表始终不备份 | `flag_enable='0'` 或 `priority` 冲突导致插入失败 | 查登记行与唯一约束 |

## 9. 实现现状与已知限制

| 项 | 状态 |
|---|---|
| 登记驱动 + priority 倒序 + 单表失败继续 | ✅ |
| 6 段 cron（39001）+ 60 s 配置缓存 + 轮次互斥 | ✅ |
| `pg_dump -Fc` + `--schema-only` / `--data-only` + 压缩映射 | ✅ |
| S3 兼容上传（`oss` 抽象 + `oss/s3` 适配器 + `oss/local` 骨架） | ✅ |
| 表级上传开关 `flag_upload` + 全局开关 39002 `enabled` | ✅ |
| 上传失败保留本地、不影响其它表 | ✅ |
| `where_clause` 部分数据备份 | ⛔ 仅预留字段：代码检测到非空时**打印警告并按整表备份** |
| `bak_method='copy'` | ⛔ 约束允许但未实现，执行固定走 `pg_dump` |
| 备份结果落库（备份历史表 / 成功失败可查） | ⛔ 只有服务日志 |
| 失败重试与断点续传 | ⛔ 单轮内失败即跳过，等下一轮 |
| 备份文件保留期与自动清理（本地/远端） | ⛔ 无内建策略，依赖运维或桶生命周期规则 |
| 手动触发一轮的 CLI/接口入口 | ⛔ 无；需等待 cron 命中 |
| 恢复自动化 | ⛔ 手工 `pg_restore` |

## 10. 相关文档

- [Brs.md](../FinvQuant/Brs.md) — 使用手册：配置步骤、R2 令牌申请、恢复命令、手动验证
- [2610-011-BrsBackup.md](../Refactor/2610-011-BrsBackup.md) — 方案与取舍（为何不用 `pg_dumpall`、为何表级登记）
- [DbSchemaSpec.md](../DevSpec/DbSchemaSpec.md) — schema 隔离红线（`brs` schema 的归属）
- [CdeuWorkflowDesign.md](CdeuWorkflowDesign.md) — 行情表的归档链路（与本文互补：大表走导出上传，配置/字典表走备份）
- [QmioDesign.md](QmioDesign.md) — 行情数据整表搬运工具

## 11. 源码索引

| 文件 | 职责 |
|---|---|
| `internal/meta/finv/brs/brs.go` | 调度主循环、`RunOnce`、`pg_dump` 执行、上传编排、39002 解析 |
| `internal/meta/base/cron/cron.go` | 6 段 cron 解析与 `Matches` |
| `internal/meta/base/oss/oss.go` | `Store` 接口、`Config`、注册式工厂 `Register` / `New` |
| `internal/meta/base/oss/s3/s3.go` | S3 兼容适配器（`aws-sdk-go-v2`、path-style、静态凭证） |
| `internal/meta/base/oss/local/local.go` | 本地目录适配器（单测/离线） |
| `internal/meta/finv/sys/syscfg.go` | `sys_config` 读取（39001/39002 等） |
| `cmd/server/main.go` | 启动时 `go brs.Start(...)`，注入连接参数与备份目录 |
| `Deploy/Migrations/V200060__brs_backup_table.sql` | `brs` schema、登记表、39001 与 12 行种子 |
| `Deploy/Migrations/V200061__brs_upload.sql` | `flag_upload` 列、39002 默认 JSON |
