# 行情采集系统配置接口

> 三个只读/单写接口，服务于「行情 → 行情采集 → 行情采集系统配置」页（需求 11343）。
> 实现：`internal/api/handler/quote_collect_sysconfig.go`、
> `internal/meta/finv/quote/collectcfg/`（`catalog.go` 目录 + `store.go` 读写校验）
> 关联菜单：[`Menu/Quote/RealtimeCollect/CollectSysConfig.md`](../../Menu/Quote/RealtimeCollect/CollectSysConfig.md)

## 1. 总览

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/API/V1/Meta/Finv/Quote/Collect/Config/List` | 目录内 5 项配置的当前值（敏感项回掩码） |
| POST | `/API/V1/Meta/Finv/Quote/Collect/Config/Save` | 保存单条（只改已存在的行） |
| GET | `/API/V1/Meta/Finv/Quote/Collect/Config/Secret` | 取敏感项明文（**base64** 编码，需登录） |

共同约定：

| 项 | 说明 |
|---|---|
| 认证 | 需登录（`Authorization: Bearer <token>`），未登录 401 |
| 范围 | 只有 20013 / 20014 / 20015 / 20016 / 20018；其它编号一律 `400` |
| 不新增 / 不删除 | 没有 Create / Delete 接口；配置项由迁移发放 |
| 日志 | 只记 `config_no` / 条目数 / `length`，**不记值内容** |

## 2. GET .../Config/List

无请求参数。

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "config_no": 20016,
        "name": "moomoo OpenAPI 凭据",
        "module": "moomoo 接入",
        "sensitive": true,
        "value_format": "{AppKey}|{Ed25519 PKCS#8 私钥 PEM}",
        "description": "加密货币 / 美股港股指数快照采集用……",
        "empty_hint": "空串 = 未配置，回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY",
        "value": "****|****",
        "masked": true,
        "has_value": true,
        "gmt_modify": "2026-11-20T02:32:09.626146Z"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `items[].config_no` | int | 配置编号 |
| `items[].name` | string | 配置项名称 |
| `items[].module` | string | 分组：`moomoo 接入` / `Supabase 证券参数` / `USC 映射规则` |
| `items[].sensitive` | bool | 是否敏感（列表只回掩码） |
| `items[].value_format` | string | 值格式说明 |
| `items[].description` | string | 用途与生效时机 |
| `items[].empty_hint` | string | 留空时的行为 |
| `items[].value` | string | 当前值；敏感项为掩码（见下） |
| `items[].masked` | bool | `true` = `value` 是掩码，原文需走 Secret 接口 |
| `items[].has_value` | bool | 库里是否已有非空值 |
| `items[].gmt_modify` | string\|null | 最近修改时间（RFC3339） |

**掩码口径**：

| 编号 | 掩码 | 说明 |
|---|---|---|
| 20013 | `{PROJECT_ID}\|****` | 项目 ID 不是秘密，保留便于确认连的是哪个项目 |
| 20016 | `****\|****` | AppKey 与私钥都是凭据，一律不露字符 |
| 其它 | 原值 | 非敏感项直接回原值 |

## 3. POST .../Config/Save

请求体：

```json
{ "config_no": 20015, "value": "0 30 2 * * *" }
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `config_no` | int | 必须在 20013 / 20014 / 20015 / 20016 / 20018 之内 |
| `value` | string | 新值；语义随配置项不同（见下） |

各配置项的值语义：

| 编号 | 值格式 | 空串 | 特殊值 |
|---|---|---|---|
| 20013 | `{PROJECT_ID}\|{APIKEY}` | 清空（未配置） | — |
| 20014 | `table` / `schema.table` | 清空（回退 `finv_quote_secu_key`） | — |
| 20015 | 6 段 cron | 清空（关闭自动同步） | — |
| 20016 | `{AppKey}\|{私钥 PEM}` | —（AppKey 不能为空） | 私钥段空 = **保留原私钥**；`__CLEAR__` = 只清私钥 |
| 20018 | `{enable}\|{base_url}` | 清空（直连） | 只填地址（无竖线）= 视为启用 |

响应：

```json
{ "code": 0, "message": "保存完成", "data": { "config_no": 20015 } }
```

错误：

| `code` | HTTP | 触发场景 |
|---|---|---|
| `400` | 400 | 编号不在范围 / 值格式非法 / 配置项不存在（缺种子迁移） |
| `2006` | 500 | 数据库写入失败 |
| `401` | 401 | 未登录 |

## 4. GET .../Config/Secret?configNo=20016

只允许 20013 / 20016（非敏感项走 `400`）。返回 **base64 编码**的原值：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "config_no": 20016,
    "value_base64": "MGYwYmE0NmNjZTM0Y2E3Zjg4NGU0ZTBmY2RjMzA5ZDU3TUM0Q0FRQXdCUVlESzJWd0...",
    "length": 97
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `config_no` | int | 配置编号 |
| `value_base64` | string | 原值的 base64（UTF-8 明文编码），前端本地解码 |
| `length` | int | 原值字符数（排障对照用） |

为什么是 base64 而不是明文：明文直接进响应体会被浏览器历史、代理日志、关键词扫描留痕；
base64 需要显式解码，且只有本接口能取到（列表接口永远只有掩码）。

## 5. 前端交互

| 操作 | 行为 |
|---|---|
| 进入页面 | `GET .../Config/List` |
| 修改 | 弹窗编辑 → `POST .../Config/Save` → 重新拉列表 |
| 查看明文 | `GET .../Config/Secret` → base64 解码 → 需再点「显示」才渲染 |
| 刷新 | 重新拉列表（旧入口改完用这个同步） |
