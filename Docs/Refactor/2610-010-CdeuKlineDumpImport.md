# cdeu 分钟行情导出/导入工具方案（灌数据测试用）

> 状态：**方案草案（待批准，未实施）**
> 日期：2026-10-28
> 目标表：`cdeu.finv_quote_secu_kline_min`（19 列，主键 `(usc, ts)`）
> 关联：[`2610-005-FinvQuoteDeDubExportUpload.md`](2610-005-FinvQuoteDeDubExportUpload.md)（业务导出/上传链路，与本工具用途不同）

## 1. 背景与目标

联调、压测、回测验证时需要**快速**准备或搬移 `cdeu.finv_quote_secu_kline_min` 的数据：

- 从生产/预发导出到文件，再灌入测试库；
- 同一份数据集反复重建（幂等）；
- 大数据量（当前约 280 万行，未来千万级）下仍要快。

**目标**

1. 一条命令完成导出/导入，无需写 SQL；
2. 导出文件可压缩、可校验、可跨环境传输；
3. 导入支持幂等（重复导入不报错）与显式覆盖两种模式；
4. 不影响在线服务（只读导出、独立连接）；
5. 文件格式通用（可用 `zcat`/`csv` 工具查看与裁剪）。

**非目标**

- 不做业务级归档/上传（已有 CDEU 导出上传链路）；
- 不做跨 PG 大版本/跨架构的物理备份（用 `pg_dump`）；
- 首期不做 HTTP API（先脚本，Go CLI 作为后续增强）。

## 2. 方案对比

### 2.1 交付形态

| 方案 | 速度 | 实现量 | 适用 |
|---|---|---|---|
| **A. psql 脚本（`COPY`/`\copy` + gzip）** | **最快**（服务端 COPY 协议） | 小（纯脚本） | 推荐：测试/运维 |
| B. Go CLI（pgx `CopyTo`/`CopyFrom`） | 快 | 中 | 后续可加进度/校验/断点 |
| C. HTTP API（`/API/V1/.../Dump`） | 受网络/内存影响 | 大 | 不适合大数据量 |
| D. `pg_dump`/`pg_restore` 整库 | 中 | 零 | 整库迁移，不适合单表灌数 |

**推荐 A**：`COPY` 是 PostgreSQL 最快的批量通道；脚本可直接用 `FINV_*` 配置连接，零部署成本。

### 2.2 文件格式

| 格式 | 体积 | 导入速度 | 可读性 | 结论 |
|---|---|---|---|---|
| **CSV + gzip** | 小 | 快 | 好（`zcat`/Excel） | **推荐（默认）** |
| COPY binary | 最小 | 最快（约快 20–40%） | 差 | 可选 `--format binary` |
| MVsv | 中 | 慢（应用层解析） | 好 | 已有行情交换格式，灌数不选 |

**推荐 CSV.gz**：`\copy ... WITH (FORMAT csv, HEADER true)`，`zcat` 可直接查看；二进制仅作为纯速度选项。

### 2.3 导入语义

| 模式 | 行为 | 速度 | 风险 |
|---|---|---|---|
| **`append`（默认）** | COPY 到临时表 → `INSERT ... ON CONFLICT (usc,ts) DO NOTHING` | 快 | 无（幂等） |
| `upsert` | 同上，冲突时 `DO UPDATE` 覆盖 | 中 | 覆盖已有数据 |
| `replace` | `TRUNCATE` + 直接 `COPY` | 最快 | 破坏性，需 `--force` |

**推荐**：默认 `append`（幂等、安全）；`upsert`/`replace` 显式指定，`replace` 必须加 `--force`。

## 3. 推荐方案设计

### 3.1 交付物

```
Deploy/Script/CdeuKline.sh          # 唯一实现（export / import / verify）
Deploy/Script/README.md             # 补充用法（第三节）
```

> 与现有 `Deploy/Script/*.sh` 风格一致：单文件、`--help`、严格 `set -euo pipefail`；
> 随安装包分发到 `<PREFIX>/Deploy/Script/`。

### 3.2 命令

```bash
# 导出（默认 CSV.gz，含 gmt_create/gmt_update，附带 .sha256）
sudo -u finvquant bash CdeuKline.sh export \
  --out /data/App/FinvQuant/Export/CdeuKline

# 按证券/日期过滤导出
sudo -u finvquant bash CdeuKline.sh export \
  --where "usc = 'GCMain' AND date >= 20260101" \
  --out /data/App/FinvQuant/Export/CdeuKline

# 导入（默认 append：冲突跳过，幂等）
sudo -u finvquant bash CdeuKline.sh import \
  /data/App/FinvQuant/Export/CdeuKline/cdeu_kline_20261028_120000.csv.gz

# 覆盖式重建（破坏性，需 --force）
sudo -u finvquant bash CdeuKline.sh import FILE --mode replace --force

# 校验文件（sha256 + gzip 完整性 + 行数）
bash CdeuKline.sh verify FILE
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--out DIR` | `<PREFIX>/Export/CdeuKline` | 导出目录（不存在则创建，0750） |
| `--format csv\|binary` | `csv` | 文件格式 |
| `--where SQL` | 无 | 附加过滤（仅导出） |
| `--mode append\|upsert\|replace` | `append` | 导入模式 |
| `--force` | 关 | `replace` 必须显式确认 |
| `--no-gmt` | 关 | 不导出 `gmt_create/gmt_update`（导入时用 `now()`） |
| `--pg-url URL` | 从 `finvquant.env` 读取 | 覆盖连接串 |

### 3.3 文件命名与目录

```
<OUT>/cdeu_kline_<YYYYMMDD_HHMMSS>.csv.gz
<OUT>/cdeu_kline_<YYYYMMDD_HHMMSS>.csv.gz.sha256
```

- 文件名带时间戳，避免覆盖；
- `.sha256` 与数据文件同名追加后缀，导入前自动校验（`--skip-verify` 可跳过）；
- 目录权限 `finvquant:finvquant 0750`，与 `Export/` 一致。

### 3.4 导出实现（一致性快照）

```sql
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
\copy (
  SELECT usc, ts, date, "time", open, high, low, close, paocd, volume,
         turnover, change_price, change_ratio, ext_field, last_close,
         remark, provider, gmt_create, gmt_update
    FROM cdeu.finv_quote_secu_kline_min
   WHERE <where>
) TO PROGRAM 'gzip -1 > <file>.tmp' WITH (FORMAT csv, HEADER true);
COMMIT;
```

- `REPEATABLE READ READ ONLY` 保证同一快照，避免导出过程中数据漂移；
- `gzip -1` 优先速度（可配 `--level`）；
- 写 `.tmp` 后 `mv` 原子改名，避免半文件；
- 完成后 `gzip -t` + `sha256sum` 生成校验文件。

### 3.5 导入实现

**append（默认）**

```sql
BEGIN;
CREATE TEMP TABLE tmp_cdeu_kline (LIKE cdeu.finv_quote_secu_kline_min) ON COMMIT DROP;
\copy tmp_cdeu_kline FROM PROGRAM 'zcat <file>' WITH (FORMAT csv, HEADER true);
INSERT INTO cdeu.finv_quote_secu_kline_min
SELECT * FROM tmp_cdeu_kline
ON CONFLICT (usc, ts) DO NOTHING;
COMMIT;
```

**upsert**：同上，`DO UPDATE SET` 覆盖除 `usc/ts` 外的业务列与 `gmt_update`。

**replace（`--force`）**

```sql
BEGIN;
TRUNCATE cdeu.finv_quote_secu_kline_min;
\copy cdeu.finv_quote_secu_kline_min FROM PROGRAM 'zcat <file>' WITH (FORMAT csv, HEADER true);
COMMIT;
```

- 临时表 `LIKE` 不带约束/索引，COPY 最快；主表 INSERT 时维护索引；
- 单事务：失败整体回滚，不会留下半套数据；
- 导入前校验 `.sha256` 与 gzip 完整性（`--skip-verify` 可跳过）。

### 3.6 校验

| 阶段 | 检查 |
|---|---|
| 导出后 | `gzip -t`、`wc -l`（行数含表头）、`sha256sum`、打印行数 |
| 导入前 | `.sha256` 比对、`gzip -t` |
| 导入后 | 目标表 `count(*)`（过滤条件时按 `where` 统计）、抽样 `md5`（可选） |

### 3.7 性能预估（参考，实施时实测）

| 数据量 | 导出 CSV.gz | 导入 append | 导入 replace |
|---|---|---|---|
| 100 万行 | 3–8s | 8–20s | 5–10s |
| 280 万行 | 10–25s | 25–60s | 15–30s |
| 1000 万行 | 40–90s | 2–4min | 1–2min |

（取决于磁盘、CPU、`shared_buffers`；binary 格式约快 20–40%。）

### 3.8 安全

- 脚本以 `finvquant` 用户运行，从 `finvquant.env` 读取连接信息（不落命令行）；
- 导出目录限 `0750`，文件 `0640`；
- `replace` 需 `--force`，并在执行前打印目标表与预计影响（行数）；
- 导入前校验 sha256，防止半文件/损坏文件；
- 不导出/导入其它 schema 的同名表，避免误操作。

## 4. 待确认的决策点

| # | 决策点 | 选项 | 建议默认 |
|---|---|---|---|
| 1 | 交付形态 | psql 脚本 / Go CLI / HTTP API | **psql 脚本** |
| 2 | 文件格式 | CSV.gz / binary / 两者 | **CSV.gz 默认，binary 可选** |
| 3 | 导入默认模式 | append / upsert / replace | **append（幂等）** |
| 4 | 是否允许 replace | 需 `--force` / 禁止 | **需 `--force`** |
| 5 | 过滤导出 | 支持 `--where` / 仅全表 | **支持** |
| 6 | 是否导出 gmt 列 | 导出 / `--no-gmt` | **导出** |
| 7 | 文件目录 | `<PREFIX>/Export/CdeuKline` / 自定义 | **默认目录 + `--out`** |
| 8 | 校验文件 | sha256 / 无 | **sha256** |
| 9 | 分片导出（按 usc/年） | 首期支持 / 后续 | **后续** |
| 10 | Go CLI 子命令 | 首期 / 后续 | **后续** |

## 5. 实施计划（批准后）

| 阶段 | 内容 | 产出 |
|---|---|---|
| P1 | `CdeuKline.sh`：export/import/verify + 参数解析 | 脚本 + `--help` |
| P2 | 导入三种模式、sha256/gzip 校验、原子改名 | 脚本完善 |
| P3 | `Deploy/Script/README.md` 用法、`Install.md` 入口 | 文档 |
| P4 | 性能实测（100 万/280 万行）并回填本文档 | 测试记录 |

## 6. 验收标准

```bash
# 导出
bash Deploy/Script/CdeuKline.sh export --out /tmp/kline
ls /tmp/kline/cdeu_kline_*.csv.gz /tmp/kline/cdeu_kline_*.sha256
zcat /tmp/kline/cdeu_kline_*.csv.gz | head -2

# 导入（幂等，跑两次行数不变）
bash Deploy/Script/CdeuKline.sh import /tmp/kline/cdeu_kline_*.csv.gz
bash Deploy/Script/CdeuKline.sh import /tmp/kline/cdeu_kline_*.csv.gz
sudo -u postgres psql -d finv -tAc "select count(*) from cdeu.finv_quote_secu_kline_min"

# replace 需 --force
bash Deploy/Script/CdeuKline.sh import FILE --mode replace            # 应拒绝
bash Deploy/Script/CdeuKline.sh import FILE --mode replace --force    # 通过

# 校验
bash Deploy/Script/CdeuKline.sh verify /tmp/kline/cdeu_kline_*.csv.gz
```

- 导出/导入行数一致；重复 append 不增加行数；
- 失败时整体回滚（表行数不变）；
- 导出期间在线服务无锁表/超时。

## 7. 风险与回滚

| 风险 | 缓解 |
|---|---|
| `replace` 误清生产表 | 必须 `--force`；执行前打印库名/表名/行数；建议先备份 |
| 大文件导入撑爆磁盘/内存 | 流式 `COPY`，不整文件读入内存；导入前检查磁盘空间 |
| 半文件 | `.tmp` + 原子改名 + sha256 |
| 索引维护慢 | `replace` 模式可选 `--fast`（先删索引后重建，仅测试库） |
