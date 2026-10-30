package jobmeta

// All 返回全部归档 JOB 的运行时描述，顺序即调度与页面展示顺序。
func All() []Runtime {
	return []Runtime{
		CalcDailyIngestStats,
		CalcDailyExportBudget,
		RefreshQuoteMaturity,
		MarkSealedJobsFrozen,
		CheckIngestReadiness,
		RunDueExportJobs,
		EvaluateExportCondition,
		ExplainExportDecision,
		UploadPendingExportFiles,
		ScanPostSealPatches,
		// 需求 10717：CleanExpiredExportArtifacts 已停用，职责拆为下列三个 JOB；
		// 需求 10723：新增 PurgeHistoryRuns 清理历史流程运行数据。
		PurgeExportedMonth,
		PurgeLocalExportFiles,
		CleanExportStorage,
		PurgeStaleDynamic,
		PurgeHistoryRuns,
		RecoverTimeoutExportJobs,
		CheckExportJobHealth,
	}
}
