package cdeu

// 待录入证券「入队」：把证券键登记进本地键表 `futu.finv_futu_secu_key`（需求 11153）。
//
// 背景：待录入证券（快照 `usc` 为空且键表里查不到）有两类
//  1. 富途还没给它建键 —— 需要把键**登记进键表**，采集与后续链路才有依据（本文件负责这一步，「入队」）；
//  2. 键表里已经有键，但 USC 映射规则还解不出来 —— 这种情况要去「USC 映射规则」菜单补规则。
//
// **两步走，人工确认**（需求 11154）：前端点「入队」只打开对话框，
// 走 `PreviewPendingSecuEnqueue` 展示将要写入的字段与键表现状；
// 人工核对（可改 instrument）后点对话框里的「确认入队」才真正 `EnqueuePendingSecu` 落库。
//
// 入队字段来源（全部由快照行派生，不采信前端传值）：
//
//	ftmm_symbol ← upper(市场标签 || '.' || 代码)
//	market      ← 市场标签
//	code        ← 快照 stock_code
//	stock_id    ← 快照 stock_id
//	instrument  ← instrument_type（富途品种码）为主、market_type 兜底，见 keyInstrumentByInstrumentType
//	              —— 人工可在对话框里覆盖（仅限词表内取值）
//
// 键表主键是 `ftmm_symbol`，所以「已存在」判定就是主键查询；插入用
// `ON CONFLICT (ftmm_symbol) DO NOTHING` + `RETURNING`，并发下也不会重复入队。

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

// PendingSecuEnqueuePreview 「入队」对话框的预检结果（只读，供人工核对）。
type PendingSecuEnqueuePreview struct {
	StockID        int64  `json:"stock_id"`
	Name           string `json:"name"`
	FtmmSymbol     string `json:"ftmm_symbol"`
	Market         string `json:"market"`
	Code           string `json:"code"`
	InstrumentType string `json:"instrument_type"`
	MarketType     string `json:"market_type"`
	// Instrument 按品种码推导出的键表 instrument（人工可改，词表见 KeyInstrumentOptions）。
	Instrument string `json:"instrument"`
	// 键表现状：Existed=true 表示已经入过队，此次不能重复插入。
	Existed  bool   `json:"existed"`
	Status   string `json:"status"`
	USC      string `json:"usc"`
	Priority int    `json:"priority"`
	// Writable 是否允许提交入队（= !Existed）。
	Writable bool `json:"writable"`
	// Hint 给前端的下一步说明。
	Hint string `json:"hint"`
}

// PendingSecuEnqueueResult 入队结果（接口返回与前端提示用）。
type PendingSecuEnqueueResult struct {
	FtmmSymbol string `json:"ftmm_symbol"`
	Market     string `json:"market"`
	Code       string `json:"code"`
	Instrument string `json:"instrument"`
	StockID    int64  `json:"stock_id"`
	// Enqueued 本次是否真的插入了键表（true=入队成功）。
	Enqueued bool `json:"enqueued"`
	// Existed 键表里已存在该 ftmm_symbol（此时不入队，需人工补 USC）。
	Existed  bool   `json:"existed"`
	Status   string `json:"status"`
	USC      string `json:"usc"`
	Priority int    `json:"priority"`
	Hint     string `json:"hint"`
}

// pendingSecuKeyDraft 快照行派生出的键表待写入字段（preview 与 enqueue 共用）。
type pendingSecuKeyDraft struct {
	Name           string
	FtmmSymbol     string
	Market         string
	Code           string
	StockID        int64
	InstrumentType string
	MarketType     string
	Instrument     string
}

// derivePendingSecuKeyDraft 读取快照行并派生键表字段（不做任何写操作）。
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
	// 大写归一与写入链路同口径（需求 11143：ftmm_symbol 英文字母统一大写）
	draft.FtmmSymbol = symbolpkg.Normalize(marketLabel + "." + stockCode)
	draft.Instrument = keyInstrumentByInstrumentType(
		parseIntOr(instrumentTypeRaw, 0), parseIntOr(marketTypeRaw, 0))
	return draft, nil
}

// readSecuKeyState 读键表现状：exists=false 表示可以入队。
func (s *Store) readSecuKeyState(ctx context.Context, ftmmSymbol string) (
	exists bool, status, usc string, priority int, err error) {
	var (
		statusPtr, uscPtr *string
		priorityValue     int
	)
	queryErr := s.pool.QueryRow(ctx, `
SELECT status, usc, priority
  FROM `+tableSecuKey+`
 WHERE ftmm_symbol = $1`, ftmmSymbol).Scan(&statusPtr, &uscPtr, &priorityValue)
	switch {
	case queryErr == nil:
		return true, derefString(statusPtr), derefString(uscPtr), priorityValue, nil
	case errors.Is(queryErr, pgx.ErrNoRows):
		return false, "", "", 0, nil
	default:
		return false, "", "", 0, queryErr
	}
}

// PreviewPendingSecuEnqueue 「入队」对话框预检：派生字段 + 键表现状（只读，需求 11153/11154）。
func (s *Store) PreviewPendingSecuEnqueue(ctx context.Context, stockID int64) (PendingSecuEnqueuePreview, error) {
	var preview PendingSecuEnqueuePreview
	draft, err := s.derivePendingSecuKeyDraft(ctx, stockID)
	if err != nil {
		return preview, err
	}
	exists, status, usc, priority, err := s.readSecuKeyState(ctx, draft.FtmmSymbol)
	if err != nil {
		return preview, fmt.Errorf("PreviewPendingSecuEnqueue：查询键表失败: %w", err)
	}

	preview.StockID = draft.StockID
	preview.Name = draft.Name
	preview.FtmmSymbol = draft.FtmmSymbol
	preview.Market = draft.Market
	preview.Code = draft.Code
	preview.InstrumentType = draft.InstrumentType
	preview.MarketType = draft.MarketType
	preview.Instrument = draft.Instrument
	preview.Existed = exists
	preview.Status = status
	preview.USC = usc
	preview.Priority = priority
	preview.Writable = !exists
	if exists {
		preview.Hint = existingKeyHit(draft.FtmmSymbol, status, usc)
	} else {
		preview.Hint = fmt.Sprintf("键表尚无 %s，确认后写入 status=%s / flag_enable=1 / priority=10；"+
			"入队后到「USC 映射规则」跑一次计算即可解析出 usc", draft.FtmmSymbol, StatusPendingKey)
	}
	return preview, nil
}

// EnqueuePendingSecu 把一只待录入证券登记进键表（需求 11153）。
//
// instrument 允许人工覆盖（对话框里核对/改正派生值）；为空时用派生值，非法取值直接拒绝。
// 返回 `Enqueued=true` 表示本次插入成功；`Existed=true` 表示键表里已有该键（未插入，
// 需要到「USC 映射规则」菜单补 USC，之后本页可用「手动补录USC」完成同步）。
func (s *Store) EnqueuePendingSecu(ctx context.Context, stockID int64, instrument string) (PendingSecuEnqueueResult, error) {
	var result PendingSecuEnqueueResult
	draft, err := s.derivePendingSecuKeyDraft(ctx, stockID)
	if err != nil {
		return result, err
	}

	instrumentValue := strings.ToLower(strings.TrimSpace(instrument))
	if instrumentValue == "" {
		instrumentValue = draft.Instrument
	} else if !keyInstrumentAllowed(instrumentValue) {
		return result, fmt.Errorf("%w: instrument=%q 不在键表词表内（可选 %s）",
			ErrRealtimeStockInvalid, instrument, strings.Join(KeyInstrumentOptions, " / "))
	}

	result.StockID = draft.StockID
	result.Market = draft.Market
	result.Code = draft.Code
	result.FtmmSymbol = draft.FtmmSymbol
	result.Instrument = instrumentValue

	// 已存在则不入队：把现状回给前端，指引去「USC 映射规则」补 USC
	exists, status, usc, priority, err := s.readSecuKeyState(ctx, draft.FtmmSymbol)
	if err != nil {
		return result, fmt.Errorf("EnqueuePendingSecu：查询键表失败: %w", err)
	}
	if exists {
		result.Existed = true
		result.Status = status
		result.USC = usc
		result.Priority = priority
		result.Hint = existingKeyHit(draft.FtmmSymbol, status, usc)
		return result, nil
	}

	// 不存在 → 插入（键表默认值正好适配入队语义：
	//    status='PENDING' 待处理、flag_enable=1 启用、priority=10 最低优先级；usc 留空等规则引擎解析）
	var inserted string
	err = s.pool.QueryRow(ctx, `
INSERT INTO `+tableSecuKey+` AS k
       (instrument, market, code, ftmm_symbol, stock_id, status, flag_enable, priority)
VALUES ($1, $2, $3, $4, $5, 'PENDING', 1, 10)
ON CONFLICT (ftmm_symbol) DO NOTHING
RETURNING k.ftmm_symbol`,
		instrumentValue, draft.Market, draft.Code, draft.FtmmSymbol, draft.StockID,
	).Scan(&inserted)
	switch {
	case err == nil:
		result.Enqueued = true
		result.Status = StatusPendingKey
		result.Hint = fmt.Sprintf("已入队 %s（instrument=%s）；到「USC 映射规则」跑一次计算后，"+
			"本页可用「手动补录USC」完成同步", result.FtmmSymbol, instrumentValue)
		return result, nil
	case errors.Is(err, pgx.ErrNoRows):
		// 并发下被别人抢先插入：按「已存在」处理，不报错
		exists2, status2, usc2, priority2, readErr := s.readSecuKeyState(ctx, draft.FtmmSymbol)
		if readErr != nil {
			return result, fmt.Errorf("EnqueuePendingSecu：复核键表失败: %w", readErr)
		}
		result.Existed = exists2
		result.Status = status2
		result.USC = usc2
		result.Priority = priority2
		result.Hint = existingKeyHit(draft.FtmmSymbol, status2, usc2)
		return result, nil
	default:
		return result, fmt.Errorf("EnqueuePendingSecu：写入键表失败: %w",
			explainUniqueViolation(err, tableSecuKey))
	}
}

// existingKeyHit 组装「键表已存在」时的提示文案。
func existingKeyHit(ftmmSymbol, status, usc string) string {
	parts := []string{"键表已存在该记录（" + ftmmSymbol + "，status=" + orPlaceholder(status)}
	if usc != "" {
		parts = append(parts, "usc="+usc)
	} else {
		parts = append(parts, "usc 仍为空")
	}
	return strings.Join(parts, "，") + "）；请到「USC 映射规则」菜单补全对应的 USC，之后本页可用「手动补录USC」完成同步"
}

func orPlaceholder(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
