// Package uscbackfill 实现「本地 usc 回推上游」（需求 11084 重构，方向与 Tab 1/2 相反）：
//
//	本地 futu.finv_futu_secu_key.usc + priority（73,212 行）
//	    ──按 ftmm_symbol 关联──▶ 上游 public.finv_quote_secu_key.usc + priority
//
// 回推 **usc 与 priority 两列**（需求 11142 起 priority 一起同步），关联键是 ftmm_symbol
// （上游主键、本地主键，双边唯一）；两列任一不同即按同一行一次 PATCH 写回。
// 上游值已等于本地值 → 记「未变」且不发请求（幂等，重跑收敛）；为空 → 「填入」；
// 有值但不同 → 「覆盖」。上游没有该 ftmm_symbol → 「无对应」，跳过。
//
// 历史逻辑（已废弃，需求 11084）：一次运行写两张上游表——① public.finv_quote_secu_meta.usc
// 按 stock_id 关联、② public.finv_quote_secu_key 的 usc + stock_id 按 ftmm_symbol 关联，
// 并从 futu.finv_futu_quote_collect 拼 usc / stock_id、对同一 stock_id 多条 usc 做「歧义」裁决。
// 现本地 usc 已由 USC 映射规则（需求 11072）统一产出入库，上游只需照抄，故上述逻辑全部删除。
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

// 上游被回推的表。
const (
	// KeyTable 上游证券采集清单表（回推 usc）。
	KeyTable = "public.finv_quote_secu_key"
	// KeyKeyColumn 关联键（上游主键）。
	KeyKeyColumn = "ftmm_symbol"
	// KeyUSCColumn 被回推的列之一：usc。
	KeyUSCColumn = "usc"
	// KeyPriorityColumn 被回推的列之二：priority（优先级，整数）。
	KeyPriorityColumn = "priority"
)

// 本地来源表。
const (
	// LocalKeyTable 本地证券键表（usc 的唯一权威来源）。
	LocalKeyTable = "futu.finv_futu_secu_key"
)

// 从上游取回的列（关联键 + 两个待回推列的现值，用于判断填/覆盖/未变）。
const keyUpstreamSelect = KeyKeyColumn + "," + KeyUSCColumn + "," + KeyPriorityColumn

// 批量大小取值（沿用其它两个 Tab 的档位）。
const (
	// DefaultBatchSize 默认每批键值个数。
	DefaultBatchSize = 500
	// MinBatchSize 最小批量。
	MinBatchSize = 100
	// MaxBatchSize 最大批量。
	MaxBatchSize = 5000
)

// patchConcurrency 逐行 PATCH 的并发度（串行 7.3 万行太慢，并发 8 实测约 12~16 分钟）。
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

// Request 一次回推的入参。
type Request struct {
	// BatchSize 每批处理的 ftmm_symbol 个数（留空或 <=0 取 500）。
	BatchSize int
}

// Progress 一批完成时的进度快照（计数均为**累计值**）。
type Progress struct {
	Batch      int   // 批次号（从 1 开始）
	Offset     int   // 本批在序列里的起点
	Fetched    int   // 本批处理的 ftmm_symbol 个数
	RowsDone   int   // 累计已处理
	RowsTotal  int64 // 累计待处理（本地 usc 非空行数）
	Fill       int   // 上游为空 → 填入
	Overwrite  int   // 上游已有且不同 → 覆盖
	Keep       int   // 上游已一致 → 未变
	Missing    int   // 上游无该 ftmm_symbol → 跳过
	SkipEmpty  int   // 本地 usc 为空 → 跳过
	DurationMs int64
}

// Result 一次回推的结果。
type Result struct {
	KeyTable   string `json:"key_table"`
	KeyColumn  string `json:"key_column"`
	KeyKey     string `json:"key_key"`
	LocalTable string `json:"local_table"`
	LocalRows  int64  `json:"local_rows"`      // 本地键表总行数
	RowsTotal  int64  `json:"rows_total"`      // 待处理（usc 非空）行数
	SkipEmpty  int64  `json:"local_empty_usc"` // 本地 usc 为空被跳过的行数
	Batches    int    `json:"batches"`
	Fetch      int    `json:"fetch"` // 上游命中行数
	Fill       int    `json:"fill"`
	Overwrite  int    `json:"overwrite"`
	Keep       int    `json:"keep"`
	Missing    int    `json:"missing"`
	DurationMs int64  `json:"duration_ms"`
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

	// KeyTable / KeyColumn / KeyKey 上游表与关联键。
	KeyTable  string `json:"key_table"`
	KeyColumn string `json:"key_column"`
	KeyKey    string `json:"key_key"`
	// LocalTable 本地来源表。
	LocalTable string `json:"local_table"`
	// LocalRows 本地键表总行数。
	LocalRows int64 `json:"local_rows"`
	// LocalWithUSC 本地 usc 非空的行数（= 一次回推的待处理量）。
	LocalWithUSC int64 `json:"local_with_usc"`
	// LocalEmptyUSC 本地 usc 为空的行数（跳过）。
	LocalEmptyUSC int64 `json:"local_empty_usc"`

	// BatchSizeDefault 默认批量。
	BatchSizeDefault int `json:"batch_size_default"`
}

// LocalKeyRow 本地证券键表的一行（ftmm_symbol 主键 + usc + priority）。
type LocalKeyRow struct {
	FtmmSymbol string
	USC        string
	Priority   int
}

// keyTarget 一个 ftmm_symbol 要回推到的目标值（usc + priority 一起回推）。
type keyTarget struct {
	USC      string
	Priority int
}

// Service usc 回推服务（无状态）。
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

// Status 返回状态快照：只读本地（配置 + 本地键表的行数与 usc 覆盖情况）。
func (s *Service) Status(ctx context.Context) (StatusSnapshot, error) {
	snapshot := StatusSnapshot{
		KeyTable:         KeyTable,
		KeyColumn:        KeyUSCColumn,
		KeyKey:           KeyKeyColumn,
		LocalTable:       LocalKeyTable,
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
	stat, err := s.store.KeyLocalStat(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.LocalRows = stat.Rows
	snapshot.LocalWithUSC = stat.WithUSC
	snapshot.LocalEmptyUSC = stat.EmptyUSC
	return snapshot, nil
}

// Sync 执行一次回推（无进度回调）；见 SyncWithProgress。
func (s *Service) Sync(ctx context.Context, req Request) (Result, error) {
	return s.SyncWithProgress(ctx, req, nil)
}

// SyncWithProgress 执行一次回推：按 ftmm_symbol 分批读上游、比对、逐行 PATCH usc + priority。
//
// 每批结束回调 onProgress（可为 nil），供异步运行写实时日志。
// 任一批写入失败即返回错误（已写入的批次不回滚——回推是幂等的，重跑即可收敛）。
func (s *Service) SyncWithProgress(ctx context.Context, req Request, onProgress func(Progress)) (Result, error) {
	started := s.now()
	result := Result{
		KeyTable:   KeyTable,
		KeyColumn:  KeyUSCColumn,
		KeyKey:     KeyKeyColumn,
		LocalTable: LocalKeyTable,
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

	stat, err := s.store.KeyLocalStat(ctx)
	if err != nil {
		return result, err
	}
	localRows, err := s.store.ReadLocalUSC(ctx)
	if err != nil {
		return result, err
	}
	result.LocalRows = stat.Rows
	result.RowsTotal = int64(len(localRows))
	result.SkipEmpty = stat.EmptyUSC

	order, target := buildTarget(localRows)
	result.RowsTotal = int64(len(order))

	batchSize := NormalizeBatchSize(req.BatchSize)
	client := s.newClient(values.SupabaseConn)
	log.Printf("[cdeu][uscbackfill] writeback start target=%s.%s(key=%s) local=%s local_rows=%d keys=%d skip_empty=%d batch_size=%d project=%s",
		KeyTable, KeyUSCColumn, KeyKeyColumn, LocalKeyTable, result.LocalRows, len(order),
		result.SkipEmpty, batchSize, values.SupabaseConn.ProjectID)

	seen := 0
	for batch := 0; seen < len(order); batch++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		batchStart := s.now()
		end := seen + batchSize
		if end > len(order) {
			end = len(order)
		}
		batchKeys := order[seen:end]

		upstream, err := client.FetchByTextKeys(ctx, KeyTable, keyUpstreamSelect, KeyKeyColumn, batchKeys)
		if err != nil {
			return result, err
		}
		current, err := indexKeyUpstream(upstream)
		if err != nil {
			return result, err
		}

		jobs := make([]patchJob, 0, len(batchKeys))
		batchFill, batchOverwrite, batchKeep, batchMissing := 0, 0, 0, 0
		for _, symbol := range batchKeys {
			up, ok := current[symbol]
			want := target[symbol]
			switch classify(up, ok, want) {
			case classMissing:
				batchMissing++
				continue
			case classKeep:
				batchKeep++
				continue
			case classFill:
				batchFill++
			case classOverwrite:
				batchOverwrite++
			}
			jobs = append(jobs, patchJob{
				key: symbol,
				values: map[string]any{
					KeyUSCColumn:      want.USC,
					KeyPriorityColumn: want.Priority,
				},
			})
		}

		affected, err := patchAll(ctx, client, KeyTable, KeyKeyColumn, jobs)
		if err != nil {
			return result, err
		}

		result.Batches = batch + 1
		result.Fetch += len(batchKeys) - batchMissing
		result.Fill += batchFill
		result.Overwrite += batchOverwrite
		result.Keep += batchKeep
		result.Missing += batchMissing

		batchElapsed := s.now().Sub(batchStart).Milliseconds()
		log.Printf("[cdeu][uscbackfill] batch=%d offset=%d keys=%d upstream_hit=%d fill=%d overwrite=%d keep=%d missing=%d patched=%d duration_ms=%d",
			batch+1, seen, len(batchKeys), len(batchKeys)-batchMissing, batchFill, batchOverwrite,
			batchKeep, batchMissing, affected, batchElapsed)

		if onProgress != nil {
			onProgress(Progress{
				Batch:      batch + 1,
				Offset:     seen,
				Fetched:    len(batchKeys),
				RowsDone:   end,
				RowsTotal:  result.RowsTotal,
				Fill:       result.Fill,
				Overwrite:  result.Overwrite,
				Keep:       result.Keep,
				Missing:    result.Missing,
				SkipEmpty:  int(result.SkipEmpty),
				DurationMs: batchElapsed,
			})
		}
		seen = end
	}

	result.DurationMs = s.now().Sub(started).Milliseconds()
	log.Printf("[cdeu][uscbackfill] writeback done target=%s.%s keys=%d fill=%d overwrite=%d keep=%d missing=%d batches=%d duration_ms=%d",
		KeyTable, KeyUSCColumn, result.RowsTotal, result.Fill, result.Overwrite, result.Keep,
		result.Missing, result.Batches, result.DurationMs)
	return result, nil
}

// patchJob 一条待回写记录（key 为上游关联键）。
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

// 回推分类取值（classify 的返回值）。
const (
	classMissing   = "missing"
	classFill      = "fill"
	classOverwrite = "overwrite"
	classKeep      = "keep"
)

// classify 判定一个 ftmm_symbol 的回推动作（usc 与 priority 两列一起判）：
// 上游没有该键 → missing（跳过）；任一列为空 → fill；两列都一致 → keep（不发请求）；其余 → overwrite。
func classify(up keyUpstreamRow, exists bool, want keyTarget) string {
	if !exists {
		return classMissing
	}
	has := strings.TrimSpace(up.USC)
	sameUSC := has == want.USC
	samePriority := up.Priority != nil && *up.Priority == want.Priority
	switch {
	case sameUSC && samePriority:
		return classKeep
	case has == "" || up.Priority == nil:
		return classFill
	default:
		return classOverwrite
	}
}

// buildTarget 把本地行归并成「ftmm_symbol 升序序列 + 目标 usc 映射」，跳过键或值为空的行。
//
// ftmm_symbol 是本地键表主键，正常不会重复；仍按「先到先得」保留首个非空值，保证重跑结果稳定。
func buildTarget(rows []LocalKeyRow) ([]string, map[string]keyTarget) {
	order := make([]string, 0, len(rows))
	target := make(map[string]keyTarget, len(rows))
	for _, row := range rows {
		symbol := strings.TrimSpace(row.FtmmSymbol)
		value := strings.TrimSpace(row.USC)
		if symbol == "" || value == "" {
			continue
		}
		if _, exists := target[symbol]; !exists {
			order = append(order, symbol)
		}
		target[symbol] = keyTarget{USC: value, Priority: row.Priority}
	}
	sort.Strings(order)
	return order, target
}

// keyUpstreamRow 上游表（finv_quote_secu_key）一行的现值。
//
// Priority 用指针区分「NULL」与 0：上游优先级为空同样需要回填。
type keyUpstreamRow struct {
	FtmmSymbol string
	USC        string
	Priority   *int
}

// indexKeyUpstream 把上游原始 JSON 行解析成 ftmm_symbol → 现值（usc 为 NULL 时留空串）。
func indexKeyUpstream(raw []json.RawMessage) (map[string]keyUpstreamRow, error) {
	rows := make(map[string]keyUpstreamRow, len(raw))
	for _, item := range raw {
		var parsed struct {
			FtmmSymbol *string `json:"ftmm_symbol"`
			USC        *string `json:"usc"`
			Priority   *int    `json:"priority"`
		}
		if err := json.Unmarshal(item, &parsed); err != nil {
			return nil, fmt.Errorf("解析上游行失败: %w", err)
		}
		if parsed.FtmmSymbol == nil || strings.TrimSpace(*parsed.FtmmSymbol) == "" {
			continue
		}
		row := keyUpstreamRow{FtmmSymbol: *parsed.FtmmSymbol, Priority: parsed.Priority}
		if parsed.USC != nil {
			row.USC = *parsed.USC
		}
		rows[row.FtmmSymbol] = row
	}
	return rows, nil
}
