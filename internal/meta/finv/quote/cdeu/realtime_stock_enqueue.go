package cdeu

// 待录入证券「入队」：把证券键登记进本地键表 `futu.finv_futu_secu_key`（需求 11153）。
//
// 三步走、人工确认（需求 11154 / 11155 / 11156）：
//
//	① 探测   POST .../Pending/Enqueue/Probe —— 按当前（可能已人工改过的）键字段查键表，返回「存在 / 不存在」；
//	② 核对   前端据探测结果决定按钮：不存在 → 「确认入队」；存在 → 「更新键」；键字段改动后按钮禁用，必须重新探测；
//	③ 提交   POST .../Pending/Enqueue（插入）/ POST .../Pending/Enqueue/Update（更新），服务端再校验一次存在性。
//
// 写入键表的字段（均可人工修改，只有 `ftmm_symbol` 由 `market + code` 派生后只读）：
//
//	ftmm_symbol ← upper(market || '.' || code)   （主键，随 market/code 变化）
//	market / code                                （可改）
//	stock_id                                     （可改，默认快照证券编号）
//	instrument                                   （可改，必须落在键表词表内）
//	status                                       （可改，PENDING/QUEUED/DONE/FAILED/SKIP）
//	flag_enable                                  （可改，0/1）
//	priority                                     （可改，1..1000000）
//	usc                                          （不写，留空等「USC 映射规则」解析）

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	symbolpkg "github.com/acanx/finvquant/internal/meta/finv/quote/symbol"
)

// tableSecuKey 本地证券键表（与 secukey 包同一张表，显式写 schema 避免 search_path 影响）。
const tableSecuKey = "futu.finv_futu_secu_key"

// StatusPendingKey 键表 status 的「待处理」取值（与 V300138 CHECK 约束一致）。
const StatusPendingKey = "PENDING"

// KeyInstrumentOptions 键表 `instrument` 的合法词表（取自 `futu.finv_futu_secu_key` 实际取值）。
//
// 注意与采集表 `futu.finv_futu_quote_collect.instrument` 不同：那边板块写作 `sectors`（复数），
// 键表写作 `sector`（单数）。混用会导致 USC 映射规则匹配不到。
var KeyInstrumentOptions = []string{"stock", "etf", "index", "sector", "futures", "crypto", "forex", "market"}

// KeyStatusOptions 键表 `status` 的合法词表（与 V300138 CHECK 约束一致）。
var KeyStatusOptions = []string{"PENDING", "QUEUED", "DONE", "FAILED", "SKIP"}

// keyInstrumentAllowed 判定 instrument 是否在键表词表内。
func keyInstrumentAllowed(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, option := range KeyInstrumentOptions {
		if option == normalized {
			return true
		}
	}
	return false
}

// keyStatusAllowed 判定 status 是否在键表词表内。
func keyStatusAllowed(value string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	for _, option := range KeyStatusOptions {
		if option == normalized {
			return true
		}
	}
	return false
}

// keyInstrumentByInstrumentType 由富途品种码 / 市场类型推出键表要求的 `instrument`。
//
// instrument_type（快照字段，富途品种码）：3/12→stock、4→etf、6→index、7→sector、9→futures、11→crypto；
// 缺失（0）时退回 market_type：11→forex（外汇）、17→crypto（加密货币）、其余按股票处理。
func keyInstrumentByInstrumentType(instrumentType, marketType int) string {
	switch instrumentType {
	case 4:
		return "etf"
	case 6:
		return "index"
	case 7:
		return "sector"
	case 9:
		return "futures"
	case 11:
		return "crypto"
	case 3, 12:
		return "stock"
	}
	// instrument_type 缺失：用 market_type 兜底（HK=1 / US=2 / A股=4 / FX=11 / CC=17）
	switch marketType {
	case 11:
		return "forex"
	case 17:
		return "crypto"
	default:
		return "stock"
	}
}

// PendingSecuKeyFields 键表一行可写字段（`usc` 只读返回，不写入）。
type PendingSecuKeyFields struct {
	FtmmSymbol string `json:"ftmm_symbol"`
	Market     string `json:"market"`
	Code       string `json:"code"`
	StockID    int64  `json:"stock_id"`
	Instrument string `json:"instrument"`
	Status     string `json:"status"`
	FlagEnable int    `json:"flag_enable"`
	Priority   int    `json:"priority"`
	USC        string `json:"usc"`
}

// PendingSecuKeyProbeRequest 探测请求：只允许改 `market` / `code`（其余字段不影响「键是否存在」）。
type PendingSecuKeyProbeRequest struct {
	StockID int64  `json:"stock_id"`
	Market  string `json:"market"`
	Code    string `json:"code"`
}

// PendingSecuKeyProbe 探测结果。
type PendingSecuKeyProbe struct {
	StockID int64                 `json:"stock_id"`
	Planned PendingSecuKeyFields  `json:"planned"`
	Existed bool                  `json:"existed"`
	Current *PendingSecuKeyFields `json:"current"`
	Hint    string                `json:"hint"`
}

// PendingSecuKeyWriteResult 插入 / 更新结果。
type PendingSecuKeyWriteResult struct {
	FtmmSymbol string `json:"ftmm_symbol"`
	// Action insert / update。
	Action string `json:"action"`
	// Affected 影响行数（insert=1 表示新增，update=1 表示改到）。
	Affected int64 `json:"affected"`
	// Existed insert 时目标键已存在（未写入，前端应改走「更新键」）。
	Existed bool   `json:"existed"`
	Hint    string `json:"hint"`
}

// pendingSecuKeyDraft 快照行派生出的默认写键字段。
type pendingSecuKeyDraft struct {
	Name           string
	Market         string
	Code           string
	StockID        int64
	InstrumentType string
	MarketType     string
	Instrument     string
}

// derivePendingSecuKeyDraft 读取快照行并派生默认字段（不做任何写操作）。
func (s *Store) derivePendingSecuKeyDraft(ctx context.Context, stockID int64) (pendingSecuKeyDraft, error) {
	var draft pendingSecuKeyDraft
	if stockID <= 0 {
		return draft, fmt.Errorf("%w: stock_id 必须为正整数", ErrRealtimeStockInvalid)
	}
	var name, marketLabel, stockCode, instrumentTypeRaw, marketTypeRaw string
	if err := s.pool.QueryRow(ctx, `
SELECT COALESCE(btrim(name), ''),
       COALESCE(upper(btrim(market_label)), ''),
       COALESCE(btrim(stock_code), ''),
       COALESCE(instrument_type, ''),
       COALESCE(market_type, '')
  FROM `+tableRealtimeStock+`
 WHERE stock_id = $1`, stockID).Scan(
		&name, &marketLabel, &stockCode, &instrumentTypeRaw, &marketTypeRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return draft, fmt.Errorf("%w: stock_id=%d 不存在", ErrRealtimeStockInvalid, stockID)
		}
		return draft, fmt.Errorf("derivePendingSecuKeyDraft：读取快照失败: %w", err)
	}
	if marketLabel == "" || stockCode == "" {
		return draft, fmt.Errorf("%w: 快照行缺少 市场标签 或 代码，无法派生 ftmm_symbol",
			ErrRealtimeStockInvalid)
	}
	draft.StockID = stockID
	draft.Name = name
	draft.Market = marketLabel
	draft.Code = stockCode
	draft.InstrumentType = strings.TrimSpace(instrumentTypeRaw)
	draft.MarketType = strings.TrimSpace(marketTypeRaw)
	draft.Instrument = keyInstrumentByInstrumentType(
		parseIntOr(instrumentTypeRaw, 0), parseIntOr(marketTypeRaw, 0))
	return draft, nil
}

// buildFtmmSymbol 键表主键：`upper(market || '.' || code)`（需求 11143：英文字母统一大写）。
func buildFtmmSymbol(market, code string) string {
	return symbolpkg.Normalize(strings.TrimSpace(market) + "." + strings.TrimSpace(code))
}

// normalizeKeyFields 校验并归一化待写入字段（`ftmm_symbol` 一律由 market+code 重建，不采信传入值）。
func normalizeKeyFields(fields PendingSecuKeyFields) (PendingSecuKeyFields, error) {
	fields.Market = strings.ToUpper(strings.TrimSpace(fields.Market))
	fields.Code = strings.TrimSpace(fields.Code)
	fields.Instrument = strings.ToLower(strings.TrimSpace(fields.Instrument))
	fields.Status = strings.ToUpper(strings.TrimSpace(fields.Status))
	fields.USC = ""

	if fields.Market == "" || fields.Code == "" {
		return fields, fmt.Errorf("%w: market / code 不能为空", ErrRealtimeStockInvalid)
	}
	fields.FtmmSymbol = buildFtmmSymbol(fields.Market, fields.Code)
	if !keyInstrumentAllowed(fields.Instrument) {
		return fields, fmt.Errorf("%w: instrument=%q 不在键表词表内（可选 %s）",
			ErrRealtimeStockInvalid, fields.Instrument, strings.Join(KeyInstrumentOptions, " / "))
	}
	if fields.Status == "" {
		fields.Status = StatusPendingKey
	}
	if !keyStatusAllowed(fields.Status) {
		return fields, fmt.Errorf("%w: status=%q 不在键表词表内（可选 %s）",
			ErrRealtimeStockInvalid, fields.Status, strings.Join(KeyStatusOptions, " / "))
	}
	if fields.FlagEnable != 0 && fields.FlagEnable != 1 {
		return fields, fmt.Errorf("%w: flag_enable 只能是 0 或 1", ErrRealtimeStockInvalid)
	}
	if fields.Priority < 1 || fields.Priority > 1_000_000 {
		return fields, fmt.Errorf("%w: priority 必须在 1..1000000 之间", ErrRealtimeStockInvalid)
	}
	if fields.StockID <= 0 {
		return fields, fmt.Errorf("%w: stock_id 必须为正整数", ErrRealtimeStockInvalid)
	}
	return fields, nil
}

// readSecuKeyFields 按主键读键表一行；不存在返回 (nil, nil)。
func (s *Store) readSecuKeyFields(ctx context.Context, ftmmSymbol string) (*PendingSecuKeyFields, error) {
	var (
		fields  PendingSecuKeyFields
		stockID *int64
		usc     *string
	)
	err := s.pool.QueryRow(ctx, `
SELECT ftmm_symbol, market, code, stock_id, instrument, status, flag_enable, priority, usc
  FROM `+tableSecuKey+`
 WHERE ftmm_symbol = $1`, ftmmSymbol).Scan(
		&fields.FtmmSymbol, &fields.Market, &fields.Code, &stockID, &fields.Instrument,
		&fields.Status, &fields.FlagEnable, &fields.Priority, &usc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stockID != nil {
		fields.StockID = *stockID
	}
	fields.USC = derefString(usc)
	return &fields, nil
}

// ProbePendingSecuKey 探测键表是否已有该键（需求 11155：每次提交前都要先探测）。
//
// `market` / `code` 可省略（用快照派生值）；改动后主键随之变化，因此探测结果只对本次键有效。
func (s *Store) ProbePendingSecuKey(ctx context.Context, req PendingSecuKeyProbeRequest) (PendingSecuKeyProbe, error) {
	var probe PendingSecuKeyProbe
	draft, err := s.derivePendingSecuKeyDraft(ctx, req.StockID)
	if err != nil {
		return probe, err
	}

	market := strings.TrimSpace(req.Market)
	if market == "" {
		market = draft.Market
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = draft.Code
	}
	planned := PendingSecuKeyFields{
		Market:     market,
		Code:       code,
		StockID:    draft.StockID,
		Instrument: draft.Instrument,
		Status:     StatusPendingKey,
		FlagEnable: 1,
		Priority:   10,
	}
	if planned, err = normalizeKeyFields(planned); err != nil {
		return probe, err
	}

	current, err := s.readSecuKeyFields(ctx, planned.FtmmSymbol)
	if err != nil {
		return probe, fmt.Errorf("ProbePendingSecuKey：查询键表失败: %w", err)
	}
	probe.StockID = draft.StockID
	probe.Planned = planned
	probe.Current = current
	probe.Existed = current != nil
	if probe.Existed {
		probe.Hint = fmt.Sprintf("键表已有 %s（status=%s，usc=%s），只能用「更新键」改它，不能重复入队",
			planned.FtmmSymbol, orPlaceholder(current.Status), orPlaceholder(current.USC))
	} else {
		probe.Hint = fmt.Sprintf("键表没有 %s，可以「确认入队」写入", planned.FtmmSymbol)
	}
	return probe, nil
}

// InsertPendingSecuKey 入队：插入键表（需求 11153）。
//
// 插入前**再查一次**存在性（需求 11155：每次插入前都验证），并用
// `ON CONFLICT (ftmm_symbol) DO NOTHING` 兜住并发；已存在时返回 `Existed=true` 不写，
// 前端应改走「更新键」。
func (s *Store) InsertPendingSecuKey(ctx context.Context, fields PendingSecuKeyFields) (PendingSecuKeyWriteResult, error) {
	result := PendingSecuKeyWriteResult{Action: "insert"}
	normalized, err := normalizeKeyFields(fields)
	if err != nil {
		return result, err
	}
	result.FtmmSymbol = normalized.FtmmSymbol

	if existing, readErr := s.readSecuKeyFields(ctx, normalized.FtmmSymbol); readErr != nil {
		return result, fmt.Errorf("InsertPendingSecuKey：查询键表失败: %w", readErr)
	} else if existing != nil {
		result.Existed = true
		result.Hint = fmt.Sprintf("键表已有 %s（status=%s，usc=%s），未重复入队；如需改字段请用「更新键」",
			normalized.FtmmSymbol, orPlaceholder(existing.Status), orPlaceholder(existing.USC))
		return result, nil
	}

	var inserted string
	err = s.pool.QueryRow(ctx, `
INSERT INTO `+tableSecuKey+` AS k
       (ftmm_symbol, market, code, stock_id, instrument, status, flag_enable, priority)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (ftmm_symbol) DO NOTHING
RETURNING k.ftmm_symbol`,
		normalized.FtmmSymbol, normalized.Market, normalized.Code, normalized.StockID,
		normalized.Instrument, normalized.Status, normalized.FlagEnable, normalized.Priority,
	).Scan(&inserted)
	switch {
	case err == nil:
		result.Affected = 1
		result.Hint = fmt.Sprintf("已入队 %s（instrument=%s，status=%s）；到「USC 映射规则」跑一次计算后，"+
			"本页可用「手动补录USC」完成同步", normalized.FtmmSymbol, normalized.Instrument, normalized.Status)
		return result, nil
	case errors.Is(err, pgx.ErrNoRows):
		// 并发下被别人抢先插入
		result.Existed = true
		result.Hint = fmt.Sprintf("键表已有 %s（并发写入），未重复入队；如需改字段请用「更新键」", normalized.FtmmSymbol)
		return result, nil
	default:
		return result, fmt.Errorf("InsertPendingSecuKey：写入键表失败: %w",
			explainUniqueViolation(err, tableSecuKey))
	}
}

// UpdatePendingSecuKey 更新键：只改非主键字段（需求 11155「更新键」场景）。
//
// 主键 `ftmm_symbol` 不可改（要改 market/code 等于换一条键，那是入队场景）。
// 不存在时返回 `Existed=false` + 提示改用入队，不静默插入。
func (s *Store) UpdatePendingSecuKey(ctx context.Context, fields PendingSecuKeyFields) (PendingSecuKeyWriteResult, error) {
	result := PendingSecuKeyWriteResult{Action: "update"}
	normalized, err := normalizeKeyFields(fields)
	if err != nil {
		return result, err
	}
	result.FtmmSymbol = normalized.FtmmSymbol

	if existing, readErr := s.readSecuKeyFields(ctx, normalized.FtmmSymbol); readErr != nil {
		return result, fmt.Errorf("UpdatePendingSecuKey：查询键表失败: %w", readErr)
	} else if existing == nil {
		result.Hint = fmt.Sprintf("键表没有 %s，无法更新；请先「确认入队」", normalized.FtmmSymbol)
		return result, nil
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableSecuKey+`
   SET stock_id    = $2,
       instrument  = $3,
       status      = $4,
       flag_enable = $5,
       priority    = $6
 WHERE ftmm_symbol = $1`,
		normalized.FtmmSymbol, normalized.StockID, normalized.Instrument,
		normalized.Status, normalized.FlagEnable, normalized.Priority)
	if err != nil {
		return result, fmt.Errorf("UpdatePendingSecuKey：更新键表失败: %w", err)
	}
	result.Affected = tag.RowsAffected()
	if result.Affected == 0 {
		result.Hint = fmt.Sprintf("键表没有 %s，未更新；请先「确认入队」", normalized.FtmmSymbol)
		return result, nil
	}
	result.Hint = fmt.Sprintf("已更新键 %s（instrument=%s，status=%s，flag_enable=%d，priority=%d）",
		normalized.FtmmSymbol, normalized.Instrument, normalized.Status,
		normalized.FlagEnable, normalized.Priority)
	return result, nil
}

func orPlaceholder(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
