package job

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

func init() {
	// PURGE_LOCAL_EXPORT_FILES —— 归档复核阶段的 PROCESS_PURGE 第二个 JOB（需求 10744）。
	//
	// 由系统配置 20011 控制：'1' 清理（默认） / '0' 不清理。
	// 与 JOB_PURGE_EXPORTED_MONTH（删主表）同属归档数据清理，执行顺序排在它之后：
	// 先释放数据库里的该月行情，再释放本地文件，避免「文件已删但库里还能重导」的窗口。
	//
	// 判据：该证券月 BASE+全部 PATCH 上传成功、远端校验通过、最后一个文件上传完成
	// 已超过 24 小时（syscfg.PurgeLocalExportFilesDelay），且没有待处理补丁。
	// 删除范围包含同月的 MANIFEST（.json）——它不上传远端，但同属本次导出产物。
	dedup.RegisterJobExecutor(dedup.JobPurgeLocalExportFiles, func(ctx context.Context, svc *dedup.Service, result *dedup.Result) error {
		if !svc.PurgeLocalExportFilesEnabled(ctx) {
			svc.ReportJobProgress(ctx, "PURGE", 100, "系统配置 20011=0：保留本地导出文件，跳过清理")
			result.Details = map[string]any{
				"purge_local_files": false,
				"deleted":           0,
				"failed":            0,
				"delay_hours":       int(syscfg.PurgeLocalExportFilesDelay.Hours()),
			}
			log.Printf("[cdeu] %s 跳过：20011=0", dedup.JobPurgeLocalExportFiles)
			return nil
		}

		delay := syscfg.PurgeLocalExportFilesDelay
		svc.ReportJobProgress(ctx, "PURGE", 0,
			fmt.Sprintf("扫描已交付且超过 %d 小时的本地文件", int(delay.Hours())))
		items, err := svc.Store().LocalFilesForSettledMonths(ctx, delay, localCleanupLimit)
		if err != nil {
			return err
		}

		deleted, failed := int64(0), int64(0)
		for index, item := range items {
			if ctx.Err() != nil {
				break
			}
			if index%50 == 0 {
				svc.ReportJobProgress(ctx, "PURGE", float64(index)/float64(len(items))*100,
					fmt.Sprintf("已删除 %d / %d 个本地文件", deleted, len(items)))
			}
			// 文件不存在视为已清理（幂等）：仍标记 LOCAL_DELETED，避免反复扫描。
			if removeErr := os.Remove(item.FilePath); removeErr != nil && !os.IsNotExist(removeErr) {
				failed++
				log.Printf("[cdeu] 删除本地文件失败 file_id=%d path=%s err=%v",
					item.FileID, item.FilePath, removeErr)
				continue
			}
			if err := svc.Store().MarkLocalDeleted(ctx, item.FileID); err != nil {
				failed++
				log.Printf("[cdeu] 标记本地清理失败 file_id=%d err=%v", item.FileID, err)
				continue
			}
			deleted++
			log.Printf("[cdeu] 已删除本地文件 file_id=%d usc=%s biz_month=%d type=%s v%d path=%s",
				item.FileID, item.Usc, item.BizMonth, item.FileType, item.Version, item.FilePath)
		}

		result.ProcessedRows = deleted
		result.Details = map[string]any{
			"purge_local_files": true,
			"candidates":        len(items),
			"deleted":           deleted,
			"failed":            failed,
			"delay_hours":       int(delay.Hours()),
		}
		log.Printf("[cdeu] %s 完成 candidates=%d deleted=%d failed=%d delay=%dh",
			dedup.JobPurgeLocalExportFiles, len(items), deleted, failed, int(delay.Hours()))
		return nil
	})
}
