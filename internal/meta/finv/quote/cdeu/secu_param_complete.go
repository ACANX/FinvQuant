package cdeu

// 证券参数补全（需求 11161）。
//
// 背景：原来「待录入证券」页的「录入」按钮一次写两张表（public.finv_security +
// futu.finv_futu_quote_collect），实际使用中两张表的缺口并不同步 —— 采集配置可能已经有行、
// 而规范证券参数还缺；也可能反过来。一把梭会把已存在的那张表连带覆盖成「已存在（未覆盖）」，
// 看不出到底补了哪张。
//
// 需求 11161 把这件事拆成独立菜单「证券参数补全」，一张表一个 Tab：
//   - security → `public.finv_security`（规范证券参数）
//   - collect  → `futu.finv_futu_quote_collect`（采集参数）
// 三者（清单 / 预填 / 写入）都用同一套口径：
//
//	目标键 usc = COALESCE(NULLIF(btrim(快照.usc), ''), upper(市场标签 || '.' || 代码))
//
// 清单只列「目标表里没有该键」的行；写入只写目标表那一张，按主键
// `ON CONFLICT (usc) DO NOTHING`（幂等，不覆盖人工数据）。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// secuParamTargetKeyExpr 补全用的目标 usc：**快照 usc 优先，为空才退回派生键**。
//
// 为什么优先快照 usc：`usc` 是「统一证券代码」，不是「市场.代码」的机械拼接 ——
// 库里既有 `00700`（裸代码）、`AAVE.FTASB`（加密货币的交易所变体）、`SHCI`（指数）
// 这类取值，仅靠 `market_label || '.' || stock_code` 推不出来。快照 usc 与
// `public.finv_security` / `futu.finv_futu_quote_collect` 的存量口径一致（裸代码为主），
// 用它当键才不会造出与存量互不相认的行。派生键只在快照 usc 为空时兜底。
//
// 该表达式必须与清单筛选、预填、写入三处完全一致，否则会出现「补完还留在清单里」。
const secuParamTargetKeyExpr = `COALESCE(NULLIF(upper(btrim(t.usc)), ''), ` + realtimeStockFtmmExpr + `)`

// secuParamExistsCondFmt 判断目标表是否已有该证券的条件（表别名固定为 x）。
//
// 同时匹配「目标键」与「派生键」两种形态：快照 usc 是裸代码、主数据表里已有
// `US.STX` 这种点号形态时，不能当成缺失（否则一补就撞 (exchange_code, security_code)
// 或 (market_code, futu_symbol) 唯一约束）。
const secuParamExistsCondFmt = `(x.usc = %s OR x.usc = ` + realtimeStockFtmmExpr + `)`

// 补全目标表（取值即接口的 `target` 参数）。
const (
	// SecuParamTargetSecurity 规范证券参数表 public.finv_security。
	SecuParamTargetSecurity = "security"
	// SecuParamTargetCollect 采集参数表 futu.finv_futu_quote_collect。
	SecuParamTargetCollect = "collect"
)

// secuParamCompleteTable 把 target 映射成表名；非法取值返回错误（白名单，防注入）。
func secuParamCompleteTable(target string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(target)) {
	case SecuParamTargetSecurity:
		return "public.finv_security", nil
	case SecuParamTargetCollect:
		return "futu.finv_futu_quote_collect", nil
	default:
		return "", fmt.Errorf("%w: target 只能是 security 或 collect（收到 %q）",
			ErrRealtimeStockInvalid, target)
	}
}

// SecuParamCompleteRequest 单表补全请求体。
//
// `usc` / `stock_id` / `ftmm_symbol` 一律服务端派生，不采信请求体；
// `security` / `collect` 各传一份（前端只填目标表那份，另一份留空）。
type SecuParamCompleteRequest struct {
	// Target 补全哪张表：security 或 collect。
	Target string `json:"target"`
	// StockID 快照表主键，用来定位证券与派生 usc。
	StockID int64 `json:"stock_id"`
	// Remark 人工备注（服务端会加 [AUTO-FILL] 标记与时间）。
	Remark   string                    `json:"remark"`
	Security PendingSecuEnrollSecurity `json:"security"`
	Collect  PendingSecuEnrollCollect  `json:"collect"`
}

// SecuParamCompleteResult 单表补全结果。
type SecuParamCompleteResult struct {
	// Target 本次补全的表（回落为规范化后的取值）。
	Target string `json:"target"`
	// USC 写入用的主键（服务端派生）。
	USC string `json:"usc"`
	// Created true = 新插入；false = 该 usc 已存在，未覆盖。
	Created bool `json:"created"`
	// Remark 实际写入的备注（含 [AUTO-FILL] 标记）。
	Remark string `json:"remark"`
}

// SecuParamPrefill 单表补全的预填（需求 11161）。
//
// `Fields` 只含目标表那一组字段——两张表混在一起展示会让人不知道点「补全」会写哪张。
type SecuParamPrefill struct {
	Target   string                    `json:"target"`
	Table    string                    `json:"table"`
	StockID  int64                     `json:"stock_id"`
	USC      string                    `json:"usc"`
	Name     string                    `json:"name"`
	Exists   bool                      `json:"exists"`
	Fields   []PendingSecuPrefillField `json:"fields"`
	Warnings []string                  `json:"warnings"`
}

// SecuParamPrefill 按目标表返回预填字段（复用 PendingSecuPrefill 的全部推导规则）。
//
// 不入库、只读；`Exists` 表示目标表里已经有该 usc 的行（此时前端应提示「已存在」而不给提交）。
func (s *Store) SecuParamPrefill(ctx context.Context, target string, stockID int64) (SecuParamPrefill, error) {
	table, err := secuParamCompleteTable(target)
	if err != nil {
		return SecuParamPrefill{}, err
	}
	full, err := s.PendingSecuPrefill(ctx, stockID)
	if err != nil {
		return SecuParamPrefill{}, err
	}

	// 目标键必须与清单筛选、写入同口径（见 secuParamTargetKeyExpr 的说明）
	targetUSC, err := s.secuParamTargetUSC(ctx, stockID)
	if err != nil {
		return SecuParamPrefill{}, err
	}
	if targetUSC == "" || targetUSC == "." {
		return SecuParamPrefill{}, fmt.Errorf("%w: stock_id=%d 的快照缺 市场标签/代码，无法确定 usc",
			ErrRealtimeStockInvalid, stockID)
	}

	result := SecuParamPrefill{
		Target:   strings.ToLower(strings.TrimSpace(target)),
		Table:    table,
		StockID:  full.StockID,
		USC:      targetUSC,
		Name:     full.Name,
		Warnings: full.Warnings,
	}
	if result.Target == SecuParamTargetCollect {
		result.Fields = full.Collect
	} else {
		result.Fields = full.Security
	}
	// 字段里的 usc 也统一成目标键，别让弹窗显示一个值、写库写另一个值
	for i := range result.Fields {
		if result.Fields[i].Key == "usc" {
			result.Fields[i].Value = targetUSC
		}
	}

	// 目标表里是否已有该键：有则不该再「补全」（写入是 DO NOTHING，点了也是白点）
	exists, err := s.secuParamRowExists(ctx, table, targetUSC)
	if err != nil {
		return result, err
	}
	result.Exists = exists
	return result, nil
}

// secuParamTargetUSC 按快照行算出补全用的目标 usc（与清单筛选、写入同一表达式）。
func (s *Store) secuParamTargetUSC(ctx context.Context, stockID int64) (string, error) {
	var usc string
	stmt := "SELECT " + secuParamTargetKeyExpr + " FROM " + tableRealtimeStock + " t WHERE t.stock_id = $1"
	if err := s.pool.QueryRow(ctx, stmt, stockID).Scan(&usc); err != nil {
		return "", fmt.Errorf("SecuParamPrefill：读取快照 usc 失败: %w", err)
	}
	return strings.TrimSpace(usc), nil
}

// secuParamRowExists 目标表里是否已有该 usc 的行。
func (s *Store) secuParamRowExists(ctx context.Context, table, usc string) (bool, error) {
	if strings.TrimSpace(usc) == "" {
		return false, nil
	}
	var exists bool
	// 表名来自白名单（secuParamCompleteTable），不拼接外部输入
	stmt := "SELECT EXISTS (SELECT 1 FROM " + table + " WHERE usc = $1)"
	if err := s.pool.QueryRow(ctx, stmt, usc).Scan(&exists); err != nil {
		return false, fmt.Errorf("SecuParamPrefill：检查目标表失败: %w", err)
	}
	return exists, nil
}

// CompleteSecuParam 把一只证券写入**指定的一张**参数表（需求 11161）。
//
// 单事务、只写目标表：
//   - `public.finv_security` 或 `futu.finv_futu_quote_collect`，按主键 `ON CONFLICT DO NOTHING`；
//   - 已存在时返回 `created=false`（不覆盖人工数据，也不报错）；
//   - **不写**另一张表，**不回写**快照 usc（那是「待采证券录入」页「手动补录USC」/「入队」的职责）。
//
// 必填校验：目标表那一组字段必填缺失 → 400；非目标表的那一组完全不看。
func (s *Store) CompleteSecuParam(ctx context.Context, req SecuParamCompleteRequest) (SecuParamCompleteResult, error) {
	var result SecuParamCompleteResult
	table, err := secuParamCompleteTable(req.Target)
	if err != nil {
		return result, err
	}
	target := strings.ToLower(strings.TrimSpace(req.Target))
	result.Target = target
	if req.StockID <= 0 {
		return result, fmt.Errorf("%w: stock_id 必须为正整数", ErrRealtimeStockInvalid)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("CompleteSecuParam：开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 快照行决定 usc / 代码 / 市场标签：不采信请求体，避免写进奇怪的主键
	var (
		stockID                     int64
		usc, stockCode, marketLabel string
	)
	if err := tx.QueryRow(ctx, `
SELECT t.stock_id,
       `+secuParamTargetKeyExpr+`,
       COALESCE(btrim(t.stock_code), ''),
       COALESCE(upper(btrim(t.market_label)), '')
  FROM `+tableRealtimeStock+` t
 WHERE t.stock_id = $1`, req.StockID).Scan(&stockID, &usc, &stockCode, &marketLabel); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, fmt.Errorf("%w: stock_id=%d 不存在", ErrRealtimeStockInvalid, req.StockID)
		}
		return result, fmt.Errorf("CompleteSecuParam：读取快照失败: %w", err)
	}
	if usc == "." || usc == "" {
		return result, fmt.Errorf("%w: 快照的 市场标签/代码 为空，无法派生 usc", ErrRealtimeStockInvalid)
	}
	result.USC = usc
	result.Remark = buildAutoRemark(req.Remark, time.Now())

	switch target {
	case SecuParamTargetSecurity:
		sec := req.Security
		if err := normalizePendingSecuSecurity(&sec, stockCode); err != nil {
			return result, err
		}
		created, err := insertSecuSecurity(ctx, tx, usc, sec, result.Remark)
		if err != nil {
			return result, fmt.Errorf("CompleteSecuParam：%w", err)
		}
		result.Created = created
	case SecuParamTargetCollect:
		col := req.Collect
		if err := normalizePendingSecuCollect(&col, stockCode, marketLabel); err != nil {
			return result, err
		}
		created, err := insertSecuCollect(ctx, tx, usc, stockID, col, result.Remark)
		if err != nil {
			return result, fmt.Errorf("CompleteSecuParam：%w", err)
		}
		result.Created = created
	default:
		// secuParamCompleteTable 已挡住，这里只是兜底
		return result, fmt.Errorf("%w: target=%q 无法补全", ErrRealtimeStockInvalid, table)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("CompleteSecuParam：提交事务失败: %w", err)
	}
	return result, nil
}
