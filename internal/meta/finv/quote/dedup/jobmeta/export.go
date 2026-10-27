package jobmeta

// RunDueExportJobs 是证券月 BASE/INCR 导出的父 JOB。
// 单证券、单月、单类型（BASE/INCR）的动态 JOB 复用同一套 LOAD → EXPORT → REGISTER → CLEANUP 逻辑，
// 只是入参不同，因此不按证券月份拆文件。
var RunDueExportJobs = Runtime{
	Kind: "RUN_DUE_EXPORT_JOBS", Interval: 60, Windowed: true, Schedule: "低峰窗口内每 60 秒一轮",
	Name:    "到期导出任务执行",
	Summary: "在低峰窗口内生成导出候选、领取证券月任务，执行 LOAD → EXPORT → CLEANUP 全流程。",
	Flow:    "确认窗口与预算 → 生成批次清单/候选 → 行锁领取任务 → LOAD 按日/keyset 分页装载中转表 → EXPORT 写 MVsv + Manifest 并登记导出文件 → CLEANUP 清理中转 → 任务置 EXPORTED。",
	Details: "单轮最多处理 FINV_QUOTE_DEDUB_MAX_JOBS_PER_RUN 个任务；动态 JOB 按 (usc, biz_month) 复用同一执行入口；三阶段短事务、行锁+租约+心跳，支持暂停续跑与多 worker 防重。",
	Cautions: "窗口外触发会直接跳过；预算耗尽会暂停并保存游标；不要在窗口内手工清理中转表；" +
		"装载期间主表行数变化会重装当日并记录日志。",
}
