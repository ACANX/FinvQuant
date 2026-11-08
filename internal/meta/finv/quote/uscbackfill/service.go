// Package uscbackfill 实现「上游 usc 回填」（需求 11054）：
// 把本地 futu.finv_futu_quote_collect 的 usc **按 stock_id 关联**写回上游 Supabase
// public.finv_quote_secu_meta.usc（上游该表 3,673 行 usc 为 NULL）。
//
// 与 Tab 1/2 的方向相反：那两个 Tab 是「上游 → 本地」，本包是「本地 → 上游」，
// 会对线上 Supabase 执行 PATCH，因此口径必须严格：
//
//   - 关联键是 stock_id（两个表都有、上游唯一且 NOT NULL），**不是** usc；
//   - 每条上游行最多被写一次；写入值就是本地该 stock_id 对应的 usc；
//   - 上游 usc 已等于目标值 → 记为「未变」，不发请求（幂等）；
//   - 本地同一个 stock_id 有多条 usc（实测 70 组，如 CC.DASHTRY.FTUS / DASHTRY）时，
//     取**等于上游 ftmm_symbol** 的那条（需求 11055）；都不等则跳过并计入「歧义」；
//   - 本地有、上游没有的 stock_id（实测 13,438 个，期货/加密 FTUS 等 futu 独有）→ 计入「无对应」。
package uscbackfill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 上游/本地表与关联键。
const (
	// UpstreamTable 上游表（PostgREST 上走 Accept-Profile/Content-Profile 头）。
	UpstreamTable = "public.finv_quote_secu_meta"
	// LocalTable 本地采集属性表（usc 的来源）。
	LocalTable = "futu.finv_futu_quote_collect"
	// KeyColumn 两表关联键。
	KeyColumn = "stock_id"
	// UpstreamUSCColumn 上游被回填的列。
	UpstreamUSCColumn = "usc"
	// upstreamSelect 从上游取回的列（关联键 + 现值 + 歧义判定用）。
	upstreamSelect = "stock_id,usc,ftmm_symbol"
)

// 批量大小取值（沿用其它两个 Tab 的档位）。
const (
	// DefaultBatchSize 默认每批 stock_id 个数。
	DefaultBatchSize = 500
	// MinBatchSize 最小批量。
	MinBatchSize = 100
	// MaxBatchSize 最大批量。
	MaxBatchSize = 5000
)

// patchConcurrency 逐行 PATCH 的并发度（串行 3,676 行要 9 分钟，并发 8 约 1 分钟）。
const patchConcurrency = 8

// NormalizeBatchSize 把请求里的批量大小收敛到 100~5000，<=0 时取默认 500。
func NormalizeBatchSize(raw int) int {
	if raw <= 0 {
		return DefaultBatchSize
	}
	if raw < MinBatchSize {
		return MinBatchSize
	}
	if raw > MaxBatchSize {
		return MaxBatchSize
	}
	return raw
}

// Request 一次回填的入参。
type Request struct {
	// BatchSize 每批处理的 stock_id 个数（留空或 <=0 取 500）。
	BatchSize int
}

// Progress 一批完成时的进度快照（计数均为**累计值**）。
type Progress struct {
	Batch     int   // 当前批次号（从 1 开始）
	Offset    int   // 本批在本地待处理序列里的起点
	Fetched   int   // 本批处理的 stock_id 个数
	RowsTotal int64 // 待处理 stock_id 总数
	RowsDone  int   // 累计处理 stock_id 数
	Fill      int   // 累计「上游为空 → 填入」
	Overwrite int   // 累计「上游已有 → 覆盖」
	Unchanged int   // 累计「上游已等于目标值 → 未变」
	Ambiguous int   // 累计「本地多条 usc 且都不等于 ftmm_symbol → 歧义跳过」
	NoTarget  int   // 累计「上游没有该 stock_id → 无对应」
	ElapsedMs int64 // 本批耗时
}

// Result 一次回填的结果。
type Result struct {
	UpstreamTable string `json:"upstream_table"`
	LocalTable    string `json:"local_table"`
	KeyColumn     string `json:"key_column"`
	RowsTotal     int64  `json:"rows_total"` // 待处理 stock_id 总数
	LocalRows     int64  `json:"local_rows"` // 本地源表行数（同一 stock_id 可能多行）
	Batches       int    `json:"batches"`
	Fill          int    `json:"fill"`      // 上游为空 → 填入
	Overwrite     int    `json:"overwrite"` // 上游已有 → 覆盖
	Unchanged     int    `json:"unchanged"` // 上游已一致 → 未变
	Ambiguous     int    `json:"ambiguous"` // 本地多条 usc 无法定值 → 跳过
	NoTarget      int    `json:"no_target"` // 上游无该 stock_id → 跳过
	ElapsedMs     int64  `json:"elapsed_ms"`
}

// TableStat 单表行数统计。
type TableStat struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

// StatusSnapshot 「连接与目标表」卡片快照（只读本地，不访问 Supabase）。
type StatusSnapshot struct {
	// Configured 系统配置 20013 是否已配置且格式合法。
	Configured bool `json:"configured"`
	// ProjectID 上游项目 ID。
	ProjectID string `json:"project_id"`
	// APIKeyMasked API Key 脱敏前缀。
	APIKeyMasked string `json:"api_key_masked"`
	// ConfigError 20013 未配置 / 非法的原因。
	ConfigError string `json:"config_error,omitempty"`
	// UpstreamTable 上游被回填的表。
	UpstreamTable string `json:"upstream_table"`
	// UpstreamColumn 上游被回填的列。
	UpstreamColumn string `json:"upstream_column"`
	// LocalTable 本地 usc 来源表。
	LocalTable string `json:"local_table"`
	// KeyColumn 关联键。
	KeyColumn string `json:"key_column"`
	// LocalRows 本地源表行数。
	LocalRows int64 `json:"local_rows"`
	// LocalStockIDs 本地去重后的 stock_id 个数（= 一次回填的待处理量）。
	LocalStockIDs int64 `json:"local_stock_ids"`
	// LocalUSCPlaceholder 本地 usc 等于 futu_symbol 的行数（占位行，仅供页面提示）。
	LocalUSCPlaceholder int64 `json:"local_usc_placeholder"`
	// BatchSizeDefault 默认批量。
	BatchSizeDefault int `json:"batch_size_default"`
}

// LocalRow 本地源表的一行。
type LocalRow struct {
	USC        string
	StockID    int64
	FutuSymbol string
}

// Service usc 回填服务（无状态）。
type Service struct {
	cfg       *syscfg.Store
	store     *Store
	newClient func(syscfg.SupabaseConn) *secukey.Client
	now       func() time.Time
}

// NewService 创建服务；pool 为空时（未接库的单测场景）所有读写返回可读错误。
func NewService(pool *pgxpool.Pool) *Service {
	service := &Service{
		cfg:       syscfg.NewStore(pool),
		newClient: func(conn syscfg.SupabaseConn) *secukey.Client { return secukey.NewClient(conn) },
		now:       time.Now,
	}
	if pool != nil {
		service.store = NewStore(pool)
	}
	return service
}

// Status 返回状态快照：只读本地（配置 + 本地源表行数与待处理量）。
func (s *Service) Status(ctx context.Context) (StatusSnapshot, error) {
	snapshot := StatusSnapshot{
		UpstreamTable:    UpstreamTable,
		UpstreamColumn:   UpstreamUSCColumn,
		LocalTable:       LocalTable,
		KeyColumn:        KeyColumn,
		BatchSizeDefault: DefaultBatchSize,
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return snapshot, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		snapshot.ConfigError = values.SupabaseConnErr.Error()
	} else {
		snapshot.Configured = true
		snapshot.ProjectID = values.SupabaseConn.ProjectID
		snapshot.APIKeyMasked = values.SupabaseConn.MaskedAPIKey()
	}
	if s.store == nil {
		return snapshot, nil
	}
	stat, err := s.store.LocalStat(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.LocalRows = stat.Rows
	snapshot.LocalStockIDs = stat.StockIDs
	snapshot.LocalUSCPlaceholder = stat.Placeholder
	return snapshot, nil
}

// Sync 执行一次回填（无进度回调）；见 SyncWithProgress。
func (s *Service) Sync(ctx context.Context, req Request) (Result, error) {
	return s.SyncWithProgress(ctx, req, nil)
}

// SyncWithProgress 执行一次回填：按 stock_id 升序分批，先取上游现值再逐行 PATCH。
//
// 每批结束回调 onProgress（可为 nil），供异步运行写实时日志。
// 任一批写入失败即返回错误（已写入的批次不回滚——回填是幂等的，重跑即可收敛）。
func (s *Service) SyncWithProgress(ctx context.Context, req Request, onProgress func(Progress)) (Result, error) {
	started := s.now()
	result := Result{UpstreamTable: UpstreamTable, LocalTable: LocalTable, KeyColumn: KeyColumn}

	values, err := s.cfg.Load(ctx)
	if err != nil {
		return result, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		return result, fmt.Errorf("Supabase 连接不可用: %w", values.SupabaseConnErr)
	}
	if s.store == nil {
		return result, fmt.Errorf("数据库连接池未初始化，无法读取 %s", LocalTable)
	}

	rows, err := s.store.ReadLocalRows(ctx)
	if err != nil {
		return result, err
	}
	result.LocalRows = int64(len(rows))
	groups, order := groupByStockID(rows)
	result.RowsTotal = int64(len(order))
	if len(order) == 0 {
		result.ElapsedMs = s.now().Sub(started).Milliseconds()
		return result, nil
	}

	batchSize := NormalizeBatchSize(req.BatchSize)
	client := s.newClient(values.SupabaseConn)
	log.Printf("[cdeu][uscbackfill] sync start upstream=%s.%s local=%s key=%s stock_ids=%d local_rows=%d batch_size=%d project=%s",
		UpstreamTable, UpstreamUSCColumn, LocalTable, KeyColumn, len(order), len(rows), batchSize,
		values.SupabaseConn.ProjectID)

	seen := 0
	for batch := 1; seen < len(order); batch++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		batchStart := s.now()
		end := seen + batchSize
		if end > len(order) {
			end = len(order)
		}
		batchIDs := order[seen:end]

		upstream, err := client.FetchByIntIDs(ctx, UpstreamTable, upstreamSelect, KeyColumn, batchIDs)
		if err != nil {
			return result, err
		}
		current, err := indexUpstream(upstream)
		if err != nil {
			return result, err
		}

		jobs := make([]patchJob, 0, len(batchIDs))
		batchFill, batchOverwrite, batchUnchanged := 0, 0, 0
		batchAmbiguous, batchNoTarget := 0, 0
		for _, id := range batchIDs {
			up, ok := current[id]
			if !ok {
				batchNoTarget++
				continue
			}
			target, decided := pickUSC(groups[id], up.FtmmSymbol)
			if !decided {
				batchAmbiguous++
				continue
			}
			if strings.TrimSpace(up.USC) == target {
				batchUnchanged++
				continue
			}
			if strings.TrimSpace(up.USC) == "" {
				batchFill++
			} else {
				batchOverwrite++
			}
			jobs = append(jobs, patchJob{stockID: id, usc: target, previous: up.USC})
		}

		affected, err := patchAll(ctx, client, jobs)
		if err != nil {
			return result, err
		}

		result.Batches = batch
		result.Fill += batchFill
		result.Overwrite += batchOverwrite
		result.Unchanged += batchUnchanged
		result.Ambiguous += batchAmbiguous
		result.NoTarget += batchNoTarget

		batchElapsed := s.now().Sub(batchStart).Milliseconds()
		log.Printf("[cdeu][uscbackfill] batch=%d offset=%d stock_ids=%d upstream_hit=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d patched=%d elapsed_ms=%d",
			batch, seen, len(batchIDs), len(batchIDs)-batchNoTarget, batchFill, batchOverwrite,
			batchUnchanged, batchAmbiguous, batchNoTarget, affected, batchElapsed)

		if onProgress != nil {
			onProgress(Progress{
				Batch:     batch,
				Offset:    seen,
				Fetched:   len(batchIDs),
				RowsTotal: result.RowsTotal,
				RowsDone:  end,
				Fill:      result.Fill,
				Overwrite: result.Overwrite,
				Unchanged: result.Unchanged,
				Ambiguous: result.Ambiguous,
				NoTarget:  result.NoTarget,
				ElapsedMs: batchElapsed,
			})
		}

		seen = end
	}

	result.ElapsedMs = s.now().Sub(started).Milliseconds()
	log.Printf("[cdeu][uscbackfill] sync done stock_ids=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d batches=%d elapsed_ms=%d",
		result.RowsTotal, result.Fill, result.Overwrite, result.Unchanged, result.Ambiguous,
		result.NoTarget, result.Batches, result.ElapsedMs)
	return result, nil
}

// patchJob 一条待回写记录。
type patchJob struct {
	stockID  int64
	usc      string
	previous string
}

// patchAll 并发回写一批（并发度 patchConcurrency）；任一行失败即取消其余任务并返回错误。
func patchAll(ctx context.Context, client *secukey.Client, jobs []patchJob) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		done     int
	)
	queue := make(chan patchJob)
	for worker := 0; worker < patchConcurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range queue {
				if runCtx.Err() != nil {
					return
				}
				if err := client.PatchTextColumn(runCtx, UpstreamTable, KeyColumn, job.stockID,
					UpstreamUSCColumn, job.usc); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					cancel()
					return
				}
				mu.Lock()
				done++
				mu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		select {
		case <-runCtx.Done():
			break
		case queue <- job:
		}
		if runCtx.Err() != nil {
			break
		}
	}
	close(queue)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if firstErr != nil {
		return done, fmt.Errorf("回写上游失败（已成功 %d / %d 行，重跑可续）: %w", done, len(jobs), firstErr)
	}
	return done, nil
}

// upstreamRow 上游一行的现值。
type upstreamRow struct {
	StockID    int64
	USC        string
	FtmmSymbol string
}

// indexUpstream 把上游原始 JSON 行解析成 stock_id → 现值。
//
// 上游 stock_id 为 NOT NULL 且唯一；若出现重复（口径变化）按后者覆盖并记日志，
// 避免整次回填因为脏数据直接失败。
func indexUpstream(raw []json.RawMessage) (map[int64]upstreamRow, error) {
	rows := make(map[int64]upstreamRow, len(raw))
	for _, item := range raw {
		var rawRow struct {
			StockID    *int64  `json:"stock_id"`
			USC        *string `json:"usc"`
			FtmmSymbol *string `json:"ftmm_symbol"`
		}
		if err := json.Unmarshal(item, &rawRow); err != nil {
			return nil, fmt.Errorf("解析上游行失败: %w", err)
		}
		if rawRow.StockID == nil {
			continue
		}
		row := upstreamRow{StockID: *rawRow.StockID}
		if rawRow.USC != nil {
			row.USC = *rawRow.USC
		}
		if rawRow.FtmmSymbol != nil {
			row.FtmmSymbol = *rawRow.FtmmSymbol
		}
		if _, exists := rows[row.StockID]; exists {
			log.Printf("[cdeu][uscbackfill] 上游 stock_id 重复 stock_id=%d（按后者取值）", row.StockID)
		}
		rows[row.StockID] = row
	}
	return rows, nil
}

// groupByStockID 按 stock_id 归并本地行，并返回 stock_id 升序序列（分批依据）。
func groupByStockID(rows []LocalRow) (map[int64][]LocalRow, []int64) {
	groups := make(map[int64][]LocalRow, len(rows))
	order := make([]int64, 0, len(rows))
	for _, row := range rows {
		if _, exists := groups[row.StockID]; !exists {
			order = append(order, row.StockID)
		}
		groups[row.StockID] = append(groups[row.StockID], row)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	return groups, order
}

// pickUSC 从同一 stock_id 的本地行里定出要回填的 usc（需求 11055）。
//
// 单条即取该条；多条时取等于上游 ftmm_symbol 的那条（上游行的身份标识），
// 都不等则返回 decided=false（计入歧义，交由人工处理，不猜）。
func pickUSC(rows []LocalRow, ftmmSymbol string) (target string, decided bool) {
	if len(rows) == 0 {
		return "", false
	}
	if len(rows) == 1 {
		value := strings.TrimSpace(rows[0].USC)
		return value, value != ""
	}
	want := strings.TrimSpace(ftmmSymbol)
	for _, row := range rows {
		if value := strings.TrimSpace(row.USC); want != "" && value == want {
			return value, true
		}
	}
	return "", false
}
