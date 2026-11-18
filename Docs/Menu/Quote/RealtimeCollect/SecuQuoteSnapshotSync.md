# 行情快照同步（SecuQuoteSnapshotSync）

> 菜单 key：`quote-realtime-snapshot-sync`（需求 11281 起保留原 key，仅换分组，未改名）
> 路由：`/Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSync`（需求 11255 由 `FTStockQuoteSnapshotSync` 改名；需求 11281 随菜单迁入「行情采集」分组，由 `Quote/Realtime/` 改为 `Quote/RealtimeCollect/`，旧路径保留重定向）
> 所属：导航「行情 → 行情采集 → 行情快照同步」（需求 11158 由「行情快照手动同步」改名；需求 11281 由「实时行情」分组迁入新建的「行情采集」分组）
> 视图：`Web/src/views/Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSyncView.vue`
> 关联需求：10982（本页）、10976（配置表）、10977（同步服务）、11255（路由改名）、11256（占位 Tab）、11273/11274（期货 Tab 实装）、11294（沪深京指数 Tab 实装）
> 关联接口：[`API/Cdeu/StockSnapshotSyncRun.md`](../../../API/Cdeu/StockSnapshotSyncRun.md)（异步启动 + 轮询）
> 关联表：`cdeu.finv_cdeu_secu_realtime_sync_config`（股票等品种配置）、`cdeu.finv_quote_secu_realtime`（股票结果）、
> `cdeu.finv_quote_cdeu_future_sync_config`（期货分类配置）、`cdeu.finv_quote_secu_realtime_future`（期货结果）

## 1. 用途

手动触发「证券列表采集」并按市场分页拉取 moomoo `get-stock-list`，把结果写入
`cdeu.finv_quote_secu_realtime`，同时查看每一步的状态与逐页日志。

## 2. 页面结构

### 2.0 证券品种 Tab（需求 10986）

页面顶部按**证券品种**分 Tab，Tab 列表 = 配置表的 `instrument_symbol` 去重（当前 `stock`）+ 占位品种（需求 11256，固定顺序补齐）：

| 品种符号 | Tab 名 | 来源 |
|---|---|---|
| `stock` | 股票 | 配置表（当前唯一有配置行的品种） |
| `cnetf` | 沪深京ETF | BDFS 指数采集（需求 11263，固定来源，无需配置行；符号 `cnetf`，需求 11265） |
| `index` | 指数 | **实装（需求 11333）**：收藏的美股/港股指数（`quote.finv_quote_secu_fav` + moomoo OpenAPI snapshot），固定来源，结果写 `cdeu.finv_quote_secu_realtime`（`provider=MMAPI`、`instrument_type=6`）；配置表若有启用的 `index` 行则优先走列表采集 |
| `future` | 期货 | **实装（需求 11274）**：配置表 `cdeu.finv_quote_cdeu_future_sync_config`，结果 `cdeu.finv_quote_secu_realtime_future` |
| `crypto` | 加密货币 | 占位 |
| `bsestock` | 北交所股票 | BDFS 板块采集（需求 11262，固定来源，无需配置行） |
| `etf` | ETF | **实装（需求 11279）**：moomoo `get-etf-list`（HK `plateId=900075` / US `plateId=10002799`），结果写 `cdeu.finv_quote_secu_realtime`；与 `cnetf` 刻意分开 |
| `cnindex` | 沪深京指数 | **实装（需求 11294）**：中信 XetApp `GetQuoteSnapshotCNAIndex`（上游 4 池 + 深市 1200 + 北交所 1400 合并去重），固定来源、无需配置行；结果写 `cdeu.finv_quote_secu_realtime`（`provider=ZXET`、`instrument_type=6`） |
| `otc` | 中国OTC | 占位（需求 11266） |
| `sector` | 板块 | 占位 |
| `portfolio` | 投资组合 | 占位 |
| `gold` | 黄金 | 占位 |
| `forex` | 外汇 | 配置表（插行后出现） |
| `option` | 期权 | 配置表（插行后出现） |

**占位 Tab（需求 11256 / 11266）**：`sector` / `portfolio` / `otc` / `gold`
按上表顺序**跟在配置表已有品种之后常显**（`future` 已于需求 11274、`etf` 已于需求 11279 实装，均移出占位清单），由前端 `PLACEHOLDER_SYMBOLS` 硬编码（不依赖配置行）：
配置表暂无该品种的行时，Tab 照常可点，配置表显示「该品种为占位 Tab：配置表暂无该品种的采集行，暂不参与采集」，
「开始同步」按钮禁用；后续接入时只要往 `cdeu.finv_cdeu_secu_realtime_sync_config` 插入同符号的配置行，
即自动转为可采集（无需改前端）。

**固定来源 Tab（需求 11262）**：`bsestock`（北交所股票）已接入 BDFS 采集，由前端 `FIXED_SYMBOLS` 常显，
不读配置表、也不需要配置行。Tab 顺序统一由前端 `SYMBOL_ORDER` 决定
（`stock` / `index` / `etf` / `future` / `crypto` / `bsestock` / **`cnindex`** / **`cnetf`** / `sector` / `portfolio` / **`otc`** / `gold`），
`etf`（占位「ETF」）在「指数」之后，`bsestock` 在「期货 / 加密货币」之后，
`cnindex`（沪深京指数，XetApp 源，需求 11294）与 `cnetf`（沪深京ETF，BDFS 源）紧随其后——「开始同步」直接可用，配置表区显示
「该品种为 BDFS 板块采集（板块 GN2309 北交所上市企业）…结果写入 `cdeu.finv_quote_secu_realtime`（provider=BDFS）」。

#### 2.0.1 BDFS 板块采集（`bsestock`，需求 11262）

| 项 | 内容 |
|---|---|
| 数据源 | 百度股市通 FinScope `GET /sapi/v1/constituents`（经 MetaExt `component/quote/baidu/finscope` 的 `GetAllConstituents`） |
| 板块 | `block` / `GN2309`（北交所上市企业），`market=ab` |
| 翻页 | 按**偏移量** `pn += rn` 递增（`rn` = 页面「每页条数」，默认 500），返回条数 < `rn` 即末页；另有 200 页上限防死循环 |
| 落库表 | `cdeu.finv_quote_secu_realtime`（`V300209`，主键 `usc`，`provider` 固定 `BDFS`） |
| usc 解析 | 按 `(exchange, code)` 关联 `public.finv_exchange.exchange_flag` + `public.finv_security.security_code`；解析不到的行跳过，日志先给汇总，再**逐条打印 `exchange` / `code` / `name`**（最多 200 条）便于登记入库 |
| 写入语义 | 按 `usc` 整行覆盖（`ON CONFLICT (usc) DO UPDATE`），`stock_id` 恒为 NULL（BDFS 不提供源系统 ID） |
| 节流 | 无页间间隔（单次全量可在数百毫秒内完成）；「页间隔 / 重试 / 退避」参数对该 Tab 不生效 |

**字段映射（BDFS 成分股 → `cdeu.finv_quote_secu_realtime`）**：

| 目标列 | 来源 | 口径 |
|---|---|---|
| `usc` | `(exchange, code)` → `public.finv_security.usc` | 主键 |
| `stock_code` | `code` | 原样 |
| `name` | `name` | 原样 |
| `market_type` | 固定 `4` | CN A 股市场类型编码（与参考表口径一致：1=HK、2=US、4=CN） |
| `market_label` | 交易所标志 | `BJ`（北交所板块）、`SH` / `SZ`（复用其它板块时） |
| `instrument_type` | 固定按来源 | `bsestock` → `3`（股票）；`cnetf` → `4`（ETF）；富途品种类型键值，见 2.0.2 |
| `currency` | 固定 `CNY` | A 股 |
| `price_nominal` | `rawData.lastPx` | 元 |
| `change_price` | `rawData.pxChange` | 元，可负 |
| `change_ratio` | `rawData.pxChangeRate / 100` | 表内口径为**小数比例**（`+0.83%` → `0.0083`） |
| `trade_volumn` | `rawData.volume × 100` | 接口单位为**手**，表内按**股** |
| `trade_trunover` | `rawData.amount` | 元 |
| `market_value` | `rawData.marketValue` | 元 |
| `price_direct` | `pxChange` 符号 | `+` 涨 / `-` 跌 / `=` 平（**持平为 `=`，不是 `0`**） |
| `page` | 本次采集页码 | 从 1 起（`行序号 / 每页条数 + 1`） |
| `dt_update` | 采集时刻 | Unix 毫秒 |
| `provider` | 固定 `BDFS` | — |
| `remark` | 固定文本 | `来源 BDFS 板块 GN2309（BDFS）` |
| 其余列 | 无来源 | 写 NULL（`hasQuote=0` 时行情列同样写 NULL，不写 0） |

#### 2.0.2 BDFS ETF 采集（`cnetf`，需求 11263 / 11265）

| 项 | 内容 |
|---|---|
| 数据源 | 同 2.0.1：百度股市通 FinScope `GET /sapi/v1/constituents`（MetaExt `finscope`） |
| 来源 | `financeType=index` / `code=399380`（国证ETF，实测返回 1687 条沪深 ETF，`SH` 922 + `SZ` 765） |
| 落库表 | `cdeu.finv_quote_secu_realtime`，`provider` 固定 `BDFS`，`instrument_type` 固定 `4`（ETF 键值，V300220 起） |
| 市场列 | `market_type` 固定 `4`（CN A 股）；`market_label` 按每条记录的交易所写 `SH` / `SZ` / `BJ` |
| 其余字段 | 与 2.0.1 映射表完全一致（`price_nominal` / `change_ratio` / `trade_volumn` 等） |
| Tab 符号 | `cnetf`（与占位 Tab 的 `etf` 区分） |
| Tab 顺序 | 位于「沪深京指数」之后（`SYMBOL_ORDER`），市场下拉固定「CN A 股（沪深京）」不可改，每页默认 500 |

ETF 代码同样按 `(exchange, code)` 解析 `public.finv_security.usc`；实测 1687 条命中 1503 条，
未命中 184 条（`SH` 111 / `SZ` 73）会在日志逐条打印，登记主数据后重跑即可补全。

扩展方式：往 `cdeu.finv_cdeu_secu_realtime_sync_config` 插一行对应 `instrument_symbol` 的配置，
**无需改前端**——Tab 自动出现，该 Tab 的市场下拉也由该品种的配置行生成。

沪深京指数（`cnindex`）无「市场」下拉（固定 CN A 股），无「每页条数」语义（6 次请求一次取全量），
「页间隔 / 重试 / 退避」对该 Tab 不生效；单次超时至少 120 秒（6 次串行请求）。

切换 Tab 时：清空「市场 / 计划数量覆盖」筛选值，其余节奏参数保留；结果表、汇总与日志按当前品种过滤
（日志按 `market` 字段前缀 `品种/市场` 匹配，任务级日志始终显示）。

#### 2.0.3 ETF 列表采集（`etf`，需求 11279）

| 项 | 内容 |
|---|---|
| 数据源 | moomoo `GET /quote-api/quote-v2/get-etf-list`（经 MetaExt `quoteapiv2.GetEtfList`） |
| 采集范围 | 配置表 `cdeu.finv_cdeu_secu_realtime_sync_config` 的 `instrument_symbol='etf'` 行（`V300218` seed：`1`=HK、`2`=US），`count_plan` 空=全量 |
| 板块映射 | 市场由 `plateId` 决定：`1`→`900075`（港股 ETF）、`2`→`10002799`（美股 ETF）；未登记市场直接报错不发请求（`etfPlateIDByMarket`）；请求体 `plate_id` 可覆盖 |
| 参数 | `routeType=1`、`rankType=6`（AUM 降序）、`leverage` 空串也参与签名、`page` 0-based、`pageSize` 默认 50 |
| 落库表 | `cdeu.finv_quote_secu_realtime`（主键 `usc`，`provider=FTMM`，`instrument_type` 由接口给出＝`4`） |
| 与 `cnetf` 的区别 | `cnetf` 是 BDFS 板块来源的沪深京 ETF，走 BDFS 分支；本 Tab 是 moomoo 的港美 ETF 列表 |

#### 2.0.4 沪深京指数采集（`cnindex`，需求 11294）

| 项 | 内容 |
|---|---|
| 数据源 | 中信 XetApp `POST /reqxml`（经 MetaExt `component/quote/citics/xetapp` 的 `GetQuoteSnapshotCNAIndex`） |
| 池 | 上游 4 池（中证 1100 / 国证 / 股转 / 大盘）+ **深市 `1200`** + **北交所 `1400`**，共 6 次请求、按内部码去重；实测去重后 1239 只（上游文档 `IndexCodeMapping.md`） |
| 落库表 | `cdeu.finv_quote_secu_realtime`（`V300209`，主键 `usc`，`provider` 固定 `ZXET`，`instrument_type` 固定 `6`） |
| 代码映射 | 接口返回的是**站点内部码**（`1A0001`/`2A01`/`H50001`…），按上游映射表（`xet_index_code_mapping.tsv`，1239 行，go:embed）还原为交易所/官方代码与编码域 |
| usc 构造 | 按编码域直接构造（不查 `finv_security`）：`SH`+4 位（`000001`→`SH0001`）、`BSE50`（899050）、其余官方码原样（`399001` / `899001` / `931743` / `980028` / `CES300` / `716567`） |
| market_label | 按站点市场号：`1100`→`SH`、`1200`→`SZ`、`1400`→`BJ`、其它（`7100` 国证非标）→`CN` |
| 写入语义 | 按 `usc` upsert（`ON CONFLICT (usc) DO UPDATE`，有值才覆盖）；`page` 恒为 NULL（指数无分页），`stock_id` 恒为 NULL |
| 日志 | 池级统计 + 未知接口码（映射表无此码）逐条打印（上限 50） |

**字段映射（XetApp 指数 → `cdeu.finv_quote_secu_realtime`）**：

| 目标列 | 来源 | 口径 |
|---|---|---|
| `usc` | 映射表 + `XetIndexUSC` | 按编码域直接构造（见上），不查 `finv_security` |
| `stock_code` | 映射表「交易所/官方代码」 | 不含市场前缀（`000001` / `399001` / `931743`） |
| `name` | `Row.Name` | 已剥离序号前缀 |
| `market_type` | 固定 `4` | CN A 股市场类型编码 |
| `market_label` | 映射表「市场号」 | `SH` / `SZ` / `BJ` / `CN` |
| `instrument_type` | 固定 `6` | 富途口径 6=指数（需求 11301 由 `4` 纠正，与 moomoo ETF 的 `4` 区分） |
| `currency` | 名称后缀 | 含 `HKD`→`HKD`、含 `USD`→`USD`，其余 `CNY` |
| `price_nominal` | `Row.Last` | 点 |
| `change_price` | `Row.ChangePrice` | 点，可负 |
| `change_ratio` | `Row.ChangeRatio / 100` | 表内**小数比例** |
| `price_amplitude` | `Row.Amplitude / 100` | 表内小数比例 |
| `buysell_ratio` | `Row.BidAskRatio / 100` | 表内小数比例 |
| `volumn_ratio` | `Row.VolumeRatio` | 量比 |
| `trade_volumn` | `Row.Volume × 100` | 手 → 股 |
| `trade_trunover` | `Row.Amount` | 元 |
| `market_value` | `Row.MarketCap` | **成分合计** |
| `total_shares` | `Row.TotalShares` | **成分合计** |
| `circulation_total_shares` | `Row.FloatShares` | **成分合计** |
| `circulation_market_value` | `Row.FloatMarketCap` | **成分合计** |
| `price_direct` | `ChangePrice` 符号 | `+` / `-` / `=`（持平） |
| `page` | 无 | NULL |
| `dt_update` | 采集时刻 | Unix 毫秒 |
| `provider` | 固定 `ZXET` | — |
| `remark` | 固定文本 | 含接口码、编码域、成分市场、市场号，并注明「市值/股本为成分合计」 |
| 其余列 | 无来源 | 写 NULL（无行情时行情列同样写 NULL） |

#### 2.0.5 期货列表采集（`future`，需求 11274）

| 项 | 内容 |
|---|---|
| 数据源 | moomoo `GET /quote-api/quote-v2/get-futures-list`（经 MetaExt `quoteapiv2.GetFuturesList`） |
| 采集范围 | 配置表 `cdeu.finv_quote_cdeu_future_sync_config` 的启用行，一行=一个「市场 + 分类」；种子 `futures_type='1'..'8'`、`market_type='0'`（聚合口径） |
| 参数 | 每页默认 **50**、页码 0-based、页间 2s、重试 2 次；请求必带 `_` 毫秒时间戳（签名输入） |
| 落库表 | `cdeu.finv_quote_secu_realtime_future`（`V300216`，**主键单列 `stock_id`**） |
| 字段口径 | 只把已验证的位置化字段提列：`1`=合约代码、`2`=名称、`3`=价格、`4`=涨跌幅（去 `%` 按百分点）、`19`=市场标签；其余进 `fields_json`，`field_list_json` 留档；首页日志打一条字段字典采样 |
| usc 解析 | `stock_id` → `futu.finv_futu_quote_collect.usc`，再 `upper(市场标签.合约代码)` → `futu.finv_futu_secu_key.usc`；都不命中写 NULL |
| 下架标记 | 整轮「全部启用分类都成功」才把 `dt_last_seen < 本轮开始时刻` 的行置 `flag_online='0'`，单分类/部分失败跳过 |
| 页面 | 配置表区显示 8 行分类（分类 / 分类名称（空显示「待确认」）/ 市场 / 计划 / 分类合约总数 / 本轮可采集）；参数区「期货分类」下拉（首项「全部期货分类」）；市场下拉隐藏；结果表首列按分类显示 |

#### 2.0.6 收藏加密货币采集（`crypto`，需求 11316）

| 项 | 内容 |
|---|---|
| 数据源 | `quote.finv_quote_secu_fav`（收藏表）中属于加密货币的 `usc` → moomoo OpenAPI `POST /api/v1.0/quote/snapshot`（`CC.<usc>`，一次最多 400 个） |
| 落库表 | `cdeu.finv_quote_secu_realtime_crypto`（主键 `usc`，`provider` 固定 `MMAPI`） |
| 触发 | 本 Tab「开始同步」→ `POST .../FTStockQuoteSnapshotSync/Run`（`instrument_symbol=crypto`）；运行状态、逐批日志、轮询与其它 Tab 共用 |
| 凭据配置入口 | 「加密货币采集配置（数据库）」**上方**的「moomoo OpenAPI 凭据」卡片：AppKey + 私钥（Ed25519 PEM），保存到 `sys.sys_config` 20016（`AppKey\|私钥 PEM` 单配置项）；私钥只填中间 base64 那一行即可（粘完整 PEM 也兼容），输入框右侧 👁 可切换明文；留空 = 不修改，页面只回显掩码（需求 11318 / 11319 / 11320） |
| 凭据生效 | 采集时优先读 `sys_config`，未配置才回退环境变量 `FINV_MOOMOO_APP_KEY` / `FINV_MOOMOO_PRIVATE_KEY`；两处都没有时运行日志给 ERROR，任务直接失败 |
| 参数 | `page_size` 固定 400 且不可改（moomoo 接口单次上限）；`interval_ms` / `count_plan` 等不生效（一次取全量）；`request_timeout_ms` 作为本次采集总超时 |
| 日志 | 启动一行 INFO（provider + 结果表）、每批一行 INFO（codes / rows / 新增 / 覆盖）、失败一行 ERROR；采集完成后结果区显示批次数与新增/覆盖行数 |
| 空收藏 | 收藏表里没有加密货币时打「跳过采集」，任务按成功结束（新增/覆盖均为 0） |

#### 2.0.7 收藏的美股/港股指数采集（`index`，需求 11333）

| 项 | 内容 |
|---|---|
| 数据源 | `quote.finv_quote_secu_fav`（收藏表）中 `security_type ∈ {StockIndex, Index}` 且 `exchange_flag ∈ {HK, US}` 的 `usc` |
| code 规则 | **优先** `futu.finv_futu_usc_map.ftmm_symbol`（`status='ACTIVE'`、`instrument='index'`，如 `HSI`→`HK.800000`、`HSIII`→`HK.800806`、`DJI`→`US..DJI`）；查不到才按港股 `HK.<security_code>`、美股 `US..<security_code>`（**双点**）拼；别名 `HK.HSI`、单点 `US.DJI` 实测均不命中 |
| 落库表 | `cdeu.finv_quote_secu_realtime`（主键 `usc`，`provider` 固定 `MMAPI`，`instrument_type=6`，`market_type` 港股 `1` / 美股 `2`，`market_label` `HK` / `US`） |
| 覆盖语义 | `COALESCE(EXCLUDED.col, t.col)` **有值才覆盖**：只写身份列与行情列，不碰 `rmb_*` / `market_value` 等其它来源的列（需求 11287 同口径） |
| 触发 | 本 Tab「开始同步」→ `POST .../FTStockQuoteSnapshotSync/Run`（`instrument_symbol=index`）；运行状态、逐批日志、轮询与其它 Tab 共用。配置行优先：配置表存在启用的 `index` 行时仍走通用列表采集 |
| 凭据配置入口 | 与「加密货币」Tab **共用**同一张「moomoo OpenAPI 凭据」卡片（`sys.sys_config` 20016），切到本 Tab 自动加载，不需要重复填写 |
| 参数 | `page_size` 固定 400 且不可改（moomoo 接口单次上限）；`interval_ms` / `count_plan` 等不生效（一次取全量）；`request_timeout_ms` 作为本次采集总超时 |
| 未返回 code | 上游没返回的 code 记入结果 `Missing` 并在日志打一行 WARN，不报错、不写库（多为上游未覆盖的长尾指数）；`security_code` 是字母别名的指数靠 `ftmm_symbol` 兜底解决（需求 11334，如 `HSIII`→`HK.800806`），修前它会一直出现在 WARN 里 |
| 独立接口 | `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Index/Fav/Snapshot/Sync` 可直接触发（见 [IndexFavSnapshotSync.md](../../../API/Cdeu/IndexFavSnapshotSync.md)） |

### 2.1 采集配置（只读表格）

展示**当前 Tab 品种**的配置，数据来自 `cdeu.finv_cdeu_secu_realtime_sync_config`（`flag_enable='1'`）：

| 列 | 说明 |
|---|---|
| 品种 | `instrument_symbol`（当前 `stock`） |
| 市场 | `market_type` 映射为 `HK 港股` / `US 美股` / `CN A 股` |
| 计划数量 | `count_plan`，空显示「全量」 |
| 市场证券总数 | `count_exists`（采集成功后回写，空显示 `—`） |
| 本轮可采集 | `total_available`（采集成功后回写，空显示 `—`） |

### 2.2 本次采集参数（可编辑）

| 控件 | 字段 | 说明 |
|---|---|---|
| 市场 | `market_type` | 「全部市场」+ 当前品种配置中出现过的市场（由配置行生成）；**`bsestock`（BDFS）固定 `BJ` 且下拉禁用**，不可改为其它市场 |
| 计划数量覆盖 | `count_plan` | 留空用配置值 |
| 每页条数 | `page_size` | 默认 50；**`bsestock`（BDFS）默认 500**（接口 `rn`），切到该 Tab 或点「重置参数」时回到 500 |
| 页间隔(ms) | `interval_ms` | 默认 6000，避免被服务端限流 |
| 重试次数 | `max_retries` | 默认 2 |
| 退避(ms) | `retry_backoff_ms` | 默认 2000 |
| 页数上限 | `max_pages` | 0=不限 |
| 开始同步 / 刷新状态 / 重置参数 | — | 「开始同步」只采**当前 Tab 品种**（请求体带 `instrument_symbol`）；任务运行中或该品种无配置行（含占位 Tab）时按钮置灰 |

### 2.3 汇总与结果

- 状态徽章：运行中 / 成功 / 部分成功 / 失败 + `run_id`（全局最近一次运行）
- 汇总行：耗时、收到、新增、覆盖、错误摘要（仅当前品种）
- 结果表格：当前品种各市场的页数 / 收到 / 新增 / 覆盖 / 市场总数 / 本轮可采 / 结果

### 2.4 采集执行日志

等宽字体滚动区（最大高度 320px，自动滚到底部），每行格式：

```
[HH:mm:ss] LEVEL (品种/市场) 消息
```

日志级别：`INFO` 常规进度、`WARN` 重试、`ERROR` 失败。逐页输出示例：

```
[18:03:53] INFO  (stock/1) 开始采集：市场总数 3337，本轮计划 60，按每页 20 共 3 页
[18:17:47] INFO  (stock/1) 第 1/1 页：50 条（累计 新增 0 / 覆盖 50）
[U] 02328   [U] 00267   [U] 02259   [U] 06160   [U] 01276   [U] 06030   [U] 00016   [U] 09618   [U] 80016   [U] 01919
[U] 03993   [U] 00992   [U] 02601   [U] 06809   [U] 00001   [U] 00669   [U] 02378   [U] 89618   [U] 03986   [U] 02513
[U] ...（每页最多 5 行 × 每行 10 个）
[18:17:47] INFO  (stock/1) 采集结束：1 页、收到 50 条、新增 0、覆盖 50
```

逐页明细规则：

| 项 | 说明 |
|---|---|
| 排版 | 每个证券一格，**每行 10 个**，每页 50 条时正好 5 行 |
| 标记 | `[I]` = 新增（该 `stock_id` 此前不在表中）、`[U]` = 更新 |
| 列宽 | 每格固定 11 显示列，不足补空格，多行严格对齐 |
| 内容 | `[标记] 证券代码`（只展示代码，不含名称；代码为空时改用 `stock_id`） |
| 输出位置 | 前端日志面板（`white-space: pre-wrap`）+ 容器控制台 `docker logs` |

#### 2.4.1 采集结束后自动补人民币计价指标（需求 11288）

证券列表采集（`stock` / `etf`）与 BDFS 采集（`bsestock` / `cnetf`）写的是同一张
`cdeu.finv_quote_secu_realtime`，采集跑完后会自动执行一次 `SyncRMBMetrics(step=all)`，
并在日志末尾追加一行：

```
[HH:mm:ss] INFO  人民币计价指标已自动补全（证券列表采集）：汇率 12 行、金额 340 行、耗时 860 ms
```

- 幂等：`SyncRMBMetrics` 只在值变化时更新，重复采集不会产生额外写入；
- 失败只记 `WARN`，不改变采集本身的结果状态（采集成功仍是成功）；
- 缺汇率币种会在同一行日志里列出（`；缺汇率币种：HKD、JPY`）；
- 期货采集写的是 `cdeu.finv_quote_secu_realtime_future`，与人民币指标无关，**不触发**。

### 2.5 说明卡片（页面最底部）

按 [`ExplainCardSpec`](../../../DevSpec/ExplainCardSpec.md)：页面**静态文字说明**不进标题下方，
统一收在页面最底部**唯一**一张「说明」卡片里，分四节：`表口径`（三张结果表与主键）/
`数据源`（get-stock-list、get-etf-list、get-futures-list、BDFS 固定来源）/
`配置与回写`（两张配置表驱动 Tab、`count_exists`/`total_available`、`etf` 与 `cnetf` 的区别）/
`读法与注意事项`（`[I]`/`[U]`、`usc` 未解析去向、期货下架标记时机）。

## 3. 交互流程

1. 进入页面：拉取最近一次运行状态与配置（`GET .../FTStockQuoteSnapshotSync/Run`）。
2. 调整参数后点「开始同步」：`POST .../FTStockQuoteSnapshotSync/Run`，拿到 `run_id`。
3. 前端每 1.5 秒轮询同一 GET 接口，任务为 `RUNNING` 时持续刷新日志；结束即停止轮询。
4. 「刷新状态」可手动拉一次；「重置参数」恢复默认值。

## 4. 约束

- 后端同一时间只允许**一个**运行中的采集任务，重复启动返回 400 并提示当前 `run_id`。
- 运行状态与日志保存在内存：**进程重启后清空**，历史运行不落库。
- 日志上限：每次运行 500 行，内存保留最近 10 次运行。

## 日志口径（需求 10989）

- 每轮首条日志 `开始采集` 之后会打印一行 `首页采样`，形如
  `首页采样 cfg=stock/1(计划 2500) 样本=00700|TENCENT|4.07T 00005|HSBC|2.51T 01299|AIA|795.2B`。
- 该行直接反映服务端返回的**语言与单位口径**：中文为 `腾讯控股|4.07万亿`，英文为 `TENCENT|4.07T`。
- 采集请求固定携带 `accept-language: zh-CN,zh;q=0.9,en;q=0.8`；数值解析同时支持 `万亿/亿/万` 与 `T/B/M/K`。
- 若本轮 `total` 与配置表记录的上次值相差 2 倍以上，日志会打印 `警告 ... 可能被按 IP/语言返回了不同口径`，并在页面日志里以 WARN 展示——同一接口在不同出口 IP/语言下返回的**集合**本身就可能不同（不只是单位）。
