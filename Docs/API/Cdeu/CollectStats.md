# GET /API/V1/Meta/Finv/Quote/Secu/Collect/Stats — 表规模统计

> 只读接口：返回状态总览页需要的「关键表当前行数」。
> 机器可读契约：[`CollectStats.yml`](CollectStats.yml)（OpenAPI 3.1），两者须成对更新。
>
> - 实现：`internal/api/handler/secu_collect.go`（`SecuFtmmCollect.Stats`）、
>   `internal/api/router.go`、`internal/meta/finv/quote/cdeu/store_table_stats.go`
> - 关联需求：11285（状态总览页表规模指标）
> - 关联菜单：[`Menu/Quote/RealtimeCollect/Overview.md`](../../Menu/Quote/RealtimeCollect/Overview.md)

## 1. 概述

| 项 | 说明 |
|---|---|
| 认证 | 需登录（`Authorization: Bearer <token>`），未登录返回 401 |
| 参数 | **无**（表名由服务端白名单固定，不接受请求拼表名） |
| 语义 | 每张表 `SELECT count(*)`，只读、不落库、不缓存 |
| 表清单 | `cdeu.finv_quote_secu_indicator`（证券指标表）、`futu.finv_futu_quote_collect`（富途采集参数镜像表） |
| 缺表处理 | 表不存在时该项 `exists=false`、`rows=0`，**不报错**（让页面在部分表缺失的环境照常渲染） |

## 2. 请求

```bash
curl -sS 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Secu/Collect/Stats' \
  -H 'Authorization: Bearer <token>'
```

## 3. 响应

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "tables": [
      {"table": "cdeu.finv_quote_secu_indicator", "rows": 70171, "exists": true},
      {"table": "futu.finv_futu_quote_collect", "rows": 70722, "exists": true}
    ],
    "rows_total": 140893,
    "elapsed_ms": 12
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.tables[]` | array | 按固定顺序返回（与白名单一致） |
| `data.tables[].table` | string | schema 全限定表名 |
| `data.tables[].rows` | integer(int64) | 当前行数；`exists=false` 时恒为 0 |
| `data.tables[].exists` | boolean | 该环境是否存在这张表 |
| `data.rows_total` | integer(int64) | 各项 `rows` 之和（便于一眼看总规模） |
| `data.elapsed_ms` | integer | 本次统计耗时 |

## 4. 行为说明

1. **表名白名单**：表清单来自 `store_table_stats.go` 的 `tableStatsTargets` 常量，
   由 `cdeu.TableStatsTargets()` 传给 handler；请求无法指定表名，也不存在 SQL 拼接面。
2. **两张表的行数应基本相当**（都是一行一证券）：差值可用来判断采集覆盖是否有缺口
   （例如指标表少了几百行，说明有证券没回推指标）。
3. **不做缓存**：每次调用都实时 `count(*)`；7 万行量级在本地库是毫秒级，
   但不要把它挂到高频轮询上（状态总览页只在进入页面 / 点「刷新」时调用一次）。
4. 表数量固定为 2，实现上用「存在性判断 + count」两次往返/表，不做批量拼 SQL 优化。

## 5. 错误目录

| `code` | HTTP | 触发场景 |
|---|---|---|
| `2006` | 500 | 数据库统计失败（连接异常、权限不足等），message 带具体原因 |
| `401` | 401 | 未登录或 token 失效 |

## 6. 前端

- 页面：行情 → 行情采集 → 状态总览（`/Meta/Finv/Quote/RealtimeCollect/Overview`）
- 展示：两张指标卡（表名 + 千分位行数），统计失败时在卡片区显示错误文案，不影响同一页的配置与运行结果
- 请求时机：进入页面时与 `.../Run` **并发**发起（`Promise.allSettled`，互不阻塞）；点「刷新」重新拉取
