# GET/POST /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/* — 五日分钟行情采集配置

> 所属：行情 → 行情采集 → 五日分钟行情采集配置
> 数据表：Supabase `public.finv_quote_secu_ft`（**本地库无副本**，经 PostgREST 访问）
> 连接来源：`sys.sys_config` 20013 `{PROJECT_ID}|{APIKEY}`（见 [CollectSysConfig.md](CollectSysConfig.md)）
> 关联需求：11344（查询）、11345（新增）、11346（编辑）
> 实现：`internal/api/handler/quote_collect_fiveday_ft.go`、`internal/meta/finv/quote/secuft/`、
> 通道 `internal/meta/finv/quote/secukey/client.go`（`FetchQuery` / `InsertRow`）
> 关联菜单：[`Menu/Quote/RealtimeCollect/FiveDayFt.md`](../../Menu/Quote/RealtimeCollect/FiveDayFt.md)

## 1. 总览

| 方法 | 路径 | 用途 | 需求 |
|---|---|---|---|
| GET | `.../FiveDayFt/Page` | 分页查询（关键字 + 5 维筛选 + 白名单排序） | 11344 |
| GET | `.../FiveDayFt/FilterOptions` | 下拉候选（表内 DISTINCT） | 11344 |
| GET | `.../FiveDayFt/Lookup` | 按 usc 联查本地 4 表补全元数据（不写库） | 11345 |
| POST | `.../FiveDayFt/Create` | 新增一行采集配置 | 11345 |
| POST | `.../FiveDayFt/Update` | 编辑一行采集配置（只写改动列） | 11346 |

三条硬口径：

1. **只有「新增」与「编辑」，没有删除**：删除在 Supabase 侧进行（避免误删线上采集配置）。
2. **新增默认停采**：`flag_enable = 0`，人工确认后改成 `1` 才真正开始采集。
3. **列元数据由服务端下发**：前端不硬编码列名，表结构变化只需改后端常量。

## 2. 表结构（实测 32 列）

> 探明方式（2026-11-19）：抓 `select=*` 的 CSV 表头确认 32 列（含当天新增的 465 行数据），
> 再用 `?<col>=eq.@@@` 的类型转换报错逐列反推类型。

| 列 | 中文表头 | 类型 | 说明 |
|---|---|---|---|
| `usc` | 统一编号 | text | 与本地 `finv_security.usc` 同口径 |
| `name` | 证券名称 | text | — |
| `region` | 区域 | text | CN / HK / US / FX / CC |
| `market` | 市场 | text | 细分市场：SH/SZ/HK/BJ 或 NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX/CME/CBOT/NYMEX |
| `market_label` | 行情市场 | text | HK/SH/SZ/BJ/US/FX/CC |
| `symbol` | 证券全码 | text | 富途全码（`ftmm_symbol`），如 `HK.00700`、`US..BKX` |
| `currency` | 计价货币 | text | — |
| `timezone` | 时区 | text | IANA 名，如 `Asia/Shanghai` |
| `provider` | 提供方 | text | — |
| `type_secu` | 证券分类 | text | — |
| `type_secu_sub` | 证券子类 | text | — |
| `instrument` | 品种 | text | stock/etf/index/forex/crypto/futures |
| `flag_enable` | 采集开关 | smallint | 1 启用 / 0 禁用 |
| `futu_symbol` | 富途代码 | text | 裸码，如 `00700`、`.BKX` |
| `stock_id` | 富途ID | bigint | 富途证券内部 ID |
| `market_type` | 市场类型 | bigint | 1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密 |
| `market_code` | 市场代码 | bigint | 富途市场代码（**不是** `finv_market.market_code`） |
| `instrument_type` | 品种类型 | bigint | 3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密 |
| `sub_instrument_type` | 品种子类型 | bigint | 如 3002 / 4002 / 6001 |
| `req_section` | 订阅段 | integer | 富途行情订阅段，0 表示未设 |
| `weight_priority` | 采集权重 | numeric | 采集优先级，默认 50 |
| `weight_frequency` | 采集频率 | numeric | 采集频率权重，默认 10 |
| `ts_latest_data` | 最新数据时间戳 | bigint | 采集统计 |
| `count_last_fetch` | 累计采集次数 | bigint | 采集统计 |
| `count_fail` | 累计失败次数 | bigint | 采集统计 |
| `count_stale` | 连续陈旧次数 | bigint | 采集统计 |
| `dt_update` | 数据更新时间 | timestamptz | — |
| `dt_last_check` | 最近检查时间 | timestamptz | 采集统计 |
| `dt_last_fetch` | 最近采集时间 | timestamptz | 采集统计 |
| `gmt_update` | 更新时间 | timestamptz | — |
| `gmt_create` | 创建时间 | timestamptz | — |
| `remark` | 备注 | text | — |

**当前数据量（2026-11-19 实测）**：465 行。

## 3. 分页查询 `GET .../FiveDayFt/Page`

### 3.1 请求参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `page` | query | integer | 否 | `1` | 页码（≥1） |
| `pageSize` | query | integer | 否 | `15` | 每页条数，1–200（超出截断） |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `usc` / `symbol` / `futu_symbol` / `name` |
| `region` | query | string | 否 | 空 | 精确匹配 `region`；空或 `all` 不限 |
| `marketLabel` | query | string | 否 | 空 | 精确匹配 `market_label` |
| `instrument` | query | string | 否 | 空 | 精确匹配 `instrument` |
| `typeSecu` | query | string | 否 | 空 | 精确匹配 `type_secu` |
| `provider` | query | string | 否 | 空 | 精确匹配 `provider` |
| `flagEnable` | query | string | 否 | 空 | `1` 启用 / `0` 禁用；其它值视为不限 |
| `sortBy` | query | string | 否 | `usc` | 白名单：32 列全部可排序，非法值回退 `usc` |
| `sortOrder` | query | string | 否 | `asc` | `asc` / `desc` |

### 3.2 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "list": [
      {
        "usc": "000001", "name": "平安银行", "region": "CN", "market": "SZ",
        "market_label": "SZ", "symbol": "SZ.000001", "instrument": "stock", "flag_enable": 1
      }
    ],
    "total": 465,
    "page": 1,
    "pageSize": 15,
    "table": "finv_quote_secu_ft",
    "elapsed_ms": 320,
    "columns": [{ "name": "usc", "title": "统一编号", "kind": "text", "sortable": true }]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.list` | array | 本页行，键为原表列名；`null` 原样保留 |
| `data.total` | integer | 满足条件的总行数（`Content-Range` 尾段；取不到为 0） |
| `data.page` | integer | 服务端收敛后的页码 |
| `data.pageSize` | integer | 服务端收敛后的每页条数 |
| `data.table` | string | 源表名 |
| `data.elapsed_ms` | integer | 请求 Supabase 耗时 |
| `data.columns` | array | 列元数据（`name` / `title` / `kind` / `sortable` / `note`） |

### 3.3 错误

| `code` | HTTP | 触发场景 |
|---|---|---|
| `2007` | 502 | Supabase 不可达 / 20013 未配置 / 鉴权失败 / 表不存在 |
| `401` | 401 | 未登录 |

**offset 越界不是错误**：PostgREST 在 `offset` 超过总行数时返回 `416` + `PGRST103`，
服务端把它当作「本页为空」，`total` 取 `Content-Range` 尾段原样返回。

## 4. 下拉候选 `GET .../FiveDayFt/FilterOptions`

按 `region` / `market_label` / `instrument` / `type_secu` / `provider` 各拉一次 DISTINCT（每维上限 500），
返回 `{"options":{"region":["CN","HK",...], ...}}`。

## 5. 元数据补全 `GET .../FiveDayFt/Lookup?usc=00700`（需求 11345）

按 `usc` **联查本地 4 张表**（不写库）：

```sql
public.finv_security s
LEFT JOIN futu.finv_futu_quote_collect c ON c.usc = s.usc
LEFT JOIN futu.finv_futu_secu_key      k ON k.usc = s.usc
LEFT JOIN cdeu.finv_quote_secu_indicator i ON i.usc = s.usc
```

（各表按 `usc` 唯一，实测 `GROUP BY usc HAVING count(*)>1` 全为 0；仍用 `LATERAL ... LIMIT 1` 兜住将来重复。）

### 5.1 补全口径（用 465 行真实数据校准）

| 列 | 来源 | 一致率 | 说明 |
|---|---|---|---|
| `name` | `finv_security.security_name_cn`（空回退 `security_name`） | 456/459 | 3 行是人工改过的名称 |
| `region` | `market_label` 映射（SH/SZ/BJ→CN、HK→HK、US→US、FX→FX、CC→CC；`BK` 板块按 `exchange_code` 11/13→CN、12/21→HK、30→US） | 457/459 | — |
| `market` | `quote_collect.market_label`（**仅建议值**） | 254/459 | 205 行是细分市场（NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX/CME/CBOT/NYMEX），本地无该粒度 → 提示核对 |
| `market_label` | `quote_collect.market_label` | 447/459 | 12 行 `BK*` 板块在现网用了交易所 |
| `symbol` | `quote_collect.ftmm_symbol` → `secu_key.ftmm_symbol` → `indicator.ftmm_symbol` | 447/459 | 全码，如 `HK.00700`、`US..BKX` |
| `currency` | `finv_security.currency_type` | 429/459 | 外汇对取报价货币 |
| `timezone` | `finv_security.tz` | 347/459 | IANA 名 |
| `provider` | 默认 `FTMM` | 459/459 | 现网该列全部是 FTMM |
| `type_secu` | `finv_security.type_secu` | 367/459 | 现网更细（ADR/FX/Future/Crypto/LOF） |
| `type_secu_sub` | `finv_security.type_secu_sub` | 40/459 | 本地缺失较多，建议手填 |
| `stock_id` | `quote_collect.stock_id` | 457/459 | — |
| `market_type` | `quote_collect.market_type` | 457/459 | — |
| `market_code` | `quote_collect.market_code` | 457/459 | 富途市场代码 |
| `instrument_type` | `quote_collect.instrument_type` | 457/459 | — |
| `sub_instrument_type` | `quote_collect.sub_instrument_type` | 457/459 | — |
| `req_section` | `quote_collect.req_section` | 457/459 | varchar 转 integer，空→0 |
| `futu_symbol` | `quote_collect.futu_symbol` | 457/459 | 裸码 |
| `instrument` | `quote_collect.instrument` 归一（`etfs`→`etf`、`sectors`→`index`、`currency`→`forex`；**`futures` 保持原样**） | 457/459 | — |
| `weight_priority` | 默认 `50` | — | 现网 437/465 是 50 |
| `weight_frequency` | 默认 `10` | — | 现网 465 行全部 10 |
| `flag_enable` | 默认 `0`（先停采） | — | 现网存量全是 1 |
| 采集统计列 | 新增时留空 | — | `ts_latest_data` / `count_*` / `dt_last_check` / `dt_last_fetch` |
| 时间列 | 服务端写 `now()` | — | `gmt_create` / `gmt_update` / `dt_update` |

### 5.2 响应

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "usc": "00700",
    "fields": { "usc": "00700", "symbol": "HK.00700", "region": "HK", "flag_enable": 0 },
    "sources": { "symbol": "富途采集", "name": "证券主数据", "region": "市场映射" },
    "missing": ["type_secu_sub"],
    "notes": ["「市场」列在现网数据里有 44% 是细分市场……请核对后再提交"],
    "in_security": true,
    "has_indicator": false,
    "exists_in_supabase": true
  }
}
```

| 字段 | 说明 |
|---|---|
| `fields` | 32 列建议值（补不出来的为 `null`） |
| `sources` | 每个字段的取值来源**短标签**：`输入` / `证券主数据` / `富途采集` / `键表` / `指标表` / `市场映射` / `默认值` / `留空`（最长 8 字，前端「来源」列固定窄宽） |
| `missing` | 补不出来、需要手填或核对的列（**不含**时间列与采集统计列） |
| `notes` | 提示：无富途采集记录 / 细分市场需核对 / 已有指标数据 / Supabase 已存在 |
| `in_security` | `false` 表示该 usc 不在本地证券主数据，禁止新增 |
| `exists_in_supabase` | Supabase 里已存在该 usc（前端直接拦下） |

## 6. 新增 `POST .../FiveDayFt/Create`（需求 11345）

### 6.1 请求体

```json
{
  "usc": "00700",
  "fields": {
    "symbol": "HK.00700", "futu_symbol": "00700", "name": "腾讯控股",
    "market_type": 1, "market_code": 1, "instrument_type": 3, "sub_instrument_type": 3002,
    "flag_enable": 0
  }
}
```

- `fields` 只接受 32 列里**除 `usc` 与三列时间列**之外的 28 列，出现未知列或时间列返回 400。
- 未提供的列填服务端默认值：`flag_enable=0`、`provider=FTMM`、`req_section=0`、
  `weight_priority=50`、`weight_frequency=10`。
- 三列时间列由服务端写 `now()`；采集统计列不写。
- 采集统计列（`ts_latest_data` / `count_*` / `dt_last_check` / `dt_last_fetch`）允许传，但前端不传。

### 6.2 响应

```json
{ "code": 0, "message": "新增成功（采集开关默认关闭，需手动开启）", "data": { "usc": "00700", "row": {} } }
```

### 6.3 错误

| `code` | HTTP | 触发场景 |
|---|---|---|
| `400` | 400 | usc 为空 / usc 不在本地证券主数据 / 未知列或时间列 / 非法 JSON |
| `403` | 403 | **Supabase RLS 未开放 insert**（`42501 new row violates row-level security policy`）/ 角色缺 GRANT |
| `2007` | 502 | Supabase 不可达 / 20013 未配置 |

**实测（2026-11-19）**：现有 20013 的 publishable key 对该表 **UPDATE/DELETE 可用**（PATCH/DELETE 条件不匹配返回
`204`），但 **INSERT 被 RLS 拒绝**，返回 `401 + 42501`。需要 Supabase 侧加策略：

```sql
create policy "finv_quote_secu_ft_insert"
  on public.finv_quote_secu_ft
  for insert to anon, authenticated
  with check (true);
```

（`sb_publishable_...` 这类 key 映射到 `anon` 角色。）

## 7. 编辑 `POST .../FiveDayFt/Update`（需求 11346）

### 7.1 请求体

```json
{ "usc": "00700", "fields": { "flag_enable": 1, "remark": "人工确认后开启" } }
```

- `usc` 定位键（主键，**不可修改**）。
- `fields` 只写请求里出现的列：**只提交改动列**，没出现的列保持原值。
- 可编辑 **22 列**（32 列去掉 `usc`、5 个时间列、4 个采集统计非时间列）：

| 列名 | 中文 |
|---|---|
| `name` | 证券名称 |
| `region` | 区域 |
| `market` | 市场 |
| `market_label` | 行情市场 |
| `symbol` | 证券全码 |
| `currency` | 计价货币 |
| `timezone` | 时区 |
| `provider` | 提供方 |
| `type_secu` | 证券分类 |
| `type_secu_sub` | 证券子类 |
| `stock_id` | 富途ID |
| `market_type` | 市场类型 |
| `market_code` | 市场代码 |
| `instrument_type` | 品种类型 |
| `sub_instrument_type` | 品种子类型 |
| `req_section` | 订阅段 |
| `weight_priority` | 采集权重 |
| `weight_frequency` | 采集频率 |
| `futu_symbol` | 富途代码 |
| `flag_enable` | 采集开关 |
| `instrument` | 品种 |
| `remark` | 备注 |

**不可修改**（传了直接 400）：

| 列 | 原因 |
|---|---|
| `usc` | 主键，换它等于换证券 |
| `gmt_create` | 创建时间，由服务端维护 |
| `gmt_update` | 更新时间，每次更新自动刷新 |
| `dt_update` | 数据更新时间，由服务端维护 |
| `dt_last_check` | 采集统计时间，由采集链路维护 |
| `dt_last_fetch` | 采集统计时间，由采集链路维护 |
| `ts_latest_data` | 采集统计，人工改会让口径失真 |
| `count_last_fetch` | 采集统计，人工改会让口径失真 |
| `count_fail` | 采集统计，人工改会让口径失真 |
| `count_stale` | 采集统计，人工改会让口径失真 |

- 传 `null` 表示把该列置空（不是「忽略」）；整个 `fields` 为空或全是 `null` 返回 400。

### 7.2 响应

```json
{ "code": 0, "message": "保存完成", "data": { "usc": "00700", "updated": 2, "row": {} } }
```

| 字段 | 说明 |
|---|---|
| `usc` | 定位的 usc |
| `updated` | 实际写入的列数（不含服务端自动刷新的 `gmt_update`） |
| `row` | Supabase 返回的更新后行（`Prefer: return=representation`） |

### 7.3 错误

| `code` | HTTP | 触发场景 |
|---|---|---|
| `400` | 400 | usc 为空 / `fields` 为空或全 `null` / 出现不可编辑列 / 值不是合法 JSON |
| `403` | 403 | **更新被 RLS 静默过滤**（记录存在但影响 0 行） |
| `404` | 404 | `usc` 在采集配置表里不存在 |
| `2007` | 502 | Supabase 不可达 / 20013 未配置 |

**为什么 403 与 404 要分开**：PostgREST 的 UPDATE 在 RLS 策略不满足时是
**静默过滤**——HTTP `200` + `content-range: */*` + 空数组，不报 `42501`（与 INSERT 不同）。
所以服务端在「影响 0 行」时会再查一次该 `usc` 是否存在：存在 → 403（策略问题），
不存在 → 404（记录问题）。实测响应：

```text
PATCH /rest/v1/finv_quote_secu_ft?usc=eq.00700
→ HTTP 200, content-range: */*, body []
```

需要在 Supabase 侧补策略（`sb_publishable_...` 映射到 `anon` 角色）：

```sql
create policy "finv_quote_secu_ft_update"
  on public.finv_quote_secu_ft
  for update to anon, authenticated
  using (true) with check (true);
```

## 8. 排序与分页稳定性

- 排序列追加 `usc` 作为兜底键（`sortBy=usc` 时不重复追加），避免同值行翻页重复/丢失。
- 前端每页条数选项 `15 / 20 / 50 / 100 / 200`，默认 15（需求指定）。

## 9. 实测记录（2026-11-19）

| 场景 | 结果 |
|---|---|
| `Page`（全默认） | `code=0`、`total=465`、`pageSize=15`、`columns=32` |
| `Page?page=2`（越界） | `code=0` + `list=[]`（PostgREST 416 按空页处理） |
| `Page?pageSize=1000` | `pageSize=200`（截断生效） |
| `Page?sortBy=evil` | 回退 `usc`，`code=0` |
| `FilterOptions` | 5 个维度返回 DISTINCT 值 |
| `Lookup?usc=00700` | `symbol=HK.00700`、`region=HK`、`flag_enable=0`、`exists_in_supabase=true` |
| `Lookup?usc=BKX` | `symbol=US..BKX`、`futu_symbol=.BKX`、`has_indicator=true` |
| `Lookup?usc=CSI300` | `symbol=null`、`missing` 含富途编码列（无采集记录） |
| `Lookup?usc=__no_such__` | `in_security=false` |
| `Create`（合法） | **403**（RLS 未开放 insert，需先执行上面的策略 SQL） |
| `Update` 存在的 `00700` | **403** + 「记录存在但更新影响 0 行：被 RLS 静默过滤」（直连 PATCH 实测 `200` + `*/*` + `[]`） |
| `Update` 不存在的 usc | `404` |
| `Update` 传时间列 / 采集统计列 | `400` + 「不允许人工修改（可编辑 22 列）」 |
| `Update` 空 `fields` | `400` + 「没有要修改的字段」 |
| `Create`（不在主数据 / 未知列 / 时间列） | `400` + 明确提示 |
