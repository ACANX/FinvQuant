# FinvQuant 通用量化回测 — 策略定义规范（BacktestStrategySpec）

> 版本：v1（2026-08-06）
> 适用范围：FinvQuant 通用量化回测引擎（`internal/backtest`），服务端 `POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Save` 保存策略时校验。

## 1. 设计目标

- **通用性**：同一套策略定义模型支持 ETF / 股票 / 场外基金 / 国内期货 / 美股期货 / 黄金、石油等商品期货 / 积存金等任意**已导入分钟行情**（`finv_quote_secu_kline_min`）的证券；
- **结构化**：策略定义以 JSONB 持久化（`finv_quant_backtest_strategy.definition`），可机器求值、可版本化（`definition_version`）、可复现（任务保存策略快照）；
- **可扩展**：指标类型、信号函数、数量模式均为插件式注册，后续可平滑扩展多标的、多周期、机器学习类策略（`strategy_type` 已预留 `MACHINE_LEARNING`）；
- **高度自定义**：指标参数、信号表达式、规则限制（时间点/频率/次数/数量）、风控（止损/止盈/仓位）均可自由组合。

## 2. 定义结构（JSON 模型 v1）

```jsonc
{
  "version": "1",                       // 定义模型版本（结构演进时递增）
  "strategy_type": "RULE_BASED",        // 策略类型：RULE_BASED（当前实现）
  "description": "策略说明（可选）",

  "universe": {                         // 标的池
    "securities": ["GCMain"]            // 证券代码（finv_security.usc / security_code）
  },

  "data": {                             // 数据与撮合配置
    "period": "Min",                    // 回测数据周期：Min / Hour / Day
    "price_field": "close",             // 指标默认取值字段：open/high/low/close/volume/turnover
    "warmup_bars": 30,                  // 预热 bar 数（该区间内不产生交易）
    "fill_mode": "NEXT_BAR_OPEN"        // 成交模式：NEXT_BAR_OPEN（信号收盘确认、次根开盘成交，默认）
                                        //            CURRENT_CLOSE（当前 bar 收盘成交，可能有轻微未来偏差）
  },

  "indicators": [                       // 指标管线（按声明顺序计算，id 全局唯一）
    { "id": "ma_fast", "type": "MA",  "params": { "window": 5,  "field": "close" } },
    { "id": "ma_slow", "type": "MA",  "params": { "window": 20, "field": "close" } },
    { "id": "rsi14",   "type": "RSI", "params": { "window": 14, "field": "close" } },
    { "id": "dif",     "type": "MACD","params": { "fast": 12, "slow": 26, "signal": 9, "field": "close", "output": "dif" } },
    { "id": "boll_up", "type": "BOLL", "params": { "window": 20, "k": 2, "field": "close", "output": "upper" } }
  ],

  "signals": {                          // 买卖信号（布尔表达式，引用指标 id / 字段 / 内置函数）
    "buy":  "cross_up(ma_fast, ma_slow) AND rsi14 < 70",
    "sell": "cross_down(ma_fast, ma_slow) OR rsi14 > 80"
  },

  "rules": {                            // 交易规则（买卖方向各一）
    "buy": {
      "action": "BUY",
      "quantity_type": "ALL_IN",        // ALL_IN 全部可用资金 / ALL 清仓 / FIXED 固定数量 / PERCENT 可用资金百分比 / AMOUNT 固定金额
      "quantity": 0,                    // FIXED/PERCENT/AMOUNT 时生效
      "max_per_day": 0,                 // 每日最大触发次数（0=不限）
      "max_per_run": 0,                 // 整个回测最大触发次数（0=不限）
      "allowed_times": [],              // 限定交易时间点（hhmmss 字符串，空=不限），如 ["093000","140000"]
      "allow": true                     // 规则开关（false=禁用该方向交易）
    },
    "sell": { /* 同上，quantity_type 通常为 ALL */ }
  },

  "risk": {                             // 风控
    "stop_loss_pct": 3,                 // 止损（相对持仓成本，%；0=关闭）
    "take_profit_pct": 0,               // 止盈（相对持仓成本，%；0=关闭）
    "max_position_pct": 100,            // 单标的仓位上限（占净资产 %）
    "max_positions": 1,                 // 最大持仓数（单标的回测为 0/1，多标的预留）
    "max_trades_per_day": 0,            // 每日最大成交笔数（0=不限）
    "min_interval_bars": 0              // 相邻交易最小间隔 bar 数（0=不限）
  },

  "cost": {                             // 成本覆盖（任务级 > 策略级 > 账户级）
    "commission_rate": 0.0003,          // 手续费率（按成交金额比例，单边）
    "slippage_pct": 0.0001              // 滑点（按成交价比例，单边）
  }
}
```

## 3. 指标类型（type）

| 类型 | 参数 | 说明 |
|------|------|------|
| `MA` | window, field | 简单移动平均 |
| `EMA` | window, field | 指数移动平均 |
| `RSI` | window, field | 相对强弱指标（Wilder 平滑） |
| `MACD` | fast, slow, signal, field, output(`dif`/`dea`/`hist`) | 指数平滑异同移动平均 |
| `BOLL` | window, k, field, output(`mid`/`upper`/`lower`) | 布林带 |
| `ATR` | window | 平均真实波幅 |
| `STDDEV` | window, field | 滚动标准差 |
| `HHV` | window, source(默认 high) | 滚动最高值 |
| `LLV` | window, source(默认 low) | 滚动最低值 |

未就绪位置值为 NaN，NaN 参与比较恒为 `false`（自动规避指标预热期误触发）。

## 4. 信号表达式语法

```
expr      := or_expr
or_expr   := and_expr (OR and_expr)*
and_expr  := not_expr (AND not_expr)*
not_expr  := NOT not_expr | comparison
comparison:= additive (== | != | > | >= | < | <=) additive
additive  := multiplicative ((+|-) multiplicative)*
multiplicative := primary ((*|/) primary)*
primary   := NUMBER | IDENT | FUNC(args) | ( expr )
```

- **标识符**：指标 id、字段名（open/high/low/close/volume/turnover），取当前 bar 的值；
- **内置函数**：
  - `cross_up(a, b)`：a 上穿 b（`a[i]>b[i]` 且 `a[i-1]<=b[i-1]`）；
  - `cross_down(a, b)`：a 下穿 b；
  - `ref(id, n)`：n 根 bar 前的值（**n≥0 硬约束，无未来函数**：词法不支持负数字面量，但算术可构造负数（如 `ref(close, 0-5)`）——编译期对常量负偏移直接报错，运行期对动态负偏移返回 NaN，比较恒 false）；
  - `highest(id, n)` / `lowest(id, n)`：最近 n 根（含当前）最大/最小值（n≥1，约束同 ref）；
  - `abs(x)`：绝对值；
- **运算符**：算术 `+ - * /`、比较 `== != > >= < <=`、逻辑 `AND OR NOT`（大小写不敏感）；
- 表达式在**保存策略时编译校验**：语法/函数/参数个数/偏移常量/嵌套深度（≤64，防栈溢出）错误立即返回；
- **标识符交叉校验**：保存时收集信号表达式引用的全部标识符，与 `indicators` 声明 + 字段白名单（open/high/low/close/volume/turnover）比对，未声明标识符（如“指标名写错”）立即报错，不再等到运行期才 FAILED；
- `signals.buy` / `signals.sell` **至少一个非空**（全空 = 无交易逻辑，拒绝保存）。

## 5. 成交与撮合

- 信号在当前 bar **收盘价**确认，默认 **下一根 bar 开盘价**成交（`NEXT_BAR_OPEN`，无未来函数）；
- 成交价含滑点：`price * (1 ± slippage_pct/100)`（买入加价、卖出减价）；
- 手续费：`amount * commission_rate`，买卖双边收取；
- 数量模式（买入 / 卖出方向均生效）：
  - `ALL_IN`：可用资金（扣除手续费）全额买入（买入方向）；
  - `ALL`：清仓（卖出方向）；
  - `FIXED`：固定数量（资金不足时按可用资金折算；卖出侧超过持仓时按持仓折算）；
  - `PERCENT`：可用资金 × 百分比（卖出侧为持仓 × 百分比）；
  - `AMOUNT`：固定金额（卖出侧金额折算为数量，不超过持仓）；
  - 方向语义约束：买入禁 `ALL`、卖出禁 `ALL_IN`（保存校验拒绝）；
- 保证金模式：账户 `margin_mode=FULL`（默认）全额占用现金；`FUTURES`（预留）按 `margin_rate` 占用保证金，支持期货杠杆回测；
- 止损/止盈：基于持仓成本价，当 bar 最低价触及止损价（或最高价触及止盈价）时，以触发价成交（intrabar 限价单语义）。

## 6. 限制条件（交易频率/次数/时间点/数量）

| 维度 | 配置位置 | 说明 |
|------|----------|------|
| 交易时间点 | `rules.*.allowed_times` / 任务级 `options.allowed_times` | 仅允许在指定 hhmmss 触发 |
| 每日次数 | `rules.*.max_per_day` / `risk.max_trades_per_day` | 规则级 / 全局每日笔数 |
| 全回测次数 | `rules.*.max_per_run` | 整个回测内触发上限 |
| 最小间隔 | `risk.min_interval_bars` | 相邻两笔成交最小 bar 间隔 |
| 数量 | `rules.*.quantity_type` + `quantity` | 见上节 |
| 回测开关 | 策略/账户 `allow_backtest` + 任务 `options.enable_backtest` | 三层开关，任一关闭拒绝执行 |

## 7. 回测报告（run.report JSONB）

| 指标 | 字段 | 对应需求 |
|------|------|----------|
| 初始资金 / 期末总资产 | `initial_capital` / `final_equity` | ① |
| 总收益额 / 总收益率 | `total_profit` / `total_return_pct` | ②③⑥ |
| 年化收益率 | `annual_return_pct` | ⑧ |
| 最大回撤（含区间） | `max_drawdown_pct` / `max_drawdown_start_ts` / `max_drawdown_end_ts` | ⑧ |
| 夏普比率 / 年化波动率 | `sharpe_ratio` / `volatility_pct` | ⑧ |
| 最大投入金额 | `max_invested` | ④ |
| 平均投入金额（持仓期时间加权） | `avg_invested` + `invested_days` | ⑤ |
| 交易统计 | `trade_count` / `buy_count` / `sell_count` / `win_count` / `loss_count` / `win_rate_pct` / `profit_factor` / `total_fee` | ⑧ |
| 单期收益分布 | `best_day_pct` / `worst_day_pct` / `profit_days` / `loss_days` | ⑧ |
| 信号归因 | `trade_signal_detail`（信号名→笔数） | ⑧ |

**曲线数据**（`finv_quant_backtest_equity`，按报告精度 Day/Hour/Min 逐点落库）：
- ① 账户余额曲线：`equity`（总资产=现金+持仓市值，持仓换算现金）与 `cash`；
- ② 投资收益率曲线：`roi`；
- ③ 投资收益额曲线：`profit`；
- ⑦ 持仓金额曲线：`position_value`（另含 `position_qty`）；
- ⑧ 回撤曲线：`drawdown`。

**成交记录**（`finv_quant_backtest_trade`）：时间/方向/价格/数量/金额/手续费/平仓盈亏/成交后持仓与现金/触发信号，支持任意时点账户状态回放。

## 8. 持久化与回看

- 策略/账户/任务/曲线/成交/链路追踪全部落 PostgreSQL（`finv_quant_` 前缀表，见迁移 V22~V27）；
- 任务保存**策略与账户快照**：策略/账户后续修改不影响历史任务结果（可复现）；
- 前端「回测分析」按 runId 拉取报告与曲线，支持持久化保存与随时回看。

## 8.5 链路追踪（需求⑨）

持仓变动详细情况链路追踪分析，三个维度（对应迁移 V27 三张表）：

**① 资金流水明细**（`finv_quant_backtest_cashflow`）：
- 类型：`INITIAL_DEPOSIT`（初始资金注入）/ `BUY_PAY`（买入付款）/ `SELL_RECEIVE`（卖出收款）/ `FEE`（手续费）/ `MARGIN_HOLD`（保证金占用）/ `MARGIN_RELEASE`（保证金释放）；
- 记录每次资金变动的金额、变动前后现金余额（流水连续可校验：下一条 cash_before = 上一条 cash_after），关联成交记录。

**② 持仓变化明细**（`finv_quant_backtest_position_log`）：
- 动作：`OPEN`（开仓）/ `ADD`（加仓）/ `REDUCE`（减仓）/ `CLOSE`（平仓）；
- 记录变动前后持仓数量与加权成本（avg_cost_before/after），关联成交记录与触发信号。

**③ 交易事件追踪**（`finv_quant_backtest_event_trace`，覆盖 FR-10 八项登记信息）：
- ① 触发原因（`trigger_reason`）：买入信号 / 卖出信号 / 止损 / 止盈；② 触发时间（`trigger_ts/date/time`）；
- ③ 事件结果（`exec_status`）：`PENDING`（挂单）/ `FILLED`（成交）/ `REJECTED`（拒绝）/ `EXPIRED`（过期，回测结束未成交）；④ 结束时间（`exec_ts/date/time`）；
- ⑤ 委托下单时间（`order_ts/date/time`，信号确认后立即下单，= 触发时点）；
- ⑥ 委托耗时：触发 → 成交的 bar 数（latency_bars）与秒数（latency_sec）；⑦ 事件存活时间（`alive_sec` = 结束时间 − 触发时间）；
- ⑧ 未成交原因（`reject_reason`）：资金不足 / 超过规则每日最大触发次数 / 超过规则回测总触发次数 / 超过每日最大成交笔数 / 未满足最小交易间隔 / 不在允许交易时间点内 / 不在环境交易时段内 / T+N 交收限制 / 涨停无法买入 / 跌停无法卖出 / 已达最大持仓 / 无持仓可卖 / 回测结束委托未执行；
- 报告 `event_stats` 汇总：触发/成交/拒绝/过期计数、平均委托耗时（仅统计信号驱动订单，止损/止盈 intrabar 成交 latency=0 不计入，避免拉低均值）、未成交原因分布、触发原因分布。

## 9. 扩展路线

- [ ] 多标的组合回测（universe 多证券、组合净值，模型已预留）
- [ ] 机器学习策略类型（`MACHINE_LEARNING`）
- [ ] 期货保证金模式实盘化（`FUTURES`）
- [ ] 更丰富指标与信号函数（自定义函数注册表）
- [ ] 参数寻优 / 批量回测

## 10. 环境与模板（自适应）

### 10.1 环境模型（`finv_quant_environment`）

回测 / 模拟盘 / 仿真 / 实盘交易环境的配置差异、不同市场（COMEX 黄金 vs 沪深 ETF 等）的交易约束与交易规则差异、不同地区的习惯偏好差异，统一建模为环境：

| 字段 | 说明 |
|------|------|
| `env_type` | BACKTEST 回测 / PAPER 模拟盘 / SIMULATION 仿真 / LIVE 实盘 |
| `region` / `market_code` | 地区与市场（CN/US/HK...） |
| `config.trading_sessions` | 交易时段（hhmmss 数组，如 COMEX `082000-133000`、沪深 `093000-113000 + 130000-150000`），非时段信号 → 拒绝事件「不在环境交易时段内」 |
| `config.trading_rules` | T+N 交收、涨跌停幅度、合约乘数、`tick_size` 最小变动单位（成交价自动对齐）——**全部在引擎中生效**：T+N 当日买入持仓不可卖出（卖出信号/止损止盈均拒绝并登记事件）；涨跌停基于前一交易日收盘价，买入触及涨停/卖出触及跌停拒绝成交；合约乘数用于成交金额/持仓市值/数量折算（股票 ETF=1，期货如 COMEX 黄金=100）；tick_size 成交价自动对齐 |
| `config.cost` | 成本基准（手续费/滑点） |
| `config.fill_mode` | 撮合模式（NEXT_BAR_OPEN 默认 / CURRENT_CLOSE 当前 bar 收盘立即成交），覆盖策略 data.fill_mode |
| `config.currency` | 计价币种（创建任务时与环境币种不一致的账户将被拒绝启动） |
| `config.preferences` | 地区习惯偏好（自定义扩展，如日期格式、涨跌配色） |

**成本覆盖链**：环境 > 任务（options）> 策略（definition.cost）> 账户。

### 10.2 模板模型（`finv_quant_template`）

策略模板 / 账户模板 / 环境模板三类（`template_type`），相同部分（环境、约束、规则、限制、策略）复用，差异部分自定义：

- 内置模板（`is_builtin=1`，`user_id='system'` 全局可见，**禁止修改/删除**）：双均线/RSI/布林带/MACD 四个策略模板 + COMEX 黄金/沪深 ETF 两个环境模板（种子 V100020）；
- 自定义模板：用户按需保存（服务端强制 `is_builtin='0'`，防伪造内置标识），策略创建时可选关联 `template_id`；
- 环境模板内容可直接复制为环境（env_code/env_name/config）。

### 10.3 动态切换与自适应

- 回测任务创建时指定 `env_id`（缺省取账户默认环境 → 系统默认回测环境），任务保存**环境快照**（API 响应字段 `environment_snapshot`，DB 列 `env_snapshot`）保证可复现；
- 引擎自适应：交易时段过滤（含跨午夜时段，如夜盘 210000-023000）、tick_size 价格对齐、T+N/涨跌停/合约乘数/撮合模式/币种、成本覆盖链；
- 前端「环境管理」维护环境与模板，黄金期货回测验证页支持环境下拉切换。

## 11. 多用户 / 多子账户 / 组合回测预留

- **多用户**：策略/账户/任务均带 `user_id`（默认 `default`，内置环境/模板为 `system` 全局可见）；**列表查询按用户隔离、Get/Toggle/Delete/创建任务均做归属校验**（他人数据不可读、不可改、不可删）；接入 JWT/RBAC 后与登录态绑定；
- **单用户多子账户**：账户支持 `group_id` 分组（主账户 = 分组根，子账户通过 group_id 关联），一个用户可拥有多个主/子账户；
- **组合回测预留**：当前阶段单标的回放；策略定义 `universe.securities` 已支持多标的声明，组合净值与多标的撮合在后续版本实现。

## 12. API 路径规范

- 后端：`/API/V1/Meta/Finv/Quant/Backtest/**`（策略/账户/任务/报告/曲线/成交/链路追踪/环境/模板）；
- 前端：菜单与路由统一加 `Meta/Finv/Quant/` 前缀（如 `/meta/finvquant/backtest/gold-futures`）。

## 13. 部署注记

- **迁移 V22~V28 + 种子 V100019/V100020 需在全新库一次性应用**（本 PR 未发布过中间态：V22~V26 在 PR 内完成改名与加列、V27/V28 为新增表，自研迁移 runner 无 checksum 校验，若部署过中间态需重建或人工核对）；
- 服务启动时自动把历史遗留的 RUNNING/PENDING 任务标记为 FAILED（重启恢复机制，避免悬挂）。
