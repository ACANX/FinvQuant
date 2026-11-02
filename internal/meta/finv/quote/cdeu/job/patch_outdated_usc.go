package job

import (
	"context"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

func init() {
	cdeu.RegisterJobExecutor(cdeu.JobPatchOutdatedUsc, func(ctx context.Context, svc *cdeu.Service, result *cdeu.Result) error {
		// 防重（需求 10932）：同一时刻只允许一个补丁实例，手动触发与工作流触发共用该判断。
		if svc.UscPatchRunner().Busy() {
			log.Printf("[cdeu] %s 跳过：已有补丁实例在运行", cdeu.JobPatchOutdatedUsc)
			result.Message = "跳过：已有补丁实例在运行"
			return nil
		}
		svc.ReportJobProgress(ctx, "PATCH", 0, "读取生效补丁规则")
		stats, err := svc.Store().PatchOutdatedUsc(ctx, 5000, func(s cdeu.UscPatchStats) {
			svc.ReportJobProgress(ctx, "PATCH", 0,
				"已处理批次 "+itoa(s.Batches)+"，迁移 "+itoa(s.MovedRows)+" 行，冲突 "+itoa(s.ConflictRows)+" 行")
		})
		if err != nil {
			return err
		}
		result.Details = map[string]any{
			"rule_count":    stats.RuleCount,
			"batches":       stats.Batches,
			"scanned_rows":  stats.ScannedRows,
			"moved_rows":    stats.MovedRows,
			"conflict_rows": stats.ConflictRows,
			"deleted_rows":  stats.DeletedRows,
			"from_ts":       stats.FromTs,
			"to_ts":         stats.ToTs,
		}
		log.Printf("[cdeu] %s 完成 rules=%d batches=%d moved=%d conflict=%d deleted=%d",
			cdeu.JobPatchOutdatedUsc, stats.RuleCount, stats.Batches,
			stats.MovedRows, stats.ConflictRows, stats.DeletedRows)
		return nil
	})
}

// itoa 轻量整数转字符串，避免为一行日志引入依赖。
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
