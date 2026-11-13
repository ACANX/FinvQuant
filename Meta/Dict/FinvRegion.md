# 区域字典（FinvRegion）

> 所属：FinvQuant 元数据字典 · 存放：`Meta/Dict/FinvRegion.md`
> 用途：`public.finv_region` 的取值字典——**区域简写（`region`）为键，区域中文名称为值**，
> 供交易所 / 市场 / 证券按区域归类、筛选与展示时统一口径。
> 数据表：`public.finv_region`（主键 `idx`、唯一键 `region`，含 `name` 中文名）；
> 建表与变更：[`V200006`](../../Deploy/Migrations/V200006__finv_region.sql)、[`V200011`](../../Deploy/Migrations/V200011__finv_region_allow_idx_zero.sql)、
> [`V200012`](../../Deploy/Migrations/V200012__finv_region_add_name.sql)、[`V300004`](../../Deploy/Migrations/V300004__finv_region_seed.sql)（种子）、
> [`V300077`](../../Deploy/Migrations/V300077__finv_region_add_fx_crypto_and_us.sql)（补 `FX`/`CC`、`USA`→`US`）、
> [`V300078`](../../Deploy/Migrations/V300078__finv_region_crypto_to_cc.sql)（`CRYPTO`→`CC`）。
> 关联：[`Docs/DataDictMapping/FinvRegion.md`](../../Docs/DataDictMapping/FinvRegion.md)（表结构与字段映射）。

## 1. 说明

- 键：`region` 列的区域简写（大写字母，2~6 位；唯一约束 `uq_finv_region_region`）。
- 值：`name` 列的区域中文名称（规范中文名，如 `美国`、`中国大陆`）。
- `IDX` 列：对应表字段 `idx`（主键），即该地区的地区编号；仅用于排序与兼容富途历史 region 原值，业务判断以 `region` 为准。
- 排序：按 `IDX` 升序（保留位在前，其后为非地域类别与区域段）。
- `idx` 编码规则：`0` = 未知/默认，`1` = 全球（历史笼统口径），`2`~`3` = 非地域市场类别（`FX` 外汇、`CC` 加密货币），
  `101`~`123` = 国家/地区（富途 region 原值 + 99，原值 `1` 保留给 `GLOBAL`，`2`~`100` 段预留扩展）。
- 该字典覆盖当前库内全部 **27 条**记录；新增区域须同步本文件、`finv_futu_mapping_region` 与相关前端映射。

## 2. 字典表

| 键 | 值 | IDX | 备注 |
|----|----|----:|------|
| `UNK` | 未知/默认 | 0 | 未知/默认，兜底值（无区域信息时使用） |
| `GLOBAL` | 全球/外汇/加密货币 | 1 | 历史「全球/外汇/加密货币」笼统口径，新数据按 `FX` / `CC` 细分 |
| `FX` | 外汇 | 2 | `V300077` 新增（需求 10877）：外汇与 FX-CFD 交易所使用，替代 `GLOBAL` 笼统口径 |
| `CC` | 加密货币 | 3 | `V300077` 先以 `CRYPTO` 新增，`V300078` 改简写为 `CC`；加密货币类交易所/市场使用 |
| `CN` | 中国大陆 | 101 | 富途自选股 region 原值 2，国家/地区段起点 |
| `HK` | 香港 | 102 | 富途自选股 region 原值 3 |
| `US` | 美国 | 103 | 富途原值 4，`V300077` 由 `USA` 统一为 `US`（与 `finv_exchange.region` 一致） |
| `JP` | 日本 | 104 | 富途自选股 region 原值 5 |
| `SG` | 新加坡 | 105 | 富途自选股 region 原值 6 |
| `MY` | 马来西亚 | 106 | 富途自选股 region 原值 7 |
| `AU` | 澳大利亚 | 107 | 富途自选股 region 原值 8 |
| `CA` | 加拿大 | 108 | 富途自选股 region 原值 9 |
| `DE` | 德国 | 109 | 富途自选股 region 原值 10 |
| `GB` | 英国 | 110 | 富途自选股 region 原值 11 |
| `IT` | 意大利 | 111 | 富途自选股 region 原值 12 |
| `TW` | 台湾 | 112 | 富途自选股 region 原值 13 |
| `FR` | 法国 | 113 | 富途自选股 region 原值 14 |
| `NL` | 荷兰 | 114 | 富途自选股 region 原值 15 |
| `PT` | 葡萄牙 | 115 | 富途自选股 region 原值 16 |
| `BE` | 比利时 | 116 | 富途自选股 region 原值 17 |
| `KR` | 韩国 | 117 | 富途自选股 region 原值 18 |
| `IN` | 印度 | 118 | 富途自选股 region 原值 19 |
| `ES` | 西班牙 | 119 | 富途自选股 region 原值 20 |
| `GR` | 希腊 | 120 | 富途自选股 region 原值 21 |
| `ZA` | 南非 | 121 | 富途自选股 region 原值 22 |
| `BR` | 巴西 | 122 | 富途自选股 region 原值 23 |
| `ID` | 印度尼西亚 | 123 | 富途自选股 region 原值 24 |

## 3. 维护约定

- 新增/修改区域一律走迁移（`Deploy/Migrations/V30xxxx__*.sql`），不允许手工改库；本文件在迁移落地后同步。
- `region` 简写是唯一键且被 `finv_exchange.region`、`finv_market.region`、证券归一化逻辑引用，
  **改简写等于改跨表引用**（如 `USA`→`US` 需带 `UPDATE` 联动，见 `V300077`），不得只改本表。
- `idx` 只用于排序与兼容历史富途 region 原值，业务判断以 `region` 为准，不要用 `idx` 做分支。
- 值为规范中文名；简称、别称（如 `中国大陆`/`大陆`）只写在「备注」，避免同一区域出现两种展示值。
- 具体核对方式：`SELECT idx, region, name FROM public.finv_region ORDER BY idx;` 的行集合应与 §2 表格逐行一致。
