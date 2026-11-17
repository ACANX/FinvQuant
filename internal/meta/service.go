// Package meta 提供元数据管理服务（finv_exchange / finv_market / finv_security 字典维护）。
package meta

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FlagEnable 启用标志常量（与 V17/V18 迁移约定一致：'0'=禁用，'1'=启用）。
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
	ExchangeCode      int     `json:"exchange_code"`
	ExchangeFlag      string  `json:"exchange_flag"`
	ExchangeAbbr      string  `json:"exchange_abbr"`
	ExchangeName      string  `json:"exchange_name"`
	ExchangeAbbrCN    string  `json:"exchange_abbr_cn"`
	EnMarketType      string  `json:"en_market_type"`
	Region            string  `json:"region"`
	BaseCurrency      string  `json:"base_currency"`
	FtListExchangeCode *string `json:"ft_list_exchange_code"`
	FlagEnable        string  `json:"flag_enable"`
}

// Market finv_market 交易市场行。
type Market struct {
	MarketCode      int    `json:"market_code"`
	MarketFlag      string `json:"market_flag"`
	MarketAbbr      string `json:"market_abbr"`
	MarketName      string `json:"market_name"`
	EnSecurityType  string `json:"en_security_type"`
	ExchangeCode    int    `json:"exchange_code"` // 所属交易所代码（对齐 finv_exchange.exchange_code，缺省 0）
	BaseCurrency    string `json:"base_currency"`
	FlagEnable      string `json:"flag_enable"`
}

// Security finv_security 证券代码行。
type Security struct {
	USC             string  `json:"usc"`
	ExchangeCode    int     `json:"exchange_code"`
	MarketCode      int     `json:"market_code"` // 交易市场代码（对齐 finv_market.market_code，缺省 0）
	SecurityType    string  `json:"security_type"`
	SecurityCode    string  `json:"security_code"`
	SecurityName    string  `json:"security_name"`
	SecurityNameCN  string  `json:"security_name_cn"`
	SecurityNameFull *string `json:"security_name_full"`
	CurrencyType    string  `json:"currency_type"`
	InitDate        int     `json:"init_date"`
	Timezone        *string `json:"timezone"`
	Tz              *string `json:"tz"`
	FlagEnable      string  `json:"flag_enable"`
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
			OR market_abbr ILIKE $%d
			OR market_name ILIKE $%d
			OR en_security_type ILIKE $%d
		)`, len(args), len(args), len(args), len(args), len(args)))
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
SELECT market_code, market_flag, market_abbr, market_name, en_security_type, exchange_code, base_currency, flag_enable
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
		if err := rows.Scan(&m.MarketCode, &m.MarketFlag, &m.MarketAbbr, &m.MarketName,
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
    market_flag = $2, market_abbr = $3, market_name = $4,
    en_security_type = $5, exchange_code = $6, base_currency = $7
WHERE market_code = $1`,
			m.MarketCode, m.MarketFlag, m.MarketAbbr, m.MarketName,
			m.EnSecurityType, m.ExchangeCode, m.BaseCurrency)
		return m.MarketCode, err
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO finv_market
    (market_code, market_flag, market_abbr, market_name, en_security_type, exchange_code, base_currency, flag_enable)
VALUES ($1, $2, $3, $4, $5, $6, $7, '1')`,
		m.MarketCode, m.MarketFlag, m.MarketAbbr, m.MarketName,
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

// ListSecurities 分页查询证券代码；keyword 匹配 usc/证券代码/证券名称（任一）。
// 排序：启用的（flag_enable='1'）优先展示，禁用的排后面，同状态按 usc 升序。
func (s *Service) ListSecurities(ctx context.Context, pager Pager, keyword, flagEnable string) ([]Security, int, error) {
	pager.Normalize()
	where := []string{"1=1"}
	args := []any{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = append(where, fmt.Sprintf(`(
			usc ILIKE $%d
			OR security_code ILIKE $%d
			OR security_name ILIKE $%d
			OR security_name_cn ILIKE $%d
		)`, len(args), len(args), len(args), len(args)))
	}
	if flagEnable == FlagEnable || flagEnable == FlagDisable {
		args = append(args, flagEnable)
		where = append(where, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM finv_security WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT usc, exchange_code, market_code, security_type, security_code, security_name, security_name_cn,
       security_name_full, currency_type, init_date, timezone, tz, flag_enable
FROM finv_security
WHERE `+cond+`
ORDER BY flag_enable DESC, usc ASC
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
			&sec.InitDate, &sec.Timezone, &sec.Tz, &sec.FlagEnable); err != nil {
			return nil, 0, err
		}
		list = append(list, sec)
	}
	return list, total, rows.Err()
}

// SaveSecurity 新增或更新证券代码（usc 存在则更新，否则新增）。
func (s *Service) SaveSecurity(ctx context.Context, sec Security) (string, error) {
	if sec.USC == "" {
		return "", fmt.Errorf("usc 不能为空")
	}
	if sec.ExchangeCode <= 0 {
		return "", fmt.Errorf("exchange_code 必须为正整数")
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

