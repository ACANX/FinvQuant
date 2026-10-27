# 登录认证 API

> 基路径：`/API/V1` · 处理器：`internal/api/handler/auth.go` · 服务：`internal/meta/finv/auth/`
> 需求：10761

## 认证模型

| 项 | 说明 |
|---|---|
| 凭据存储 | `sys.sys_user.password`（argon2id PHC 串）+ `password_salt`（hex 盐），**无明文** |
| 会话载体 | 随机令牌（32 字节 → hex 64 字符），存 Redis `finv:auth:session:<token>` |
| 传递方式 | 请求头 `Authorization: Bearer <token>` |
| 有效期 | `sys_config` 20012（分钟，默认 480）；命中且剩余不足一半时滑动续期 |
| 吊销 | 登出即时 `DEL` Redis 键，令牌立刻失效 |

## 公开端点（不校验令牌）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/Health/Live` | 存活探针（容器健康检查依赖，必须放行） |
| GET | `/Health/Ready` | 就绪探针 |
| GET | `/Version` | 版本信息 |
| POST | `/Auth/Login` | 登录 |

> 除上述路径外的**所有** `/API/V1/**` 接口都要求有效令牌，缺失或失效统一返回 `401`。

---

## POST /Auth/Login

校验用户名口令并签发会话。

### 请求

```json
{ "uid": "finv", "password": "********" }
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `uid` | string | ✅ | 用户名（去首尾空白） |
| `password` | string | ✅ | 口令原文 |

### 成功响应（200）

```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "token": "6f1c...（64 字符）",
    "expires_at": "2026-10-28T11:34:50.180288798Z",
    "ttl_seconds": 28799,
    "user": { "uid": "finv", "nick_name": "finv" }
  }
}
```

### 失败响应

| HTTP | code | message | 触发条件 |
|---|---|---|---|
| 400 | 400 | 请求体格式非法 | JSON 解析失败 |
| 401 | 401 | 用户名或密码错误 | 口令不匹配 **或** 账号不存在（两者文案相同，防账号枚举） |
| 403 | 403 | 账号已停用 | `flag_delete='1'` |
| 500 | 2006 | 登录失败: … | 数据库 / Redis 异常 |

> **防账号枚举**：用户不存在时服务端仍执行一次等价耗时的 argon2 计算，使两条失败路径响应时间接近。

---

## POST /Auth/Logout 🔒

吊销当前会话。

### 请求

无请求体（可传 `{}`）；需携带 `Authorization` 头。

### 成功响应（200）

```json
{ "code": 0, "message": "已退出登录" }
```

| HTTP | code | 说明 |
|---|---|---|
| 401 | 401 | 未登录或登录已过期 |
| 500 | 2006 | 退出登录失败: … |

---

## GET /Auth/Profile 🔒

返回当前登录用户资料。

### 成功响应（200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "uid": "finv",
    "nick_name": "finv",
    "email": "finv@finvquant.local",
    "authority": "0",
    "enable": true,
    "expires_at": "2026-10-28T11:34:50.180288798Z",
    "ttl_seconds": 28799
  }
}
```

> 响应**不包含** `password` 与 `password_salt`。

---

## 通用 401 响应

任何受保护接口在以下情形返回：

```json
{ "code": 401, "message": "未登录或登录已过期" }
```

| 情形 |
|---|
| 缺少 `Authorization` 头 |
| 头格式不是 `Bearer <token>` |
| 令牌在 Redis 中不存在（已登出 / 已过期 / 伪造） |
| 会话记录已超过 `expires_at` |

前端据此清空本地会话并跳转 `/login?redirect=<来源路径>`。

---

## 口令轮换（运维）

不走迁移，避免把新口令固化进版本库：

1. 用 `internal/meta/finv/auth` 的 `GenerateSalt()` + `Hash(plain, salt)` 生成新盐与 PHC 串；
2. 执行：

```sql
UPDATE sys.sys_user
   SET password = '<新的 PHC 串>',
       password_salt = '<新的 hex 盐>',
       modify_by = '<操作人>'
 WHERE uid = 'finv';
```

`gmt_modify` 由触发器 `trg_sys_user_set_gmt_modify` 自动刷新，`gmt_create` 不变。

## 相关配置

| 配置号 | 名称 | 类型 | 默认 | 说明 |
|---|---|---|---|---|
| 20012 | 登录会话有效期(分钟) | int | 480 | 合法区间 5~43200；越界或非法时回退默认 |
