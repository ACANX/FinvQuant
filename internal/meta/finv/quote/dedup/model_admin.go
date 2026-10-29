package dedup

import "time"

// 本文件定义归档运维台（查询 + 管理员编辑）所需的 DTO。
// 字段与 cdeu.finv_quote_cdeu_* 表列一一对应（时间列统一用 time.Time，可空列用指针）。

// AdminFilter 运维台通用分页过滤条件（各接口按需取用其中字段）。
type AdminFilter struct {
	Usc      string // 证券代码精确匹配
	Keyword  string // 模糊匹配（如 usc / 文件名 / 错误信息）
	Status   string // 状态精确匹配
	Kind     string // Job 类型精确匹配
	BizMonth int    // 业务月份 yyyymm
	FileID   int64  // 文件 ID
	JobID    int64  // 任务 ID
	DateFrom string // yyyy-MM-dd（含）
	DateTo   string // yyyy-MM-dd（含）
	Page     int
	PageSize int
}

// IngestStat 每日入口流量统计（cdeu.finv_quote_cdeu_ingest_stats）。
type IngestStat struct {
	StatDate   time.Time `db:"stat_date" json:"stat_date"`
	TotalRows  int64     `db:"total_rows" json:"total_rows"`
	TotalBytes int64     `db:"total_bytes" json:"total_bytes"`
	GmtUpdate  time.Time `db:"gmt_update" json:"gmt_update"`
}

// DailyBudget 每日导出预算（cdeu.finv_quote_cdeu_daily_budget）。
type DailyBudget struct {
	BizDate       time.Time `db:"biz_date" json:"biz_date"`
	BaseRows      int64     `db:"base_rows" json:"base_rows"`
	BaseBytes     int64     `db:"base_bytes" json:"base_bytes"`
	P75Rows       *int64    `db:"p75_rows" json:"p75_rows"`
	BacklogRatio  *float64  `db:"backlog_ratio" json:"backlog_ratio"`
	SlaPressure   *float64  `db:"sla_pressure" json:"sla_pressure"`
	Factor        float64   `db:"factor" json:"factor"`
	BudgetRows    int64     `db:"budget_rows" json:"budget_rows"`
	BudgetBytes   int64     `db:"budget_bytes" json:"budget_bytes"`
	HardCapRows   int64     `db:"hard_cap_rows" json:"hard_cap_rows"`
	ConsumedRows  int64     `db:"consumed_rows" json:"consumed_rows"`
	ConsumedBytes int64     `db:"consumed_bytes" json:"consumed_bytes"`
	Status        string    `db:"status" json:"status"`
	GmtCreate     time.Time `db:"gmt_create" json:"gmt_create"`
}

// MaturityRow 证券日成熟度（cdeu.finv_quote_cdeu_quote_maturity_mon）。
type MaturityRow struct {
	Usc            string     `db:"usc" json:"usc"`
	TradeDate      int        `db:"trade_date" json:"trade_date"`
	BizMonth       int        `db:"biz_month" json:"biz_month"`
	ExpectedRows   int        `db:"expected_rows" json:"expected_rows"`
	ActualRows     int        `db:"actual_rows" json:"actual_rows"`
	MaturityStatus string     `db:"maturity_status" json:"maturity_status"`
	GmtFirstSeen   *time.Time `db:"gmt_first_seen" json:"gmt_first_seen"`
	GmtLastArrival *time.Time `db:"gmt_last_arrival" json:"gmt_last_arrival"`
	GmtSeale       *time.Time `db:"gmt_seale" json:"gmt_seale"`
	GmtUpdate      time.Time  `db:"gmt_update" json:"gmt_update"`
}

// WatermarkRow 扫描水位（cdeu.finv_quote_cdeu_scan_watermark）。
type WatermarkRow struct {
	WatermarkName string    `db:"watermark_name" json:"watermark_name"`
	LastValue     time.Time `db:"last_value" json:"last_value"`
	GmtUpdate     time.Time `db:"gmt_update" json:"gmt_update"`
}

// PriorityRow 证券优先级与 SLA（V200051 起数据源为 cdeu.finv_quote_cdeu_usc）。
//
// 只包含已配置覆盖的证券；未配置的证券不在此列表出现（在 UscRow 里为 NULL）。
type PriorityRow struct {
	Usc               string    `db:"usc" json:"usc"`
	PriorityRank      int       `db:"priority_rank" json:"priority_rank"`
	SlaDays           int       `db:"sla_days" json:"sla_days"`
	MaxWaitDays       int       `db:"max_wait_days" json:"max_wait_days"`
	PriorityGmtUpdate time.Time `db:"priority_gmt_update" json:"priority_gmt_update"`
}

// CalendarRow 交易日历（cdeu.finv_quote_cdeu_trade_calendar）。
type CalendarRow struct {
	Usc          string `db:"usc" json:"usc"`
	TradeDate    int    `db:"trade_date" json:"trade_date"`
	FlagTradeDay bool   `db:"flag_trade_day" json:"flag_trade_day"`
}

// UscRow 证券主数据（cdeu.finv_quote_cdeu_usc）。
type UscRow struct {
	Usc              string     `db:"usc" json:"usc"`
	Category         *string    `db:"category" json:"category"`
	DayExpectRows    *int       `db:"day_expect_rows" json:"day_expect_rows"`
	DateList         *time.Time `db:"date_list" json:"date_list"`
	DateDelist       *time.Time `db:"date_delist" json:"date_delist"`
	FlagAutoDiscover bool       `db:"flag_auto_discover" json:"flag_auto_discover"`
	Region           *string    `db:"region" json:"region"`
	Market           *string    `db:"market" json:"market"`
	Provider         *string    `db:"provider" json:"provider"`
	Name             *string    `db:"name" json:"name"`
	Timezone         *string    `db:"timezone" json:"timezone"`
	Currency         *string    `db:"currency" json:"currency"`
	// 归档优先级覆盖配置（V100051 由 finv_dedup_usc_priority 合并而来）：
	// 三列同时为 NULL 表示未配置，分别按 100 / 5 / 7 兜底。
	PriorityRank      *int       `db:"priority_rank" json:"priority_rank"`
	SlaDays           *int       `db:"sla_days" json:"sla_days"`
	MaxWaitDays       *int       `db:"max_wait_days" json:"max_wait_days"`
	PriorityGmtUpdate *time.Time `db:"priority_gmt_update" json:"priority_gmt_update"`
	GmtCreate         time.Time  `db:"gmt_create" json:"gmt_create"`
	GmtUpdate         time.Time  `db:"gmt_update" json:"gmt_update"`
}

// RunLogRow Job 运行日志（cdeu.finv_quote_cdeu_job_run_log_export_file）。
type RunLogRow struct {
	RunID           int64      `db:"run_id" json:"run_id"`
	JobKind         string     `db:"job_kind" json:"job_kind"`
	JobID           *int64     `db:"job_id" json:"job_id"`
	Usc             *string    `db:"usc" json:"usc"`
	BizMonth        *int       `db:"biz_month" json:"biz_month"`
	GmtStart        time.Time  `db:"gmt_start" json:"gmt_start"`
	GmtFinish       *time.Time `db:"gmt_finish" json:"gmt_finish"`
	Status          *string    `db:"status" json:"status"`
	ProcessedRows   *int64     `db:"processed_rows" json:"processed_rows"`
	ProcessedBytes  *int64     `db:"processed_bytes" json:"processed_bytes"`
	Error           *string    `db:"error" json:"error"`
	Stage           *string    `db:"stage" json:"stage"`
	ProgressPercent *float64   `db:"progress_percent" json:"progress_percent"`
	ProgressDetail  *string    `db:"progress_detail" json:"progress_detail"`
	GmtHeartbeat    *time.Time `db:"gmt_heartbeat" json:"gmt_heartbeat"`
}

// RunLogLine Job 运行详细日志行（cdeu.finv_quote_cdeu_job_run_log_line）。
type RunLogLine struct {
	LineID          int64     `db:"line_id" json:"line_id"`
	RunID           int64     `db:"run_id" json:"run_id"`
	Level           string    `db:"level" json:"level"`
	Stage           *string   `db:"stage" json:"stage"`
	Message         string    `db:"message" json:"message"`
	ProgressPercent *float64  `db:"progress_percent" json:"progress_percent"`
	GmtCreate       time.Time `db:"gmt_create" json:"gmt_create"`
}

// RunLogDetail Job 运行汇总 + 详细日志行。
type RunLogDetail struct {
	RunLogRow
	Lines []RunLogLine `json:"lines"`
}

// JobRuntimeState 单个 Job 的运行快照（总览页可视化：是否在跑、当前阶段、最近结果与 24 小时统计）。
type JobRuntimeState struct {
	JobKind    string `json:"job_kind"`
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Flow       string `json:"flow"`
	Details    string `json:"details"`
	Cautions   string `json:"cautions"`
	Schedule   string `json:"schedule"`
	DailyAt    string `json:"daily_at,omitempty"`
	Interval   int    `json:"interval_seconds,omitempty"`
	Windowed   bool   `json:"windowed"`
	InProcess  bool   `json:"in_process"`
	ElapsedSec int64  `json:"elapsed_seconds"`
	// Stale 为 true 表示存在未结束记录但心跳已超时（进程崩溃 / 卡住），需人工关注。
	Stale           bool       `json:"stale"`
	HeartbeatAgeSec *int64     `json:"heartbeat_age_seconds"`
	Running         *RunLogRow `json:"running,omitempty"`
	Last            *RunLogRow `json:"last,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at"`
	LastFailureAt   *time.Time `json:"last_failure_at"`
	Success24h      int64      `json:"success_24h"`
	Failed24h       int64      `json:"failed_24h"`
	Paused24h       int64      `json:"paused_24h"`
	Total24h        int64      `json:"total_24h"`
}

// JobRunAggregate 单个 Job 在统计窗口内的运行结果聚合。
type JobRunAggregate struct {
	JobKind       string
	Success       int64
	Failed        int64
	Paused        int64
	Total         int64
	LastSuccessAt *time.Time
	LastFailureAt *time.Time
}

// UploadLogRow 上传审计（cdeu.finv_quote_cdeu_export_file_upload_log）。
type UploadLogRow struct {
	LogID         int64      `db:"log_id" json:"log_id"`
	FileID        int64      `db:"file_id" json:"file_id"`
	Attempt       int        `db:"attempt" json:"attempt"`
	TargetPath    string     `db:"target_path" json:"target_path"`
	GmtStart      time.Time  `db:"gmt_start" json:"gmt_start"`
	GmtFinish     *time.Time `db:"gmt_finish" json:"gmt_finish"`
	Status        string     `db:"status" json:"status"`
	UploadedBytes *int64     `db:"uploaded_bytes" json:"uploaded_bytes"`
	Error         *string    `db:"error" json:"error"`
}

// PatchRow 补丁任务（cdeu.finv_quote_cdeu_export_file_patch，不含 tstzrange 列）。
type PatchRow struct {
	PatchID      int64      `db:"patch_id" json:"patch_id"`
	JobID        int64      `db:"job_id" json:"job_id"`
	Usc          string     `db:"usc" json:"usc"`
	BizMonth     int        `db:"biz_month" json:"biz_month"`
	PatchLevel   string     `db:"patch_level" json:"patch_level"`
	PatchVersion int        `db:"patch_version" json:"patch_version"`
	SourceRows   int64      `db:"source_rows" json:"source_rows"`
	PatchRows    int64      `db:"patch_rows" json:"patch_rows"`
	Reason       string     `db:"reason" json:"reason"`
	Status       string     `db:"status" json:"status"`
	PatchFileID  *int64     `db:"patch_file_id" json:"patch_file_id"`
	GmtCreate    time.Time  `db:"gmt_create" json:"gmt_create"`
	GmtFinish    *time.Time `db:"gmt_finish" json:"gmt_finish"`
	LastError    *string    `db:"last_error" json:"last_error"`
}

// ---------------------------------------------------------------------
// 写操作入参
// ---------------------------------------------------------------------

// UscUpsertInput 证券主数据 upsert 入参（指针字段为 nil 表示不改该列）。
type UscUpsertInput struct {
	Usc              string
	Category         *string
	DayExpectRows    *int
	DateList         *string // yyyy-MM-dd
	DateDelist       *string // yyyy-MM-dd
	FlagAutoDiscover *bool
	Region           *string
	Market           *string
	Provider         *string
	Name             *string
	Timezone         *string
	Currency         *string
}

// PriorityUpsertInput 优先级覆盖设置入参（V200051 起落到 finv_dedup_usc）。
//
// 三列同为一组：要么全有值（设置覆盖），要么通过 SetUscPriority(ctx, usc, nil) 整组清空。
type PriorityUpsertInput struct {
	Usc          string
	PriorityRank int
	SlaDays      int
	MaxWaitDays  int
}

// CalendarUpsertInput 交易日历 upsert 入参。
type CalendarUpsertInput struct {
	Usc          string
	TradeDate    int
	FlagTradeDay bool
}

// BudgetOverrideInput 当日预算覆盖入参。
type BudgetOverrideInput struct {
	BizDate     string // yyyy-MM-dd，空表示「业务时区的今天」
	BudgetRows  int64
	HardCapRows int64
	Factor      float64
	Status      string
}
