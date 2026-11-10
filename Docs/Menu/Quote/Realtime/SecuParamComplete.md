# 证券参数补全（SecuParamComplete）

> 菜单 key：`quote-realtime-secu-param-complete`
> 路由：`/Meta/Finv/Quote/Realtime/SecuParamComplete`
> 所属：导航「行情 → 实时行情 → 证券参数补全」（需求 11161）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/SecuParamCompleteView.vue`
> 对话框：`Web/src/views/Meta/Finv/Quote/Realtime/SecuParamCompleteDialog.vue`
> 关联需求：11161（本页）、10996 / 11147（原「录入」按钮的预填规则，原样迁入）、
> 11145 / 11146 / 11153–11156（`usc` 由「[待采证券录入](PendingSecu.md)」负责）
> 关联接口：
> [`API/Cdeu/SecuParamCompletePending.md`](../../../API/Cdeu/SecuParamCompletePending.md)、
> [`API/Cdeu/SecuParamCompletePrefill.md`](../../../API/Cdeu/SecuParamCompletePrefill.md)、
> [`API/Cdeu/SecuParamComplete.md`](../../../API/Cdeu/SecuParamComplete.md)

## 1. 用途

把「[待采证券录入](PendingSecu.md)」页表格「操作」列里原来的「录入」按钮拆出来，独立成菜单，
并按**目标表**分成两个 Tab：

| Tab | 目标表 | 补什么 |
|-----|--------|--------|
| 规范证券参数 | `public.finv_security` | 证券主数据：`security_type` / `security_code` / `name` / `currency_type` / `exchange_code` 等 |
| 采集参数 | `futu.finv_futu_quote_collect` | futu 采集配置：`futu_symbol` / `market_type` / `market_code` / `instrument_type` / `sub_instrument_type` / `instrument` / `flag_enable` 等 |

拆开的理由：两张表的缺口并不同步，一次写两张看不出补了哪张；而且「补全」是**单表动作**，
补完规范证券参数不应该顺带写采集配置。

> 本页**只读快照、只写目标表**。`usc` 本身由「待采证券录入」页负责
> （「入队」登记键 → USC 规则解析 → 「手动补录USC」回写快照），本页按已有的 `usc` 补齐主数据。

## 2. 列出口径

源表：`cdeu.finv_cdeu_secu_realtime_stock`（一行一证券，采集写入的全量在售证券）。

一个证券算「待补全」= 目标表里**没有**它：

```sql
NOT EXISTS (
  SELECT 1 FROM <目标表> x
   WHERE x.usc = COALESCE(NULLIF(upper(btrim(t.usc)), ''), upper(btrim(t.market_label) || '.' || btrim(t.stock_code)))
      OR x.usc = upper(btrim(t.market_label) || '.' || btrim(t.stock_code))
)
```

| 概念 | 表达式 | 说明 |
|------|--------|------|
| 目标键 | `COALESCE(NULLIF(upper(btrim(t.usc)), ''), 派生键)` | 补全实际写入的主键；快照 `usc` 优先 |
| 派生键 | `upper(btrim(t.market_label) \|\| '.' \|\| btrim(t.stock_code))` | 与 `futu.finv_futu_secu_key.ftmm_symbol` 同口径，仅兜底 |

为什么目标键优先用快照 `usc` 而不是派生键：`usc` 是「统一证券代码」，不是「市场.代码」的机械拼接 ——
库里同时存在 `00700`（裸代码）、`AAVE.FTASB`（加密货币的交易所变体）、`SHCI`（指数）这类取值，
派生键只对很规整的股票代码成立。快照 `usc` 与两张主数据表的口径一致（实测以裸代码为主）。

为什么存在性判断同时接受两种形态：快照 `usc` 可能是裸代码（`STX`）而目标表已有 `US.STX`，
只比一种形态会把它当缺失，一补就撞 `(exchange_code, security_code)` /
`(market_code, futu_symbol)` 唯一约束。两种形态任一命中即视为「已有」。

实测（本机库）：待补 `public.finv_security` 635 行、`futu.finv_futu_quote_collect` 638 行。

## 3. 页面结构

### 3.1 标题区

标题「证券参数补全」，右侧 chip 显示当前 Tab 的目标表名（`public.finv_security` /
`futu.finv_futu_quote_collect`），副标题一句话说明「点补全只写当前 Tab 那一张表，已存在不改动」。

### 3.2 Tab

`v-tabs` 两个页签：

| 页签 | 图标 | 目标表 |
|------|------|--------|
| 规范证券参数 | `mdi-shield-key-outline` | `public.finv_security` |
| 采集参数 | `mdi-server-network` | `futu.finv_futu_quote_collect` |

切换 Tab 保留筛选条件，页码回到第 1 页重新查询。

### 3.3 筛选与操作（单行 flex，窄屏自动换行）

| 控件 | 说明 |
|------|------|
| 关键字 | 匹配 `usc` / `stock_code` / `name` / 派生键 |
| 市场 | `HK` / `SH` / `SZ` / `BJ` / `US`，默认全部 |
| 排序字段 | 总市值（默认）/ 成交额 / 涨跌幅 / 证券编号 / 更新时间 |
| 方向 | 降序（默认）/ 升序 |
| 查询 / 刷新 / 重置 | 查询重置到第 1 页；刷新保持当前页 |

### 3.4 列表列

`usc`（写入键）、证券编号、代码、名称、市场、品种类型、币种、总市值、快照时间、操作。

- **`usc`（写入键）**：表头带 ⓘ 悬浮说明「快照 usc 有值就用它，为空时用派生键」；
  取值来自快照 `usc`，为空时显示派生键并在末尾加 `*` 标记（`*` = 派生值，见 ExplainCardSpec 图例）；
- **品种类型**走字典显示中文（口径取自 `futu.finv_futu_quote_collect.instrument` 实际取值）：
  `3`/`12` 股票、`4` ETF、`6` 指数、`7` 板块、`9` 期货、`11` 加密货币；未登记值原样显示，空值 `—`；
- **操作**列只有一个「补全」按钮，点它**只打开对话框**（不写库），悬浮说明写清目标表名。

### 3.5 分页

底部左侧「共 N 条，第 x / y 页」，右侧每页条数（20/50/100/200）与页码控件。

### 3.6 说明卡片

页面最底部唯一一个「说明」卡片（`ExplainCardSpec`），五段：① 列表口径（含两种形态匹配的原因）、
② 补全只写一张表（`ON CONFLICT DO NOTHING`，已存在不改）、③ 两步走 + 来源标记图例、
④ 与其它菜单的关系（`usc` 由「待采证券录入」负责；`remark` 带 `[AUTO-FILL]` 前缀）、⑤ 品种类型字典。

## 4. 补全对话框

点行内「补全」打开，**两步走、人工确认**（需求 11154 的既定做法）：

| 步 | 动作 | 接口 | 写库 |
|:--:|------|------|:----:|
| ① | 点「补全」→ 打开对话框，拉预填 | `GET .../Secu/Param/Complete/Prefill?target=&stockId=` | 否 |
| ② | 核对字段（可改）→ 点「写入」 | `POST .../Secu/Param/Complete` | 只写目标表 |

### 4.1 对话框结构

| 区块 | 内容 |
|------|------|
| 标题 | 「补全规范证券参数」/「补全采集参数」+ 证券代码与名称 chip；右上角「待核对 N 项」 |
| 副标题 | 目标表名 + 「字段已按快照 / 字典 / 参考同市场自动预填，核对后点写入；已存在的 usc 不会覆盖」 |
| 证券摘要 | 快照只读：证券编号 / 代码 / 名称 / 市场 / 品种类型 / 币种 |
| 已存在警告 | `exists=true` 时 `v-alert type=warning`：该 `usc` 已在目标表存在，本次不写、不做覆盖，「写入」按钮禁用 |
| 风险警告 | 后端返回的 `warnings[]` 逐条展示（字典查不到交易所、缺币种、唯一约束预检冲突等） |
| 写入键 | `usc` 只读输入框 + 「自动」chip；服务端重新推导，前端改了也不采信 |
| 字段区 | **只展示目标表那一组字段**（另一张表完全不出现），每个字段右侧一个小标签标来源 |
| 人工备注 | 选填，服务端会加 `[AUTO-FILL]` 前缀与时间 |

### 4.2 预填来源标注（沿用需求 11147 的规则）

| 标签 | source | 含义 |
|------|--------|------|
| 快照 | `snapshot` | 快照行直接带过来（代码 / 名称 / 市场 / 品种类型 / 币种） |
| 字典 | `dictionary` | 查 `public.finv_exchange` 等字典表得到（`exchange_code`、基准币种、类型映射） |
| 参考 | `reference` | 同市场同品种已配置行的众数（`market_code` / `sub_instrument_type` / `instrument`），**必须核对** |
| 缺省 | `default` | 表默认值或代码缺省，仅供占位（中文名先取英文名、`flag_enable='0'`） |
| 待填 | `manual` | 推不出来，必须人工填写 |

「待核对 N 项」= 来源为 `reference` / `default` / `manual` 的字段数。

### 4.3 派生规则（与原「录入」一致）

| 字段 | 规则 |
|------|------|
| `usc` | 快照 `usc` 优先，为空用 `upper(市场标签.代码)`；服务端派生，不采信请求体 |
| `exchange_code` | `public.finv_exchange.exchange_flag = 市场标签`（SH=11 / SZ=12 / BJ=13 / HK=21 / US=30） |
| `currency_type` | 快照 `currency` 优先，缺失用 `finv_exchange.base_currency` |
| `security_type` | `instrument_type`：3/12→Stock、4→ETF、6→StockIndex、11→Crypto |
| `market_type` | 快照 `market_type` 优先，缺失按市场推 HK=1 / US=2 / A股=4 |
| `market_code` | 同市场同品种已配置行的众数（参考值） |
| `sub_instrument_type` | `instrument_type * 1000 + 2`（3→3002、4→4002） |
| `instrument` | 3→stock、4→etf、6→index、7→sectors、11→crypto |
| `flag_enable` | 默认 `'0'`（停用），核对后再开采集 |

### 4.4 提交与幂等

- 目标表那一组**必填全部有值**才启用「写入」（禁用时提示还缺哪几项）；
- 单事务只写目标表，按主键 `ON CONFLICT (usc) DO NOTHING`：已存在不覆盖，返回 `created=false`，
  前端 toast 提示「已存在，未改动」；
- 非目标表的那一组字段**完全不看**（前端留空，后端只读目标组）；
- 唯一约束冲突（`23505`，老库同代码存在旧口径 usc）会被翻译成可读提示；
- 成功后 toast 报写入结果并刷新当前清单（该行随即从清单消失）。

### 4.5 `remark` 溯源

目标表的 `remark` 由服务端生成：

```
[AUTO-FILL] 待采证券补录 2026-01-15T10:20:30+08:00；备注：<人工备注>
```

排查时 `WHERE remark LIKE '[AUTO-FILL]%'` 即可列出全部由补全程序写入的行。

## 5. 与其它菜单的关系

```
采集写入快照（usc 可能为空）
  → 「待采证券录入」：入队（键表）/ 手动补录USC（回写快照 usc）
  → 「证券参数补全」：按 usc 补 public.finv_security（规范证券参数 Tab）
                    按 usc 补 futu.finv_futu_quote_collect（采集参数 Tab）
  → 该行从对应 Tab 的待补清单消失
```

## 6. 仍未实现

| 项 | 状态 |
|----|------|
| 单只补全（预填 + 写单表 + 已存在保护） | ✅ 已实现 |
| 批量补全（按筛选结果一次补 N 条） | ⏳ 未做（写入涉及人工核对，暂不提供一键全自动） |
| 补全后自动开采集（`flag_enable='1'`） | ⏳ 未做，保持默认停用 |
