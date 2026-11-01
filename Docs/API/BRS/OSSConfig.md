# GET/POST /API/V1/Meta/BRS/Backup/Finv/OSSConfig/* — 对象存储配置

BRS 备份产物上传的 S3 兼容对象存储配置，对应系统配置 `sys.sys_config` 39002 的 `json_value`（`data_type='4'`，JSON）。

> 生效时机：每轮备份前读取 39002，**保存后下一轮备份生效，无需重启服务**。
>
> 存储键名：`sys.sys_config` 39002 的 JSON 内凭证键为 **`ak` / `sk`**（迁移 `V300082` 由 `access_key`/`secret_key` 改名），
> 而**本接口的 wire 字段仍是 `access_key` / `secret_key`**（由服务层映射），前端展示与调用方式不变。历史旧键由服务层兜底读取。
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
    "enabled": false,
    "provider": "s3",
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
| `enabled` | bool | `true` 启用上传；`false` 仅本地备份（默认） |
| `provider` | string | 固定 `s3`（经 `internal/meta/base/oss` 抽象） |
| `endpoint` | string | 对象存储地址；AWS S3 留空走默认域名 |
| `bucket` | string | 桶名（需提前创建） |
| `region` | string | R2 用 `auto`；AWS 填实际区域；MinIO 常用 `us-east-1` |
| `access_key` | string | 访问凭证 Access Key ID |
| `secret_key` | string | **恒为空**（不回传明文） |
| `prefix` | string | 对象 key 前缀，最终 key 为 `<prefix>/<yyyyMMdd>/<schema>.<table>.dump`（文件名不带时间戳） |
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
| `enabled` | bool | ✅ | 是否启用上传 |
| `provider` | string | 可选 | 空则 `s3`；首期仅支持 `s3` |
| `endpoint` | string | 条件 | `enabled=true` 时必填 |
| `bucket` | string | 条件 | `enabled=true` 时必填 |
| `region` | string | 可选 | 空则 `auto` |
| `access_key` | string | 条件 | `enabled=true` 时必填 |
| `secret_key` | string | 可选 | **留空表示不修改**；`enabled=true` 且库内无现值时必填 |
| `prefix` | string | 可选 | 空则 `brs/`；结尾自动补 `/` |

### 响应（成功 HTTP 200）

返回保存后的配置视图（结构同 Get，`secret_key` 仍为空）。

```json
{ "code": 0, "message": "保存成功", "data": { "enabled": true, "provider": "s3", "secret_key": "", "secret_key_set": true, "prefix": "brs/" } }
```

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | `provider` 非 `s3`；`enabled=true` 而 `endpoint`/`bucket`/`access_key`/`secret_key` 缺失（且库内无旧密钥）；39002 配置行不存在（提示执行迁移 `V300061`） |
| 500 | 2006 | 数据库读写失败 |

## 3. 数据来源与关联

| 项 | 说明 |
|------|------|
| 配置行 | `sys.sys_config` 39002（`V300061` 初始化，`data_type='4'`，JSON；`V300082` 把凭证键改为 `ak`/`sk`） |
| 服务读取 | `internal/meta/finv/brs/ossconfig.go`（`LoadOSSConfig` / `SaveOSSConfig`，负责 `access_key`↔`ak`、`secret_key`↔`sk` 映射）、`brs.go` 的 `loadOssStore`（每轮备份构造客户端，读 `ak`/`sk`，兼容旧键） |
| 上传实现 | `internal/meta/base/oss`（默认适配器 s3，基于 aws-sdk-go-v2，不使用 minio-go） |
| 表级开关 | `brs.brs_bak_rdb_table.flag_upload`（`'1'` 上传 / `'0'` 只留本地），见 [BackupTable.md](BackupTable.md) |
| 每日调度 | `sys.sys_config` 39001（6 段 cron，默认每天 03:00:00） |
