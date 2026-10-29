package jobmeta

var CalcDailyIngestStats = Runtime{
	Kind: "CALC_DAILY_INGEST_STATS", DailyAt: "00:05", Schedule: "每日 00:05",
	Name:    "每日入口流量统计",
	Summary: "按 gmt_create 聚合主表近期入口流量，写入 cdeu.finv_quote_cdeu_ingest_stats，为每日导出预算提供基线。",
	Flow:    "读取主表 → 按业务时区日期分组统计行数与字节数 → 幂等 upsert 每日统计（同一天重跑覆盖）。",
	Details: "覆盖近 30 天窗口；统计结果包含 total_rows / total_bytes / gmt_update。",
	Cautions: "依赖 FINV_QUOTE_DEDUB_TIMEZONE 的时区换算；统计缺失时预算计算会按 20 万行兜底，" +
		"需检查主表 gmt_create 是否正常写入。",
}
