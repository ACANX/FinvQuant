# GET/POST /API/V1/Meta/BRS/Backup/Finv/Table/* — 备份登记维护

BRS（Backup & Restore Service）备份范围登记表 `brs.brs_bak_rdb_table_full` 的维护接口：分页查询、新增/修改、启用/禁用。

> 登记直接决定每日备份（`sys.sys_config` 39001 的 6 段 cron）执行哪些表：只备份 `flag_enable='1'` 的行，按 `priority` **倒序**（越大越先）。修改后下一轮备份即生效，无需重启服务。
>
> 前端页面：`/Meta/BRS/Backup/Finv/Table`（`Web/src/views/Meta/BRS/Backup/Finv/BrsBackupTableView.vue`）。

## 1. 分页查询备份登记

- **方法**：`GET`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Table/List`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | 可选 | 关键字，匹配 `schema_name` / `table_name` / `remark`（不区分大小写，任一包含即命中） |
| `flagEnable` | string | 可选 | 按启用状态过滤：`0`（停用）/ `1`（启用）；空表示全部 |
| `page` | int | 可选 | 页码（从 1 开始，默认 1） |
| `pageSize` | int | 可选 | 每页条数（默认 20，上限 500） |

> 排序：`priority` 倒序、`idn` 升序（与备份执行顺序一致）。

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 25,
    "page": 1,
    "pageSize": 20,
    "list": [
      {
        "idn": 1,
        "rdb_type": "postgresql",
        "db": "finv",
        "schema_name": "sys",
        "table_name": "sys_user",
        "flag_enable": "1",
        "priority": 1000,
        "bak_scope": "both",
        "bak_method": "pg_dump",
        "compression": "zstd",
        "flag_upload": "1",
        "where_clause": null,
        "remark": "用户与权限",
        "creator": "system",
        "gmt_create": "2026-10-20T03:12:44.123456+08:00",
        "gmt_update": "2026-10-20T03:12:44.123456+08:00"
      }
    ]
  }
}
```

## 2. 新增/修改备份登记

- **方法**：`POST`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Table/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `idn` | int | ✅ | 主键；`0` 表示新增，非 0 表示更新该行 |
| `rdb_type` | string | 可选 | 数据库类型，空则默认 `postgresql`（首期仅支持该值） |
| `db` | string | 可选 | 数据库名，空则默认 `finv`（登记归属库；**本次备份执行仍连当前库**，不按该字段分库） |
| `schema_name` | string | ✅ | schema 名（如 `sys` / `cdeu`） |
| `table_name` | string | ✅ | 表名（如 `sys_user`） |
| `flag_enable` | string | 可选 | `1`（启用，默认）/ `0`（停用） |
| `priority` | int | ✅ | 执行优先级，非负整数，**全局唯一**，越大越先 |
| `bak_scope` | string | 可选 | `both`（默认）/ `schema` / `data` |
| `bak_method` | string | 可选 | `pg_dump`（默认）/ `copy` |
| `compression` | string | 可选 | `zstd`（默认）/ `gzip` / `none` |
| `flag_upload` | string | 可选 | `1`（默认，上传对象存储）/ `0`（只留本地） |
| `where_clause` | string | 可选 | 预留：部分数据备份条件（首期 `pg_dump` 不生效） |
| `remark` | string | 可选 | 备注 |

> 唯一性：`(rdb_type, db, schema_name, table_name)` 唯一（同一数据库内的同一张表只能登记一次）；`priority` 全局唯一。

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "保存成功", "data": { "idn": 13 } }
```

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | 请求体解析失败；字段校验失败（`schema_name`/`table_name` 为空、`priority` 为负、枚举值越界） |
| 400 | 400 | 唯一键冲突：「该表已登记…」或「优先级 priority 已被其他登记占用…」 |
| 404 | 404 | `idn` 非 0 但该行不存在 |
| 500 | 2006 | 数据库写入失败 |

## 3. 启用/禁用备份登记

- **方法**：`POST`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Table/Toggle`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `idn` | int | ✅ | 备份登记主键 |
| `flag_enable` | string | ✅ | `1`（启用）/ `0`（停用） |

### 响应（成功 HTTP 200）

```json
{ "code": 0, "message": "切换成功", "data": { "idn": 1, "flag_enable": "0" } }
```

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | `idn` 非正、`flag_enable` 非 `0`/`1`、请求体解析失败 |
| 404 | 404 | 该 `idn` 不存在 |
| 500 | 2006 | 数据库写入失败 |

## 4. 数据来源与关联

| 项 | 说明 |
|------|------|
| 表 | `brs.brs_bak_rdb_table_full`（迁移 `V300060`；`flag_upload` 由 `V300061` 增加；`db` 由 `V200090` 增加，默认 `finv`） |
| 备份调度 | `sys.sys_config` 39001（6 段 cron，默认每天 03:00:00） |
| 对象存储 | `sys.sys_config` 39002（JSON，`enabled=false` 时不上传）；页面见 `/Meta/BRS/Backup/Finv/OSSConfig`（占位） |
| 服务实现 | `internal/meta/finv/brs`（`LoadTables` 读登记执行备份；`Registry` 提供本页维护能力） |
