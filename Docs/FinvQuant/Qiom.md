# qmio — FinvQuant 数据搬运工具

> 安装位置：`<PREFIX>/bin/qmio`（与 `finvquant` 二进制**同目录**）
> 源码：`cmd/qmio` · 依赖：`internal/meta/finv/quote/mvsv`（MVSV-2）
> 用途：快速导出/导入 `cdeu.finv_quote_secu_kline_min`，用于灌数据、联调、压测。

## 1. 简介

`qmio`（Quick Market IO）是独立小工具，通过命令行完成：

| 子命令 | 作用 |
|---|---|
| `export` | 把 `cdeu.finv_quote_secu_kline_min` 导出为 MVSV-2 文件（默认 gzip） |
| `import` | 从 MVSV-2 文件导入（默认 `upsert`，幂等） |
| `verify` | 校验文件格式、行数与 `.sha256` |

不依赖主服务，可随时执行；导出使用 `REPEATABLE READ READ ONLY` 快照，不影响在线服务。

## 2. 安装

由安装包/安装脚本自动放到 `<PREFIX>/bin/qmio`，并创建软链 `/usr/local/bin/qmio`。
升级/卸载脚本会同步处理 `qmio` 与 `qmio.prev`。

```bash
ls -l /data/App/FinvQuant/bin/qmio
qmio --help
```

## 3. 命令

### 3.1 导出

```bash
qmio export \
  --out /data/App/FinvQuant/Export/CdeuKline \
  [--where "usc = 'GCMain' AND date >= 20260101"] \
  [--no-gmt] [--no-gzip] \
  [--pg-url "postgres://user:pass@127.0.0.1:5432/finv?sslmode=disable"]
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--out DIR` | `/data/App/FinvQuant/Export/CdeuKline` | 导出目录（自动创建） |
| `--where SQL` | 无 | 附加过滤（列名直接写，如 `usc = 'GCMain'`） |
| `--no-gmt` | 关 | 不导出 `gmt_create/gmt_update`（17 列） |
| `--no-gzip` | 关 | 不压缩 |
| `--pg-url` | 从 `FINV_PG_*` 环境变量读取 | 覆盖连接串 |

产物：

```
cdeu_kline_YYYYMMDD_HHMMSS.mvsv.gz
cdeu_kline_YYYYMMDD_HHMMSS.mvsv.gz.sha256
```

### 3.2 导入

```bash
# 默认 upsert（冲突覆盖业务列，幂等）
qmio import /data/App/FinvQuant/Export/CdeuKline/cdeu_kline_*.mvsv.gz

# 只插入不覆盖
qmio import FILE --mode append

# 清空后重建（破坏性，必须 --force）
qmio import FILE --mode replace --force
```

| 参数 | 默认 | 说明 |
|---|---|---|
| `--mode` | `upsert` | `upsert` / `append` / `replace` |
| `--force` | 关 | `replace` 必须显式确认 |
| `--batch` | `1000` | 每批写入行数 |
| `--skip-verify` | 关 | 跳过 `.sha256` 校验 |
| `--pg-url` | 从 `FINV_PG_*` 读取 | 覆盖连接串 |

`upsert` 语义：

- 文件含 `gmt_create/gmt_update`：按文件值写入/覆盖；
- `--no-gmt` 文件：插入用数据库默认值，冲突时刷新 `gmt_update=now()`。

### 3.3 校验

```bash
qmio verify /data/App/FinvQuant/Export/CdeuKline/cdeu_kline_*.mvsv.gz
```

输出 `Format` / `Count` / `Field`；存在 `.sha256` 时自动比对。

## 4. 文件格式

MVSV-2（多证券、单文件全表），字段与列名见
[`Docs/DataFormat/MvsvFileFormat.md`](../DataFormat/MvsvFileFormat.md) 的「MVSV-2」章节。

- 默认 19 列（含 `gmt_*`），`--no-gmt` 为 17 列；
- 导入按文件头 `Field` 的**列名**映射，不依赖列序；
- 旧 `MVSV-1` 文件仍可被既有解析链路读取，本工具只处理 `MVSV-2`。

## 5. 示例流程

```bash
# 1) 从生产导出
qmio export --out /tmp/kline

# 2) 传到测试机后导入（upsert，重复执行安全）
qmio import /tmp/kline/cdeu_kline_*.mvsv.gz

# 3) 校验行数
sudo -u postgres psql -d finv -tAc "select count(*) from cdeu.finv_quote_secu_kline_min"
```

## 6. 注意事项

- `replace` 会 `TRUNCATE` 目标表，执行前会打印库名/表名/当前行数，必须 `--force`；
- 导入默认单事务，失败整体回滚；
- 导出文件含行情数据，目录权限 `0750`、文件 `0640`，注意访问控制；
- 大数据量导入建议在低峰期执行（批量 `1000` 行/批，可按磁盘调整 `--batch`）。
