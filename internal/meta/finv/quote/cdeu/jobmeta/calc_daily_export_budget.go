package jobmeta

var CalcDailyExportBudget = Runtime{
	Kind: "CALC_DAILY_EXPORT_BUDGET", DailyAt: "00:10", Schedule: "每日 00:10",
	Name:     "每日导出预算计算",
	Summary:  "结合近 30 天入口基线、积压天数与 SLA 压力，计算当日导出预算与硬上限。",
	Flow:     "读取入口基线 / 待装载行数 / SLA 压力 → factor=clamp(1.20+0.20*sla+0.05*min(backlog,5),1.20,2.00) → budget=min(p75,base*factor) → 写入 finv_dedup_daily_budget。",
	Details:  "每日一条预算记录；导出过程中由 RUN_DUE_EXPORT_JOBS 累加 consumed_rows / consumed_bytes。",
	Cautions: "预算耗尽会令导出任务暂停并保存游标，次日续跑；管理员可通过 Budget/Override 临时覆盖当日预算。",
}
