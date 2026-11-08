// Package uscbackfill 实现「上游 usc 回填」（需求 11054/11056）：
// **方向与 Tab 1/2 相反（本地 → 上游）**，一次运行写两张上游表：
//
//	① public.finv_quote_secu_meta.usc          按 stock_id 关联（需求 11054）
//	   来源：本地 futu.finv_futu_quote_collect（24,418 行 → 24,348 个 stock_id）
//	② public.finv_quote_secu_key.usc + .stock_id  按 ftmm_symbol 关联（需求 11056/11057）
//	   来源：本地 futu.finv_futu_secu_key（候选键集）⋈ futu.finv_futu_quote_collect
//	        （用 market_label || '.' || futu_symbol 重建 ftmm_symbol，提供 usc + stock_id）
//
// 关联键固定：meta 表用 stock_id（两表都有、上游唯一且 NOT NULL），
// key 表用 ftmm_symbol（上游主键）。两条链路都**不是**「只更新值为 NULL 的行」：
// 上游值已等于目标值 → 记「未变」且不发请求（幂等）；为空 → 「填入」；有值但不同 → 「覆盖」。
//
// 本地同一个 stock_id 有多条 usc（实测 70 组，如 CC.DASHTRY.FTUS / DASHTRY）时，
// 取**等于 ftmm_symbol** 的那条（需求 11055）；都不等则跳过并计入「歧义」。
package uscbackfill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 上游被回填的两张表。
const (
	// MetaTable 上游证券元数据宽表（回填 usc）。
	MetaTable = "public.finv_quote_secu_meta"
	// MetaUSCColumn 表 ① 被回填的列。
	MetaUSCColumn = "usc"
	// MetaKeyColumn 表 ① 的关联键。
	MetaKeyColumn = "stock_id"

	// KeyTable 上游证券采集清单表（回填 usc + stock_id）。
	KeyTable = "public.finv_quote_secu_key"
	// KeyKeyColumn 表 ② 的关联键（上游主键）。
	KeyKeyColumn = "ftmm_symbol"
	// KeyUSCColumn 表 ② 被回填的列之一。
	KeyUSCColumn = "usc"
	// KeyStockIDColumn 表 ② 被回填的列之二。
	KeyStockIDColumn = "stock_id"
)

// 本地来源表。
const (
	// LocalMetaTable 表 ① 的 usc 来源（采集属性表）。
	LocalMetaTable = "futu.finv_futu_quote_collect"
	// LocalKeyTable 表 ② 的 ftmm_symbol + stock_id 来源（本地证券键表）。
	LocalKeyTable = "futu.finv_futu_secu_key"
)

// 从上游取回的列。
const (
	metaUpstreamSelect = "stock_id,usc,ftmm_symbol"
	keyUpstreamSelect  = "ftmm_symbol,usc,stock_id"
)

// 阶段标识（日志与进度用）。
const (
	// PhaseMeta 阶段 ①：public.finv_quote_secu_meta.usc（按 stock_id）。
	PhaseMeta = "meta"
	// PhaseKey 阶段 ②：public.finv_quote_secu_key.usc + stock_id（按 ftmm_symbol）。
	PhaseKey = "key"
)

// 批量大小取值（沿用其它两个 Tab 的档位）。
const (
	// DefaultBatchSize 默认每批键值个数。
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
	// BatchSize 每批处理的键值个数（留空或 <=0 取 500）。
	BatchSize int
}

// Progress 一批完成时的进度快照（计数均为**累计值**）。
type Progress struct {
	Phase        string // meta / key
	Batch        int    // 当前阶段内的批次号（从 1 开始）
	Offset       int    // 本批在该阶段序列里的起点
	Fetched      int    // 本批处理的键值个数
	RowsDone     int    // 两个阶段累计已处理（进度分子）
	RowsTotal    int64  // 两个阶段累计待处理（进度分母）
	Fill         int    // ① 上游为空 → 填入
	Overwrite    int    // ① 上游已有且不同 → 覆盖
	Unchanged    int    // ① 上游已一致 → 未变
	Ambiguous    int    // ① 本地多条 usc 无法定值 → 跳过
	NoTarget     int    // ① 上游无该 stock_id → 跳过
	KeyFill      int    // ② 上游 usc/stock_id 为空 → 填入
	KeyOverwrite int    // ② 上游已有且不同 → 覆盖
	KeyUnchanged int    // ② 上游已一致 → 未变
	KeyAmbiguous int    // ② 本地多条 usc 无法定值 → 跳过
	KeyNoTarget  int    // ② 上游无该 ftmm_symbol → 跳过
	ElapsedMs    int64
}

// Result 一次回填的结果。
type Result struct {
	MetaTable  string `json:"meta_table"`
	MetaColumn string `json:"meta_column"`
	MetaKey    string `json:"meta_key"`
	LocalTable string `json:"local_table"`
	RowsTotal  int64  `json:"rows_total"` // ① 待处理 stock_id 总数
	LocalRows  int64  `json:"local_rows"` // ① 本地源表行数
	Batches    int    `json:"batches"`
	Fill       int    `json:"fill"`
	Overwrite  int    `json:"overwrite"`
	Unchanged  int    `json:"unchanged"`
	Ambiguous  int    `json:"ambiguous"`
	NoTarget   int    `json:"no_target"`

	KeyTable      string `json:"key_table"`
	KeyColumns    string `json:"key_columns"`
	KeyKey        string `json:"key_key"`
	KeyLocalRows  int64  `json:"key_local_rows"`
	KeyResolvable int64  `json:"key_resolvable"`
	KeyUnresolved int64  `json:"key_unresolved"`
	KeyBatches    int    `json:"key_batches"`
	KeyFill       int    `json:"key_fill"`
	KeyOverwrite  int    `json:"key_overwrite"`
	KeyUnchanged  int    `json:"key_unchanged"`
	KeyAmbiguous  int    `json:"key_ambiguous"`
	KeyNoTarget   int    `json:"key_no_target"`

	ElapsedMs int64 `json:"elapsed_ms"`
}

// TableStat 单表行数统计。
type TableStat struct {
	Table string `json:"table"`
	Rows  int64  `json:"rows"`
}

// StatusSnapshot 「连接与源表」卡片快照（只读本地，不访问 Supabase）。
type StatusSnapshot struct {
	// Configured 系统配置 20013 是否已配置且格式合法。
	Configured bool `json:"configured"`
	// ProjectID 上游项目 ID。
	ProjectID string `json:"project_id"`
	// APIKeyMasked API Key 脱敏前缀。
	APIKeyMasked string `json:"api_key_masked"`
	// ConfigError 20013 未配置 / 非法的原因。
	ConfigError string `json:"config_error,omitempty"`

	// MetaTable / MetaColumn / MetaKey 表 ① 与关联键。
	MetaTable  string `json:"meta_table"`
	MetaColumn string `json:"meta_column"`
	MetaKey    string `json:"meta_key"`
	// LocalTable 表 ① 的 usc 来源。
	LocalTable string `json:"local_table"`
	// LocalRows 表 ① 来源表行数。
	LocalRows int64 `json:"local_rows"`
	// LocalStockIDs 表 ① 来源去重后的 stock_id 个数。
	LocalStockIDs int64 `json:"local_stock_ids"`
	// LocalUSCPlaceholder 表 ① 来源里 usc = futu_symbol 的行数（占位行，仅提示）。
	LocalUSCPlaceholder int64 `json:"local_usc_placeholder"`

	// KeyTable / KeyColumns / KeyKey 表 ② 与关联键。
	KeyTable   string `json:"key_table"`
	KeyColumns string `json:"key_columns"`
	KeyKey     string `json:"key_key"`
	// KeyLocalRows 本地证券键表行数（73,212）。
	KeyLocalRows int64 `json:"key_local_rows"`
	// KeyResolvable 能定出 usc + stock_id 的行数（= 一次运行的可回填量）。
	KeyResolvable int64 `json:"key_resolvable"`
	// KeyUnresolved 定不出 usc 或没有 stock_id 的行数（本地无依据，跳过）。
	KeyUnresolved int64 `json:"key_unresolved"`

	// BatchSizeDefault 默认批量。
	BatchSizeDefault int `json:"batch_size_default"`
}

// LocalRow 本地采集属性表的一行（表 ① 来源）。
type LocalRow struct {
	USC        string
	StockID    int64
	FutuSymbol string
}

// LocalKeyRow 本地证券键表的一行（表 ② 来源，已按 stock_id 关联到 usc）。
type LocalKeyRow struct {
	FtmmSymbol string
	USC        string
	StockID    int64
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

// Status 返回状态快照：只读本地（配置 + 两张来源表的行数与可回填量）。
func (s *Service) Status(ctx context.Context) (StatusSnapshot, error) {
	snapshot := StatusSnapshot{
		MetaTable:        MetaTable,
		MetaColumn:       MetaUSCColumn,
		MetaKey:          MetaKeyColumn,
		LocalTable:       LocalMetaTable,
		KeyTable:         KeyTable,
		KeyColumns:       KeyUSCColumn + " + " + KeyStockIDColumn,
		KeyKey:           KeyKeyColumn,
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
	meta, err := s.store.MetaLocalStat(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.LocalRows = meta.Rows
	snapshot.LocalStockIDs = meta.StockIDs
	snapshot.LocalUSCPlaceholder = meta.Placeholder

	keyStat, err := s.store.KeyLocalStat(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.KeyLocalRows = keyStat.Rows
	snapshot.KeyResolvable = keyStat.Resolvable
	snapshot.KeyUnresolved = keyStat.Rows - keyStat.Resolvable
	return snapshot, nil
}

// Sync 执行一次回填（无进度回调）；见 SyncWithProgress。
func (s *Service) Sync(ctx context.Context, req Request) (Result, error) {
	return s.SyncWithProgress(ctx, req, nil)
}

// SyncWithProgress 执行一次回填：先跑阶段 ①（meta 表），再跑阶段 ②（key 表）。
//
// 每批结束回调 onProgress（可为 nil），供异步运行写实时日志。
// 任一批写入失败即返回错误（已写入的批次不回滚——回填是幂等的，重跑即可收敛）。
func (s *Service) SyncWithProgress(ctx context.Context, req Request, onProgress func(Progress)) (Result, error) {
	started := s.now()
	result := Result{
		MetaTable:  MetaTable,
		MetaColumn: MetaUSCColumn,
		MetaKey:    MetaKeyColumn,
		LocalTable: LocalMetaTable,
		KeyTable:   KeyTable,
		KeyColumns: KeyUSCColumn + " + " + KeyStockIDColumn,
		KeyKey:     KeyKeyColumn,
	}

	values, err := s.cfg.Load(ctx)
	if err != nil {
		return result, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		return result, fmt.Errorf("Supabase 连接不可用: %w", values.SupabaseConnErr)
	}
	if s.store == nil {
		return result, fmt.Errorf("数据库连接池未初始化，无法读取本地来源表")
	}

	localRows, err := s.store.ReadLocalRows(ctx)
	if err != nil {
		return result, err
	}
	localKeyRows, err := s.store.ReadLocalKeyRows(ctx)
	if err != nil {
		return result, err
	}

	metaGroups, metaOrder := groupByStockID(localRows)
	keyGroups, keyOrder := groupByFtmmSymbol(localKeyRows)

	keyStat, err := s.store.KeyLocalStat(ctx)
	if err != nil {
		return result, err
	}
	result.LocalRows = int64(len(localRows))
	result.RowsTotal = int64(len(metaOrder))
	result.KeyLocalRows = keyStat.Rows
	result.KeyResolvable = int64(len(keyOrder))
	if result.KeyLocalRows > result.KeyResolvable {
		result.KeyUnresolved = result.KeyLocalRows - result.KeyResolvable
	}

	batchSize := NormalizeBatchSize(req.BatchSize)
	client := s.newClient(values.SupabaseConn)
	log.Printf("[cdeu][uscbackfill] sync start meta=%s.%s(key=%s) key_table=%s.%s(key=%s) local_meta_rows=%d stock_ids=%d local_key_rows=%d key_resolvable=%d batch_size=%d project=%s",
		MetaTable, MetaUSCColumn, MetaKeyColumn, KeyTable, KeyUSCColumn+"+"+KeyStockIDColumn, KeyKeyColumn,
		len(localRows), len(metaOrder), result.KeyLocalRows, len(keyOrder), batchSize,
		values.SupabaseConn.ProjectID)

	totalUnits := result.RowsTotal + result.KeyResolvable
	doneUnits := 0

	// ---------- 阶段 ①：public.finv_quote_secu_meta.usc（按 stock_id） ----------
	seen := 0
	for batch := 0; seen < len(metaOrder); batch++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		batchStart := s.now()
		end := seen + batchSize
		if end > len(metaOrder) {
			end = len(metaOrder)
		}
		batchIDs := metaOrder[seen:end]

		upstream, err := client.FetchByIntIDs(ctx, MetaTable, metaUpstreamSelect, MetaKeyColumn, batchIDs)
		if err != nil {
			return result, err
		}
		current, err := indexMetaUpstream(upstream)
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
			target, decided := pickUSC(metaGroups[id], up.FtmmSymbol)
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
			jobs = append(jobs, patchJob{
				key:    strconv.FormatInt(id, 10),
				values: map[string]any{MetaUSCColumn: target},
			})
		}

		affected, err := patchAll(ctx, client, MetaTable, MetaKeyColumn, jobs)
		if err != nil {
			return result, err
		}

		result.Batches = batch + 1
		result.Fill += batchFill
		result.Overwrite += batchOverwrite
		result.Unchanged += batchUnchanged
		result.Ambiguous += batchAmbiguous
		result.NoTarget += batchNoTarget

		batchElapsed := s.now().Sub(batchStart).Milliseconds()
		doneUnits += end - seen
		log.Printf("[cdeu][uscbackfill] phase=meta batch=%d offset=%d keys=%d upstream_hit=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d patched=%d elapsed_ms=%d",
			batch+1, seen, len(batchIDs), len(batchIDs)-batchNoTarget, batchFill, batchOverwrite,
			batchUnchanged, batchAmbiguous, batchNoTarget, affected, batchElapsed)

		if onProgress != nil {
			onProgress(Progress{
				Phase:     PhaseMeta,
				Batch:     batch + 1,
				Offset:    seen,
				Fetched:   len(batchIDs),
				RowsDone:  doneUnits,
				RowsTotal: totalUnits,
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

	// ---------- 阶段 ②：public.finv_quote_secu_key.usc + stock_id（按 ftmm_symbol） ----------
	seen = 0
	for batch := 0; seen < len(keyOrder); batch++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		batchStart := s.now()
		end := seen + batchSize
		if end > len(keyOrder) {
			end = len(keyOrder)
		}
		batchKeys := keyOrder[seen:end]

		upstream, err := client.FetchByTextKeys(ctx, KeyTable, keyUpstreamSelect, KeyKeyColumn, batchKeys)
		if err != nil {
			return result, err
		}
		current, err := indexKeyUpstream(upstream)
		if err != nil {
			return result, err
		}

		jobs := make([]patchJob, 0, len(batchKeys))
		batchFill, batchOverwrite, batchUnchanged := 0, 0, 0
		batchAmbiguous, batchNoTarget := 0, 0
		for _, symbol := range batchKeys {
			up, ok := current[symbol]
			if !ok {
				batchNoTarget++
				continue
			}
			picked, decided := pickUSC(keyGroups[symbol], symbol)
			if !decided {
				batchAmbiguous++
				continue
			}
			// 目标值：usc = 本地定值，stock_id = 本地键表记录的 stock_id（同组一致）。
			var stockID int64
			for _, row := range keyGroups[symbol] {
				if strings.TrimSpace(row.USC) == picked {
					stockID = row.StockID
					break
				}
			}
			if stockID == 0 {
				batchAmbiguous++
				continue
			}

			uscSame := strings.TrimSpace(up.USC) == picked
			sidSame := up.StockID != nil && *up.StockID == stockID
			if uscSame && sidSame {
				batchUnchanged++
				continue
			}
			if strings.TrimSpace(up.USC) == "" && up.StockID == nil {
				batchFill++
			} else {
				batchOverwrite++
			}
			jobs = append(jobs, patchJob{
				key: symbol,
				values: map[string]any{
					KeyUSCColumn:     picked,
					KeyStockIDColumn: stockID,
				},
			})
		}

		affected, err := patchAll(ctx, client, KeyTable, KeyKeyColumn, jobs)
		if err != nil {
			return result, err
		}

		result.KeyBatches = batch + 1
		result.KeyFill += batchFill
		result.KeyOverwrite += batchOverwrite
		result.KeyUnchanged += batchUnchanged
		result.KeyAmbiguous += batchAmbiguous
		result.KeyNoTarget += batchNoTarget

		batchElapsed := s.now().Sub(batchStart).Milliseconds()
		doneUnits += end - seen
		log.Printf("[cdeu][uscbackfill] phase=key batch=%d offset=%d keys=%d upstream_hit=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d patched=%d elapsed_ms=%d",
			batch+1, seen, len(batchKeys), len(batchKeys)-batchNoTarget, batchFill, batchOverwrite,
			batchUnchanged, batchAmbiguous, batchNoTarget, affected, batchElapsed)

		if onProgress != nil {
			onProgress(Progress{
				Phase:        PhaseKey,
				Batch:        batch + 1,
				Offset:       seen,
				Fetched:      len(batchKeys),
				RowsDone:     doneUnits,
				RowsTotal:    totalUnits,
				Fill:         result.Fill,
				Overwrite:    result.Overwrite,
				Unchanged:    result.Unchanged,
				Ambiguous:    result.Ambiguous,
				NoTarget:     result.NoTarget,
				KeyFill:      result.KeyFill,
				KeyOverwrite: result.KeyOverwrite,
				KeyUnchanged: result.KeyUnchanged,
				KeyAmbiguous: result.KeyAmbiguous,
				KeyNoTarget:  result.KeyNoTarget,
				ElapsedMs:    batchElapsed,
			})
		}
		seen = end
	}

	result.ElapsedMs = s.now().Sub(started).Milliseconds()
	log.Printf("[cdeu][uscbackfill] sync done meta: stock_ids=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d batches=%d | key: resolvable=%d fill=%d overwrite=%d unchanged=%d ambiguous=%d no_target=%d batches=%d | elapsed_ms=%d",
		result.RowsTotal, result.Fill, result.Overwrite, result.Unchanged, result.Ambiguous,
		result.NoTarget, result.Batches, result.KeyResolvable, result.KeyFill, result.KeyOverwrite,
		result.KeyUnchanged, result.KeyAmbiguous, result.KeyNoTarget, result.KeyBatches, result.ElapsedMs)
	return result, nil
}

// patchJob 一条待回写记录（key 为上游关联键的字符串形式）。
type patchJob struct {
	key    string
	values map[string]any
}

// patchAll 并发回写一批（并发度 patchConcurrency）；任一行失败即取消其余任务并返回错误。
func patchAll(ctx context.Context, client *secukey.Client, table, keyColumn string, jobs []patchJob) (int, error) {
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
				if err := client.PatchColumns(runCtx, table, keyColumn, job.key, job.values); err != nil {
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
		return done, fmt.Errorf("回写上游 %s 失败（已成功 %d / %d 行，重跑可续）: %w", table, done, len(jobs), firstErr)
	}
	return done, nil
}

// metaUpstreamRow 表 ①（finv_quote_secu_meta）一行的现值。
type metaUpstreamRow struct {
	StockID    int64
	USC        string
	FtmmSymbol string
}

// keyUpstreamRow 表 ②（finv_quote_secu_key）一行的现值。
type keyUpstreamRow struct {
	FtmmSymbol string
	USC        string
	StockID    *int64 // 上游为 NULL 时保持 nil（用于区分「空」与「0」）
}

// indexMetaUpstream 把上游原始 JSON 行解析成 stock_id → 现值。
func indexMetaUpstream(raw []json.RawMessage) (map[int64]metaUpstreamRow, error) {
	rows := make(map[int64]metaUpstreamRow, len(raw))
	for _, item := range raw {
		var parsed struct {
			StockID    *int64  `json:"stock_id"`
			USC        *string `json:"usc"`
			FtmmSymbol *string `json:"ftmm_symbol"`
		}
		if err := json.Unmarshal(item, &parsed); err != nil {
			return nil, fmt.Errorf("解析上游行失败: %w", err)
		}
		if parsed.StockID == nil {
			continue
		}
		row := metaUpstreamRow{StockID: *parsed.StockID}
		if parsed.USC != nil {
			row.USC = *parsed.USC
		}
		if parsed.FtmmSymbol != nil {
			row.FtmmSymbol = *parsed.FtmmSymbol
		}
		if _, exists := rows[row.StockID]; exists {
			log.Printf("[cdeu][uscbackfill] 上游 stock_id 重复 stock_id=%d（按后者取值）", row.StockID)
		}
		rows[row.StockID] = row
	}
	return rows, nil
}

// indexKeyUpstream 把上游原始 JSON 行解析成 ftmm_symbol → 现值。
func indexKeyUpstream(raw []json.RawMessage) (map[string]keyUpstreamRow, error) {
	rows := make(map[string]keyUpstreamRow, len(raw))
	for _, item := range raw {
		var parsed struct {
			FtmmSymbol *string `json:"ftmm_symbol"`
			USC        *string `json:"usc"`
			StockID    *int64  `json:"stock_id"`
		}
		if err := json.Unmarshal(item, &parsed); err != nil {
			return nil, fmt.Errorf("解析上游行失败: %w", err)
		}
		if parsed.FtmmSymbol == nil || *parsed.FtmmSymbol == "" {
			continue
		}
		row := keyUpstreamRow{FtmmSymbol: *parsed.FtmmSymbol, StockID: parsed.StockID}
		if parsed.USC != nil {
			row.USC = *parsed.USC
		}
		rows[row.FtmmSymbol] = row
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

// groupByFtmmSymbol 按 ftmm_symbol 归并本地证券键表行，并返回 ftmm_symbol 升序序列。
func groupByFtmmSymbol(rows []LocalKeyRow) (map[string][]LocalRow, []string) {
	groups := make(map[string][]LocalRow, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, exists := groups[row.FtmmSymbol]; !exists {
			order = append(order, row.FtmmSymbol)
		}
		groups[row.FtmmSymbol] = append(groups[row.FtmmSymbol], LocalRow{
			USC:     row.USC,
			StockID: row.StockID,
		})
	}
	sort.Strings(order)
	return groups, order
}

// pickUSC 从同一键的多条本地行里定出要回填的 usc（需求 11055）。
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
