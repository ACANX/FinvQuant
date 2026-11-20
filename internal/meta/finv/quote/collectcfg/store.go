package collectcfg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
	"github.com/acanx/finvquant/internal/meta/finv/quote/uscrule"
	syscfg "github.com/acanx/finvquant/internal/meta/finv/sys"
)

// InputError 取值或编号不合法（handler 映射为 HTTP 400）。
//
// 与「数据库不可用」区分：前者是用户填错，后者是服务端故障（500）。
type InputError struct {
	msg string
}

// Error 实现 error 接口。
func (e *InputError) Error() string { return e.msg }

// inputErrorf 构造 InputError。
func inputErrorf(format string, a ...any) error {
	return &InputError{msg: fmt.Sprintf(format, a...)}
}

// 敏感项掩码里 key 段的占位符。
const maskedSegment = "****"

// 20016 私钥的「保留原值 / 清空」语义（与 cdeu.SaveMoomooConfig 一致）。
const (
	privKeyKeep  = ""
	privKeyClear = "__CLEAR__"
)

// Store 「行情采集系统配置」的数据访问（只查 + 改）。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建数据访问对象。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// List 读取目录内全部配置的当前值（敏感项回掩码）。
//
// 目录是静态的：库里缺行时该项 Value 为空、HasValue=false，不报错——页面据此提示
// 「配置项不存在，请执行对应种子迁移」而不是整页失败。
func (s *Store) List(ctx context.Context) ([]View, error) {
	rows, err := s.pool.Query(ctx, `
SELECT config_no, COALESCE(str_value, ''), gmt_modify
  FROM sys.sys_config
 WHERE config_no = ANY($1::int[])`, configNos())
	if err != nil {
		return nil, fmt.Errorf("读取行情采集系统配置失败: %w", err)
	}
	defer rows.Close()

	type row struct {
		value     string
		gmtModify *time.Time
	}
	loaded := make(map[int]row, len(catalog))
	for rows.Next() {
		var no int
		var item row
		if err := rows.Scan(&no, &item.value, &item.gmtModify); err != nil {
			return nil, fmt.Errorf("解析行情采集系统配置失败: %w", err)
		}
		loaded[no] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取行情采集系统配置失败: %w", err)
	}

	out := make([]View, 0, len(catalog))
	for _, item := range catalog {
		current := loaded[item.ConfigNo]
		view := View{
			Item:      item,
			Value:     current.value,
			HasValue:  strings.TrimSpace(current.value) != "",
			GmtModify: current.gmtModify,
		}
		if item.Sensitive {
			view.Masked = true
			view.Value = Mask(item.ConfigNo, current.value)
		}
		out = append(out, view)
	}
	return out, nil
}

// SecretValue 取敏感项的原始值（供 handler 转 base64 返回给已登录用户）。
//
// 只允许目录内标记为敏感的项；非敏感项直接读列表即可，走本方法视为调用错误。
func (s *Store) SecretValue(ctx context.Context, configNo int) (string, error) {
	item, ok := ItemByNo(configNo)
	if !ok {
		return "", inputErrorf("配置项 %d 不在「行情采集系统配置」范围内", configNo)
	}
	if !item.Sensitive {
		return "", inputErrorf("配置项 %d 不是敏感项，明文请从列表接口读取", configNo)
	}
	var raw string
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(str_value, '')
  FROM sys.sys_config
 WHERE config_no = $1`, configNo).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", inputErrorf("系统配置 %d 不存在，请先执行对应种子迁移", configNo)
	}
	if err != nil {
		return "", fmt.Errorf("读取系统配置 %d 失败: %w", configNo, err)
	}
	return raw, nil
}

// Save 校验并写入单个配置项的 str_value（只更新已存在的行）。
//
// 各配置项的口径复用既有模块的解析函数，避免两处校验漂移：
//   - 20013 复用 syscfg.ParseSupabaseConn（空串 = 未配置）
//   - 20014 复用 syscfg.NormalizeSupabaseTableName（空串 = 回退默认表名）
//   - 20015 复用 uscrule.ParseSchedule（空串 = 关闭自动同步）
//   - 20016 复用 webapi.ParseEd25519PrivateKey 校验私钥；空私钥 = 保留原值，__CLEAR__ = 只清私钥
//   - 20018 复用 cdeu.ValidateMoomooQuoteProxy（空串 = 直连）
func (s *Store) Save(ctx context.Context, configNo int, value string) (string, error) {
	if _, ok := ItemByNo(configNo); !ok {
		return "", inputErrorf("配置项 %d 不在「行情采集系统配置」范围内（本页只改 20013–20018 中的采集相关项）", configNo)
	}

	normalized := strings.TrimSpace(value)
	if err := validateValue(configNo, normalized); err != nil {
		return "", err
	}
	switch configNo {
	case ConfigNoSupabaseSecuKeyTable:
		table, err := syscfg.NormalizeSupabaseTableName(normalized)
		if err != nil {
			return "", err
		}
		normalized = table
	case ConfigNoMoomooCredential:
		merged, err := s.mergeMoomooCredential(ctx, normalized)
		if err != nil {
			return "", err
		}
		normalized = merged
	}

	tag, err := s.pool.Exec(ctx, `
UPDATE sys.sys_config
   SET str_value = $2, gmt_modify = CURRENT_TIMESTAMP(6), modify_by = 'finv'
 WHERE config_no = $1`, configNo, normalized)
	if err != nil {
		return "", fmt.Errorf("保存系统配置 %d 失败: %w", configNo, err)
	}
	if tag.RowsAffected() == 0 {
		return "", inputErrorf("系统配置 %d 不存在，请先执行对应种子迁移", configNo)
	}
	return normalized, nil
}

// validateValue 纯校验（不查库），口径复用各模块既有解析函数，避免两处漂移。
//
// 空串一律合法（各配置项的空串语义见 catalog 的 EmptyHint）。
func validateValue(configNo int, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch configNo {
	case ConfigNoSupabaseConn:
		if _, err := syscfg.ParseSupabaseConn(value); err != nil {
			return inputErrorf("%v", err)
		}
	case ConfigNoSupabaseSecuKeyTable:
		if _, err := syscfg.NormalizeSupabaseTableName(value); err != nil {
			return inputErrorf("%v", err)
		}
	case ConfigNoUscRuleCron:
		if uscrule.ParseSchedule(value) == nil {
			return inputErrorf("cron 表达式非法（需 6 段：秒 分 时 日 月 周）: %q", value)
		}
	case ConfigNoMoomooCredential:
		appKey, keyPart, found := strings.Cut(value, "|")
		if strings.TrimSpace(appKey) == "" {
			return inputErrorf("AppKey 不能为空（格式 {AppKey}|{私钥 PEM}；留空私钥 = 保留原值）")
		}
		if !found || strings.TrimSpace(keyPart) == privKeyKeep || strings.TrimSpace(keyPart) == privKeyClear {
			return nil
		}
		if _, err := webapi.ParseEd25519PrivateKey(keyPart); err != nil {
			return inputErrorf("%v", err)
		}
	case ConfigNoMoomooQuoteProxy:
		if err := cdeu.ValidateMoomooQuoteProxy(value); err != nil {
			return inputErrorf("%v", err)
		}
	default:
		return inputErrorf("配置项 %d 不在「行情采集系统配置」范围内", configNo)
	}
	return nil
}

// mergeMoomooCredential 处理 20016 的私钥保留/清空语义，返回真正要落库的 `{AppKey}|{PEM}`。
func (s *Store) mergeMoomooCredential(ctx context.Context, raw string) (string, error) {
	appKey, keyPart, found := strings.Cut(raw, "|")
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return "", errors.New("AppKey 不能为空（格式 {AppKey}|{私钥 PEM}；留空私钥 = 保留原值）")
	}
	if !found {
		keyPart = privKeyKeep
	}

	current, err := s.currentMoomooCredential(ctx)
	if err != nil {
		return "", err
	}
	pem := current
	switch strings.TrimSpace(keyPart) {
	case privKeyKeep:
		// 留空 = 不修改私钥
	case privKeyClear:
		pem = ""
	default:
		if _, err := webapi.ParseEd25519PrivateKey(keyPart); err != nil {
			return "", err
		}
		pem = strings.TrimSpace(keyPart)
	}
	return appKey + "|" + pem, nil
}

// currentMoomooCredential 读 20016 现有私钥段（缺行/无竖线时为空）。
func (s *Store) currentMoomooCredential(ctx context.Context) (string, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(str_value, '')
  FROM sys.sys_config
 WHERE config_no = $1`, ConfigNoMoomooCredential).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 moomoo 凭据失败: %w", err)
	}
	_, keyPart, found := strings.Cut(strings.TrimSpace(raw), "|")
	if !found {
		return "", nil
	}
	return keyPart, nil
}

// Mask 敏感项掩码：
//
//	20013 `{PROJECT_ID}|****` —— 项目 ID 不是秘密，保留便于确认连的是哪个项目；
//	20016 `****|****`         —— AppKey 与私钥都是凭据，一律不露字符。
//
// 未配置时返回空串（页面据 HasValue 判断）。
func Mask(configNo int, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	switch configNo {
	case ConfigNoSupabaseConn:
		projectID, _, found := strings.Cut(strings.TrimSpace(raw), "|")
		if !found || strings.TrimSpace(projectID) == "" {
			return maskedSegment
		}
		return strings.TrimSpace(projectID) + "|" + maskedSegment
	case ConfigNoMoomooCredential:
		return maskedSegment + "|" + maskedSegment
	default:
		return maskedSegment
	}
}

// configNos 目录内全部编号。
func configNos() []int {
	out := make([]int, 0, len(catalog))
	for _, item := range catalog {
		out = append(out, item.ConfigNo)
	}
	return out
}
