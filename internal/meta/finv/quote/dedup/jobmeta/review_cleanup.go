package jobmeta

// PurgeExportedMonth 归档复核阶段：把「远端 BASE 已完整交付」的证券月从 dedup 主表释放。
var PurgeExportedMonth = Runtime{
	Kind: "PURGE_EXPORTED_MONTH", DailyAt: "06:10", Schedule: "每日 06:10",
	Name:     "已归档行情数据清理",
	Summary:  "扫描「全部文件已上传」的证券月，按系统配置 20007 删除 dedup 主表中的该月行情数据。",
	Flow:     "聚合 finv_dedup_export_file → 筛出 BASE 与全部 PATCH 均已 UPLOADED 且主表仍有数据的证券月 → 逐月分批删除。",
	Details:  "扫描式判据天然覆盖「本次上传完成」与「历史遗留已上传未清理」两种来源；20007='0' 时整个 JOB 跳过。",
	Cautions: "只删 dedup schema（加工侧缓冲区），quote schema 的回测副本不动；删除不可逆，执行前应确认远端文件校验通过。",
}

// PurgeLocalExportFiles 归档复核阶段：按 20011 清理本地导出文件。
var PurgeLocalExportFiles = Runtime{
	Kind: "PURGE_LOCAL_EXPORT_FILES", DailyAt: "06:15", Schedule: "每日 06:15",
	Name:     "本地导出文件清理",
	Summary:  "按系统配置 20011 删除已交付证券月的本地 .mvsv 与 Manifest 文件。",
	Flow:     "筛出「BASE+PATCH 全部 UPLOADED 且远端校验通过、最后一个文件上传完成已超 24 小时、无待处理补丁」的证券月 → 逐文件 os.Remove → 标记 cleanup_status=LOCAL_DELETED。",
	Details:  "20011='0' 时整个 JOB 跳过；单轮最多 500 个文件；文件不存在视为已清理（幂等）。",
	Cautions: "删除后无法从本机恢复，但远端仓库仍有副本；24 小时延迟用于留出人工干预窗口。",
}

// CleanExportStorage 归档复核阶段：只清理中转表残留。
var CleanExportStorage = Runtime{
	Kind: "CLEAN_EXPORT_STORAGE", DailyAt: "06:20", Schedule: "每日 06:20",
	Name:     "中转表残留清理",
	Summary:  "清理 export schema 中转表中没有对应文件登记的残留行。",
	Flow:     "删除无主中转数据 → 中转表为空则 TRUNCATE。",
	Details:  "本地文件清理已迁至 PURGE_LOCAL_EXPORT_FILES（需求 10744）。",
	Cautions: "只清中转表，不影响已登记文件与主表数据。",
}

// PurgeStaleDynamic 归档复核阶段：清理历史运行动态实例。
var PurgeStaleDynamic = Runtime{
	Kind: "PURGE_STALE_DYNAMIC", DailyAt: "06:30", Schedule: "每日 06:30",
	Name:     "历史动态JOB实例清理",
	Summary:  "按系统配置 20008 的保留期清理 cdeu_job_dynamic 历史实例。",
	Flow:     "筛选「所属 workflow_run 已终态 + 实例自身已终态 + 早于保留期」→ 分批删除。",
	Details:  "单轮最多 5000 条；保留天数取自 20008（默认 30，越界回退默认）。",
	Cautions: "只删终态实例，运行中实例永不删除。",
}

// PurgeHistoryRuns 归档复核阶段：按策略清理历史流程运行数据。
var PurgeHistoryRuns = Runtime{
	Kind: "PURGE_HISTORY_RUNS", DailyAt: "06:40", Schedule: "每日 06:40",
	Name:     "历史流程运行数据清理",
	Summary:  "按系统配置 20009/20010 的策略（天数或保留个数）整体清理 workflow_run 及其全部子表。",
	Flow:     "选目标 workflow_run → 依次删 job_step_dynamic / job_dynamic / job_step_run / job_run / process_run / phase_run / 运行日志 / workflow_run。",
	Details:  "策略 20009：Off / Days / Count；阈值 20010 默认 30。永远保留最近 1 个 run。",
	Cautions: "孤儿 run（cdeu_job_run 有记录但 cdeu_workflow_run 已缺失）也会按 workflow_run_id 一并清理；删除不可逆。",
}
