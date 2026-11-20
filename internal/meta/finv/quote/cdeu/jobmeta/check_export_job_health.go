package jobmeta

var CheckExportJobHealth = Runtime{
	Kind: "CHECK_EXPORT_JOB_HEALTH", Interval: 900, Schedule: "每 15 分钟",
	Name:     "归档健康检查",
	Summary:  "汇总归档链路指标、抽检 MVsv 文件结构并输出告警日志。",
	Flow:     "buildMetrics 汇总任务/文件/预算/积压 → 抽检最近 20 个文件（16 行头、空行、Count 行数）→ 按阈值生成告警并写运行日志。",
	Details:  "告警覆盖积压、预算因子、SLA、上传重试耗尽、MVsv 结构异常等；仅写日志，不发送外部通知。",
	Cautions: "MVsv 异常需人工核对文件与磁盘；告警码与阈值见告警码表，处理前不要直接删除异常文件。",
}
