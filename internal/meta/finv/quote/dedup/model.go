// Package dedup 实现证券行情月归档的自动化链路：
// 成熟度扫描 → 导出候选 → LOAD（主表→中转表）→ EXPORT（中转表→MVsv 文件）
// → CLEANUP（清理中转数据）→ 登记导出文件（上传任务唯一数据源）
// → UPLOAD（上传独享存储库并校验 checksum）→ 补丁收口与生命周期清理。
//
// 设计依据：Docs/Refactor/2610-005-FinvQuoteDeDubExportUpload.md
// （方案稿 v5.0：Docs 目录内为落地版说明，本包与之逐条对应）。
//
// 关键约束（不可违反）：
//   - 不修改 cdeu.finv_quote_secu_kline_min 与 export.finv_quote_secu_kline_min 的结构；
//   - 新增表全部在 dedup schema（finv_dedup_* 前缀），全库无外键；
//   - 导出与上传文件格式均为 .mvsv（16 行文件头 + 1 行空行 + 管道分隔数据行），不生成 .csv；
//   - 所有 SQL 必须写 schema 全限定名（见 Docs/DevSpec/DbSchemaSpec.md §3）；
//   - 单任务三阶段各自短事务，支持暂停续跑、多 worker 安全领取（行锁 + 租约 + 心跳）。
package dedup

import (
	"strings"
	"time"
)

// Job 类型标识（全大写、动词 + 对象、含义具体，与方案稿 2.2 节一致）。
const (
	// JobCalcDailyIngestStats 每日 00:05：按 gmt_create 聚合主表入口流量，写入 finv_dedup_ingest_stats。
	JobCalcDailyIngestStats = "CALC_DAILY_INGEST_STATS"
	// JobCalcDailyExportBudget 每日 00:05：计算当日导出预算，写入 finv_dedup_daily_budget。
	JobCalcDailyExportBudget = "CALC_DAILY_EXPORT_BUDGET"
	// JobRefreshQuoteMaturity 每日 00:10：增量扫描主表刷新成熟度，并自动发现新 usc。
	JobRefreshQuoteMaturity = "REFRESH_QUOTE_MATURITY"
	// JobMarkSealedJobsFrozen 每日 00:20：到达 seal_date 的证券月收口冻结，输出差异报告。
	JobMarkSealedJobsFrozen = "MARK_SEALED_JOBS_FROZEN"
	// JobRunDueExportJobs 低峰窗口内：生成候选、领取任务并执行 LOAD→EXPORT→CLEANUP。
	JobRunDueExportJobs = "RUN_DUE_EXPORT_JOBS"
	// JobUploadPendingExportFiles 全天：扫描待上传文件并上传独享存储库、校验远端 checksum。
	JobUploadPendingExportFiles = "UPLOAD_PENDING_EXPORT_FILES"
	// JobScanPostSealPatches 每日：扫描收口后到达的数据，生成 L1/L2/L3 补丁。
	JobScanPostSealPatches = "SCAN_POST_SEAL_PATCHES"
	// JobCleanExpiredExportArtifacts 每日：清理超期中转残留、过期本地文件与主表旧数据。
	//
	// Deprecated（需求 10717）：自 V200048 起已拆为 JobCleanExportStorage（存储）、
	// JobPurgeExportedMonth（主表）、JobPurgeStaleDynamic（动态实例）三个独立 JOB，
	// 并整体迁至 PHASE_REVIEW（归档复核）。保留常量仅供历史运行记录追溯。
	JobCleanExpiredExportArtifacts = "CLEAN_EXPIRED_EXPORT_ARTIFACTS"
	// JobRecoverTimeoutExportJobs 每 5 分钟：回收心跳/租约超时的 RUNNING 任务。
	JobRecoverTimeoutExportJobs = "RECOVER_TIMEOUT_EXPORT_JOBS"
	// JobCheckExportJobHealth 每 15 分钟：检查积压、预算因子、SLA、上传重试、MVsv 结构并告警。
	JobCheckExportJobHealth = "CHECK_EXPORT_JOB_HEALTH"

	// 以下四个 JOB 均属 PHASE_REVIEW（归档复核），见需求 10716–10723。
	//
	// JobPurgeExportedMonth 归档复核：扫描「全部文件已上传」的证券月，按 20007 释放 dedup 主表数据。
	// 扫描式判据同时覆盖「本次上传完成」与「历史遗留已上传未清理」，不再区分两者。
	JobPurgeExportedMonth = "PURGE_EXPORTED_MONTH"
	// JobPurgeLocalExportFiles 归档复核：按 20011 清理本地导出的 .mvsv / Manifest 文件（需求 10744）。
	// 排在 JobPurgeExportedMonth 之后执行；判据为「该月全部文件上传成功且超过 24 小时」。
	JobPurgeLocalExportFiles = "PURGE_LOCAL_EXPORT_FILES"
	// JobCleanExportStorage 归档复核：清理中转表残留与保留期到期的本地导出文件。
	JobCleanExportStorage = "CLEAN_EXPORT_STORAGE"
	// JobPurgeStaleDynamic 归档复核：按 20008 保留期清理历史动态 JOB 实例。
	JobPurgeStaleDynamic = "PURGE_STALE_DYNAMIC"
	// JobPurgeHistoryRuns 归档复核：按 20009/20010 策略清理历史流程运行数据（workflow_run 及其子表）。
	JobPurgeHistoryRuns = "PURGE_HISTORY_RUNS"

	// JobCheckIngestReadiness 采集阶段：检查 quote 侧数据新鲜度（需求 10741）。
	// 采集本身由外部程序负责，本 JOB 只做只读体检，把「最新交易日/滞后天数/总行数」写入运行明细。
	JobCheckIngestReadiness = "CHECK_INGEST_READINESS"
	// JobEvaluateExportCondition 决策阶段：只读评估哪些证券月满足 BASE 导出条件，
	// 并给出各排除原因的分布（需求 10741）。
	// 判定口径与 GenerateExportBatch 的 BASE 分支一致，只是不落批次表——
	// 让「导出条件判定」从内嵌逻辑变成可观测的独立步骤。
	JobEvaluateExportCondition = "EVALUATE_EXPORT_CONDITION"
	// JobExplainExportDecision 决策阶段：逐条解释「哪些证券月能导出、哪些不能、为什么不能」，
	// 并给出可导出项的文件名、本地路径、远端路径（需求 10832）。
	// 与 EVALUATE_EXPORT_CONDITION 的分工：后者只给各原因的数量汇总，
	// 前者给到「哪个证券月、什么原因、缺哪个字段、差哪几天、会生成什么文件」。
	JobExplainExportDecision = "EXPLAIN_EXPORT_DECISION"
)

// AllJobKinds 返回全部 Job 标识（顺序与方案稿 2.2 节编排一致）。
//
// 需求 10717：CLEAN_EXPIRED_EXPORT_ARTIFACTS 已停用，不再登记调度；
// 其职责由 PURGE_EXPORTED_MONTH / CLEAN_EXPORT_STORAGE / PURGE_STALE_DYNAMIC 接管。
func AllJobKinds() []string {
	return []string{
		JobCalcDailyIngestStats,
		JobCalcDailyExportBudget,
		JobRefreshQuoteMaturity,
		JobMarkSealedJobsFrozen,
		JobRunDueExportJobs,
		JobUploadPendingExportFiles,
		JobScanPostSealPatches,
		JobCheckIngestReadiness,
		JobEvaluateExportCondition,
		JobExplainExportDecision,
		JobPurgeExportedMonth,
		JobPurgeLocalExportFiles,
		JobCleanExportStorage,
		JobPurgeStaleDynamic,
		JobPurgeHistoryRuns,
		JobRecoverTimeoutExportJobs,
		JobCheckExportJobHealth,
	}
}

// 导出任务状态（finv_dedup_export_job.status）。
const (
	JobStatusWaiting   = "WAITING"
	JobStatusLoading   = "LOADING"
	JobStatusLoaded    = "LOADED"
	JobStatusExporting = "EXPORTING"
	JobStatusExported  = "EXPORTED"
	JobStatusPaused    = "PAUSED"
	JobStatusFailed    = "FAILED"
	JobStatusFrozen    = "FROZEN"
)

// 数据成熟度状态（finv_dedup_quote_maturity_mon.maturity_status）。
const (
	MaturityImmature = "IMMATURE"
	MaturityPartial  = "PARTIAL"
	MaturityComplete = "COMPLETE"
	MaturityFrozen   = "FROZEN"
)

// 导出文件类型（finv_dedup_export_file.file_type）。
const (
	FileTypeBase     = "BASE"
	FileTypePatch    = "PATCH"
	FileTypeManifest = "MANIFEST"
)

// 导出文件的导出状态（finv_dedup_export_file.export_status）。
const (
	ExportStatusExported   = "EXPORTED"
	ExportStatusSuperseded = "SUPERSEDED"
	ExportStatusSkipped    = "SKIPPED"
)

// 上传状态（finv_dedup_export_file.upload_status）。
const (
	UploadStatusPending   = "PENDING_UPLOAD"
	UploadStatusUploading = "UPLOADING"
	UploadStatusUploaded  = "UPLOADED"
	UploadStatusFailed    = "UPLOAD_FAILED"
	UploadStatusSkipped   = "SKIPPED"
)

// 网盘归档状态（finv_dedup_export_file.network_status）。
const (
	NetworkStatusNone     = "NONE"
	NetworkStatusArchived = "ARCHIVED"
)

// 本地清理状态（finv_dedup_export_file.cleanup_status）。
const (
	CleanupStatusNone         = "NONE"
	CleanupStatusLocalDeleted = "LOCAL_DELETED"
)

// 补丁级别（finv_dedup_export_file_patch.patch_level）。
const (
	PatchLevelL1Hot  = "L1_HOT"
	PatchLevelL2Warm = "L2_WARM"
	PatchLevelL3Cold = "L3_COLD"
)

// 补丁任务状态（finv_dedup_export_file_patch.status）。
const (
	PatchStatusPending = "PENDING"
	PatchStatusRunning = "RUNNING"
	PatchStatusDone    = "DONE"
	PatchStatusFailed  = "FAILED"
)

// 补丁原因（finv_dedup_export_file_patch.reason）。
const (
	PatchReasonLateArrival   = "LATE_ARRIVAL"
	PatchReasonUpstreamDelay = "UPSTREAM_DELAY"
	PatchReasonCorrection    = "CORRECTION"
)

// RunStatus 运行日志结果（finv_dedup_job_run_log_export_file.status）。
const (
	RunStatusSuccess = "SUCCESS"
	RunStatusFailed  = "FAILED"
	RunStatusPaused  = "PAUSED"
)

// Job 证券月导出任务（cdeu.finv_quote_cdeu_export_job 一行）。
type Job struct {
	JobID             int64      `db:"job_id" json:"job_id"`
	Usc               string     `db:"usc" json:"usc"`
	BizMonth          int        `db:"biz_month" json:"biz_month"`
	PriorityRank      int        `db:"priority_rank" json:"priority_rank"`
	FreezeDate        time.Time  `db:"freeze_date" json:"freeze_date"`
	SealDate          time.Time  `db:"seal_date" json:"seal_date"`
	Status            string     `db:"status" json:"status"`
	LoadCursorDate    *int       `db:"load_cursor_date" json:"load_cursor_date"`
	LoadCursorTS      *int64     `db:"load_cursor_ts" json:"load_cursor_ts"`
	LoadedRows        int64      `db:"loaded_rows" json:"loaded_rows"`
	BaseVersion       int        `db:"base_version" json:"base_version"`
	PatchCount        int        `db:"patch_count" json:"patch_count"`
	OldestPendingDate *int       `db:"oldest_pending_date" json:"oldest_pending_date"`
	LockedBy          *string    `db:"locked_by" json:"locked_by"`
	LockedUntil       *time.Time `db:"locked_until" json:"locked_until"`
	GmtHeartbeat      *time.Time `db:"gmt_heartbeat" json:"gmt_heartbeat"`
	Attempt           int        `db:"attempt" json:"attempt"`
	GmtNextRun        *time.Time `db:"gmt_next_run" json:"gmt_next_run"`
	LastError         *string    `db:"last_error" json:"last_error"`
	GmtCreate         time.Time  `db:"gmt_create" json:"gmt_create"`
	GmtUpdate         time.Time  `db:"gmt_update" json:"gmt_update"`
}

// File 导出文件登记（cdeu.finv_quote_cdeu_export_file 一行）。
type File struct {
	FileID                 int64      `db:"file_id" json:"file_id"`
	JobID                  int64      `db:"job_id" json:"job_id"`
	Usc                    string     `db:"usc" json:"usc"`
	BizMonth               int        `db:"biz_month" json:"biz_month"`
	FileType               string     `db:"file_type" json:"file_type"`
	Version                int        `db:"version" json:"version"`
	FileName               string     `db:"file_name" json:"file_name"`
	FilePath               string     `db:"file_path" json:"file_path"`
	TargetFileName         string     `db:"target_file_name" json:"target_file_name"`
	UploadTarget           *string    `db:"upload_target" json:"upload_target"`
	Format                 string     `db:"format" json:"format"`
	Compression            string     `db:"compression" json:"compression"`
	RowCount               int64      `db:"row_count" json:"row_count"`
	FileSize               int64      `db:"file_size" json:"file_size"`
	Checksum               string     `db:"checksum" json:"checksum"`
	ManifestPath           *string    `db:"manifest_path" json:"manifest_path"`
	SealDate               *time.Time `db:"seal_date" json:"seal_date"`
	ExportStatus           string     `db:"export_status" json:"export_status"`
	UploadStatus           string     `db:"upload_status" json:"upload_status"`
	UploadAttempt          int        `db:"upload_attempt" json:"upload_attempt"`
	GmtNextRetry           *time.Time `db:"gmt_next_retry" json:"gmt_next_retry"`
	GmtUploadStart         *time.Time `db:"gmt_upload_start" json:"gmt_upload_start"`
	GmtUploadFinish        *time.Time `db:"gmt_upload_finish" json:"gmt_upload_finish"`
	RemoteChecksumVerified bool       `db:"remote_checksum_verified" json:"remote_checksum_verified"`
	NetworkStatus          string     `db:"network_status" json:"network_status"`
	CleanupStatus          string     `db:"cleanup_status" json:"cleanup_status"`
	GmtCreate              time.Time  `db:"gmt_create" json:"gmt_create"`
	GmtUpdate              time.Time  `db:"gmt_update" json:"gmt_update"`
}

// UscInfo 证券主数据（MVsv 文件头与文件名的来源）。
type UscInfo struct {
	Usc              string
	Category         *string
	DayExpectRows    *int
	DateList         *time.Time
	DateDelist       *time.Time
	FlagAutoDiscover bool
	Region           *string
	Market           *string
	Provider         *string
	Name             *string
	Timezone         *string
	Currency         *string
}

// Complete 判断文件头与文件名所需的元数据是否齐备（缺失则不生成导出候选）。
func (u *UscInfo) Complete() bool {
	if u == nil {
		return false
	}
	for _, value := range []*string{u.Region, u.Market, u.Provider, u.Name, u.Timezone, u.Currency} {
		if value == nil || strings.TrimSpace(*value) == "" {
			return false
		}
	}
	return true
}

// Meta 转换为 MVsv 文件头 / 文件名所需的元数据（调用前须 Complete() 为真）。
func (u *UscInfo) Meta(bizMonth int) FileMeta {
	meta := FileMeta{Usc: u.Usc, BizMonth: bizMonth}
	if u.Region != nil {
		meta.Region = *u.Region
	}
	if u.Market != nil {
		meta.Market = *u.Market
	}
	if u.Provider != nil {
		meta.Provider = *u.Provider
	}
	if u.Name != nil {
		meta.Name = *u.Name
	}
	if u.Timezone != nil {
		meta.Timezone = *u.Timezone
	}
	if u.Currency != nil {
		meta.Currency = *u.Currency
	}
	return meta
}

// Budget 每日预算快照。
type Budget struct {
	BizDate       time.Time `json:"biz_date"`
	BaseRows      int64     `json:"base_rows"`
	BaseBytes     int64     `json:"base_bytes"`
	P75Rows       int64     `json:"p75_rows"`
	BacklogRatio  float64   `json:"backlog_ratio"`
	SLAPressure   float64   `json:"sla_pressure"`
	Factor        float64   `json:"factor"`
	BudgetRows    int64     `json:"budget_rows"`
	BudgetBytes   int64     `json:"budget_bytes"`
	HardCapRows   int64     `json:"hard_cap_rows"`
	ConsumedRows  int64     `json:"consumed_rows"`
	ConsumedBytes int64     `json:"consumed_bytes"`
	Status        string    `json:"status"`
}

// RemainingRows 返回当日剩余可用预算条数（不小于 0）。
func (b *Budget) RemainingRows() int64 {
	if b == nil {
		return 0
	}
	remaining := b.BudgetRows - b.ConsumedRows
	if remaining < 0 {
		return 0
	}
	return remaining
}

// LoadPageResult LOAD 单页结果（一条 SQL 同时返回分页上界、页行数与实际插入行数）。
type LoadPageResult struct {
	MaxTS        int64 // 本页最大 ts；无数据时等于入参 ts
	PageRows     int64 // 本页从主表读到的行数
	InsertedRows int64 // 实际写入中转表的行数（ON CONFLICT 命中时小于 PageRows）
}

// DTO 分页查询过滤。
type JobFilter struct {
	Status   string
	Usc      string
	BizMonth int
	Page     int
	PageSize int
}

// FileFilter 文件分页查询过滤。
type FileFilter struct {
	UploadStatus string
	FileType     string
	Usc          string
	BizMonth     int
	Page         int
	PageSize     int
}

// Page 通用分页返回。
type Page[T any] struct {
	Total int64 `json:"total"`
	List  []T   `json:"list"`
}

// DiffRow FROZEN 差异报告一行（缺失日期 / 缺失行数 / 缺失比例）。
type DiffRow struct {
	Usc          string  `json:"usc"`
	BizMonth     int     `json:"biz_month"`
	ExpectedDays int64   `json:"expected_days"`
	MissingDays  int64   `json:"missing_days"`
	ExpectedRows int64   `json:"expected_rows"`
	ActualRows   int64   `json:"actual_rows"`
	MissingRows  int64   `json:"missing_rows"`
	MissingRatio float64 `json:"missing_ratio"`
}

// PatchCandidate 待生成补丁的证券月。
type PatchCandidate struct {
	JobID       int64
	Usc         string
	BizMonth    int
	SealDate    time.Time
	BaseVersion int
	PatchCount  int
}

// FileMeta MVsv 文件头与文件名的元数据来源（cdeu.finv_quote_cdeu_usc）。
type FileMeta struct {
	Usc      string
	BizMonth int
	Region   string
	Market   string
	Provider string
	Name     string
	Timezone string
	Currency string
}
