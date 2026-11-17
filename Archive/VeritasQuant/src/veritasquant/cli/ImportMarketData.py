"""行情导入命令（vq-import-market-data）。

将 MVSV-1 历史行情文件导入 PostgreSQL `finv_quote_secu_kline_min`
（主键 ts+market_code+secu_code，字段级覆盖式更新，V4 迁移）。

用法:
  vq-import-market-data --config Configs/DataImports/NvdaMvsv.yml
  vq-import-market-data --config Configs/DataImports/NvdaMvsv.yml --dry-run
  vq-import-market-data --config Configs/DataImports/NvdaMvsv.yml --input-dir Data/Fixtures/BatchA --mode ROW

退出码: 0 成功；2 参数无效；3 业务失败（无输入/解析/入库失败）。
"""

from __future__ import annotations

import argparse
import logging
import sys
from collections.abc import Sequence
from datetime import datetime, timezone
from pathlib import Path

import yaml

from veritasquant.application.Entrypoints import configureStandardStreams
from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.data.MvsvImport import parseMvsvPath
from veritasquant.data.QuoteRow import UpsertMode
from veritasquant.infrastructure.persistence.QuoteStore import (
    MinuteQuoteStore,
    connectQuoteDb,
)

logger = logging.getLogger("veritasquant.cli.import_market_data")

# src/veritasquant/cli/ImportMarketData.py → 仓库根
_REPO_ROOT = Path(__file__).resolve().parents[3]

_UPSERT_BATCH_ROWS = 5_000  # 单次 executemany 的行数上限


def _parseArguments(argv: Sequence[str] | None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="vq-import-market-data",
        description="导入 VeritasQuant 历史行情（MVSV-1 → PostgreSQL，字段级覆盖式更新）",
    )
    parser.add_argument("--config", required=True, help="导入配置 YAML（Configs/DataImports/*.yml）")
    parser.add_argument("--input-dir", default="", help="覆盖配置中的 MVSV 输入目录")
    parser.add_argument(
        "--mode",
        choices=[UpsertMode.Field, UpsertMode.Row],
        default="",
        help="覆盖模式：FIELD=字段级覆盖（默认）；ROW=整行覆盖",
    )
    parser.add_argument("--dry-run", action="store_true", help="仅预览待处理文件与记录数，不写库")
    parser.add_argument("--imported-by", default="", help="导入人/来源标识（默认取配置或当前用户）")
    parser.add_argument("--dsn", default="", help="PG 连接串（默认取 VQ_POSTGRES_DSN 或 VQ_POSTGRES_* 环境变量）")
    return parser.parse_args(argv)


def _loadConfig(configPath: Path) -> dict:
    """严格加载导入配置（拒绝重复键由 YAML loader 保证基础安全）。"""
    if not configPath.is_file():
        raise FileNotFoundError(f"配置文件不存在: {configPath}")
    with configPath.open("r", encoding="utf-8") as source:
        config = yaml.safe_load(source)
    if not isinstance(config, dict):
        raise ValueError("配置必须为 YAML 映射")
    for key in ("Source", "InputDir"):
        if not config.get(key):
            raise ValueError(f"配置缺少必填项: {key}")
    mode = config.get("UpsertMode", UpsertMode.Field)
    if mode not in (UpsertMode.Field, UpsertMode.Row):
        raise ValueError(f"UpsertMode 必须为 FIELD 或 ROW，实际: {mode}")
    return config


def _batchId(secuCode: str) -> str:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S")
    return f"import_{secuCode}_{stamp}"


def _importFile(
    path: Path,
    config: dict,
    store: MinuteQuoteStore | None,
    importedBy: str,
    mode: str,
    dryRun: bool,
) -> dict:
    """解析单个 MVSV 文件；store 为 None 或 dryRun 时不写库。"""
    result = parseMvsvPath(path)
    rows = result.rows

    summary: dict = {
        "path": str(path),
        "market_code": result.marketCode,
        "secu_code": result.secuCode,
        "count": len(rows),
        "file_sha256": result.contentSha256,
    }
    if dryRun or store is None:
        return summary

    batchId = _batchId(result.secuCode)
    totalUpdated = 0
    for offset in range(0, len(rows), _UPSERT_BATCH_ROWS):
        chunk = rows[offset : offset + _UPSERT_BATCH_ROWS]
        stats = store.upsertRows(
            chunk,
            ingestBatchId=batchId,
            mode=mode,
            reason="MVSV-1 导入（同键覆盖）",
            revisedBy=importedBy,
        )
        totalUpdated += stats["updated"]
    store.registerBatch(
        ingestBatchId=batchId,
        source=str(config["Source"]),
        marketCode=result.marketCode,
        secuCode=result.secuCode,
        dataVersionId=result.contentSha256,
        fileCount=1,
        recordCount=len(rows),
        mode=mode,
        tsPrecision=str(config.get("TsPrecision", "Second")),
        configHash=canonicalHash(config),
        importedBy=importedBy,
        notes=config.get("Notes"),
    )
    summary["batch_id"] = batchId
    summary["updated"] = totalUpdated
    return summary


def main(argv: Sequence[str] | None = None) -> int:
    """行情导入命令入口（vq-import-market-data）。"""
    configureStandardStreams()
    try:
        arguments = _parseArguments(argv)
    except SystemExit as error:
        return error.code if isinstance(error.code, int) else 2

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")

    configPath = Path(arguments.config)
    if not configPath.is_absolute():
        configPath = _REPO_ROOT / configPath

    try:
        config = _loadConfig(configPath)
    except (FileNotFoundError, ValueError) as error:
        logger.error("配置加载失败: %s", error)
        return 3

    inputDir = Path(arguments.input_dir) if arguments.input_dir else Path(config["InputDir"])
    if not inputDir.is_absolute():
        inputDir = _REPO_ROOT / inputDir
    if not inputDir.is_dir():
        logger.error("输入目录不存在: %s", inputDir)
        return 3

    mode = arguments.mode or str(config.get("UpsertMode", UpsertMode.Field))
    importedBy = arguments.imported_by or str(config.get("ImportedBy", "cli"))
    files = sorted(inputDir.glob("*.mvsv"))
    if not files:
        logger.error("输入目录没有 .mvsv 文件: %s", inputDir)
        return 3

    print("=" * 64)
    print("历史行情导入（MVSV-1 → PostgreSQL finv_quote_secu_kline_min）")
    print("=" * 64)
    print(f"配置: {configPath} (hash={canonicalHash(config)[:16]}…)")
    print(f"模式: {mode} | 文件数: {len(files)} | 输入目录: {inputDir}")
    if arguments.dry_run:
        print("🟡 预览模式：仅输出摘要，不写库")
    print()

    store: MinuteQuoteStore | None = None
    connection = None
    if not arguments.dry_run:
        try:
            connection = connectQuoteDb(arguments.dsn or None)
            store = MinuteQuoteStore(connection)
        except Exception as error:
            logger.error("PG 连接失败: %s", error)
            return 3

    failed: list[str] = []
    totalRows = 0
    try:
        for path in files:
            try:
                result = _importFile(path, config, store, importedBy, mode, arguments.dry_run)
            except Exception as error:  # noqa: BLE001 - 单个文件失败不阻断批次
                logger.error("文件处理失败 %s: %s", path, error)
                failed.append(str(path))
                continue
            totalRows += result["count"]
            status = "🟡 预览" if arguments.dry_run else "✅ 已导入"
            print(f"  {status} {result['secu_code']} (market={result['market_code']}): {result['count']} 条"
                  f" | batch={result.get('batch_id', '-')} | 覆盖={result.get('updated', '-')}")
    finally:
        if connection is not None:
            connection.close()

    print()
    print("=" * 64)
    if failed:
        print(f"⚠️ 完成：成功 {len(files) - len(failed)}/{len(files)} 个文件，失败 {len(failed)} 个：{failed}")
        return 3
    print(f"✅ 完成：{len(files)} 个文件，共 {totalRows} 条记录"
          + ("（预览，未写库）" if arguments.dry_run else "已写入 PostgreSQL"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
