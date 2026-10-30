package dedup

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 导出决策的判定结论与原因码（需求 10832）。
//
// 设计目标：让「为什么某个证券月没被导出」有一个**可指名道姓**的答案，
// 而不是笼统的「候选 0 个」。每个原因码都对应一条可复现的判定条件，
// 与 GenerateExportBatch / InsertCandidateJobs 的 WHERE 顺序保持一致。
const (
	ExportVerdictExportable = "EXPORTABLE"
	ExportVerdictExcluded   = "EXCLUDED"

	ExportReasonOK                 = "OK"
	ExportReasonMetadataIncomplete = "METADATA_INCOMPLETE"
	ExportReasonDelisted           = "DELISTED"
	ExportReasonCurrentMonth       = "CURRENT_MONTH"
	ExportReasonImmature           = "IMMATURE"
	ExportReasonAlreadyExported    = "ALREADY_EXPORTED"
	ExportReasonMaturityMissing    = "MATURITY_MISSING"
)

// ExportDecisionItem 单个 (usc, biz_month) 的导出判定明细。
type ExportDecisionItem struct {
	Usc          string `json:"usc"`
	BizMonth     int    `json:"biz_month"`
	Verdict      string `json:"verdict"`
	ReasonCode   string `json:"reason_code"`
	ReasonDetail string `json:"reason_detail"`
	PriorityRank int    `json:"priority_rank"`
	// Days / MatureDays 该月交易日数与已成熟天数。
	Days       int `json:"days"`
	MatureDays int `json:"mature_days"`
	// ExpectedRows 按日模板累计的应有行数；ActualRows 实际采集行数。
	ExpectedRows int64 `json:"expected_rows"`
	ActualRows   int64 `json:"actual_rows"`
	// 以下三项仅 EXPORTABLE 有值（告诉运维「导出来会叫什么、放哪里」）。
	FileName   string `json:"file_name,omitempty"`
	LocalPath  string `json:"local_path,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
	// 已存在的导出任务（ALREADY_EXPORTED 时给出）。
	JobID     *int64 `json:"job_id,omitempty"`
	JobStatus string `json:"job_status,omitempty"`
}

// ExportDecisionReport 导出决策明细报表。
type ExportDecisionReport struct {
	CurrentMonth int                  `json:"current_month"`
	Total        int                  `json:"total"`
	Exportable   int                  `json:"exportable"`
	Excluded     int                  `json:"excluded"`
	ByReason     map[string]int       `json:"by_reason"`
	Items        []ExportDecisionItem `json:"items"`
	// MaturityGaps 「行情有数据但成熟度无记录」的证券月：判定侧完全看不到的盲区。
	MaturityGaps []MaturityGap `json:"maturity_gaps,omitempty"`
	// GapScanFrom 缺口扫描的起始月份（含）。
	GapScanFrom int `json:"gap_scan_from"`
}

// ExplainExportDecision 逐条解释「哪些证券月能导出、哪些不能、为什么」（需求 10832）。
//
// 只读，不写批次表、不生成动态 JOB；供决策阶段 JOB 打印明细与落运行日志用。
//
// scanGaps 控制是否扫描「行情有数据但成熟度无记录」的盲区：该查询要扫近 6 个月的
// 行情表，只在每日一次的决策 JOB 里开启；每 60 秒一轮的导出 JOB 传 false，避免重复扫大表。
func (s *Service) ExplainExportDecision(ctx context.Context, currentMonth int, scanGaps bool) (ExportDecisionReport, error) {
	report := ExportDecisionReport{
		CurrentMonth: currentMonth,
		ByReason:     map[string]int{},
		Items:        make([]ExportDecisionItem, 0, 256),
		// 缺口扫描范围：近 6 个月（含当月）。
		GapScanFrom: currentMonth - 5,
	}

	materials, err := s.store.ExportDecisionMaterials(ctx)
	if err != nil {
		return report, fmt.Errorf("读取导出判定素材失败: %w", err)
	}

	for _, material := range materials {
		item := s.classifyExportDecision(material, currentMonth)
		report.Items = append(report.Items, item)
		report.Total++
		report.ByReason[item.ReasonCode]++
		if item.Verdict == ExportVerdictExportable {
			report.Exportable++
		} else {
			report.Excluded++
		}
	}

	report.MaturityGaps = []MaturityGap{}
	if !scanGaps {
		return report, nil
	}
	gaps, err := s.store.ExportMaturityGaps(ctx, report.GapScanFrom, 200)
	if err != nil {
		// 缺口扫描是辅助信息，失败不应让整个决策 JOB 失败（会掩盖主判定结果）。
		return report, nil
	}
	report.MaturityGaps = gaps
	report.ByReason[ExportReasonMaturityMissing] = len(gaps)
	return report, nil
}

// classifyExportDecision 按与批次生成一致的顺序给出单条判定与具体原因。
func (s *Service) classifyExportDecision(material ExportDecisionMaterial, currentMonth int) ExportDecisionItem {
	item := ExportDecisionItem{
		Usc:          material.Usc,
		BizMonth:     material.BizMonth,
		PriorityRank: material.PriorityRank,
		Days:         material.Days,
		MatureDays:   material.MatureDays,
		ExpectedRows: material.ExpectedRows,
		ActualRows:   material.ActualRows,
		JobID:        material.JobID,
	}
	if material.JobStatus != nil {
		item.JobStatus = *material.JobStatus
	}

	// 文件名与落地位置：无论能否导出都算出来，方便运维提前核对命名规则。
	meta := FileMeta{
		Usc:      material.Usc,
		BizMonth: material.BizMonth,
		Region:   derefString(material.Region),
		Market:   derefString(material.Market),
		Provider: derefString(material.Provider),
		Name:     derefString(material.Name),
		Timezone: derefString(material.Timezone),
		Currency: derefString(material.Currency),
	}
	item.FileName = meta.BaseFileName()
	item.LocalPath = filepath.Join(s.options.OutputDir, sanitizeFilePart(material.Usc),
		strconv.Itoa(material.BizMonth), item.FileName)
	item.RemotePath = s.options.ArchiveTargetPath(meta.Region, meta.Market, material.Usc, item.FileName)

	monthStart := fmt.Sprintf("%04d-%02d-01", material.BizMonth/100, material.BizMonth%100)

	switch {
	case len(missingUscFieldNames(material)) > 0:
		item.Verdict = ExportVerdictExcluded
		item.ReasonCode = ExportReasonMetadataIncomplete
		item.ReasonDetail = fmt.Sprintf(
			"证券元数据不全，缺少 %s（cdeu.finv_quote_cdeu_usc 对应列为空）；补齐后下一轮即可导出",
			strings.Join(missingUscFieldNames(material), "、"))

	case material.DateDelist != nil && material.DateDelist.Before(monthFirstDay(material.BizMonth)):
		item.Verdict = ExportVerdictExcluded
		item.ReasonCode = ExportReasonDelisted
		item.ReasonDetail = fmt.Sprintf("已退市：date_delist=%s 早于业务月首日 %s，历史数据不再导出",
			material.DateDelist.Format("2006-01-02"), monthStart)

	case material.BizMonth >= currentMonth:
		item.Verdict = ExportVerdictExcluded
		item.ReasonCode = ExportReasonCurrentMonth
		item.ReasonDetail = fmt.Sprintf("当月/未来月份不可导出：biz_month=%d 不早于当月 %d，等月末收口后再导",
			material.BizMonth, currentMonth)

	case material.MatureDays < material.Days:
		item.Verdict = ExportVerdictExcluded
		item.ReasonCode = ExportReasonImmature
		item.ReasonDetail = fmt.Sprintf(
			"成熟度未达标：该月共 %d 个交易日，仅 %d 天为 COMPLETE/FROZEN；未成熟日期（最多列 10 个）：%s",
			material.Days, material.MatureDays, formatTradeDays(material.UnmatureDays, 10))

	case isExportedJobStatus(item.JobStatus):
		item.Verdict = ExportVerdictExcluded
		item.ReasonCode = ExportReasonAlreadyExported
		fileName := derefString(material.BaseFileName)
		if fileName == "" {
			fileName = "（文件尚未登记）"
		}
		item.ReasonDetail = fmt.Sprintf("已导出/导出中：job_id=%d status=%s 已登记文件=%s",
			derefInt64(material.JobID), item.JobStatus, fileName)

	default:
		item.Verdict = ExportVerdictExportable
		item.ReasonCode = ExportReasonOK
		item.ReasonDetail = fmt.Sprintf(
			"满足全部导出条件：%d 个交易日全部成熟，按日模板应有 %d 行，实际采集 %d 行；文件名 %s；本地 %s；远端 %s",
			material.Days, material.ExpectedRows, material.ActualRows,
			item.FileName, item.LocalPath, item.RemotePath)
	}
	return item
}

// missingUscFieldNames 返回证券元数据里为空的字段中文名（顺序固定，便于日志比对）。
func missingUscFieldNames(material ExportDecisionMaterial) []string {
	checks := []struct {
		label string
		value *string
	}{
		{"region（区域）", material.Region},
		{"market（市场）", material.Market},
		{"provider（数据商）", material.Provider},
		{"name（名称）", material.Name},
		{"timezone（时区）", material.Timezone},
		{"currency（币种）", material.Currency},
	}
	missing := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.value == nil || strings.TrimSpace(*check.value) == "" {
			missing = append(missing, check.label)
		}
	}
	return missing
}

func monthFirstDay(bizMonth int) time.Time {
	return time.Date(bizMonth/100, time.Month(bizMonth%100), 1, 0, 0, 0, 0, time.UTC)
}

func isExportedJobStatus(status string) bool {
	switch status {
	case JobStatusLoading, JobStatusLoaded, JobStatusExporting, JobStatusExported:
		return true
	default:
		return false
	}
}

// formatTradeDays 把交易日列表格式化为 YYYY-MM-DD 逗号串，最多 limit 个。
func formatTradeDays(days []int, limit int) string {
	if len(days) == 0 {
		return "无"
	}
	shown := days
	suffix := ""
	if limit > 0 && len(days) > limit {
		shown = days[:limit]
		suffix = fmt.Sprintf(" 等 %d 天", len(days))
	}
	parts := make([]string, 0, len(shown))
	for _, day := range shown {
		parts = append(parts, fmt.Sprintf("%04d-%02d-%02d", day/10000, day/100%100, day%100))
	}
	return strings.Join(parts, ", ") + suffix
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// DiagnoseEmptyExportBatch 在批次/候选/动态 JOB 数量异常时，把「卡在哪一步、为什么」打进日志。
//
// 需求 10832：原先这几条链路失败时日志只有一行「候选 0 个」，运维无法区分是
// 批次清单为空、候选被元数据/成熟度排除、还是批次有但动态实例没生成。
// 本方法按链路顺序逐段判断，只在出问题时打印，不在正常路径增加开销。
func (s *Service) DiagnoseEmptyExportBatch(ctx context.Context, batchBase, batchIncr, created, dynamicCreated int64, currentMonth int) {
	if batchBase > 0 && created > 0 && dynamicCreated > 0 {
		return
	}

	log.Printf("[cdeu] 导出批次诊断 开始 BASE=%d INCR=%d 候选=%d 动态=%d 当月=%d",
		batchBase, batchIncr, created, dynamicCreated, currentMonth)

	switch {
	case batchBase == 0:
		log.Printf("[cdeu] 导出批次诊断 结论=批次清单为空：GenerateExportBatch 的 BASE 判定没有任何证券月满足，" +
			"请对照随后打印的导出决策明细（元数据不全 / 退市 / 当月 / 未成熟 / 已导出）")
	case created == 0:
		log.Printf("[cdeu] 导出批次诊断 结论=批次有 %d 条但候选为 0：这些证券月要么已在导出任务表中"+
			"（状态 LOADING/LOADED/EXPORTING/EXPORTED，属正常已导出），要么在批次生成与候选插入之间"+
			"成熟度或元数据发生变化；明细见导出决策 JOB", batchBase)
	case dynamicCreated == 0:
		log.Printf("[cdeu] 导出批次诊断 结论=候选 %d 条但动态 JOB 为 0：动态实例依赖批次行的 source_job_id "+
			"或导出任务表里同 (usc, biz_month) 的记录，请检查该证券月是否缺少导出任务行（job_category=EXPORT）",
			created)
	}

	report, err := s.ExplainExportDecision(ctx, currentMonth, false)
	if err != nil {
		log.Printf("[cdeu] 导出批次诊断 读取决策明细失败 err=%v", err)
		return
	}
	log.Printf("[cdeu] 导出批次诊断 判定分布=%v 可导出=%d 排除=%d（逐条原因见导出决策 JOB 日志）",
		report.ByReason, report.Exportable, report.Excluded)
	// 排除明细各打前 5 条，够定位问题又不刷屏。
	printedByReason := map[string]int{}
	for _, item := range report.Items {
		if item.Verdict == ExportVerdictExportable {
			continue
		}
		if printedByReason[item.ReasonCode] >= 5 {
			continue
		}
		printedByReason[item.ReasonCode]++
		log.Printf("[cdeu] 导出批次诊断 排除 %s %d 原因=%s 说明=%s",
			item.Usc, item.BizMonth, item.ReasonCode, item.ReasonDetail)
	}
}
