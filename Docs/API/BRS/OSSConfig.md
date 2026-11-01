# GET/POST /API/V1/Meta/BRS/Backup/Finv/OSSConfig/* — 对象存储配置

BRS 备份产物上传的 S3 兼容对象存储配置，对应系统配置 `sys.sys_config` 39002 的 `json_value`（`data_type='4'`，JSON）。

> 生效时机：每轮备份前读取 39002，**保存后下一轮备份生效，无需重启服务**。
>
> 上传路径还需要运行环境标识：`sys_config` **1002** 的 `str_value`（`dev`/`test`/`uat`/`prod` 等），
> 作为 key 的一级目录；未配置或不合法时**跳过对象存储上传**（本地备份保留），避免不同环境的备份混在一起。
> 1002 由一次性脚本 [`Deploy/Init/2610-002-AddRuntimeEnvConfig.sql`](../../../Deploy/Init/2610-002-AddRuntimeEnvConfig.sql) 发放。
>
> 存储键名：`sys.sys_config` 39002 的 JSON 内凭证键为 **`ak` / `sk`**（迁移 `V300082` 由 `access_key`/`secret_key` 改名），
> 而**本接口的 wire 字段仍是 `access_key` / `secret_key`**（由服务层映射），前端展示与调用方式不变。历史旧键由服务层兜底读取。

## 0. 配置格式与兼容（需求 10912）

39002 的 `json_value` 目标格式（**保存一律写这套键**）：

```json
{"enable":true,"protocol":"s3","provider":"cloudflare-r2","endpoint":"https://<account_id>.r2.cloudflarestorage.com","bucket":"finvquant","region":"APAC","ak":"…","sk":"…","prefix":"BRS/Finv/"}
```

| 键 | 语义 | 读取兼容 |
|------|------|----------|
| `enable` | 是否启用上传 | 旧键 `enabled`（**`enable` 优先**） |
| `protocol` | 传输协议（`s3`/`local`） | 误拼别名 `protocal`（仅读取，不再写出） |
| `provider` | 厂商标签（`cloudflare-r2`/`aws-s3`/…） | 旧格式 `"provider":"s3"` 表示「只有协议」：按 endpoint 推断厂商（含 `r2.cloudflarestorage.com` → `cloudflare-r2`，否则 `aws-s3`） |
| `endpoint`/`bucket`/`region`/`ak`/`sk`/`prefix` | 同前 | `access_key`/`secret_key` 旧键兜底（V300082 之前） |

兼容行为：

- **升级不改配置也能跑**：旧 JSON 读取时归一化，`GET /OSSConfig/Get` 返回新键视图；
- **一改就迁到新格式**：`POST /OSSConfig/Save` 只写 `enable`/`protocol`/`provider`，旧键（`enabled`/`protocal`/`access_key`/`secret_key`）不再写回，下次读取即新格式；
- **协议名不等同适配器**：`provider` 只是标签，驱动由 `protocol` 决定（`s3` → `internal/meta/base/oss/s3`，`local` → `.../local`）。
>
> 密钥安全：读取接口**永不返回 `secret_key` 明文**，只返回 `secret_key_set` 标记；保存时 `secret_key` 留空表示保持库内现值，只有显式传入非空值才覆盖。日志与错误信息均不带出密钥。
>
> 前端页面：`/Meta/BRS/Backup/Finv/OSSConfig`（`Web/src/views/Meta/BRS/Backup/Finv/BrsOssConfigView.vue`）。

## 1. 读取配置

- **方法**：`GET`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/OSSConfig/Get`

无查询参数。

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "enable": false,
    "protocol": "s3",
    "provider": "cloudflare-r2",
    "endpoint": "",
    "bucket": "",
    "region": "auto",
    "access_key": "",
    "secret_key": "",
    "prefix": "brs/",
    "secret_key_set": false,
    "remark": "BRS 备份上传的 S3 兼容对象存储配置（JSON）。\n字段说明：…",
    "gmt_modify": "2026-10-29T03:37:36.754789+08:00"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `enable` | bool | `true` 启用上传；`false` 仅本地备份（默认） |
| `protocol` | string | 传输协议：`s3`（S3 兼容，默认）/ `local`（本地目录适配器，开发离线用） |
| `provider` | string | 服务厂商标签：`cloudflare-r2` / `aws-s3` / 自定义（如 `minio`）/ `local`。**仅作标识**，S3 兼容厂商统一走 `s3` 适配器 |
| `endpoint` | string | 对象存储地址（scheme://host[:port]，不带路径/桶名）；AWS S3 留空走默认域名 |
| `bucket` | string | 桶名（需提前创建） |
| `region` | string | R2 用 `auto`；AWS 填实际区域；MinIO 常用 `us-east-1` |
| `access_key` | string | 访问凭证 Access Key ID |
| `secret_key` | string | **恒为空**（不回传明文） |
| `prefix` | string | 对象 key 前缀（用户指定，如 `BRS/Finv/`）；最终 key 为 `<prefix>/<env>/<yyyyMMdd>/<db>.<schema>.<table>.dump`，`env` 取 `sys_config` 1002 的运行环境标识 |
| `secret_key_set` | bool | 只读：库内是否已配置 secret_key |
| `remark` | string | 只读：39002 的字段说明（迁移 `V300061` 内置） |
| `gmt_modify` | string | 只读：最近修改时间 |

缺省归一化：`provider` 空 → `s3`，`region` 空 → `auto`，`prefix` 空 → `brs/`。

### 库内 JSON 形状（`sys.sys_config.json_value`，供运维直接查库）

```json
{"enabled":true,"provider":"s3","endpoint":"11111","bucket":"2222","region":"auto","ak":"1111","sk":"222222","prefix":"BRS/Finv/"}
```

## 2. 保存配置

- **方法**：`POST`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/OSSConfig/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `enable` | bool | ✅ | 是否启用上传 |
| `protocol` | string | 可选 | 空则 `s3`；只支持 `s3` / `local`，其它值 400 |
| `provider` | string | 可选 | 厂商标签。空或等于协议名（旧格式 `"provider":"s3"`）时按 endpoint 推断：含 `r2.cloudflarestorage.com` → `cloudflare-r2`，否则 `aws-s3`；其它值原样保存 |
| `endpoint` | string | 条件 | `enable=true` 时必填。`protocol=s3` 时**只填 `scheme://host[:port]`，不得带路径****只填 `scheme://host[:port]`，不得带路径**（如 R2 用 `https://<account_id>.r2.cloudflarestorage.com`）；桶名填 `bucket`，目录前缀填 `prefix`。带路径会 400 |
| `bucket` | string | 条件 | `enable=true` 时必填 |
| `region` | string | 可选 | 空则 `auto` |
| `access_key` | string | 条件 | `enable=true` 时必填 |
| `secret_key` | string | 可选 | **留空表示不修改**；`enable=true` 且库内无现值时必填 |
| `prefix` | string | 可选 | 空则 `brs/`；结尾自动补 `/` |

### 响应（成功 HTTP 200）

返回保存后的配置视图（结构同 Get，`secret_key` 仍为空）。

```json
{ "code": 0, "message": "保存成功", "data": { "enable": true, "protocol": "s3", "provider": "cloudflare-r2", "secret_key": "", "secret_key_set": true, "prefix": "BRS/Finv/" } }
```

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | `protocol` 非 `s3`/`local`；`endpoint` 无法解析或带路径（`protocol=s3` 时）；`enable=true` 而 `endpoint`/`bucket`/`access_key`/`secret_key` 缺失（且库内无旧密钥）；39002 配置行不存在（提示执行迁移 `V300061`） |
| 500 | 2006 | 数据库读写失败 |

## 3. 数据来源与关联

| 项 | 说明 |
|------|------|
| 配置行 | `sys.sys_config` 39002（`V300061` 初始化，`data_type='4'`，JSON；`V300082` 把凭证键改为 `ak`/`sk`） |
| 服务读取 | `internal/meta/finv/brs/ossconfig.go`（`LoadOSSConfig` / `SaveOSSConfig`，负责 `access_key`↔`ak`、`secret_key`↔`sk` 映射）、`brs.go` 的 `loadOssStore`（每轮备份构造客户端，读 `ak`/`sk`，兼容旧键） |
| 上传实现 | `internal/meta/base/oss`（默认适配器 s3，基于 aws-sdk-go-v2，不使用 minio-go） |
| 表级开关 | `brs.brs_bak_rdb_table_full.flag_upload`（`'1'` 上传 / `'0'` 只留本地），见 [BackupTable.md](BackupTable.md) |
| 每日调度 | `sys.sys_config` 39001（6 段 cron，默认每天 03:00:00） |
