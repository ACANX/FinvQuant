# GET / POST /API/V1/Meta/Finv/Quote/Crypto/Moomoo/Config — moomoo OpenAPI 凭据配置

> 所属：行情 → 实时行情 → 行情快照同步 →「加密货币」Tab（配置入口，需求 11318）
> 存储：`sys.sys_config` 20016（AppKey）/ 20017（Ed25519 私钥 PEM），迁移 `V300227` 发放空串占位
> 生效：收藏加密货币快照采集（`.../Crypto/Fav/Snapshot/Sync` 与 `crypto` Tab）优先使用本配置，
> 未配置时回退环境变量 `FINV_MOOMOO_APP_KEY` / `FINV_MOOMOO_PRIVATE_KEY`

## 1. GET 查询配置

### 1.1 请求

| 项 | 值 |
|---|---|
| 方法 | `GET` |
| 路径 | `/API/V1/Meta/Finv/Quote/Crypto/Moomoo/Config` |
| 认证 | `Authorization: Bearer <token>` |

### 1.2 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "app_key": "0f0ba46cce34ca7f884e4e0fcdc309d5",
    "private_key_configured": true,
    "private_key_masked": "****",
    "source": "sys_config"
  }
}
```

| 字段 | 说明 |
|---|---|
| `app_key` | 20016 的当前值（可为空） |
| `private_key_configured` | 20017 是否已配置（非空） |
| `private_key_masked` | 私钥掩码（已配置固定 `****`，**不下发明文**） |
| `source` | 当前生效来源：`sys_config` / `env`（环境变量）/ `none` |

## 2. POST 保存配置

### 2.1 请求体

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `app_key` | string | 是 | AppKey ID；空串 → 400 |
| `private_key` | string | 否 | Ed25519 PKCS#8 私钥 PEM；**留空 = 保留原值**（页面不回显明文）；`__CLEAR__` = 清空私钥 |

```json
{ "app_key": "0f0ba46cce34ca7f884e4e0fcdc309d5", "private_key": "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----" }
```

- 私钥写入前会做 `Ed25519 PKCS#8` 解析校验，非法 PEM → 400，不写库；
- 保存成功后采集链路无需重启（每次采集时读取）。

### 2.2 响应

```json
{ "code": 0, "message": "已保存" }
```

## 3. 安全约定

- 私钥存 `sys_config.str_value`（`access_level=3`，敏感），接口与页面**只回显是否已配置**；
- 日志只打印 AppKey 前 6 位（`app_key=0f0ba4***`），不打印私钥；
- 私钥明文丢失只能重新生成并重新配置（控制台可重置 AppKey 私钥）。
