# 待采证券录入（PendingSecu）

> 菜单 key：`quote-realtime-pending-secu`
> 路由：`/Meta/Finv/Quote/Realtime/PendingSecu`
> 所属：导航「行情 → 实时行情 → 待采证券录入」（需求 11160 由「待录入证券」改名）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/PendingSecuView.vue`
> 关联需求：10995（本页；一键录入为规划功能，尚未实现）、11145（按派生键先补 usc 再判断）、
> 11146（「手动补录USC」把 usc 真正写回快照表）、11153–11156（「入队」/「更新键」）、
> 11161（原「录入」按钮迁出到独立菜单「证券参数补全」）
> 关联接口：[`API/Cdeu/RealtimeStockPendingPage.md`](../../../API/Cdeu/RealtimeStockPendingPage.md)

## 1. 用途

列出 `cdeu.finv_cdeu_secu_realtime_stock` 中 **补全后 `usc` 仍为空** 的快照行。

**本页负责把 `usc` 弄进快照表**（「入队」登记键 /「手动补录USC」回写）：
`public.finv_security` 与 `futu.finv_futu_quote_collect` 的补全已拆到独立菜单
[证券参数补全](SecuParamComplete.md)（需求 11161），本页不再写任何主数据表。

补全口径（需求 11145）：先用派生键 `ftmm_symbol = upper(市场标签 || '.' || 代码)`（英文字母统一大写，
与 `futu.finv_futu_secu_key.ftmm_symbol` 同口径）左连本地键表取 `usc`；
`COALESCE(快照 usc, 键表 usc)` 仍为空才列出来 —— **键表里已登记的证券不算待录入**，不再出现在本页。

采集会把所有在售证券都写入快照表，`usc` 为空表示该证券**尚未录入证券主数据**
（新上市 / 未纳入采集范围）。这份清单是后续补录三张表的依据：

| 表 | 职责 | 补齐入口 |
|----|------|----------|
| `public.finv_security` | 证券主数据（`currency_type`、`exchange_code`） | [证券参数补全](SecuParamComplete.md)「规范证券参数」Tab |
| `futu.finv_futu_quote_collect` | futu 采集配置（`stock_id ↔ usc`） | [证券参数补全](SecuParamComplete.md)「采集参数」Tab |
| `cdeu.finv_quote_cdeu_usc` | usc 主数据（`market` / `currency` / `timezone`） | 不录（需求 11147：运行时动态生成） |

> `usc` 为空是**刻意保留的观测信号**：禁止用 `stock_code` 回填。
> 两者并非等价，直接挪用会把不同市场的同码证券映射错（如 `000001` 在 SZ 与 SH 都存在）。

## 2. 页面结构

### 2.1 说明区

顶部 info 提示解释「先按派生键补 usc、补完仍为空才算待录入」的口径，以及 `usc` 为空的业务含义与补录三张表的顺序。

### 2.2 筛选与操作（单行 flex，窄屏自动换行）

| 控件 | 说明 |
|------|------|
| 关键字 | 匹配 `stock_code` / `name` / **派生 `ftmm_symbol`**（可直接搜 `HK.06288`） |
| 市场 | `HK` / `SH` / `SZ` / `BJ` / `US`，默认全部 |
| 排序字段 | 总市值（默认）/ 成交额 / 涨跌幅 / 来源页码 / 更新时间 / 证券编号 |
| 方向 | 降序（默认）/ 升序 |
| 查询 / 刷新 / 重置 | 查询重置到第 1 页；刷新保持当前页 |
| **手动补录USC**（查询右侧） | 见 §2.5 |

### 2.3 列表列

**`usc`**（快照真实值；本页按定义都是空，空值显示 `—` 并在悬浮说明里指向「唯一标识」列）、
证券编号、代码、名称、市场、**唯一标识**（= `upper(市场标签.代码)`，与 `futu.finv_futu_secu_key.ftmm_symbol` 同口径，
即补录后将写入的 usc；表头悬浮说明给出推导公式）、**品种类型**（走字典显示中文，见下表）、币种、
现价、涨跌额、涨跌幅、总市值、成交额、来源页码、快照时间、**操作**。

品种类型字典（口径取自 `futu.finv_futu_quote_collect.instrument` 实际取值）：

| instrument_type | 页面显示 |
|----------------:|----------|
| 3 / 12 | 股票 |
| 4 | ETF |
| 6 | 指数 |
| 7 | 板块 |
| 9 | 期货 |
| 11 | 加密货币 |

未登记的值原样显示，空值显示 `—`（DictColumnSpec：表格列只显示中文，不拼 `中文[值]`）。

「操作」列只有 <strong>入队</strong> 一个按钮：

| 按钮 | 作用 | 是否写库 |
|------|------|----------|
| 入队（见 §2.6） | 把证券键登记进 `futu.finv_futu_secu_key` | 点按钮**不写**，对话框里确认后才写 |

> 需求 11161：原「录入」按钮（写两张主数据表 + 回写快照 usc）已整体迁到独立菜单
> [证券参数补全](SecuParamComplete.md)，按目标表分两个 Tab 分别补全；本页不再出现该按钮。

涨跌用整行颜色表达（红涨绿跌），与「证券行情排行」同一套配色；无数值的方向行不着色。

### 2.4 分页

底部左侧显示「共 N 条，第 x / y 页」，右侧为每页条数（20/50/100/200）与页码控件。

### 2.5 手动补录 USC（需求 11146）

「查询」按钮右侧的「手动补录USC」，调用
`POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill`，把 `usc` 为空（NULL / 空白）的快照行**真正写回**
`cdeu.finv_cdeu_secu_realtime_stock`：

| 步骤 | 说明 |
|------|------|
| 1 | 派生键 `ftmm_symbol = upper(市场标签 \|\| '.' \|\| 代码)` |
| 2 | `JOIN futu.finv_futu_secu_key` 取 `usc` |
| 3 | 只写「快照为空且键表非空」的行；键表里没有的保持不动 |

- 与列表口径完全一致（同一个派生键表达式），补录后清单只会变短，不会反向变长；
- **幂等**：只更新空值行，重复点击「本次补录」为 0，可安全反复点；
- 结果用全局 toast 提示「本次补录 N 行 / 补录前为空 M 行 / 仍无法补全 K 行 / 耗时」，随后自动刷新列表；
- `K > 0` 说明这些证券在 `futu.finv_futu_secu_key` 里也没有，需要走录入流程（需求 10996，尚未实现）。

### 2.6 入队 / 更新键（需求 11153 – 11156）

「入队」把一只待录入证券的**键**登记进本地键表 `futu.finv_futu_secu_key`，让采集与 USC 映射链路有据可依。
**三步走，必须人工确认**，且**每次提交前都要先探测**该键是否存在：

| 步 | 界面动作 | 接口 | 写库 |
|:--:|----------|------|:----:|
| ① | 点行内「入队」→ 打开「入队 / 更新键」对话框（自动探测一次） | `POST .../Enqueue/Probe` | 否 |
| ② | 核对 / 修改键字段；改了 `market` / `code` 必须再点「探测」 | `POST .../Enqueue/Probe` | 否 |
| ③ | 键不存在 → 「确认入队」；键已存在 → 「更新键」 | `POST .../Enqueue` / `.../Enqueue/Update` | 是 |

#### 按钮状态规则（需求 11155）

| 情况 | 主按钮 | 提示 |
|------|--------|------|
| 未探测 / 探测中 | 禁用 | 先点「探测」确认该键是否存在 |
| 键不存在 | 「确认入队」可点击 | 可以写入 |
| 键已存在 | 「更新键」可点击（另有「去『USC 映射规则』」） | 只能改非主键字段 |
| `market` / `code` 改动后 | 禁用 | 主键变了，上次探测失效，必须重新探测 |

服务端写入前还会**再验证一次**存在性（入队前查在 + `ON CONFLICT DO NOTHING`；更新命中 0 行即视为不存在），
所以前端状态过期也不会写错。

#### 可写字段（需求 11156）

| 字段 | 可改 | 规则 |
|------|:----:|------|
| `ftmm_symbol` | ❌ | 主键 = `upper(market + '.' + code)`，派生只读 |
| `market` | ✅ | 非空，自动大写 |
| `code` | ✅ | 非空，原样保留 |
| `stock_id` | ✅ | 正整数，默认快照证券编号 |
| `instrument` | ✅ | 词表与键表一致：`stock` / `etf` / `index` / `sector` / `futures` / `crypto` / `forex` / `market` |
| `status` | ✅ | `PENDING` / `QUEUED` / `DONE` / `FAILED` / `SKIP`，默认 `PENDING` |
| `flag_enable` | ✅ | `0` / `1`，默认 `1` |
| `priority` | ✅ | `1..1000000`，默认 `10` |
| `usc` | ❌ | 只读（入队后由「USC 映射规则」解析） |

对话框里「键表现有记录」用「字段 / 现值 / 说明」三列逐个展示，不合并。

#### 入队之后的链路

```
入队（键表出现 HK.XXXXX，usc 为空）
  → 「USC 映射规则」预演 → 应用（规则引擎解析 usc）
  → 回本页点「手动补录USC」（usc 落快照）
  → 该行离开待录入清单
```

## 3. 主数据补全已迁出（需求 11161）

原「操作」列里的「录入」按钮会一次性写 `public.finv_security` + `futu.finv_futu_quote_collect`
并回写快照 `usc`。实际使用中两张表的缺口并不同步（可能只缺一张），一把梭看不出到底补了哪张。

需求 11161 起该能力迁到独立菜单 **[证券参数补全](SecuParamComplete.md)**：

| Tab | 目标表 | 接口 |
|-----|--------|------|
| 规范证券参数 | `public.finv_security` | `GET .../Secu/Param/Complete/Pending?target=security`、`.../Prefill`、`POST .../Secu/Param/Complete` |
| 采集参数 | `futu.finv_futu_quote_collect` | 同上，`target=collect` |

字段预填规则（快照 / 字典 / 参考同市场 / 缺省 / 待填 五种来源）与派生规则原样保留，
详见新菜单文档。后端整体录入接口 `POST .../Stock/Pending/Enroll`（写两张表）仍保留，
供脚本或批量场景调用，页面入口已不再使用。

## 4. 仍未实现

| 项 | 状态 |
|----|------|
| 单只补全（预填 + 写单表） | ✅ 已实现（在「证券参数补全」菜单） |
| 批量录入（列表顶部按钮） | ⏳ 规划中，按钮保持 disabled |
| 录入后自动开采集（`flag_enable='1'`）并验证 | ⏳ 未做，保持默认停用 |
