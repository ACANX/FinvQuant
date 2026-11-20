package jobmeta

// UploadPendingExportFiles 是文件上传父 JOB；每个文件的动态上传 JOB 复用同一上传逻辑。
var UploadPendingExportFiles = Runtime{
	Kind: "UPLOAD_PENDING_EXPORT_FILES", Interval: 300, Schedule: "全天每 5 分钟",
	Name:    "待上传文件上传",
	Summary: "扫描 PENDING_UPLOAD 的导出文件，上传到独享存储库并校验远端 checksum。",
	Flow:    "领取文件 → 本地大小/SHA256 校验 → 远端 HEAD 查重 → PUT 上传 → Verify 远端 checksum → 置 UPLOADED 并写上传审计。",
	Details: "单轮最多 200 个文件；动态上传 JOB 按 file_id 复用同一执行入口；失败按指数退避重试，超过最大次数后延迟 24 小时并告警。",
	Cautions: "upload_mode=none 时整个 Job 跳过；本地文件被清理或篡改会导致校验失败；重试耗尽需人工 File/Retry；" +
		"远端已存在且 checksum 一致时直接判成功（幂等重传）。",
}
