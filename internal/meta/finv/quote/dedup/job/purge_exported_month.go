package job

import (
	"context"
	"fmt"
	"log"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

func init() {
	// PURGE_EXPORTED_MONTH —— 归档复核阶段：按 20007 释放已完整交付的证券月（需求 10716/10720）。
	//
	// 判据是「扫描式」的：只要某证券月的 BASE 与全部 PATCH 都已 UPLOADED、且 dedup 主表仍留有
	// 该月数据，就删除。因此天然覆盖两种情况：
	//   ① 本次上传刚刚完成；
	//   ② 历史上已上传但没被清理（20007 上线前遗留、或 RegisterFile 因 checksum 未变而保持
	//      UPLOADED，导致上传 JOB 根本没碰到这个文件）。
	// 原实现把 ① 内联在上传 JOB 里、② 作为 CLEAN_EXPIRED 的「沉淀月补偿」，本质是同一件事的两种写法。
	dedup.RegisterJobExecutor(dedup.JobPurgeExportedMonth, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		if !svc.PurgeAfterExportEnabled(ctx) {
			svc.ReportJobProgress(ctx, "PURGE", 100, "系统配置 20007=0：保留主表数据，跳过清理")
			result.Details = map[string]any{
				"purge_after_export": false,
				"months_purged":      0,
				"rows_purged":        0,
			}
			log.Printf("[cdeu] %s 跳过：20007=0", dedup.JobPurgeExportedMonth)
			return nil
		}

		svc.ReportJobProgress(ctx, "PURGE", 0, "扫描已完整交付的证券月")
		items, err := svc.Store().SettledSecurityMonths(ctx, settledPurgeBatch)
		if err != nil {
			return err
		}

		months, rows := int64(0), int64(0)
		for index, item := range items {
			if ctx.Err() != nil {
				break
			}
			svc.ReportJobProgress(ctx, "PURGE", float64(index)/float64(len(items))*100,
				fmt.Sprintf("清理 usc=%s biz_month=%d（%d/%d）", item.Usc, item.BizMonth, index+1, len(items)))
			deleted, purgeErr := purgeMainSecurityMonth(ctx, svc, item.Usc, item.BizMonth)
			if purgeErr != nil {
				// 单月失败不中断整体：文件已在远端，下次运行会重新扫描到。
				log.Printf("[cdeu] 证券月主表删除失败 usc=%s biz_month=%d err=%v",
					item.Usc, item.BizMonth, purgeErr)
				continue
			}
			if deleted > 0 {
				months++
				rows += deleted
				log.Printf("[cdeu] 已释放证券月 usc=%s biz_month=%d rows=%d（BASE file_id=%d）",
					item.Usc, item.BizMonth, deleted, item.FileID)
			}
		}

		result.ProcessedRows = rows
		result.Details = map[string]any{
			"purge_after_export": true,
			"months_purged":      months,
			"rows_purged":        rows,
			"candidates":         len(items),
		}
		log.Printf("[cdeu] %s 完成 candidates=%d months_purged=%d rows_purged=%d",
			dedup.JobPurgeExportedMonth, len(items), months, rows)
		return nil
	})
}

// purgeMainSecurityMonth 分批删除指定证券某月的 dedup 主表行情数据（20007=1）。
//
// 返回本次删除的总行数；单批上限 mainPurgeBatchLimit，循环直到删空。
// 只删 dedup 主表，不触碰 quote schema 的回测副本。
func purgeMainSecurityMonth(ctx context.Context, svc *dedup.Service, usc string, bizMonth int) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		deleted, err := svc.Store().PurgeMainSecurityMonth(ctx, usc, bizMonth, mainPurgeBatchLimit)
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < mainPurgeBatchLimit {
			break
		}
	}
	if total > 0 {
		svc.ReportJobProgress(ctx, "CLEANUP", 0,
			fmt.Sprintf("20007 主表清理：usc=%s biz_month=%d 删除 %d 行", usc, bizMonth, total))
	}
	return total, nil
}
