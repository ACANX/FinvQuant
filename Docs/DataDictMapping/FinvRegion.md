# FinvRegion — 区域字典映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvRegion.md`
> 数据表：`finv_region`（表结构：[`Deploy/Migrations/V6__finv_region.sql`](../../Deploy/Migrations/V6__finv_region.sql)；约束放宽：[`Deploy/Migrations/V11__finv_region_allow_idx_zero.sql`](../../Deploy/Migrations/V11__finv_region_allow_idx_zero.sql)；新增 name 列：[`Deploy/Migrations/V12__finv_region_add_name.sql`](../../Deploy/Migrations/V12__finv_region_add_name.sql)；初始数据：[`Deploy/Migrations/V100004__finv_region_seed.sql`](../../Deploy/Migrations/V100004__finv_region_seed.sql)）
> 用途：区域代码字典（idx 数字序号 + region 区域简写 + name 中文名称），供交易所/市场按区域归类与查询。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `idx` | INTEGER | PK，0~999999 | 区域序号（数字）；V11 起允许 0（未知/默认） |
| `region` | TEXT | NOT NULL, UNIQUE | 区域简写（如 `CN` / `HK` / `USA` / `JP`） |
| `name` | TEXT | 可空 | 区域中文名称（如 `中国大陆` / `香港` / `美国`，V12 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

## 2. 数据清单（25 条）

> 数据来源：富途自选股 region 字段统计（0~24）。idx 编码：保留 0（未知/默认）与 1（全球/外汇/加密货币），其余区域 idx = 原值 + 99（101~123），为 2~100 段预留扩展空间。

| idx | region | name | 对应地区 |
|----:|--------|------|----------|
| 0 | UNK | 未知/默认 | 未知/默认 |
| 1 | GLOBAL | 全球/外汇/加密货币 | 全球/外汇/加密货币 |
| 101 | CN | 中国大陆 | 中国大陆 |
| 102 | HK | 香港 | 香港 |
| 103 | USA | 美国 | 美国 |
| 104 | JP | 日本 | 日本 |
| 105 | SG | 新加坡 | 新加坡 |
| 106 | MY | 马来西亚 | 马来西亚 |
| 107 | AU | 澳大利亚 | 澳大利亚 |
| 108 | CA | 加拿大 | 加拿大 |
| 109 | DE | 德国 | 德国 |
| 110 | GB | 英国 | 英国 |
| 111 | IT | 意大利 | 意大利 |
| 112 | TW | 台湾 | 台湾 |
| 113 | FR | 法国 | 法国 |
| 114 | NL | 荷兰 | 荷兰 |
| 115 | PT | 葡萄牙 | 葡萄牙 |
| 116 | BE | 比利时 | 比利时 |
| 117 | KR | 韩国 | 韩国 |
| 118 | IN | 印度 | 印度 |
| 119 | ES | 西班牙 | 西班牙 |
| 120 | GR | 希腊 | 希腊 |
| 121 | ZA | 南非 | 南非 |
| 122 | BR | 巴西 | 巴西 |
| 123 | ID | 印度尼西亚 | 印度尼西亚 |

## 3. 说明

- **idx=0**：V6 原约束 `CHECK (idx BETWEEN 1 AND 999999)` 不允许 0；V11 放宽为 `0~999999`（DROP + ADD 同约束名，幂等），以与富途 region 源数据一一对应。
- **name 列**：V12 新增（TEXT 可空）；种子 `ON CONFLICT (idx) DO UPDATE SET name = EXCLUDED.name`，已存在数据（如手动插入）也会自动回填 name。
- **idx 段位**：0~1 为保留区（未知/默认、全球），101~123 为正式区域（原值 +99），2~100 段预留扩展。
- **关联参考**：[`FinvExchange.md`](FinvExchange.md) 中 `finv_exchange.region`（如 `CN` / `HK` / `USA` / `JP`）可对齐本表 `region` 建立区域维度归类（关联规则待定，不建物理外键，程序层控制）。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
