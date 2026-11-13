package cdeu

// 采集参数（futu.finv_futu_quote_collect）的查询与单行维护（需求 11254）。
//
// 页面：行情 → 收剔导传ETL → CDEU自动化 → 采集参数（`/Meta/Finv/Quote/Cdeu/CollectParam`）。
// 表语义：一行 = 一个证券的富途采集参数（usc 主键）；`ftmm_symbol` 是生成列
// （market_label || '.' || futu_symbol），**不可写**。
//
// 查询一律左关联 `public.finv_security`（`c.usc = s.usc`）取证券元数据，再左关联
// `public.finv_exchange` / `public.finv_market` 取交易所与市场的显示名：
// 采集表本身只有富途侧编码（market_type / market_code / instrument_type …），
// 没有「证券名称 / 分类 / 子类 / 计价货币 / 交易所 / 市场」，这些只能从主数据取。
//
// 维护范围（需求 11254 定稿，默认口径）：
//   - 可改 11 个业务列：stock_id / market_type / market_code / instrument_type /
//     sub_instrument_type / market_label / instrument / futu_symbol / req_section /
//     flag_enable / remark；
//   - 不可改 usc（主键，换 usc 等于换证券，属「入队 / 补录」场景）、
//     ftmm_symbol（生成列，PG 会拒绝显式赋值）、gmt_create / gmt_update（触发器维护）；
//   - 只做「单行编辑更新」，不提供新增行 / 删除行（本表是采集镜像，删行等于停采）。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// 本地表名（显式带 schema，避免 search_path 影响）。
const (
	// tableSecuFtmmCollect 富途采集参数表。
	tableSecuFtmmCollect = "futu.finv_futu_quote_collect"
	// tableSecuSecurity 规范证券主数据表（左关联取证券元数据）。
	tableSecuSecurity = "public.finv_security"
	// tableSecuExchange / tableSecuMarket 交易所、市场字典（取显示名）。
	tableSecuExchange = "public.finv_exchange"
	tableSecuMarket   = "public.finv_market"
)

// ErrSecuFtmmCollectInvalid 采集参数入参非法（Handler 映射 400）。
var ErrSecuFtmmCollectInvalid = errors.New("采集参数入参非法")

// ErrSecuFtmmCollectConflict 唯一键冲突（Handler 映射 409）：
// `(market_code, futu_symbol)` 已被**别的 usc** 占用，不静默覆盖别人的行。
var ErrSecuFtmmCollectConflict = errors.New("采集参数唯一键冲突")

// ErrSecuFtmmCollectNotFound 目标 usc 在采集表里不存在（Handler 映射 404）。
var ErrSecuFtmmCollectNotFound = errors.New("采集参数行不存在")

// SecuFtmmCollectOrderColumns 允许排序的列（表别名 c；列名直接拼进 SQL，必须先校验）。
var SecuFtmmCollectOrderColumns = map[string]bool{
	"usc":          true,
	"ftmm_symbol":  true,
	"futu_symbol":  true,
	"market_label": true,
	"market_type":  true,
	"market_code":  true,
	"instrument":   true,
	"stock_id":     true,
	"flag_enable":  true,
	"gmt_update":   true,
}

// SecuFtmmCollectFilter 列表查询条件（URL 参数由 Handler 解析）。
type SecuFtmmCollectFilter struct {
	// Keyword 模糊匹配 采集表 usc / ftmm_symbol / futu_symbol + 证券名称。
	Keyword string
	// MarketLabel 行情市场标识（market_label，如 HK / SH / US / FX / CC）；空或 all 不限。
	MarketLabel string
	// Instrument 富途证券类型标识（stock / etfs / index / currency / crypto / futures / sectors）；空或 all 不限。
	Instrument string
	// SecurityType 规范证券分类（public.finv_security.security_type，如 Stock / ETF / StockIndex）；空或 all 不限。
	SecurityType string
	// FlagEnable '0' / '1'；空表示不限。
	FlagEnable string
	// USCMissing 为 true 时只查「主数据里没有该证券」的行（s.usc IS NULL）。
	USCMissing bool
	// OrderBy 见 SecuFtmmCollectOrderColumns；空默认 usc。
	OrderBy string
	// Desc true 降序。
	Desc   bool
	Limit  int
	Offset int
}

// SecuFtmmCollectRow 列表行：采集参数字段 + 左关联得到的证券元数据。
type SecuFtmmCollectRow struct {
	// 采集参数（futu.finv_futu_quote_collect）
	USC               string `json:"usc"`
	FtmmSymbol        string `json:"ftmm_symbol"`
	FutuSymbol        string `json:"futu_symbol"`
	MarketLabel       string `json:"market_label"`
	MarketType        int    `json:"market_type"`
	MarketCode        int    `json:"market_code"`
	Instrument        string `json:"instrument"`
	InstrumentType    int    `json:"instrument_type"`
	SubInstrumentType int    `json:"sub_instrument_type"`
	ReqSection        string `json:"req_section"`
	StockID           int64  `json:"stock_id"`
	FlagEnable        string `json:"flag_enable"`
	Remark            string `json:"remark"`
	GmtUpdate         string `json:"gmt_update"`
	// 证券元数据（public.finv_security + 交易所 / 市场字典；未录入证券时为空）
	SecurityName       string `json:"security_name"`
	SecurityType       string `json:"security_type"`
	SecuritySubType    string `json:"security_sub_type"`
	CurrencyType       string `json:"currency_type"`
	ExchangeCode       int    `json:"exchange_code"`
	ExchangeName       string `json:"exchange_name"`
	SecurityMarketCode int    `json:"security_market_code"`
	SecurityMarketName string `json:"security_market_name"`
	SecurityExists     bool   `json:"security_exists"`
}

// SecuFtmmCollectPatch 单行更新入参：**指针为 nil 的字段不参与更新**。
//
// usc 是定位键（WHERE 条件），不是可改字段。
type SecuFtmmCollectPatch struct {
	USC               string
	StockID           *int64
	MarketType        *int
	MarketCode        *int
	InstrumentType    *int
	SubInstrumentType *int
	MarketLabel       *string
	Instrument        *string
	FutuSymbol        *string
	ReqSection        *string
	FlagEnable        *string
	Remark            *string
}

// secuFtmmCollectListColumns 列表输出列：整数列一律直接扫描，时间列取文本（前端按本地时区显示）。
const secuFtmmCollectListColumns = `
       c.usc,
       c.ftmm_symbol,
       c.futu_symbol,
       c.market_label,
       c.market_type,
       c.market_code,
       c.instrument,
       c.instrument_type,
       c.sub_instrument_type,
       COALESCE(c.req_section, '')                                   AS req_section,
       c.stock_id,
       c.flag_enable,
       COALESCE(c.remark, '')                                        AS remark,
       COALESCE(to_char(c.gmt_update AT TIME ZONE 'Asia/Shanghai',
                        'YYYY-MM-DD HH24:MI:SS'), '') AS gmt_update,
       COALESCE(NULLIF(btrim(s.security_name_cn), ''), btrim(s.security_name), '') AS security_name,
       COALESCE(s.security_type, '')                                 AS security_type,
       COALESCE(s.type_secu_sub, '')                                 AS security_sub_type,
       COALESCE(s.currency_type, '')                                 AS currency_type,
       COALESCE(s.exchange_code, 0)                                  AS exchange_code,
       COALESCE(e.exchange_abbr_cn, '')                              AS exchange_name,
       COALESCE(s.market_code, 0)                                    AS security_market_code,
       COALESCE(m.market_name, '')                                   AS security_market_name,
       (s.usc IS NOT NULL)                                           AS security_exists`

// secuFtmmCollectFrom 采集表 + 左关联主数据（证券 → 交易所 → 市场）。
const secuFtmmCollectFrom = `
  FROM ` + tableSecuFtmmCollect + ` c
  LEFT JOIN ` + tableSecuSecurity + ` s ON s.usc = c.usc
  LEFT JOIN ` + tableSecuExchange + ` e ON e.exchange_code = s.exchange_code
  LEFT JOIN ` + tableSecuMarket + ` m ON m.market_code = s.market_code`

// ListSecuFtmmCollect 分页查询采集参数（含左关联的证券元数据），返回总数与当前页。
func (s *Store) ListSecuFtmmCollect(ctx context.Context, f SecuFtmmCollectFilter) (int64, []SecuFtmmCollectRow, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}

	orderBy := strings.TrimSpace(f.OrderBy)
	if orderBy == "" {
		orderBy = "usc"
	}
	if !SecuFtmmCollectOrderColumns[orderBy] {
		return 0, nil, fmt.Errorf("%w: orderBy 非法（%s）", ErrSecuFtmmCollectInvalid, orderBy)
	}
	direction := "ASC"
	if f.Desc {
		direction = "DESC"
	}

	where, args := secuFtmmCollectWhere(f)

	var total int64
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*)"+secuFtmmCollectFrom+where, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("ListSecuFtmmCollect：统计失败: %w", err)
	}
	if total == 0 {
		return 0, nil, nil
	}

	args = append(args, f.Limit, f.Offset)
	query := "SELECT " + secuFtmmCollectListColumns + secuFtmmCollectFrom + where +
		fmt.Sprintf(" ORDER BY c.%s %s NULLS LAST, c.usc ASC LIMIT $%d OFFSET $%d",
			orderBy, direction, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("ListSecuFtmmCollect：查询失败: %w", err)
	}
	defer rows.Close()

	list := make([]SecuFtmmCollectRow, 0, f.Limit)
	for rows.Next() {
		var row SecuFtmmCollectRow
		if err := rows.Scan(
			&row.USC, &row.FtmmSymbol, &row.FutuSymbol, &row.MarketLabel,
			&row.MarketType, &row.MarketCode, &row.Instrument,
			&row.InstrumentType, &row.SubInstrumentType, &row.ReqSection,
			&row.StockID, &row.FlagEnable, &row.Remark, &row.GmtUpdate,
			&row.SecurityName, &row.SecurityType, &row.SecuritySubType, &row.CurrencyType,
			&row.ExchangeCode, &row.ExchangeName,
			&row.SecurityMarketCode, &row.SecurityMarketName, &row.SecurityExists,
		); err != nil {
			return 0, nil, fmt.Errorf("ListSecuFtmmCollect：解析行失败: %w", err)
		}
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("ListSecuFtmmCollect：读取行失败: %w", err)
	}
	return total, list, nil
}

// UpdateSecuFtmmCollect 按 usc 更新一行采集参数，返回是否有行被更新。
//
// 语义：
//   - 只写 patch 里非 nil 的列（可编辑白名单见包注释），完全没给字段 → 入参非法；
//   - usc 不存在 → ErrSecuFtmmCollectNotFound（不静默插入）；
//   - `(market_code, futu_symbol)` 撞到别的 usc → ErrSecuFtmmCollectConflict（不覆盖别人的行）；
//   - `ftmm_symbol` 由 PG 生成列自动跟随 market_label / futu_symbol 变化，不需要也不允许写。
func (s *Store) UpdateSecuFtmmCollect(ctx context.Context, patch SecuFtmmCollectPatch) (bool, error) {
	usc := strings.TrimSpace(patch.USC)
	if usc == "" {
		return false, fmt.Errorf("%w: usc 不能为空", ErrSecuFtmmCollectInvalid)
	}

	sets := make([]string, 0, 11)
	args := make([]any, 0, 12)
	args = append(args, usc)
	add := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if patch.StockID != nil {
		if *patch.StockID <= 0 {
			return false, fmt.Errorf("%w: stock_id 必须是正整数", ErrSecuFtmmCollectInvalid)
		}
		add("stock_id", *patch.StockID)
	}
	if patch.MarketType != nil {
		if *patch.MarketType <= 0 {
			return false, fmt.Errorf("%w: market_type 必须是正整数", ErrSecuFtmmCollectInvalid)
		}
		add("market_type", *patch.MarketType)
	}
	if patch.MarketCode != nil {
		if *patch.MarketCode <= 0 {
			return false, fmt.Errorf("%w: market_code 必须是正整数", ErrSecuFtmmCollectInvalid)
		}
		add("market_code", *patch.MarketCode)
	}
	if patch.InstrumentType != nil {
		if *patch.InstrumentType <= 0 {
			return false, fmt.Errorf("%w: instrument_type 必须是正整数", ErrSecuFtmmCollectInvalid)
		}
		add("instrument_type", *patch.InstrumentType)
	}
	if patch.SubInstrumentType != nil {
		if *patch.SubInstrumentType <= 0 {
			return false, fmt.Errorf("%w: sub_instrument_type 必须是正整数", ErrSecuFtmmCollectInvalid)
		}
		add("sub_instrument_type", *patch.SubInstrumentType)
	}
	if patch.MarketLabel != nil {
		label := strings.ToUpper(strings.TrimSpace(*patch.MarketLabel))
		if label == "" {
			return false, fmt.Errorf("%w: market_label 不能为空", ErrSecuFtmmCollectInvalid)
		}
		add("market_label", label)
	}
	if patch.Instrument != nil {
		instrument := strings.ToLower(strings.TrimSpace(*patch.Instrument))
		if instrument == "" {
			return false, fmt.Errorf("%w: instrument 不能为空", ErrSecuFtmmCollectInvalid)
		}
		add("instrument", instrument)
	}
	if patch.FutuSymbol != nil {
		symbol := strings.TrimSpace(*patch.FutuSymbol)
		if symbol == "" {
			return false, fmt.Errorf("%w: futu_symbol 不能为空", ErrSecuFtmmCollectInvalid)
		}
		add("futu_symbol", symbol)
	}
	if patch.ReqSection != nil {
		add("req_section", strings.TrimSpace(*patch.ReqSection))
	}
	if patch.FlagEnable != nil {
		flag := strings.TrimSpace(*patch.FlagEnable)
		if flag != "0" && flag != "1" {
			return false, fmt.Errorf("%w: flag_enable 只能是 0 或 1", ErrSecuFtmmCollectInvalid)
		}
		add("flag_enable", flag)
	}
	if patch.Remark != nil {
		add("remark", strings.TrimSpace(*patch.Remark))
	}

	if len(sets) == 0 {
		return false, fmt.Errorf("%w: 没有可更新的字段", ErrSecuFtmmCollectInvalid)
	}

	query := "UPDATE " + tableSecuFtmmCollect + " SET " + strings.Join(sets, ", ") + " WHERE usc = $1"
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, fmt.Errorf("%w: %s", ErrSecuFtmmCollectConflict,
				explainUniqueViolation(err, tableSecuFtmmCollect))
		}
		return false, fmt.Errorf("UpdateSecuFtmmCollect：更新失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, fmt.Errorf("%w: %s", ErrSecuFtmmCollectNotFound, usc)
	}
	return true, nil
}

// secuFtmmCollectWhere 拼装筛选条件（关键字 / 市场 / 品种 / 分类 / 启用 / 主数据缺失）。
func secuFtmmCollectWhere(f SecuFtmmCollectFilter) (string, []any) {
	conds := make([]string, 0, 6)
	args := make([]any, 0, 6)

	if keyword := strings.TrimSpace(f.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		idx := len(args)
		conds = append(conds, fmt.Sprintf(
			"(c.usc ILIKE $%d OR c.ftmm_symbol ILIKE $%d OR c.futu_symbol ILIKE $%d"+
				" OR COALESCE(s.security_name_cn, '') ILIKE $%d OR COALESCE(s.security_name, '') ILIKE $%d)",
			idx, idx, idx, idx, idx))
	}
	if market := normalizeFilterValue(f.MarketLabel); market != "" {
		args = append(args, strings.ToUpper(market))
		conds = append(conds, fmt.Sprintf("upper(c.market_label) = $%d", len(args)))
	}
	if instrument := normalizeFilterValue(f.Instrument); instrument != "" {
		args = append(args, strings.ToLower(instrument))
		conds = append(conds, fmt.Sprintf("lower(c.instrument) = $%d", len(args)))
	}
	if securityType := normalizeFilterValue(f.SecurityType); securityType != "" {
		args = append(args, securityType)
		conds = append(conds, fmt.Sprintf("s.security_type = $%d", len(args)))
	}
	if flag := strings.TrimSpace(f.FlagEnable); flag != "" && !strings.EqualFold(flag, "all") {
		args = append(args, flag)
		conds = append(conds, fmt.Sprintf("c.flag_enable = $%d", len(args)))
	}
	if f.USCMissing {
		conds = append(conds, "s.usc IS NULL")
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// normalizeFilterValue 归一筛选值：空或 all 视为不过滤；市场大写、品种小写。
func normalizeFilterValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "all") {
		return ""
	}
	return trimmed
}
