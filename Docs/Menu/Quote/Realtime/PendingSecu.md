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

证券编号、代码、名称、市场、品种类型、币种、现价、涨跌额、涨跌幅、总市值、成交额、来源页码、快照时间、**操作**。

「操作」列的「录入」按钮打开录入对话框（见 §3）。

涨跌用整行颜色表达（红涨绿跌），与「证券行情排行」同一套配色；无数值的方向行不着色。

### 2.4 分页

底部左侧显示「共 N 条，第 x / y 页」，右侧为每页条数（20/50/100/200）与页码控件。

## 3. 录入对话框（需求 10996）

点击行内「录入」打开对话框，把该证券按三张表各自的字段登记一遍：

| 分区 | 表 | 必填字段（NOT NULL 且无 DEFAULT） |
|------|----|-----------------------------------|
| 1 | `public.finv_security` | `usc`、`exchange_code`、`security_type`、`security_code`、`security_name`、`security_name_cn`、`currency_type` |
| 2 | `futu.finv_futu_quote_collect` | `usc`、`stock_id`、`futu_symbol`、`market_type`、`market_code`、`instrument_type`、`sub_instrument_type`、`quote_market`、`type_symbol` |
| 3 | `cdeu.finv_quote_cdeu_usc` | `usc` |

- `usc` 三表共用，在对话框顶部单独填写一次；
- 其余选填字段一并展示（含默认值提示），留空即用表默认值；
- **必填全部有值**才启用对话框的「录入」按钮，否则按钮禁用并提示缺哪几项、
  分区标题上也有「缺 N 项必填 / 必填已齐」角标；
- 预填只用**快照同名字段直传**（`stock_code`、`name`、`market_type`、`instrument_type`、
  `market_label`、`currency`），**不做任何推断**。

### 3.1 当前阶段边界

| 项 | 状态 |
|----|------|
| 三张表字段展示 + 必填校验 | ✅ 已实现 |
| 字段推断（usc / futu_symbol / quote_market / exchange_code 等） | ⏳ 未实现，规则待定 |
| 一键写入三张表 | ⏳ 未实现；点击「录入」只提示「接口尚未接入」 |
| 批量录入（列表顶部按钮） | ⏳ 规划中，按钮保持 disabled |

实现写入前需先确认的问题：

| 编号 | 待确认 |
|------|--------|
| P1 | `usc` 生成规则（观测值等于 `stock_code`，但需确认是否所有市场都如此） |
| P2 | `futu_futu_quote_collect` 各字段的来源（`quote_market` / `type_symbol` / `sub_instrument_type`） |
| P3 | `market_code` / `exchange_code` 的市场映射表（HK=21、SZ=12、SH=11 是否全覆盖） |
| P4 | 三张表的写入顺序与失败回滚策略（建议同事务，`public.finv_security` → `futu` → `cdeu`） |
| P5 | 是否需要人工复核（先落草稿表再审） |
| P6 | 录入后是否自动把 `flag_enable` 置 `'1'` 并触发一次列表采集验证 |
