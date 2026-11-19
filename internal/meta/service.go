// Package meta 提供元数据管理服务（finv_exchange / finv_market / finv_security 字典维护）。
package meta

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagEnable 启用标志常量（与 V200017/V200018 迁移约定一致：'0'=禁用，'1'=启用）。
const (
	FlagDisable = "0"
	FlagEnable  = "1"
)

// Pager 分页参数。
type Pager struct {
	Page     int
	PageSize int
}

// Normalize 归一化分页参数（page>=1，pageSize 1~500）。
func (p *Pager) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 500 {
		p.PageSize = 500
	}
}

// Exchange finv_exchange 交易所/市场字典行。
type Exchange struct {
	ExchangeCode       int     `json:"exchange_code"`
	ExchangeFlag       string  `json:"exchange_flag"`
	ExchangeAbbr       string  `json:"exchange_abbr"`
	ExchangeName       string  `json:"exchange_name"`
	ExchangeAbbrCN     string  `json:"exchange_abbr_cn"`
	EnMarketType       string  `json:"en_market_type"`
	Region             string  `json:"region"`
	BaseCurrency       string  `json:"base_currency"`
	FtListExchangeCode *string `json:"ft_list_exchange_code"`
	FlagEnable         string  `json:"flag_enable"`
}

// Market finv_market 交易市场行。
type Market struct {
	MarketCode     int    `json:"market_code"`
	MarketFlag     string `json:"market_flag"`
	MarketSymbol   string `json:"market_symbol"` // 市场符号（如 US-NSDQ / FX-CFD / SZ，空串表示未维护）
	MarketAbbr     string `json:"market_abbr"`
	MarketName     string `json:"market_name"`
	EnSecurityType string `json:"en_security_type"`
	ExchangeCode   int    `json:"exchange_code"` // 所属交易所代码（对齐 finv_exchange.exchange_code，缺省 0）
	BaseCurrency   string `json:"base_currency"`
	FlagEnable     string `json:"flag_enable"`
}

// Security finv_security 证券代码行。
type Security struct {
	USC              string  `json:"usc"`
	ExchangeCode     int     `json:"exchange_code"`
	MarketCode       int     `json:"market_code"` // 交易市场代码（对齐 finv_market.market_code，缺省 0）
	SecurityType     string  `json:"security_type"`
	SecurityCode     string  `json:"security_code"`
	SecurityName     string  `json:"security_name"`
	SecurityNameCN   string  `json:"security_name_cn"`
	SecurityNameFull *string `json:"security_name_full"`
	CurrencyType     string  `json:"currency_type"`
	InitDate         int     `json:"init_date"`
	Timezone         *string `json:"timezone"`
	Tz               *string `json:"tz"`
	FlagEnable       string  `json:"flag_enable"`
	// USN 统一证券编号（可空；编制规则见 Docs/Design/UsnCodingSchemeDesign.md）。
	USN *int `json:"usn"`
	// TypeSecu 证券分类（type_secu_sub 的父类，需求 11306）。
	TypeSecu string `json:"type_secu"`
	// TypeSecuSub 证券子类（需求 11305/11313）。
	TypeSecuSub string `json:"type_secu_sub"`
	// Remark 备注 / Provider 数据提供方（需求 11328：列配置后可展示）。
	Remark   string `json:"remark"`
	Provider string `json:"provider"`
	// GmtCreate / GmtUpdate 记录创建/更新时间（需求 11328：列配置后可展示）。
	GmtCreate time.Time `json:"gmt_create"`
	GmtUpdate time.Time `json:"gmt_update"`
	// IsFavorite 是否已收藏（quote.finv_quote_secu_fav 中存在该 usc，需求 11312）。
	IsFavorite bool `json:"is_favorite"`
}

// SecurityOption 证券下拉选项（usc 为 key，security_name_cn 为字面展示值）。
type SecurityOption struct {
	USC            string `json:"usc"`
	SecurityNameCN string `json:"security_name_cn"`
}

// Service 元数据管理服务。
type Service struct {
	pool *pgxpool.Pool
}

// NewService 创建元数据管理服务。
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// ---------------------------------------------------------------------
// finv_exchange
// ---------------------------------------------------------------------

// ListExchanges 分页查询交易所字典；keyword 匹配 code/flag/abbr/name（任一）。
// 排序：启用的（flag_enable='1'）优先展示，禁用的排后面，同状态按 exchange_code 升序。
func (s *Service) ListExchanges(ctx context.Context, pager Pager, keyword, flagEnable string) ([]Exchange, int, error) {
	pager.Normalize()
	where := []string{"1=1"}
	args := []any{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			CAST(exchange_code AS TEXT) LIKE $%d
			OR exchange_flag ILIKE $%d
			OR exchange_abbr ILIKE $%d
			OR exchange_name ILIKE $%d
			OR exchange_abbr_cn ILIKE $%d
		)`, len(args), len(args), len(args), len(args), len(args)))
	}
	if flagEnable == FlagEnable || flagEnable == FlagDisable {
		args = append(args, flagEnable)
		where = append(where, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_exchange WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT exchange_code, exchange_flag, exchange_abbr, exchange_name, exchange_abbr_cn,
       en_market_type, region, base_currency, ft_list_exchange_code, flag_enable
FROM finv_exchange
WHERE `+cond+`
ORDER BY flag_enable DESC, exchange_code ASC
LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Exchange{}
	for rows.Next() {
		var e Exchange
		if err := rows.Scan(&e.ExchangeCode, &e.ExchangeFlag, &e.ExchangeAbbr, &e.ExchangeName,
			&e.ExchangeAbbrCN, &e.EnMarketType, &e.Region, &e.BaseCurrency,
			&e.FtListExchangeCode, &e.FlagEnable); err != nil {
			return nil, 0, err
		}
		list = append(list, e)
	}
	return list, total, rows.Err()
}

// SaveExchange 新增或更新交易所字典（exchange_code 存在则更新，否则新增）。
func (s *Service) SaveExchange(ctx context.Context, e Exchange) (int, error) {
	if e.ExchangeCode <= 0 {
		return 0, fmt.Errorf("exchange_code 必须为正整数")
	}
	exists := 0
	_ = s.pool.QueryRow(ctx,
		`SELECT 1 FROM finv_exchange WHERE exchange_code = $1`, e.ExchangeCode).Scan(&exists)
	if exists == 1 {
		_, err := s.pool.Exec(ctx, `
UPDATE finv_exchange SET
    exchange_flag = $2, exchange_abbr = $3, exchange_name = $4, exchange_abbr_cn = $5,
    en_market_type = $6, region = $7, base_currency = $8,
    ft_list_exchange_code = $9
WHERE exchange_code = $1`,
			e.ExchangeCode, e.ExchangeFlag, e.ExchangeAbbr, e.ExchangeName, e.ExchangeAbbrCN,
			e.EnMarketType, e.Region, e.BaseCurrency, e.FtListExchangeCode)
		return e.ExchangeCode, err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_exchange
    (exchange_code, exchange_flag, exchange_abbr, exchange_name, exchange_abbr_cn,
     en_market_type, region, base_currency, ft_list_exchange_code, flag_enable)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, '1')`,
		e.ExchangeCode, e.ExchangeFlag, e.ExchangeAbbr, e.ExchangeName, e.ExchangeAbbrCN,
		e.EnMarketType, e.Region, e.BaseCurrency, e.FtListExchangeCode)
	return e.ExchangeCode, err
}

// ToggleExchange 切换交易所启用状态。
func (s *Service) ToggleExchange(ctx context.Context, code int, flag string) error {
	if code <= 0 {
		return fmt.Errorf("exchange_code 必须为正整数")
	}
	if flag != FlagEnable && flag != FlagDisable {
		return fmt.Errorf("flag_enable 必须为 0 或 1")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE finv_exchange SET flag_enable = $2 WHERE exchange_code = $1`, code, flag)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---------------------------------------------------------------------
// finv_market
// ---------------------------------------------------------------------

// ListMarkets 分页查询交易市场；keyword 匹配 code/flag/abbr/name/证券类型（任一）。
// 排序：启用的（flag_enable='1'）优先展示，禁用的排后面，同状态按 market_code 升序。
func (s *Service) ListMarkets(ctx context.Context, pager Pager, keyword, flagEnable string) ([]Market, int, error) {
	pager.Normalize()
	where := []string{"1=1"}
	args := []any{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			CAST(market_code AS TEXT) LIKE $%d
			OR market_flag ILIKE $%d
			OR market_symbol ILIKE $%d
			OR market_abbr ILIKE $%d
			OR market_name ILIKE $%d
			OR en_security_type ILIKE $%d
		)`, len(args), len(args), len(args), len(args), len(args), len(args)))
	}
	if flagEnable == FlagEnable || flagEnable == FlagDisable {
		args = append(args, flagEnable)
		where = append(where, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_market WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT market_code, market_flag, market_symbol, market_abbr, market_name, en_security_type, exchange_code, base_currency, flag_enable
FROM finv_market
WHERE `+cond+`
ORDER BY flag_enable DESC, market_code ASC
LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Market{}
	for rows.Next() {
		var m Market
		if err := rows.Scan(&m.MarketCode, &m.MarketFlag, &m.MarketSymbol, &m.MarketAbbr, &m.MarketName,
			&m.EnSecurityType, &m.ExchangeCode, &m.BaseCurrency, &m.FlagEnable); err != nil {
			return nil, 0, err
		}
		list = append(list, m)
	}
	return list, total, rows.Err()
}

// SaveMarket 新增或更新交易市场（market_code 存在则更新，否则新增）。
func (s *Service) SaveMarket(ctx context.Context, m Market) (int, error) {
	if m.MarketCode <= 0 {
		return 0, fmt.Errorf("market_code 必须为正整数")
	}
	exists := 0
	_ = s.pool.QueryRow(ctx,
		`SELECT 1 FROM finv_market WHERE market_code = $1`, m.MarketCode).Scan(&exists)
	if exists == 1 {
		_, err := s.pool.Exec(ctx, `
UPDATE finv_market SET
    market_flag = $2, market_symbol = $3, market_abbr = $4, market_name = $5,
    en_security_type = $6, exchange_code = $7, base_currency = $8
WHERE market_code = $1`,
			m.MarketCode, m.MarketFlag, strings.TrimSpace(m.MarketSymbol), m.MarketAbbr, m.MarketName,
			m.EnSecurityType, m.ExchangeCode, m.BaseCurrency)
		return m.MarketCode, err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_market
    (market_code, market_flag, market_symbol, market_abbr, market_name, en_security_type, exchange_code, base_currency, flag_enable)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '1')`,
		m.MarketCode, m.MarketFlag, strings.TrimSpace(m.MarketSymbol), m.MarketAbbr, m.MarketName,
		m.EnSecurityType, m.ExchangeCode, m.BaseCurrency)
	return m.MarketCode, err
}

// ToggleMarket 切换市场启用状态。
func (s *Service) ToggleMarket(ctx context.Context, code int, flag string) error {
	if code <= 0 {
		return fmt.Errorf("market_code 必须为正整数")
	}
	if flag != FlagEnable && flag != FlagDisable {
		return fmt.Errorf("flag_enable 必须为 0 或 1")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE finv_market SET flag_enable = $2 WHERE market_code = $1`, code, flag)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---------------------------------------------------------------------
// finv_security
// ---------------------------------------------------------------------

// SecurityListFilter 证券字典查询条件（需求 11330：新增 交易所/市场/类型/子类/货币/提供方 筛选）。
type SecurityListFilter struct {
	Keyword      string
	FlagEnable   string
	ExchangeCode int
	MarketCode   int
	SecurityType string
	TypeSecuSub  string
	CurrencyType string
	Provider     string
}

// SecurityFilterOptions 证券字典筛选项（各维度 DISTINCT 值，供下拉使用）。
type SecurityFilterOptions struct {
	Types        []string `json:"types"`
	TypeSecuSubs []string `json:"type_secu_subs"`
	Currencies   []string `json:"currencies"`
	Providers    []string `json:"providers"`
}

// ListSecurities 分页查询证券代码；keyword 匹配 usc/证券代码/证券名称（任一）。
//
// 排序（需求 11330）：交易所 → 市场 → 源代码 升序；同键再按 usc 兜底保证分页稳定。
func (s *Service) ListSecurities(ctx context.Context, pager Pager, filter SecurityListFilter) ([]Security, int, error) {
	pager.Normalize()
	where := []string{"1=1"}
	args := []any{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			usc ILIKE $%d
			OR security_code ILIKE $%d
			OR security_name ILIKE $%d
			OR security_name_cn ILIKE $%d
		)`, len(args), len(args), len(args), len(args)))
	}
	if filter.FlagEnable == FlagEnable || filter.FlagEnable == FlagDisable {
		args = append(args, filter.FlagEnable)
		where = append(where, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	if filter.ExchangeCode > 0 {
		args = append(args, filter.ExchangeCode)
		where = append(where, fmt.Sprintf("exchange_code = $%d", len(args)))
	}
	if filter.MarketCode > 0 {
		args = append(args, filter.MarketCode)
		where = append(where, fmt.Sprintf("market_code = $%d", len(args)))
	}
	// 文本维度：空或 all 表示不限（与其它列表页口径一致）
	for _, item := range []struct {
		value  string
		column string
	}{
		{filter.SecurityType, "security_type"},
		{filter.TypeSecuSub, "type_secu_sub"},
		{filter.CurrencyType, "currency_type"},
		{filter.Provider, "provider"},
	} {
		value := strings.TrimSpace(item.value)
		if value == "" || strings.EqualFold(value, "all") {
			continue
		}
		args = append(args, value)
		where = append(where, fmt.Sprintf("%s = $%d", item.column, len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_security WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT s.usc, s.exchange_code, s.market_code, s.security_type, s.security_code, s.security_name,
       s.security_name_cn, s.security_name_full, s.currency_type, s.init_date, s.timezone, s.tz,
       s.flag_enable, s.usn, s.type_secu, s.type_secu_sub,
       s.remark, s.provider, s.gmt_create, s.gmt_update,
       EXISTS (SELECT 1 FROM quote.finv_quote_secu_fav f WHERE f.usc = s.usc) AS is_favorite
FROM finv_security s
WHERE `+cond+`
ORDER BY s.exchange_code ASC, s.market_code ASC, s.security_code ASC, s.usc ASC
LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, pager.PageSize, (pager.Page-1)*pager.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Security{}
	for rows.Next() {
		var sec Security
		if err := rows.Scan(&sec.USC, &sec.ExchangeCode, &sec.MarketCode, &sec.SecurityType, &sec.SecurityCode,
			&sec.SecurityName, &sec.SecurityNameCN, &sec.SecurityNameFull, &sec.CurrencyType,
			&sec.InitDate, &sec.Timezone, &sec.Tz, &sec.FlagEnable,
			&sec.USN, &sec.TypeSecu, &sec.TypeSecuSub,
			&sec.Remark, &sec.Provider, &sec.GmtCreate, &sec.GmtUpdate,
			&sec.IsFavorite); err != nil {
			return nil, 0, err
		}
		list = append(list, sec)
	}
	return list, total, rows.Err()
}

// ListSecurityFilterOptions 取筛选项的 DISTINCT 值（类型 / 子类 / 货币 / 提供方）。
//
// 只查一次（UNION ALL 四段），空串与 NULL 不入选；子类按名称升序。
func (s *Service) ListSecurityFilterOptions(ctx context.Context) (SecurityFilterOptions, error) {
	var opts SecurityFilterOptions
	opts.Types = []string{}
	opts.TypeSecuSubs = []string{}
	opts.Currencies = []string{}
	opts.Providers = []string{}

	rows, err := s.pool.Query(ctx, `
SELECT 'type' AS kind, security_type AS value
  FROM finv_security
 WHERE NULLIF(btrim(security_type), '') IS NOT NULL
 GROUP BY security_type
UNION ALL
SELECT 'sub', type_secu_sub
  FROM finv_security
 WHERE NULLIF(btrim(type_secu_sub), '') IS NOT NULL
 GROUP BY type_secu_sub
UNION ALL
SELECT 'currency', currency_type
  FROM finv_security
 WHERE NULLIF(btrim(currency_type), '') IS NOT NULL
 GROUP BY currency_type
UNION ALL
SELECT 'provider', provider
  FROM finv_security
 WHERE NULLIF(btrim(provider), '') IS NOT NULL
 GROUP BY provider
ORDER BY kind, value`)
	if err != nil {
		return opts, fmt.Errorf("查询证券筛选项失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var kind, value string
		if err := rows.Scan(&kind, &value); err != nil {
			return opts, fmt.Errorf("解析证券筛选项失败: %w", err)
		}
		switch kind {
		case "type":
			opts.Types = append(opts.Types, value)
		case "sub":
			opts.TypeSecuSubs = append(opts.TypeSecuSubs, value)
		case "currency":
			opts.Currencies = append(opts.Currencies, value)
		case "provider":
			opts.Providers = append(opts.Providers, value)
		}
	}
	return opts, rows.Err()
}

// SaveSecurity 新增或更新证券代码（usc 存在则更新，否则新增）。
// 上市日期取值区间，对齐 DB 约束 finv_security_init_date_check
// （V300233 已把下限从 19500101 放宽到 10000101，支持 1950 年前上市的老公司，需求 11348）。
const (
	InitDateMin = 10000101
	InitDateMax = 21001231
)

func (s *Service) SaveSecurity(ctx context.Context, sec Security) (string, error) {
	if sec.USC == "" {
		return "", fmt.Errorf("usc 不能为空")
	}
	if sec.ExchangeCode <= 0 {
		return "", fmt.Errorf("exchange_code 必须为正整数")
	}
	if sec.InitDate < InitDateMin || sec.InitDate > InitDateMax {
		return "", fmt.Errorf("init_date 必须是 %d–%d 的 yyyymmdd 整数（20000000 表示未知）",
			InitDateMin, InitDateMax)
	}
	exists := 0
	_ = s.pool.QueryRow(ctx,
		`SELECT 1 FROM finv_security WHERE usc = $1`, sec.USC).Scan(&exists)
	if exists == 1 {
		_, err := s.pool.Exec(ctx, `
UPDATE finv_security SET
    exchange_code = $2, market_code = $3, security_type = $4, security_code = $5, security_name = $6,
    security_name_cn = $7, security_name_full = $8, currency_type = $9,
    init_date = $10, timezone = $11, tz = $12
WHERE usc = $1`,
			sec.USC, sec.ExchangeCode, sec.MarketCode, sec.SecurityType, sec.SecurityCode, sec.SecurityName,
			sec.SecurityNameCN, sec.SecurityNameFull, sec.CurrencyType,
			sec.InitDate, sec.Timezone, sec.Tz)
		return sec.USC, err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_security
    (usc, exchange_code, market_code, security_type, security_code, security_name, security_name_cn,
     security_name_full, currency_type, init_date, timezone, tz, flag_enable)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, '1')`,
		sec.USC, sec.ExchangeCode, sec.MarketCode, sec.SecurityType, sec.SecurityCode, sec.SecurityName,
		sec.SecurityNameCN, sec.SecurityNameFull, sec.CurrencyType,
		sec.InitDate, sec.Timezone, sec.Tz)
	return sec.USC, err
}

// ToggleSecurity 切换证券启用状态。
func (s *Service) ToggleSecurity(ctx context.Context, usc, flag string) error {
	if usc == "" {
		return fmt.Errorf("usc 不能为空")
	}
	if flag != FlagEnable && flag != FlagDisable {
		return fmt.Errorf("flag_enable 必须为 0 或 1")
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE finv_security SET flag_enable = $2 WHERE usc = $1`, usc, flag)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetSecurityFavorite 收藏 / 取消收藏一只证券（需求 11312）。
//
// 收藏 = 往 quote.finv_quote_secu_fav 插一行（ON CONFLICT DO NOTHING，幂等）；
// 取消 = 删除该行。收藏前校验 usc 存在于 finv_security，避免写入孤儿记录。
func (s *Service) SetSecurityFavorite(ctx context.Context, usc string, favorite bool) error {
	usc = strings.TrimSpace(usc)
	if usc == "" {
		return fmt.Errorf("usc 不能为空")
	}
	if favorite {
		var exists int
		if err := s.pool.QueryRow(ctx,
			`SELECT 1 FROM finv_security WHERE usc = $1`, usc).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("证券 %s 不存在", usc)
			}
			return err
		}
		_, err := s.pool.Exec(ctx, `
INSERT INTO quote.finv_quote_secu_fav (usc) VALUES ($1)
ON CONFLICT (usc) DO NOTHING`, usc)
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM quote.finv_quote_secu_fav WHERE usc = $1`, usc)
	return err
}

// SecurityOptions 返回证券下拉选项列表（usc 为 key，security_name_cn 为展示值，
// 仅返回启用状态的证券；供"历史行情查询"证券代码筛选组件使用）。
func (s *Service) SecurityOptions(ctx context.Context) ([]SecurityOption, error) {
	rows, err := s.pool.Query(ctx, `
SELECT usc, security_name_cn
FROM finv_security
WHERE flag_enable = '1'
ORDER BY usc ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []SecurityOption{}
	for rows.Next() {
		var o SecurityOption
		if err := rows.Scan(&o.USC, &o.SecurityNameCN); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

// LookupSecurity 按统一证券代码（usc）或源证券代码（security_code）查询证券详情，
// 供历史行情导入时「先选证券 → 自动带出信息核对」与「先选文件 → 自动匹配补全」使用。
// 匹配优先级：先精确 usc，再精确 security_code（文件头 Code 可能是两者之一）。
// 未找到时返回 (nil, nil)，由调用方决定提示方式。
func (s *Service) LookupSecurity(ctx context.Context, code string) (*Security, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("证券代码不能为空")
	}
	var sec Security
	err := s.pool.QueryRow(ctx, `
SELECT usc, exchange_code, market_code, security_type, security_code, security_name, security_name_cn,
       security_name_full, currency_type, init_date, timezone, tz, flag_enable
FROM finv_security
WHERE usc = $1 OR security_code = $1
ORDER BY (usc = $1) DESC
LIMIT 1`, code).Scan(
		&sec.USC, &sec.ExchangeCode, &sec.MarketCode, &sec.SecurityType, &sec.SecurityCode,
		&sec.SecurityName, &sec.SecurityNameCN, &sec.SecurityNameFull, &sec.CurrencyType,
		&sec.InitDate, &sec.Timezone, &sec.Tz, &sec.FlagEnable)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sec, nil
}
