package jobmeta

var RecoverTimeoutExportJobs = Runtime{
	Kind: "RECOVER_TIMEOUT_EXPORT_JOBS", Interval: 300, Schedule: "每 5 分钟",
	Name:     "超时任务回收",
	Summary:  "回收心跳/租约超时的 RUNNING 导出任务与长时间上传中的文件，避免任务永久卡死。",
	Flow:     "按 locked_until / heartbeat 判定超时 → 任务重新排队或标记失败 → 上传记录置 UPLOAD_FAILED 并安排退避重试。",
	Details:  "上传超时阈值取租约的 4 倍且不少于 10 分钟。",
	Cautions: "频繁触发说明单任务过重或数据库/网络变慢，应先查指标定位原因，而不是只提高租约阈值。",
}
