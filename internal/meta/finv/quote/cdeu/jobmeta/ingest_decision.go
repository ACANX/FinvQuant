package jobmeta

// CheckIngestReadiness 采集阶段：quote 侧数据新鲜度体检（只读）。
var CheckIngestReadiness = Runtime{
	Kind: "CHECK_INGEST_READINESS", DailyAt: "00:03", Schedule: "每日 00:03",
	Name:     "采集就绪检查",
	Summary:  "检查 quote schema 的数据新鲜度：最新交易日、滞后天数、总行数、证券数。",
	Flow:     "读取 quote.finv_quote_secu_kline_min 的 max(date) / max(gmt_create) / count(*) → 计算滞后天数并判断是否已有当月数据。",
	Details:  "采集本身由外部程序负责，本 JOB 只做只读体检，不写入任何数据。",
	Cautions: "滞后天数为 0 说明采集正常；持续大于 1 天应检查外部采集程序是否停摆。",
}

// EvaluateExportCondition 决策阶段：只读评估导出条件与排除原因分布。
var EvaluateExportCondition = Runtime{
	Kind: "EVALUATE_EXPORT_CONDITION", DailyAt: "00:15", Schedule: "每日 00:15",
	Name:     "导出条件判定",
	Summary:  "按与批次生成一致的口径，统计满足 BASE 导出条件的证券月数，以及各排除原因分布。",
	Flow:     "从 finv_dedup_quote_maturity_mon 按 (usc, biz_month) 聚合 → 依次剔除元数据不全 / 已退市 / 当月 / 未成熟 / 已导出 → 剩余即候选。",
	Details:  "只读判定，不写批次表；实际清单由 PHASE_EXPORT 的 JOB_FETCH_EXPORT_BATCH 生成。",
	Cautions: "候选数与后续批次数不一致时，通常是判定与批次生成之间数据发生了变化（如成熟度刷新）。",
}

// ExplainExportDecision 决策阶段：逐条解释导出判定结果（需求 10832）。
var ExplainExportDecision = Runtime{
	Kind: "EXPLAIN_EXPORT_DECISION", DailyAt: "00:16", Schedule: "每日 00:16",
	Name:    "导出决策明细",
	Summary: "对成熟度表里的每个证券月逐条给出「能导出 / 不能导出」以及具体原因、预计行数、文件名、本地与远端路径。",
	Flow: "读取成熟度表与证券主数据 → 依次判定元数据完整性 / 退市 / 当月 / 成熟度 / 已导出 → " +
		"可导出项算出目标文件名与落盘路径；另扫描「行情有数据但成熟度无记录」的盲区月份。",
	Details:  "只读判定，不写批次表、不生成动态 JOB；结果写入运行日志明细，逐条日志可直接定位到证券月。",
	Cautions: "若某个月份在列表里完全没出现，多半是成熟度表没有该月记录（见明细里的「成熟度缺失」清单）。",
}
