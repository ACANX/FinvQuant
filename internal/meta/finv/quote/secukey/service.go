// Package secukey 实现「证券参数同步」：把 Supabase 上的证券键表
// （默认 public.finv_quote_secu_key）分批增量同步到本地 futu.finv_futu_secu_key。
//
// 触发方式：只由前端菜单「证券参数同步」里的按钮手动触发（需求 11042/11043），
// 没有定时任务；一次请求内完成全部批次（同步执行，返回逐批日志）。
//
// 数据流：
//
//	Supabase PostgREST              本地 PostgreSQL
//	  GET /rest/v1/<源表>?limit&offset   →  INSERT ... ON CONFLICT (ftmm_symbol) DO UPDATE
//	    （按 ftmm_symbol 升序分页）           WHERE 目标行与源行 IS DISTINCT FROM
//
// 「增量」的口径：主键 ftmm_symbol 不存在则插入；已存在但任一字段与源不一致才更新，
// 完全一致的行不改写（既不产生 WAL，也不会让 gmt_update 触发器前进）。
//
// 字段映射（源 → 目标）：
//
//	ftmm_symbol → ftmm_symbol（主键，唯一键）
//	instrument  → instrument
//	market      → market
//	code        → code
//	enable      → flag_enable（NULL 视为 1）
//	priority    → priority（NULL 视为 10，越界收敛到 1~1000000）
//	status      → status（空值或非法值归一为 PENDING）
//	url         → url（NULL 保持 NULL）
//
// 本地专有列 stock_id / usc 不参与同步（源表没有这两列），同步也不清除它们。
package secukey

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 目标表（显式写 schema，避免 search_path 命中同名表）。
const (
	// TargetTable 本地证券键表。
	TargetTable = "futu.finv_futu_secu_key"
	// DefaultBatchSize 默认每批行数。
	DefaultBatchSize = 1000
	// MinBatchSize 允许的最小批量。
	MinBatchSize = 100
	// MaxBatchSize 允许的最大批量。
	MaxBatchSize = 5000
)

// 目标表 status 允许的取值（与 V300138 的 CHECK 约束一致）。
const (
	StatusPending = "PENDING"
	StatusQueued  = "QUEUED"
	StatusDone    = "DONE"
	StatusFailed  = "FAILED"
	StatusSkip    = "SKIP"
)

// allowedStatus 目标表 status 的合法集合。
var allowedStatus = map[string]struct{}{
	StatusPending: {},
	StatusQueued:  {},
	StatusDone:    {},
	StatusFailed:  {},
	StatusSkip:    {},
}

// NormalizeStatus 把源表 status 归一化为目标表可接受的取值。
//
// 空值、未知取值一律落 PENDING（安全侧：未处理的键等待采集），
// 返回值 second 为 true 表示发生了归一化（用于统计 status_reset）。
func NormalizeStatus(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if _, ok := allowedStatus[value]; ok {
		return value, false
	}
	return StatusPending, true
}

// NormalizeFlagEnable 源表 enable → 目标表 flag_enable（NULL 或非 0 视为启用）。
func NormalizeFlagEnable(raw *int) int16 {
	if raw != nil && *raw == 0 {
		return 0
	}
	return 1
}

// NormalizePriority 源表 priority → 目标表 priority（NULL 视为 10，越界收敛）。
func NormalizePriority(raw *int) int {
	const (
		fallback = 10
		minValue = 1
		maxValue = 1000000
	)
	if raw == nil {
		return fallback
	}
	value := *raw
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// NormalizeBatchSize 把请求里的批量大小收敛到 100~5000，<=0 时取默认 1000。
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

// SourceRow 源表一行（只取需要同步的列；源表其余列如 note/count_collect 不参与同步）。
type SourceRow struct {
	FtmmSymbol string  `json:"ftmm_symbol"`
	Market     string  `json:"market"`
	Code       string  `json:"code"`
	Instrument string  `json:"instrument"`
	Enable     *int    `json:"enable"`
	URL        *string `json:"url"`
	Priority   *int    `json:"priority"`
	Status     *string `json:"status"`
	// StockID 富途证券内部 ID（上游补列后纳入同步，需求 11062；NULL 时保留本地值）。
	StockID *int64 `json:"stock_id"`
	// USC 证券统一编码（上游补列后纳入同步，需求 11062；NULL 时保留本地值）。
	USC *string `json:"usc"`
}

// KeyRow 归一化后的待写入行（已通过校验，可直接进 SQL）。
type KeyRow struct {
	FtmmSymbol string
	Market     string
	Code       string
	Instrument string
	FlagEnable int16
	Priority   int
	Status     string
	URL        *string
	// StockID / USC 与源同口径；为 nil 表示源为 NULL（本地保留原值，不清空）。
	StockID *int64
	USC     *string
}

// UpsertStat 一批写入的统计（只统计**实际改动**的行）。
type UpsertStat struct {
	Insert int // 新增
	Update int // 更新（字段确实变化）
}

// BatchLog 一批同步的执行记录（前端逐行展示）。
type BatchLog struct {
	Batch    int   `json:"batch"`
	Offset   int   `json:"offset"`
	Fetched  int   `json:"fetched"`
	Insert   int   `json:"insert"`
	Update   int   `json:"update"`
	Skipped  int   `json:"skipped"`
	ElapsedM int64 `json:"elapsed_ms"`
}

// Progress 一批完成时的进度快照（供异步运行写入实时日志）。
//
// 各计数字段与 SyncResult 同口径：Insert/Update 只统计**实际改动**的行。
type Progress struct {
	Batch       int   // 当前批次号（从 1 开始）
	Offset      int   // 本批起始 offset
	Fetched     int   // 本批拉取行数
	RowsTotal   int64 // 源表总行数（取不到为 0）
	RowsReceive int   // 累计拉取行数
	Insert      int   // 累计新增
	Update      int   // 累计更新
	Unchanged   int   // 累计未变
	Skipped     int   // 累计跳过
	StatusReset int   // 累计 status 归一
	ElapsedMs   int64 // 本批耗时
}

// SyncRequest 一次同步的入参。
type SyncRequest struct {
	// SourceTable 源表名（可留空：留空取系统配置 20014，仍为空则用默认 finv_quote_secu_key）。
	SourceTable string
	// BatchSize 每批行数（留空或 <=0 取 1000，收敛到 100~5000）。
	BatchSize int
}

// SyncResult 一次同步的结果（与前端「同步结果」卡片一一对应）。
type SyncResult struct {
	SourceTable string     `json:"source_table"`
	RowsTotal   int64      `json:"rows_total"` // 源表总行数（来自 Content-Range）
	Batches     int        `json:"batches"`
	RowsReceive int        `json:"rows_receive"` // 本次实际拉取的行
	Insert      int        `json:"insert"`
	Update      int        `json:"update"`
	Unchanged   int        `json:"unchanged"`    // 与本地完全一致、未改写的行
	Skipped     int        `json:"skipped"`      // 关键字段为空而跳过的行
	StatusReset int        `json:"status_reset"` // status 非法被归一为 PENDING 的行
	ElapsedMs   int64      `json:"elapsed_ms"`
	Logs        []BatchLog `json:"logs"`
}

// StatusSnapshot 「同步状态」卡片的只读快照（不发起 Supabase 网络请求）。
type StatusSnapshot struct {
	// Configured 20013 是否已配置（格式合法）。
	Configured bool `json:"configured"`
	// ProjectID 20013 的项目 ID（敏感信息只有这一半可展示）。
	ProjectID string `json:"project_id"`
	// APIKeyMasked API Key 脱敏前缀（未配置时为空）。
	APIKeyMasked string `json:"api_key_masked"`
	// ConfigError 20013 未配置或格式非法的原因。
	ConfigError string `json:"config_error,omitempty"`
	// SourceTable 生效的源表名（20014 归一化结果）。
	SourceTable string `json:"source_table"`
	// SourceTableRaw 20014 的原始取值（可能非法，便于诊断）。
	SourceTableRaw string `json:"source_table_raw,omitempty"`
	// TargetTable 本地目标表。
	TargetTable string `json:"target_table"`
	// TargetRows 目标表当前行数。
	TargetRows int64 `json:"target_rows"`
	// TargetLastUpdate 目标表最近一次写入时间（RFC3339，空表时为空）。
	TargetLastUpdate string `json:"target_last_update,omitempty"`
	// BatchSizeDefault 默认批量。
	BatchSizeDefault int `json:"batch_size_default"`
}

// Service 证券参数同步服务（无状态；配置读 syscfg.Store，写入走 Store）。
type Service struct {
	cfg       *syscfg.Store
	store     *Store
	newClient func(syscfg.SupabaseConn) *Client
	now       func() time.Time
}

// NewService 创建服务；pool 为空时（未接库的单测场景）所有操作返回可读错误。
func NewService(pool *pgxpool.Pool) *Service {
	service := &Service{
		cfg:       syscfg.NewStore(pool),
		newClient: func(conn syscfg.SupabaseConn) *Client { return NewClient(conn) },
		now:       time.Now,
	}
	if pool != nil {
		service.store = NewStore(pool)
	}
	return service
}

// SetClientFactory 替换 Supabase 客户端工厂（单测注入用；传 nil 恢复默认）。
func (s *Service) SetClientFactory(factory func(syscfg.SupabaseConn) *Client) {
	if factory == nil {
		s.newClient = func(conn syscfg.SupabaseConn) *Client { return NewClient(conn) }
		return
	}
	s.newClient = factory
}

// Status 返回「同步状态」快照：只读本地（配置 + 目标表行数），不访问 Supabase。
func (s *Service) Status(ctx context.Context) (StatusSnapshot, error) {
	snapshot := StatusSnapshot{
		TargetTable:      TargetTable,
		BatchSizeDefault: DefaultBatchSize,
	}
	values, err := s.cfg.Load(ctx)
	if err != nil {
		return snapshot, fmt.Errorf("读取系统配置失败: %w", err)
	}
	snapshot.SourceTable = values.SupabaseSecuKeyTable
	snapshot.SourceTableRaw = values.SupabaseSecuKeyTableRaw
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
	rows, lastUpdate, err := s.store.TargetSummary(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.TargetRows = rows
	snapshot.TargetLastUpdate = lastUpdate
	return snapshot, nil
}

// SetSourceTable 保存源表名到系统配置 20014，返回归一化后的表名。
func (s *Service) SetSourceTable(ctx context.Context, raw string) (string, error) {
	table, err := syscfg.NormalizeSupabaseTableName(raw)
	if err != nil {
		return "", err
	}
	if err := s.cfg.WriteString(ctx, syscfg.KeySupabaseSecuKeyTable, table); err != nil {
		return "", err
	}
	return table, nil
}

// Sync 执行一次同步（无进度回调）；见 SyncWithProgress。
func (s *Service) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	return s.SyncWithProgress(ctx, req, nil)
}

// SyncWithProgress 执行一次同步：按 ftmm_symbol 升序逐批拉取源表并增量写入目标表。
//
// 每批写入完成后回调 onProgress（可为 nil），异步运行（Runner）据此把进度写进实时日志，
// 让前端在同步过程中就能看到批次推进，而不是等全部结束。
//
// 任一批次失败即返回错误（已写入的批次不回滚：增量写入本身就是可重入的，
// 重跑只补齐剩余部分，不会重复插入或清空已有 row）。
func (s *Service) SyncWithProgress(ctx context.Context, req SyncRequest, onProgress func(Progress)) (SyncResult, error) {
	started := s.now()
	result := SyncResult{}

	values, err := s.cfg.Load(ctx)
	if err != nil {
		return result, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		return result, fmt.Errorf("Supabase 连接不可用: %w", values.SupabaseConnErr)
	}
	if s.store == nil {
		return result, fmt.Errorf("数据库连接池未初始化，无法写入 %s", TargetTable)
	}

	table := strings.TrimSpace(req.SourceTable)
	if table == "" {
		table = values.SupabaseSecuKeyTable
	}
	normalizedTable, err := syscfg.NormalizeSupabaseTableName(table)
	if err != nil {
		return result, err
	}
	result.SourceTable = normalizedTable

	batchSize := NormalizeBatchSize(req.BatchSize)
	client := s.newClient(values.SupabaseConn)
	log.Printf("[cdeu][secukey] sync start source=%s target=%s batch_size=%d project=%s",
		normalizedTable, TargetTable, batchSize, values.SupabaseConn.ProjectID)

	seen := 0
	for batch := 1; ; batch++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		batchStart := s.now()
		rows, total, err := client.FetchBatch(ctx, normalizedTable, seen, batchSize)
		if err != nil {
			return result, err
		}
		result.RowsTotal = total
		if len(rows) == 0 {
			break
		}

		prepared, skipped, statusReset := prepareRows(rows)
		result.Skipped += skipped
		result.StatusReset += statusReset

		stat, err := s.store.UpsertBatch(ctx, prepared)
		if err != nil {
			return result, err
		}

		unchanged := len(prepared) - stat.Insert - stat.Update
		result.Batches = batch
		result.RowsReceive += len(rows)
		result.Insert += stat.Insert
		result.Update += stat.Update
		result.Unchanged += unchanged
		result.Logs = append(result.Logs, BatchLog{
			Batch:    batch,
			Offset:   seen,
			Fetched:  len(rows),
			Insert:   stat.Insert,
			Update:   stat.Update,
			Skipped:  skipped,
			ElapsedM: s.now().Sub(batchStart).Milliseconds(),
		})
		log.Printf("[cdeu][secukey] batch=%d offset=%d fetched=%d insert=%d update=%d unchanged=%d skipped=%d elapsed_ms=%d",
			batch, seen, len(rows), stat.Insert, stat.Update, unchanged, skipped, s.now().Sub(batchStart).Milliseconds())

		if onProgress != nil {
			onProgress(Progress{
				Batch:       batch,
				Offset:      seen, // 回调在 seen 自增之前触发，这里就是本批起始 offset
				Fetched:     len(rows),
				RowsTotal:   total,
				RowsReceive: result.RowsReceive,
				Insert:      result.Insert,
				Update:      result.Update,
				Unchanged:   result.Unchanged,
				Skipped:     result.Skipped,
				StatusReset: result.StatusReset,
				ElapsedMs:   s.now().Sub(batchStart).Milliseconds(),
			})
		}

		seen += len(rows)
		if len(rows) < batchSize {
			break
		}
		if total > 0 && int64(seen) >= total {
			break
		}
	}

	result.ElapsedMs = s.now().Sub(started).Milliseconds()
	log.Printf("[cdeu][secukey] sync done source=%s rows_total=%d received=%d insert=%d update=%d unchanged=%d skipped=%d status_reset=%d batches=%d elapsed_ms=%d",
		result.SourceTable, result.RowsTotal, result.RowsReceive, result.Insert, result.Update,
		result.Unchanged, result.Skipped, result.StatusReset, result.Batches, result.ElapsedMs)
	return result, nil
}

// prepareRows 校验 + 归一化一批源行：返回可直接写入的行，以及跳过/归一化的计数。
//
// 同一批内 ftmm_symbol 重复时保留最后一条（PostgreSQL 的
// 「ON CONFLICT DO UPDATE command cannot affect row a second time」不允许同批同键写入两次）。
func prepareRows(rows []SourceRow) (prepared []KeyRow, skipped int, statusReset int) {
	prepared = make([]KeyRow, 0, len(rows))
	indexBySymbol := make(map[string]int, len(rows))
	for _, row := range rows {
		symbol := strings.TrimSpace(row.FtmmSymbol)
		market := strings.TrimSpace(row.Market)
		code := strings.TrimSpace(row.Code)
		instrument := strings.TrimSpace(row.Instrument)
		if symbol == "" || market == "" || code == "" || instrument == "" {
			skipped++
			continue
		}
		status, reset := NormalizeStatus(valueOr(row.Status, ""))
		if reset {
			statusReset++
		}
		item := KeyRow{
			FtmmSymbol: symbol,
			Market:     market,
			Code:       code,
			Instrument: instrument,
			FlagEnable: NormalizeFlagEnable(row.Enable),
			Priority:   NormalizePriority(row.Priority),
			Status:     status,
			URL:        trimPtr(row.URL),
			StockID:    row.StockID,
			USC:        trimPtr(row.USC),
		}
		if pos, ok := indexBySymbol[symbol]; ok {
			prepared[pos] = item
			continue
		}
		indexBySymbol[symbol] = len(prepared)
		prepared = append(prepared, item)
	}
	return prepared, skipped, statusReset
}

// valueOr 解引用可空字符串。
func valueOr(raw *string, fallback string) string {
	if raw == nil {
		return fallback
	}
	return *raw
}

// trimPtr 去掉可空字符串的空白；空白视为 NULL。
func trimPtr(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
