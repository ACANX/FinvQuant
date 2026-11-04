# 待录入证券（PendingSecu）

> 菜单 key：`quote-realtime-pending-secu`
> 路由：`/Meta/Finv/Quote/Realtime/PendingSecu`
> 所属：导航「行情 → 实时行情 → 待录入证券」
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/PendingSecuView.vue`
> 关联需求：10995（本页；一键录入为规划功能，尚未实现）
> 关联接口：[`API/Cdeu/RealtimeStockPendingPage.md`](../../../API/Cdeu/RealtimeStockPendingPage.md)

## 1. 用途

列出 `cdeu.finv_cdeu_secu_realtime_stock` 中 **`usc` 为空**的快照行。

采集会把所有在售证券都写入快照表，`usc` 为空表示该证券**尚未录入证券主数据**
（新上市 / 未纳入采集范围）。这份清单是后续补录三张表的依据：

| 表 | 职责 |
|----|------|
| `public.finv_security` | 证券主数据（`currency_type`、`exchange_code`） |
| `futu.finv_futu_quote_collect` | futu 采集配置（`stock_id ↔ usc`） |
| `cdeu.finv_quote_cdeu_usc` | usc 主数据（`market` / `currency` / `timezone`） |

> `usc` 为空是**刻意保留的观测信号**：禁止用 `stock_code` 回填。
> 两者并非等价，直接挪用会把不同市场的同码证券映射错（如 `000001` 在 SZ 与 SH 都存在）。

## 2. 页面结构

### 2.1 说明区

顶部 info 提示解释 `usc` 为空的业务含义，并给出补录三张表的顺序。

### 2.2 筛选与操作（单行 flex，窄屏自动换行）

| 控件 | 说明 |
|------|------|
| 关键字 | 匹配 `stock_code` / `name` |
| 市场 | `HK` / `SH` / `SZ` / `BJ` / `US`，默认全部 |
| 排序字段 | 总市值（默认）/ 成交额 / 涨跌幅 / 来源页码 / 更新时间 / 证券编号 |
| 方向 | 降序（默认）/ 升序 |
| 查询 / 刷新 / 重置 | 查询重置到第 1 页；刷新保持当前页 |

### 2.3 列表列

证券编号、代码、名称、市场、品种类型、币种、现价、涨跌额、涨跌幅、总市值、成交额、来源页码、快照时间。

涨跌用整行颜色表达（红涨绿跌），与「证券行情排行」同一套配色；无数值的方向行不着色。

### 2.4 分页

底部左侧显示「共 N 条，第 x / y 页」，右侧为每页条数（20/50/100/200）与页码控件。

## 3. 一键录入（规划，尚未实现）

计划实现：勾选若干行 → 一次性写入
`public.finv_security` + `futu.finv_futu_quote_collect` + `cdeu.finv_quote_cdeu_usc`，
并在 `futu` 侧按 `market_type` 生成 `futu_symbol` / `quote_market` / `type_symbol` 等采集参数。

当前页面上该按钮为 `disabled`，hover 提示规划内容；后端**未提供写入接口**。

实现前需先确认的问题：

| 编号 | 待确认 |
|------|--------|
| P1 | `usc` 生成规则（当前观测值等于 `stock_code`，但需确认是否所有市场都如此） |
| P2 | `futu_futu_quote_collect` 各字段的来源（`quote_market` / `type_symbol` / `sub_instrument_type`） |
| P3 | 三张表的写入顺序与失败回滚策略（建议同事务） |
| P4 | 是否需要人工复核（先落草稿表再审） |
