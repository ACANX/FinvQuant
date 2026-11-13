# 采集参数（CollectParam）

> 所属：导航「行情 → 收剔导传ETL → CDEU自动化 → 采集参数」
> 路由：`/Meta/Finv/Quote/Cdeu/CollectParam`
> 视图：`Web/src/views/Meta/Finv/Quote/Cdeu/CollectParamView.vue`
> 编辑对话框：`Web/src/views/Meta/Finv/Quote/Cdeu/CollectParamDialog.vue`
> 共享配置（行结构 / 下拉选项 / 编码字典）：`Web/src/config/collectParam.ts`
> 菜单 key：`cdeu-collect-param`（父分组 `quote-cdeu`，紧跟在 `cdeu-automation`「CDEU自动化」之后）
> 数据表：`futu.finv_futu_quote_collect`（主键 `usc`；`ftmm_symbol` 为生成列）
> 左关联：`public.finv_security`（证券元数据）、`public.finv_exchange` / `public.finv_market`（交易所 / 市场显示名）
> 接口：[`GET /API/V1/Meta/Finv/Quote/Secu/Collect/Page`](../../../API/Cdeu/CollectParam.md)、
> [`POST /API/V1/Meta/Finv/Quote/Secu/Collect/Update`](../../../API/Cdeu/CollectParam.md)
> 关联需求：11254

## 1. 业务功能

查询与维护富途采集参数（一行 = 一个证券的采集配置）。采集表本身只有富途侧编码，
所以查询时左关联规范证券主数据，把「名称 / 分类 / 子类 / 计价货币 / 交易所 / 市场」带出来，
方便按证券维度核对采集配置。

本页**只改参数，不新增、不删除行**：这张表是采集链路的镜像，删行等于停采；
新增行仍由「待采证券录入 / 证券参数补全 / FT证券指标数据同步」负责。

## 2. 页面构成

### 2.1 筛选与排序

| 控件 | 说明 |
|---|---|
| 关键字 | 模糊匹配 `usc` / `ftmm_symbol` / `futu_symbol` / 证券名称（回车即查询） |
| 行情市场 | 全部 / `HK` / `SH` / `SZ` / `BJ` / `US` / `FX` / `CC`（匹配 `market_label`，切换即查询） |
| 品种 | 全部 / 股票 / ETF / 指数 / 外汇 / 加密 / 期货 / 板块（匹配 `instrument`） |
| 证券分类 | 全部 / 股票 / ETF / 指数 / 期货（匹配主数据 `security_type`） |
| 采集状态 | 全部 / 启用 / 禁用（匹配 `flag_enable`） |
| 只看主数据缺失 | 勾选后只列 `public.finv_security` 里没有该证券的行（这类行六列元数据都显示 `—`） |
| 排序字段 | 证券编码（默认，升序）/ 富途键 / 富途代码 / 行情市场 / 品种 / 更新时间 |
| 方向 | 升序（默认）/ 降序；空值一律排最后 |
| 每页 | 20 / 50（默认）/ 100 / 200；位于表格底部、页码控件左侧 |
| 查询 / 刷新 / 重置 | 与筛选控件同处一行；查询回到第 1 页，刷新按当前条件重取，重置恢复默认条件 |

### 2.2 表格列

| 分组 | 列 |
|---|---|
| 身份 | 证券编码 `usc`、富途键 `ftmm_symbol`、富途代码 `futu_symbol` |
| 证券元数据（左关联 `public.finv_security`） | 名称、分类、子类、计价货币、交易所、市场 |
| 富途编码 | 行情市场 `market_label`、市场类型 `market_type`、市场代码 `market_code`、品种 `instrument`、品种类型 `instrument_type`、子类型 `sub_instrument_type`、订阅段 `req_section` |
| 采集控制 | 采集（`flag_enable`：启用绿色 chip / 禁用默认色）、备注、更新时间 |
| 操作 | 「编辑」按钮（打开单行编辑对话框） |

展示口径：

- 富途编码列按字典翻成中文（品种 `stock`→股票、市场类型 `1`→港股、品种类型 `3`→股票 …），未登记值原样显示；
- 主数据缺失的行（`security_exists=false`）六列元数据统一显示 `—`；
- 更新时间由后端格式化为 `Asia/Shanghai` 的 `YYYY-MM-DD HH:MM:SS`；
- 列较多：单元格不换行、列宽随内容撑开，超出视口整体横向滚动。

### 2.3 单行编辑（对话框）

| 区域 | 内容 |
|---|---|
| 只读 | 证券元数据一行（名称｜分类｜子类｜货币｜交易所｜市场）、富途键、更新时间 |
| 可编辑 | `stock_id`、`market_label`、`futu_symbol`、`market_type`、`market_code`、`instrument`、`instrument_type`、`sub_instrument_type`、`req_section`、采集开关（`flag_enable`，`v-switch`）、`remark` |
| 提交 | 只提交**改动过的字段**（未改的不出现在请求体里），避免顺手覆盖别人的值 |
| 校验 | 数值列必须为正整数；`market_label` / `futu_symbol` / `instrument` 非空；无改动时提示「没有改动，无需保存」 |
| 反馈 | 成功：toast + 自动刷新列表、关闭对话框；`409` 唯一键冲突：保持对话框打开并提示冲突原因；其它错误：toast |

不可编辑：`usc`（主键，换它等于换证券，属「入队」场景）、`ftmm_symbol`（PG 生成列，自动跟随
`market_label` + `futu_symbol`）、`gmt_create` / `gmt_update`（触发器维护）。

## 3. 说明卡片（页面最底部）

按 `ExplainCardSpec` 收进唯一一张「说明」卡片，四段：表口径 / 六列证券元数据的来源 / 字段与编码口径 / 注意事项。

## 4. 依赖与口径

- `(market_code, futu_symbol)` 有唯一索引 `uk_finv_futu_quote_collect_market_symbol`：
  改成与其它证券相同的「富途市场代码 + 代码」会被拒绝（409），不覆盖别人的行。
- `ftmm_symbol` 是生成列（`market_label || '.' || futu_symbol`），写入侧不得显式赋值（需求 11060）。
- 采集开关 `flag_enable` 是**人工/同步**维护的列，本页改动即时生效；「FT证券指标数据同步」不写该列。
- 字段编码口径（富途侧）以说明卡片为准，页面下拉只显示中文短名。
