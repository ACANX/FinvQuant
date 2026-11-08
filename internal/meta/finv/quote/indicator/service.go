// Package indicator 实现「证券指标数据同步」（需求 11050）：把上游 Supabase
// public.finv_quote_secu_meta 的每一条记录**一次写三张本地表**。
//
// 字段分配（对应「证券参数同步 → 证券指标数据同步」Tab 2）：
//
//	① cdeu.finv_quote_secu_indicator：131 列同名同步（dt_update 由触发器维护，不参与写入/比对）
//	② futu.finv_futu_quote_collect：身份字段同名同步
//	   （stock_id / market_type / market_code / instrument_type / sub_instrument_type /
//	     market_label / instrument；futu_symbol 由 ftmm_symbol 去掉市场前缀派生）
//	③ futu.finv_futu_secu_key：只补 url（来源 secu_url）与 stock_id，按 ftmm_symbol 对齐
//
// 上游剩余字段（flag_option / flag_futures / flag_plate / flag_etf_theme / name_en /
// market_type 之外的 market 等）本地无对应列，不落库。
//
// 关键口径：
//   - `usc` 为空时用 `ftmm_symbol` 占位（上游 10,910 行里 3,673 行 usc 为 NULL），
//     指标表与采集属性表都以占位后的值作主键；
//   - 增量：主键不存在则插入；已存在且字段与源不同才更新；一致的行不改写；
//   - 采集属性表有唯一约束 (market_code, futu_symbol)：若该对已被**别的 usc** 占用，
//     该行跳过并计入 qc_skip（不报错、不覆盖别人的 usc）；
//   - 采集属性表不写 flag_enable（新增行取表默认 '0'），避免同步顺手把所有证券开启采集。
package indicator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// 上游源表（PostgREST 上一律带 schema，实际请求走 Accept-Profile 头）。
const (
	// SourceSchema 上游 schema。
	SourceSchema = "public"
	// SourceTableName 上游表名。
	SourceTableName = "finv_quote_secu_meta"
	// SourceTable 上游表（schema.table，供展示与请求）。
	SourceTable = SourceSchema + "." + SourceTableName
)

// 本地目标表。
const (
	// IndicatorTable 证券指标表（131 列）。
	IndicatorTable = "cdeu.finv_quote_secu_indicator"
	// QuoteCollectTable 富途采集属性表。
	QuoteCollectTable = "futu.finv_futu_quote_collect"
	// SecuKeyTable 证券键表。
	SecuKeyTable = "futu.finv_futu_secu_key"
)

// 批量大小取值（与基础参数同步保持一致）。
const (
	DefaultBatchSize = 1000
	MinBatchSize     = 100
	MaxBatchSize     = 5000
)

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

// Request 一次同步的入参。
type Request struct {
	// BatchSize 每批行数（留空或 <=0 取 1000，收敛到 100~5000）。
	BatchSize int
}

// Progress 一批完成时的进度快照。
type Progress struct {
	Batch        int   // 当前批次号（从 1 开始）
	Offset       int   // 本批起始 offset
	Fetched      int   // 本批拉取行数
	RowsTotal    int64 // 源表总行数（取不到为 0）
	RowsReceive  int   // 累计拉取
	IndInsert    int   // 指标表累计新增
	IndUpdate    int   // 指标表累计更新
	IndUnchanged int   // 指标表累计未变
	QcInsert     int   // 采集属性表累计新增
	QcUpdate     int   // 采集属性表累计更新
	QcSkip       int   // 采集属性表因唯一键撞车累计跳过
	KeyUpdate    int   // 证券键表累计补全（url / stock_id）
	ElapsedMs    int64 // 本批耗时
}

// Result 一次同步的结果。
type Result struct {
	SourceTable  string `json:"source_table"`
	RowsTotal    int64  `json:"rows_total"`
	Batches      int    `json:"batches"`
	RowsReceive  int    `json:"rows_receive"`
	IndInsert    int    `json:"ind_insert"`
	IndUpdate    int    `json:"ind_update"`
	IndUnchanged int    `json:"ind_unchanged"`
	QcInsert     int    `json:"qc_insert"`
	QcUpdate     int    `json:"qc_update"`
	QcSkip       int    `json:"qc_skip"`
	KeyUpdate    int    `json:"key_update"`
	ElapsedMs    int64  `json:"elapsed_ms"`
}

// TableStat 单表行数统计（状态卡片用）。
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
	// SourceTable 上游源表（固定 public.finv_quote_secu_meta）。
	SourceTable string `json:"source_table"`
	// Targets 三张本地目标表的行数。
	Targets []TableStat `json:"targets"`
	// BatchSizeDefault 默认批量。
	BatchSizeDefault int `json:"batch_size_default"`
}

// Service 证券指标同步服务（无状态）。
type Service struct {
	cfg       *syscfg.Store
	store     *Store
	newClient func(syscfg.SupabaseConn) *secukey.Client
	now       func() time.Time
}

// NewService 创建服务；pool 为空时（未接库的单测场景）所有写操作返回可读错误。
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

// Status 返回状态快照：只读本地（配置 + 三张目标表行数）。
func (s *Service) Status(ctx context.Context) (StatusSnapshot, error) {
	snapshot := StatusSnapshot{
		SourceTable:      SourceTable,
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
	stats, err := s.store.TableStats(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.Targets = stats
	return snapshot, nil
}

// Sync 执行一次同步（无进度回调）；见 SyncWithProgress。
func (s *Service) Sync(ctx context.Context, req Request) (Result, error) {
	return s.SyncWithProgress(ctx, req, nil)
}

// SyncWithProgress 执行一次同步：按 ftmm_symbol 升序分批拉取上游并写三张本地表。
//
// 每批写入完成后回调 onProgress（可为 nil），供异步运行写实时日志。
// 任一批失败即返回错误；已写入的批次不回滚（增量写入可重入）。
func (s *Service) SyncWithProgress(ctx context.Context, req Request, onProgress func(Progress)) (Result, error) {
	started := s.now()
	result := Result{SourceTable: SourceTable}

	values, err := s.cfg.Load(ctx)
	if err != nil {
		return result, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.SupabaseConnErr != nil {
		return result, fmt.Errorf("Supabase 连接不可用: %w", values.SupabaseConnErr)
	}
	if s.store == nil {
		return result, fmt.Errorf("数据库连接池未初始化，无法写入 %s", IndicatorTable)
	}

	batchSize := NormalizeBatchSize(req.BatchSize)
	client := s.newClient(values.SupabaseConn)
	log.Printf("[cdeu][indicator] sync start source=%s targets=%s,%s,%s batch_size=%d project=%s",
		SourceTable, IndicatorTable, QuoteCollectTable, SecuKeyTable, batchSize, values.SupabaseConn.ProjectID)

	seen := 0
	for batch := 1; ; batch++ {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		batchStart := s.now()
		rows, total, err := client.FetchRawBatch(ctx, SourceTable, "*", seen, batchSize)
		if err != nil {
			return result, err
		}
		if total > 0 {
			result.RowsTotal = total
		}
		if len(rows) == 0 {
			break
		}

		payload, err := json.Marshal(rows)
		if err != nil {
			return result, fmt.Errorf("序列化第 %d 批失败: %w", batch, err)
		}
		stat, err := s.store.UpsertBatch(ctx, payload)
		if err != nil {
			return result, err
		}

		unchanged := len(rows) - stat.IndInsert - stat.IndUpdate
		if unchanged < 0 {
			unchanged = 0
		}
		result.Batches = batch
		result.RowsReceive += len(rows)
		result.IndInsert += stat.IndInsert
		result.IndUpdate += stat.IndUpdate
		result.IndUnchanged += unchanged
		result.QcInsert += stat.QcInsert
		result.QcUpdate += stat.QcUpdate
		result.QcSkip += stat.QcSkip
		result.KeyUpdate += stat.KeyUpdate

		batchElapsed := s.now().Sub(batchStart).Milliseconds()
		log.Printf("[cdeu][indicator] batch=%d offset=%d fetched=%d ind_insert=%d ind_update=%d qc_insert=%d qc_update=%d qc_skip=%d key_update=%d elapsed_ms=%d",
			batch, seen, len(rows), stat.IndInsert, stat.IndUpdate, stat.QcInsert, stat.QcUpdate,
			stat.QcSkip, stat.KeyUpdate, batchElapsed)

		if onProgress != nil {
			onProgress(Progress{
				Batch:        batch,
				Offset:       seen,
				Fetched:      len(rows),
				RowsTotal:    result.RowsTotal,
				RowsReceive:  result.RowsReceive,
				IndInsert:    result.IndInsert,
				IndUpdate:    result.IndUpdate,
				IndUnchanged: result.IndUnchanged,
				QcInsert:     result.QcInsert,
				QcUpdate:     result.QcUpdate,
				QcSkip:       result.QcSkip,
				KeyUpdate:    result.KeyUpdate,
				ElapsedMs:    batchElapsed,
			})
		}

		seen += len(rows)
		if len(rows) < batchSize {
			break
		}
		if result.RowsTotal > 0 && int64(seen) >= result.RowsTotal {
			break
		}
	}

	result.ElapsedMs = s.now().Sub(started).Milliseconds()
	log.Printf("[cdeu][indicator] sync done source=%s rows_total=%d received=%d ind_insert=%d ind_update=%d ind_unchanged=%d qc_insert=%d qc_update=%d qc_skip=%d key_update=%d batches=%d elapsed_ms=%d",
		SourceTable, result.RowsTotal, result.RowsReceive, result.IndInsert, result.IndUpdate,
		result.IndUnchanged, result.QcInsert, result.QcUpdate, result.QcSkip, result.KeyUpdate,
		result.Batches, result.ElapsedMs)
	return result, nil
}
