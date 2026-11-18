# 期货行情排行（FTFutureRank）

> 所属：导航「行情 → 实时行情 → 期货行情排行」
> 路由：`/Meta/Finv/Quote/Realtime/FTFutureRank`
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/FutureRankView.vue`
> 菜单 key：`quote-realtime-ftfuture-rank`（父分组 `quote-realtime`，紧跟「证券行情排行」）
> 数据表：`cdeu.finv_quote_secu_realtime_future`（一行一合约，主键 `stock_id`；原名 `finv_quote_realtime_future`，需求 11325 改名）
> 分类字典：`cdeu.finv_quote_cdeu_future_sync_config`（`futures_type` → `futures_type_name`）
> 接口：[`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Future/Page`](../../../API/Cdeu/RealtimeFuturePage.md)
> 分类下拉：[`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Future/Types`](../../../API/Cdeu/RealtimeFuturePage.md)
> 关联需求：11322、11324（持仓量 / 日增仓列，提供方移入明细，明细多列）

## 1. 业务功能

展示期货**最新一张行情快照**：每行一个合约，主键 `stock_id`，整行覆盖更新。
数据由「行情快照同步」的期货 Tab（`instrument_symbol=future`）采集写入，本页只读。

## 2. 页面构成

### 2.1 筛选与排序

| 控件 | 说明 |
|---|---|
| 关键字 | 模糊匹配 `contract_code` / `name` / `usc` / `stock_id`（回车即查询） |
| 分类 | 首项「全部分类」，其余来自配置表：`指数期货[1]`、`能源化工[2]`、`农产品[3]`、`金属期货[4]`、`利率期货[5]`、`汇率期货[6]`、`加密货币期货[7]`、`股票期货[8]`；配置表缺中文名时降级为「分类 N」（DictFilterSpec：`中文[值]`、首项全部、选中即查） |
| 市场 | `全部 / HK / US / SG / JP`，切换即查询 |
| 状态 | 仅在线（默认，`flag_online='1'`）/ 全部合约 |
| 排序字段 | 涨跌幅（默认）、现价、持仓量、日增仓、更新时间、合约代码、名称、分类、市场、编号 |
| 方向 | 降序（默认）/ 升序；空值一律排在最后 |
| 每页 | 20 / 50（默认）/ 100 / 200，位于表格底部、页码控件左侧 |
| 查询 / 刷新 / 重置 | 查询回到第 1 页；刷新按当前条件重取；重置恢复默认（涨跌幅降序、仅在线） |

### 2.2 表格列

编号（`stock_id`）、合约代码（`contract_code`）、名称（`name`）、分类（`futures_type_name`）、
市场（`market_label`）、现价（`price_nominal`）、涨跌幅（`change_ratio`，百分点数值）、
持仓量（`open_interest`，万/亿折算）、日增仓（`day_increase`，带正负号）、
`usc`、在线（`flag_online`：`1` → 在线 / 其它 → 离线）、
更新时间（`dt_update`，Unix 毫秒按本地时区显示）、明细按钮。

> 持仓量与日增仓不是主表列：后端按 `field_list_json` 里 `fieldName=持仓量 / 日增仓` 定位
> `fieldId`，再取 `fields_json` 原文（实测 335 行全部有值，如 `18.00` / `+1.00`）。
> 「提供方」已从表格移到明细（需求 11324）。

### 2.3 明细弹窗

表里只有**代码 / 名称 / 现价 / 涨跌幅**是结构化列，其余字段由采集端原样留档：

- `fields_json`：`{字段编号: 原文}`，例如 `{"3": "524.30", "4": "+4.00%", "11": "2.00"}`；
- `field_list_json`：字段定义数组，含 `fieldId` / `fieldName`（如 `3` → `价格`、`11` → `持仓量`）。

弹窗按 `field_list_json` 把编号翻成字段名、取 `fields_json` 的原文渲染，
**多列自适应布局**（窄屏 1 列、中屏 2 列、宽屏 3 列），首项固定为「提供方」（`provider`）；
**仓库不臆造编号含义**（采集侧只确认了 1/2/3/4/19 的含义，其余以留档为准）。

### 2.4 展示规则

- 价格最多 4 位小数并去尾零，涨跌幅统一 `x.xx%`；空值显示 `—`；
- **红涨绿跌**：`price_direct` 为 `+` 整行红色、为 `-` 整行绿色（与证券行情排行一致）；
- 分类名缺失时显示「分类 N」，不显示空串。

## 3. 使用说明

1. 进入菜单默认「仅在线」+ 涨跌幅降序，取第一页 50 条；
2. 切换分类/市场/状态后自动回到第 1 页；
3. 表格可横向滚动；点「明细」查看该合约的完整字段留档；
4. 本页不提供编辑入口，数据由采集侧写入。

## 4. 注意事项

- 这是**快照表**（一行一台约），不保留历史序列；
- `usc` 由采集侧三级解析（`stock_id` → 键表 → usc 引擎结果表 `futu.finv_futu_usc_map`），
  实测 335 行全部有值（需求 11326）；仍解析不到的才显示 `—`；
- 同一 `stock_id` 若被多个分类采集覆盖，`futures_type` 取最后一次写入的分类（采集按分类升序）。
