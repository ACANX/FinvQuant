package jobmeta

var MarkSealedJobsFrozen = Runtime{
	Kind: "MARK_SEALED_JOBS_FROZEN", DailyAt: "00:20", Schedule: "每日 00:20",
	Name:     "收口冻结与差异报告生成",
	Summary:  "到达 seal_date 的证券月冻结，输出 FROZEN 差异报告（缺失日期 / 行数 / 比例）。",
	Flow:     "推导最大已收口月份 → 冻结对应成熟度 → 任务状态置 FROZEN → 统计最近 6 个月差异并写运行日志。",
	Details:  "差异报告最多输出 200 行；冻结后该证券月视为不可变。",
	Cautions: "冻结后新到达数据只能通过补丁链路（SCAN_POST_SEAL_PATCHES）补入；缺失比例超过 5% 会触发告警。",
}
