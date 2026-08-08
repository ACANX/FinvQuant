# FinvFutuMappingSecurity — 富途证券代码映射

> 所属：FinvQuant 数据字典映射 · 存放：`Docs/DataDictMapping/FinvFutuMappingSecurity.md`
> 数据表：`finv_futu_mapping_security`（表结构：[`Deploy/Migrations/V7__finv_futu_mapping_security.sql`](../../Deploy/Migrations/V7__finv_futu_mapping_security.sql)；增量列：[`Deploy/Migrations/V10__finv_futu_mapping_security_add_futu_symbol.sql`](../../Deploy/Migrations/V10__finv_futu_mapping_security_add_futu_symbol.sql) / [`Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql`](../../Deploy/Migrations/V17__finv_futu_mapping_add_flag_enable.sql)；初始数据：[`Deploy/Migrations/V100003__finv_futu_mapping_security_seed.sql`](../../Deploy/Migrations/V100003__finv_futu_mapping_security_seed.sql)；全量替换：[`Deploy/Migrations/V100014__finv_futu_mapping_security_seed_full.sql`](../../Deploy/Migrations/V100014__finv_futu_mapping_security_seed_full.sql)）
> 用途：FT（富途/moomoo）行情源证券内部 ID 与统一证券代码（usc）的字段映射表，供行情源数据入库时转换证券标识。

## 1. 表结构

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `futu_stock_id` | TEXT | PK | 富途证券内部 ID（moomoo stockId，如 `70000294` / `50616191183396`） |
| `futu_symbol` | TEXT | 可空 | 富途证券代码标识（如 `HK.00700` / `US.AAPL`，V10 新增） |
| `finv_usc` | TEXT | NOT NULL | 统一证券代码（关联 [FinvSecurity](FinvSecurity.md) `usc`） |
| `flag_enable` | CHAR(1) | NOT NULL DEFAULT '0' | 启用标志（`0`=禁用 / `1`=启用，V17 新增） |
| `gmt_create` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 首次插入时间 |
| `gmt_update` | TIMESTAMPTZ | NOT NULL DEFAULT now() | 最后更新时间（触发器维护） |

索引：

- `idx_finv_futu_mapping_security_usc`：`(finv_usc, futu_stock_id)`（按统一证券代码反查富途证券 ID）

## 2. 数据清单（503 条）

> V100014 全量替换 V100003（先 DELETE 存量，再 INSERT 全量 503 条）；三列与 V100003 一致（503/503 核对通过），显式含 V17 新增 `flag_enable`（默认 '0'）。数据量较大，完整清单见迁移文件：[`V100014__finv_futu_mapping_security_seed_full.sql`](../../Deploy/Migrations/V100014__finv_futu_mapping_security_seed_full.sql)。

## 3. 说明

- **映射方向**：`futu_stock_id`（富途行情源证券内部 ID）→ `finv_usc`（统一证券代码）；富途侧 ID 为字符串形式，即使全数字也按 TEXT 存储（如 16 位 `50616191183396`）。
- **futu_symbol**：富途证券代码标识（V10 新增，可空），与 `futu_stock_id`（内部数字 ID）互补，便于按富途对外代码查询；PG 的 `ADD COLUMN` 追加到表尾，逻辑上为第二列，SELECT 按字段名引用不受影响。
- **关联字典**：`finv_usc` → [FinvSecurity.md](FinvSecurity.md)（统一证券代码）；关联不建物理外键，由程序层控制（项目惯例）。
- **审计字段**：`gmt_create` / `gmt_update` 与既有表规范一致，`gmt_update` 由 `vq_set_gmt_update()` 触发器自动维护。
