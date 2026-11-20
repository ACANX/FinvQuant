# cdeu 分钟行情导出/导入工具方案（灌数据测试用，MVsv V2 + Go CLI）

> 状态：**已实施（2026-10-28）**
> 日期：2026-10-28
> 目标表：`cdeu.finv_quote_secu_kline_min`（19 列，主键 `(usc, ts)`）
> 关联：[`2610-005-FinvQuoteDeDubExportUpload.md`](2610-005-FinvQuoteDeDubExportUpload.md)（业务导出/上传链路，与本工具用途不同）

## 0. 实施记录（2026-10-28）

| 项 | 实际实现 |
|---|---|
| 程序名 | **`qmio`**（`cmd/qmio`） |
| 安装位置 | **`<PREFIX>/bin/qmio`（与 `finvquant` 同目录）**，软链 `/usr/local/bin/qmio` |
| 文件格式 | `MVSV-2`：19 列（含 `gmt_*`）/ 17 列（`--no-gmt`），单文件全表，含 `usc` |
| 导入模式 | 默认 **`upsert`**；`append` / `replace`（需 `--force`） |
| 校验 | `.sha256` + gzip 完整性 + 行数 |
| 打包/安装 | `PackSnapshot.sh` 包根目录同时产出 `finvquant` 与 `qmio`；`Install/Upgrade/Uninstall` 同步 |
| 实测 | 临时 PG：2 行导出 → 清空 → 导入 → 再导入（幂等），`--no-gmt` 17 列通过；大数据量性能待补测 |

## 1. 背景与目标

联调、压测、回测验证时需要**快速**准备或搬移 `cdeu.finv_quote_secu_kline_min` 的数据：

- 从生产/预发导出到文件，再灌入测试库；
- 同一份数据集反复重建（幂等）；
- 大数据量（当前约 280 万行，未来千万级）下仍要快。

**目标**

1. 一条命令完成导出/导入；
2. 导出文件采用**.mvsv**（项目既有行情交换格式），可压缩、可校验、可跨环境传输；
3. 导入默认 **upsert**（幂等、可覆盖）；
4. 不影响在线服务（只读导出、独立连接）；
5. 不破坏现有 MVSV-1 契约与导出/上传链路。

**非目标**

- 不做业务级归档/上传（已有 CDEU 导出上传链路）；
- 不做跨 PG 大版本/跨架构的物理备份（用 `pg_dump`）；
- 首期不做 HTTP API。

## 2. 方案对比

### 2.1 交付形态

| 方案 | 可行性 | 说明 |
|---|---|---|
| psql 脚本 | **不可行** | MVsv 需要 16 行文件头、字段类型声明与格式化数据行，`\copy` 只能产出 CSV/binary |
| **Go CLI（推荐）** | 可行 | 复用 `internal/mvsv` 与 pgx 流式读写，单二进制随安装包分发 |
| HTTP API | 不推荐 | 大数据量受网络/超时影响 |

**结论**：`.mvsv` 决定了必须走 Go CLI。

### 2.2 文件格式

| 方案 | 说明 | 结论 |
|---|---|---|
| **MVsv V2（新增 `usc` 列，单文件全表）** | 新 Format 版本，支持多证券、19 列（gmt 可选） | **采用（你的选择 B）** |
| MVsv V1（单证券 12 列） | 既有格式，不含 `usc`、`paocd`、`last_close`、`gmt` | 保持不变，仅兼容解析 |
| CSV.gz | 最快实现 | 不作为本工具格式 |

### 2.3 导入语义

| 模式 | 行为 | 默认 |
|---|---|---|
| `upsert` | 冲突时 `DO UPDATE` 覆盖业务列 | **默认** |
| `append` | 冲突时 `DO NOTHING` | 可选 |
| `replace` | `TRUNCATE` + 导入 | 可选，需 `--force` |

## 3. 推荐方案设计

### 3.1 交付物

```
cmd/qmio/                     # 新增：数据搬运 CLI（独立二进制 qmio）
  main.go
internal/mvsv/                # 扩展：MVSV-2 解析（新增 usc/多列）
internal/meta/finv/quote/dedup/mvsv.go   # 扩展：MVSV-2 写出
Deploy/Script/CdeuKline.cmd   # Windows 薄封装（可选）
```

做成**独立小工具** `qmio`（Quick Market IO，避免服务二进制变大、无需停服），
随安装包放到 `<PREFIX>/bin/qmio`。

### 3.2 命令

```bash
# 导出（默认 MVsv V2 + gzip，含 gmt 列）
qmio export \
  --out /data/App/FinvQuant/Export/CdeuKline \
  [--where "usc = 'GCMain' AND date >= 20260101"] \
  [--no-gmt] [--no-gzip]

# 导入（默认 upsert）
qmio import FILE.mvsv[.gz] [--mode upsert|append|replace] [--force]

# 校验（格式/行数/sha256）
qmio verify FILE.mvsv[.gz]
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--out DIR` | `<PREFIX>/Export/CdeuKline` | 导出目录（创建 0750） |
| `--where SQL` | 无 | 附加过滤（仅导出） |
| `--no-gmt` | 关 | 不导出 `gmt_create/gmt_update`（导入用 `now()`） |
| `--no-gzip` | 关 | 不压缩（默认 gzip） |
| `--mode` | `upsert` | `upsert` / `append` / `replace` |
| `--force` | 关 | `replace` 必须显式确认 |
| `--pg-url` | 从 `finvquant.env` 读取 | 覆盖连接串 |

### 3.3 MVSV V2 格式设计

**文件头（19 列，默认含 gmt）**

```text
# Title : "CdeuKline_20261028_120000"
# Format : "MVSV-2"
# Field : "usc|ts|date|time|open|high|low|close|last_close|volume|turnover|change_price|change_ratio|paocd|ext_field|remark|provider|gmt_create|gmt_update"
# FieldType : "String|Int|Int|Int|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|Decimal|String|String|String|Timestamp|Timestamp"
# FieldName : "Usc|Ts|Date|Time|Open|High|Low|Close|LastClose|Volume|Turnover|ChangePrice|ChangeRatio|Paocd|ExtField|Remark|Provider|GmtCreate|GmtUpdate"
# 字段名称 : "证券代码|时间戳(UTC)|交易日期|交易时间|开盘价|最高价|最低价|收盘价|上一收盘价|成交量|成交额|涨跌额|涨跌幅|累计均价|扩展字段|备注|数据来源|创建时间|更新时间"
# Count : 2812345
# DataScope : "all"
# ExportTime : "2026-10-28T12:00:00+08:00"
# TimeZone : "Asia/Shanghai"
...（其余头字段置空或固定）
```

- `--no-gmt`：去掉 `gmt_create/gmt_update` 两列，`Field/FieldType/FieldName` 同步为 17 列；
- 导入按**列名**映射（不依赖列序/列数），因此 17/19 列文件都能识别；
- `MVSV-1` 文件保持原样可解析（单证券、12 列），**不受影响**；
- 文档同步：`Docs/DataFormat/MvsvFileFormat.md` 新增「MVSV-2（多证券）」布局与示例。

### 3.4 导出实现

1. 流式查询（`REPEATABLE READ READ ONLY` 快照）：

   ```sql
   SELECT usc, ts, date, "time", open, high, low, close, last_close,
          volume, turnover, change_price, change_ratio, paocd,
          ext_field, remark, provider, gmt_create, gmt_update
     FROM cdeu.finv_quote_secu_kline_min
    WHERE <where>
    ORDER BY usc, date, ts;
   ```

2. 写入 MVSV-2（复用 `WriteMVsv` 的头/临时文件/原子 rename/Count 校验逻辑，
   新增 19 列写出函数；`gmt` 为 `time.RFC3339` 文本）；
3. 先写 `.tmp`、校验行数、原子改名，生成 `.sha256`；
4. 文件命名：`cdeu_kline_YYYYMMDD_HHMMSS.mvsv.gz`。

### 3.5 导入实现

1. 解析 MVSV-2 文件头，按 `Field` 列名映射到 DB 列；
2. 分批（如 5000 行/批）写入临时表或直接参数化 `INSERT`：
   - `upsert`：`INSERT ... ON CONFLICT (usc, ts) DO UPDATE SET <业务列> = EXCLUDED.<业务列>, gmt_update = now()`（`--no-gmt` 文件不覆盖 gmt）；
   - `append`：`ON CONFLICT DO NOTHING`；
   - `replace`：`TRUNCATE` + 纯 `INSERT`（需 `--force`）；
3. 单事务或分批事务（默认单事务，失败整体回滚；超大数据量可用 `--batch-commit`）；
4. 导入前校验 `.sha256` 与 gzip 完整性。

### 3.6 校验

| 阶段 | 检查 |
|---|---|
| 导出后 | 文件行数 = `Count`、`sha256sum`、gzip 完整性 |
| 导入前 | `.sha256` 比对、`Format=MVSV-2`、必需列（`usc`、`ts`）存在 |
| 导入后 | 打印插入/更新/跳过行数；目标表 `count(*)`（可选） |

### 3.7 性能预估（实施时实测）

| 数据量 | 导出 MVsv.gz | 导入 upsert |
|---|---|---|
| 100 万行 | 5–12s | 15–35s |
| 280 万行 | 15–35s | 45–90s |

（MVsv 为文本格式，比 COPY CSV 略慢；换取格式统一与可读性。）

### 3.8 安全

- CLI 以 `finvquant` 用户运行，连接信息从 `finvquant.env` 读取（不落命令行）；
- 导出目录 `0750`、文件 `0640`；
- `replace` 需 `--force`，执行前打印库名/表名/行数；
- 导入前 sha256 校验；失败整体回滚。

## 4. 已确认的决策

| # | 决策点 | 结果 |
|---|---|---|
| 1 | 文件格式 | **`.mvsv`（新增 MVSV-2：含 `usc`、单文件全表）** |
| 2 | 交付形态 | **Go CLI**（`.mvsv` 无法由 psql 脚本生成） |
| 3 | 导入默认模式 | **`upsert`** |
| 4 | gmt 列 | **默认导出**，`--no-gmt` 可关闭 |
| 5 | 文件目录 | `<PREFIX>/Export/CdeuKline`（`--out` 可覆盖） |
| 6 | `replace` | 需 `--force` |
| 7 | 过滤导出 | 支持 `--where` |
| 8 | 校验文件 | `.sha256` |
| 9 | 分片导出 | 不需要（MVSV-2 单文件全表） |
| 10 | 兼容性 | MVSV-1 保持可解析；V2 新增 |

## 5. 实施计划（批准后）

| 阶段 | 内容 | 产出 |
|---|---|---|
| P1 | `internal/mvsv` 支持 MVSV-2 解析（列名映射、`usc`） | 解析器 + 单测 |
| P2 | `WriteMVsvV2`（19 列、gmt 可选、流式） | 写出 + 单测 |
| P3 | `cmd/data` CLI：export / import（upsert/append/replace）/ verify | 可执行工具 |
| P4 | 文档：`MvsvFileFormat.md` V2 布局、CLI 用法、Install 包内路径 | 文档 |
| P5 | 实测（100 万/280 万行）并回填本文档 | 测试记录 |

## 6. 验收标准

```bash
# 导出
qmio export --out /tmp/kline
ls /tmp/kline/cdeu_kline_*.mvsv.gz /tmp/kline/cdeu_kline_*.sha256

# 导入（upsert，跑两次结果一致）
qmio import /tmp/kline/cdeu_kline_*.mvsv.gz
qmio import /tmp/kline/cdeu_kline_*.mvsv.gz

# 校验
qmio verify /tmp/kline/cdeu_kline_*.mvsv.gz

# replace 需 --force
qmio import FILE --mode replace            # 应拒绝
qmio import FILE --mode replace --force    # 通过
```

- 导出/导入行数一致；重复 upsert 不产生重复行；
- 失败整体回滚；
- MVSV-1 旧文件仍可被现有导入链路解析（回归测试）。

## 7. 风险与回滚

| 风险 | 缓解 |
|---|---|
| MVSV-2 影响既有 MVSV-1 解析 | 以 `Format` 分支；旧格式回归测试 |
| `replace` 误清生产表 | 必须 `--force` + 执行前打印库/表/行数 |
| 大文件内存占用 | 流式读写（不整文件读入） |
| 导入慢 | 分批 5000 行；可选 `--batch-commit` |
