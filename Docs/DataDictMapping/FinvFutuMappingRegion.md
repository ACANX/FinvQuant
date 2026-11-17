# FinvFutuMappingRegion — 富途区域映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFutuMappingRegion.md`
> 数据表：`finv_futu_mapping_region`（表结构：[`Deploy/Migrations/V13__finv_futu_mapping_region.sql`](../../Deploy/Migrations/V13__finv_futu_mapping_region.sql)；增量列：[`Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql`](../../Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql)；初始数据：[`Deploy/Migrations/V100005__finv_futu_mapping_region_seed.sql`](../../Deploy/Migrations/V100005__finv_futu_mapping_region_seed.sql)）
> 用途：富途行情源 region 字典（0~24）与 finv_region.idx 的字段映射表，供富途数据入库时转换区域标识。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `futu_region` | INTEGER | PK | 富途 region 值（如 `0` ~ `24`） |
| `abbr` | TEXT | NOT NULL | 区域简写（关联 [FinvRegion](FinvRegion.md) `region`，如 `CN` / `HK` / `USA`） |
| `name` | TEXT | NOT NULL | 区域中文名称（关联 [FinvRegion](FinvRegion.md) `name`，如 `中国大陆` / `香港`） |
| `finv_region` | INTEGER | NOT NULL，0~999999 | finv 区域序号（关联 [FinvRegion](FinvRegion.md) `idx`） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（`0`=禁用 / `1`=启用，V17 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_futu_mapping_region_finv`：`(finv_region, futu_region)`（按 finv 区域序号反查富途 region）

## 2. 数据清单（25 条）

| futu_region | abbr | name | finv_region |
|------------:|------|------|------------:|
| 0 | UNK | 未知/默认 | 0 |
| 1 | GLOBAL | 全球/外汇/加密货币 | 1 |
| 2 | CN | 中国大陆 | 101 |
| 3 | HK | 香港 | 102 |
| 4 | USA | 美国 | 103 |
| 5 | JP | 日本 | 104 |
| 6 | SG | 新加坡 | 105 |
| 7 | MY | 马来西亚 | 106 |
| 8 | AU | 澳大利亚 | 107 |
| 9 | CA | 加拿大 | 108 |
| 10 | DE | 德国 | 109 |
| 11 | GB | 英国 | 110 |
| 12 | IT | 意大利 | 111 |
| 13 | TW | 台湾 | 112 |
| 14 | FR | 法国 | 113 |
| 15 | NL | 荷兰 | 114 |
| 16 | PT | 葡萄牙 | 115 |
| 17 | BE | 比利时 | 116 |
| 18 | KR | 韩国 | 117 |
| 19 | IN | 印度 | 118 |
| 20 | ES | 西班牙 | 119 |
| 21 | GR | 希腊 | 120 |
| 22 | ZA | 南非 | 121 |
| 23 | BR | 巴西 | 122 |
| 24 | ID | 印度尼西亚 | 123 |

## 3. 说明

- **映射方向**：`futu_region`（富途原值 0~24）→ `finv_region`（finv_region.idx：0/1 保留原值，其余 = 富途原值 + 99 → 101~123）。
- **冗余字段**：`abbr` / `name` 与 [FinvRegion.md](FinvRegion.md) 保持一致，便于映射表独立查询，不建物理外键（项目惯例，程序层控制）。
- **关联字典**：`finv_region` → [FinvRegion.md](FinvRegion.md)（区域字典 idx）；`abbr` / `name` → 同表 `region` / `name`。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
