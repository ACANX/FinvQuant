# 握手的使用说明（Hello）

> 所属：FinvQuant 对外 OpenAPI **使用说明** · 存放：`Docs/OpenAPI/Usage/Handshake.md`
> 对应接口规格：[`../Region/AZ/Server/Hello.md`](../Region/AZ/Server/Hello.md)
> 需求：11242（端点）、11243（`kfp`）、11245（规格与用法拆开：本文只讲怎么用）

接口规格（参数、字段、错误码）见上面的规格文档。本文回答「**客户端怎么接、怎么自检、出问题怎么查**」。

## 1. 最短路径：三条命令确认能说上话

```bash
# 1) 静态 Key（最省事）
curl -s -H "X-API-Key: SA_LIVE_XXXXXXXXXXXXXXXXXXXXXXXXXXXX" \
  http://127.0.0.1:16001/OpenAPI/V1/Meta/Region/AZ/Server/Hello

# 2) AK/SK 签名（需自己算签名，见 Auth.md §3）
curl -s -H "Authorization: META1-HMAC-SHA256 Credential=AK_LIVE_…, Signature=…" \
  -H "X-META-TS: 1794427291" -H "X-META-Nonce: 8e2f1c9a4b7d4c0e" \
  http://127.0.0.1:16001/OpenAPI/V1/Meta/Region/AZ/Server/Hello

# 3) STS 三要素（多一个 X-META-Token）
curl -s -H "Authorization: META1-HMAC-SHA256 Credential=SAK_LIVE_…, Signature=…" \
  -H "X-META-TS: …" -H "X-META-Nonce: …" -H "X-META-Token: SST_LIVE_…" \
  http://127.0.0.1:16001/OpenAPI/V1/Meta/Region/AZ/Server/Hello
```

只看三样就算成功：

```
HTTP 200
data.handshake == "ServerHello"
data.client.credential_no == 你预期的凭证号
```

## 2. 客户端启动自检（推荐写法）

拿到 `server_time` 之后**一定要校时**——「网络通但业务接口一直 `19006`」是最难查的一类问题：

```python
import time

def handshake(call) -> dict:
    status, headers, body = call("GET", "/OpenAPI/V1/Meta/Region/AZ/Server/Hello")
    data = body["data"]
    assert status == 200, f"握手失败：HTTP {status} code={body.get('code')} {body.get('message')}"
    assert data["handshake"] == "ServerHello", "响应不是 ServerHello"
    drift = abs(int(time.time()) - data["server_time"]["unix"])
    limit = data["capabilities"]["clock_skew_seconds"]
    assert drift < limit, f"本机与平台时钟差 {drift}s，超过时间窗 {limit}s，请先校时"
    print("握手成功：", data["client"]["credential_no"], "可用接口", len(data["endpoints"]))
    # 顺手把可用接口清单缓存下来，作为客户端请求白名单，避免拼错路径
    return data
```

要点：

- **先看 `handshake` 再看 HTTP 码**：宽松模式（19001=2）下失败也会放行，只看 200 会误判；
- `capabilities.auth_mode` 为 `2` 时表示「校验并记审计但仍放行」，不要当成认证真的通过；
- `client.scope_count == 0` 表示该凭证**不限接口**（不是没权限），排查越权时先看这个；
- `client.kfp` 可直接拿去审计表核对（字段名与 `open_api_invoke_log.kfp` 一致）。

## 3. 运维巡检

```bash
curl -s -H "X-API-Key: SA_LIVE_…" \
  http://127.0.0.1:16001/OpenAPI/V1/Meta/Region/AZ/Server/Hello \
  | jq '{handshake, server, client: .client.credential_no, endpoints: (.endpoints|length)}'
```

输出示例：

```json
{ "handshake": "ServerHello",
  "server": { "name": "finvquant", "version": "0.1.5", "commit": "dev", "az": "AZ" },
  "client": "DEMO-STATIC",
  "endpoints": 5 }
```

适合做成存活探针（区分「进程活着」与「鉴权链路正常」两件事，见 §5）。

## 4. 审计对账

握手每次调用都会写一条审计（`flag_pass=1`、`status_code=200`）：

```sql
SELECT dt_request, request_id, auth_mode, access_key_id, kfp, flag_pass, status_code
  FROM open.open_api_invoke_log
 WHERE path = '/OpenAPI/V1/Meta/Region/AZ/Server/Hello'
 ORDER BY idn DESC LIMIT 20;
```

用响应体里的 `request_id` 精确回查：

```sql
SELECT dt_request, auth_mode, access_key_id, kfp, flag_pass, reason_code
  FROM open.open_api_invoke_log
 WHERE request_id = '336123BF096A0D87D6C876C418FE4ABD';
```

`kfp` 与响应体 `data.client.kfp` **同名字段同值**，客户端可自建「指纹 → 凭证号」映射表做长期核对。

## 5. 为什么不用 `/Health`

| 探针 | 回答的问题 | 认证 |
|---|---|---|
| `/Health`、`/api/v1/health/live\|ready` | 进程活着吗、依赖（PG/Redis）可用吗 | 不需要 |
| **本接口** | **我这个调用方，用这套凭证，能不能跟平台说上话** | 需要（走完整鉴权链路） |

运维上两者互补：`/Health` 挂了是平台问题；`/Health` 正常但握手失败，是调用方凭证/时钟/白名单问题。

## 6. 常见失败与处理

| 现象 | 原因 | 处理 |
|---|---|---|
| `401 / 19001` | 没带凭证 | 补 `X-API-Key` 或 `Authorization` |
| `401 / 19002` | AK 未登记、SK 不符、STS 缺 `X-META-Token` | 核对 Key/Token；`19002` 刻意合并「AK 不存在」与「SK 不匹配」防枚举 |
| `403 / 19003` | 凭证吊销/停用/过期，或调用方停用 | 页面「OpenAPI 凭证」查状态 |
| `401 / 19004` | 来源 IP 不在白名单 | 在调用方或凭证上补 `allow_cidr` |
| `401 / 19005` | 签名不匹配（含算法名与凭证登记不一致） | 核对待签串顺序、`BODY_SHA256` 是否大写 |
| `401 / 19006` | 时间戳超窗 | 用 `data.server_time.unix` 校准本地时钟后重试 |
| `401 / 19007` | nonce 重复 | 每次请求生成新 nonce |
| `429 / 19008` | 超速率/配额 | 放慢节奏或调整额度 |
| `503 / 19010` | 平台未就绪（19005 未配置 / Redis fail-close） | 联系平台方 |
| `403 / 19009` | **路径打错了** | 本接口豁免授权范围，不会返回 `19009`；看到它说明请求落到了业务接口 |

## 7. STS 场景的额外说明

- **兑换凭证不能握手**：`STS` 模式签发的长期 AK/SK（`token_fp` 为空）只能调换证接口，
  用它测连通性的方式就是调 `POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue`；
- **临时凭据可以握手**：用换来的 `SAK/SSK/SST` 调本接口，可确认三要素是否被平台接受
  （`data.client.auth_mode == "STS"`、`credential_no` 形如 `STS-47-…`）；
- 临时凭据 2 小时到期且不可续期：握手返回 `19003` 时先换一次证再试。
