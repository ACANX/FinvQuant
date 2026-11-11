package openapi

// SQL 访问层（open schema；全限定名，见 DbSchemaSpec §3）。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
)

// Store 基于 pgx 的数据访问。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建数据访问层。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool 暴露连接池（服务层签发密钥时要读 sys_config 19005）。
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// ---- 配置 ----

const selectSettingsSQL = `
SELECT config_no, COALESCE(char_value, ''), COALESCE(str_value, ''), COALESCE(int_value, 0)
  FROM sys.sys_config
 WHERE config_no BETWEEN 19001 AND 19007`

// LoadSettings 读取 19001～19006（只返回值与「是否已配置」，不回显密钥）。
func (s *Store) LoadSettings(ctx context.Context) (SettingsView, error) {
	view := SettingsView{Mode: "0", SignTimeWindow: 300, LogRetainDays: 90, SecretKID: "k1", CredCacheTTL: 10}
	if s.pool == nil {
		return view, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, selectSettingsSQL)
	if err != nil {
		return view, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			configNo int
			charVal  string
			strVal   string
			intVal   int
		)
		if err := rows.Scan(&configNo, &charVal, &strVal, &intVal); err != nil {
			return view, err
		}
		switch configNo {
		case ConfigAuthMode:
			if v := strings.TrimSpace(charVal); v != "" {
				view.Mode = v
			}
		case ConfigTimeWin:
			if intVal > 0 {
				view.SignTimeWindow = intVal
			}
		case ConfigLogRetain:
			if intVal >= 0 {
				view.LogRetainDays = intVal
			}
		case ConfigSecretKID:
			if v := strings.TrimSpace(strVal); v != "" {
				view.SecretKID = v
			}
		case ConfigSecretKey:
			view.SecretKeyReady = strings.TrimSpace(strVal) != ""
		case ConfigFallback:
			view.FallbackKeyReady = strings.TrimSpace(strVal) != ""
		case ConfigCredCacheTTL:
			if intVal >= 0 {
				view.CredCacheTTL = intVal
			}
		}
	}
	return view, rows.Err()
}

// SaveSettings 写入 19001～19003 与 19007（密钥不在此处改）。
func (s *Store) SaveSettings(ctx context.Context, req SettingsRequest) error {
	if s.pool == nil {
		return errors.New("数据库连接池未初始化")
	}
	batch := &pgx.Batch{}
	batch.Queue(`UPDATE sys.sys_config SET char_value = $2 WHERE config_no = $1`, ConfigAuthMode, req.Mode)
	batch.Queue(`UPDATE sys.sys_config SET int_value = $2 WHERE config_no = $1`, ConfigTimeWin, req.SignTimeWindow)
	batch.Queue(`UPDATE sys.sys_config SET int_value = $2 WHERE config_no = $1`, ConfigLogRetain, req.LogRetainDays)
	batch.Queue(`UPDATE sys.sys_config SET int_value = $2 WHERE config_no = $1`, ConfigCredCacheTTL, req.CredCacheTTL)
	results := s.pool.SendBatch(ctx, batch)
	defer results.Close()
	for i := 0; i < 4; i++ {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return results.Close()
}

// SecretKeyBase64 读 19005 原始值（只在签发 AK/SK 时使用，不回显给前端）。
func (s *Store) SecretKeyBase64(ctx context.Context) (string, error) {
	if s.pool == nil {
		return "", errors.New("数据库连接池未初始化")
	}
	var value string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = $1`,
		ConfigSecretKey).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// SecretKID 读 19004（缺省 k1）。
func (s *Store) SecretKID(ctx context.Context) (string, error) {
	if s.pool == nil {
		return "", errors.New("数据库连接池未初始化")
	}
	var value string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(str_value, '') FROM sys.sys_config WHERE config_no = $1`,
		ConfigSecretKID).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "k1", nil
	}
	if strings.TrimSpace(value) == "" {
		value = "k1"
	}
	return value, err
}

// UserEmailByUID 按登录账号取邮箱（新增调用方时默认填充联系邮箱）。
func (s *Store) UserEmailByUID(ctx context.Context, uid string) (string, error) {
	uid = strings.TrimSpace(uid)
	if s.pool == nil || uid == "" {
		return "", nil
	}
	var email string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(email, '') FROM sys.sys_user WHERE uid = $1`, uid).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return email, err
}

// ---- 调用方 ----

const selectClientSQL = `
SELECT c.idn, c.client_code, c.client_name, COALESCE(c.owner_uid, ''), COALESCE(c.contact_email, ''),
       COALESCE(c.contract_no, ''), COALESCE(c.allow_cidr::text[], '{}'),
       c.rate_limit_rpm, c.quota_day,
       COALESCE(to_char(c.dt_expire, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
       c.flag_enable, COALESCE(c.remark, ''),
       to_char(c.gmt_update, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       (SELECT count(*) FROM open.open_api_credential x WHERE x.client_idn = c.idn),
       (SELECT count(*) FROM open.open_api_credential x
         WHERE x.client_idn = c.idn AND x.flag_enable = '1' AND x.flag_revoke = '0')
  FROM open.open_api_client c`

// ClientPage 分页查询调用方（按 code 升序）。
func (s *Store) ClientPage(ctx context.Context, keyword string, page, pageSize int) ([]Client, int, error) {
	if s.pool == nil {
		return nil, 0, errors.New("数据库连接池未初始化")
	}
	where := ""
	args := []any{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		where = ` WHERE c.client_code ILIKE $1 OR c.client_name ILIKE $1`
	}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM open.open_api_client c`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, selectClientSQL+where+
		fmt.Sprintf(` ORDER BY c.client_code LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Client, 0, pageSize)
	for rows.Next() {
		item, err := scanClient(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// ClientByID 读单个调用方；不存在返回 ErrNotFound。
func (s *Store) ClientByID(ctx context.Context, idn int64) (Client, error) {
	if s.pool == nil {
		return Client{}, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, selectClientSQL+` WHERE c.idn = $1`, idn)
	if err != nil {
		return Client{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Client{}, ErrNotFound
	}
	item, err := scanClient(rows)
	if err != nil {
		return Client{}, err
	}
	return item, rows.Err()
}

func scanClient(rows pgx.Rows) (Client, error) {
	item := Client{}
	err := rows.Scan(&item.IDN, &item.ClientCode, &item.ClientName, &item.OwnerUID, &item.ContactEmail,
		&item.ContractNo, &item.AllowCIDR, &item.RateLimitRPM, &item.QuotaDay, &item.DTExpire,
		&item.FlagEnable, &item.Remark, &item.GMTUpdate,
		&item.CountCredential, &item.CountCredActive)
	return item, err
}

// InsertClient 新建调用方（默认停用），返回新行。
func (s *Store) InsertClient(ctx context.Context, req ClientRequest, operator string) (Client, error) {
	if s.pool == nil {
		return Client{}, errors.New("数据库连接池未初始化")
	}
	var idn int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO open.open_api_client
    (client_code, client_name, owner_uid, contact_email, contract_no, allow_cidr,
     rate_limit_rpm, quota_day, dt_expire, flag_enable, remark, create_by, modify_by)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6::cidr[],
        $7, $8, $9::timestamptz, '0', NULLIF($10, ''), NULLIF($11, ''), NULLIF($11, ''))
RETURNING idn`,
		req.ClientCode, req.ClientName, req.OwnerUID, req.ContactEmail, req.ContractNo,
		emptyToArray(req.AllowCIDR), req.RateLimitRPM, req.QuotaDay, nilIfEmpty(req.DTExpire), req.Remark, operator).Scan(&idn)
	if err != nil {
		return Client{}, err
	}
	return s.ClientByID(ctx, idn)
}

// UpdateClient 更新调用方（不含 flag_enable）。
func (s *Store) UpdateClient(ctx context.Context, req ClientRequest, operator string) (Client, error) {
	if s.pool == nil {
		return Client{}, errors.New("数据库连接池未初始化")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE open.open_api_client
   SET client_code = $2, client_name = $3, owner_uid = NULLIF($4, ''), contact_email = NULLIF($5, ''),
       contract_no = NULLIF($6, ''), allow_cidr = $7::cidr[], rate_limit_rpm = $8, quota_day = $9,
       dt_expire = $10::timestamptz, remark = NULLIF($11, ''), modify_by = NULLIF($12, '')
 WHERE idn = $1`,
		req.IDN, req.ClientCode, req.ClientName, req.OwnerUID, req.ContactEmail, req.ContractNo,
		emptyToArray(req.AllowCIDR), req.RateLimitRPM, req.QuotaDay, nilIfEmpty(req.DTExpire), req.Remark, operator)
	if err != nil {
		return Client{}, err
	}
	if tag.RowsAffected() == 0 {
		return Client{}, ErrNotFound
	}
	return s.ClientByID(ctx, req.IDN)
}

// SetClientEnable 启用/停用调用方。
func (s *Store) SetClientEnable(ctx context.Context, idn int64, enable bool, operator string) (Client, error) {
	if s.pool == nil {
		return Client{}, errors.New("数据库连接池未初始化")
	}
	flag := "0"
	if enable {
		flag = "1"
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE open.open_api_client SET flag_enable = $2, modify_by = NULLIF($3, '') WHERE idn = $1`,
		idn, flag, operator)
	if err != nil {
		return Client{}, err
	}
	if tag.RowsAffected() == 0 {
		return Client{}, ErrNotFound
	}
	return s.ClientByID(ctx, idn)
}

// ---- 凭证 ----

// scopeAggregateSQL 把授权范围明细表聚合成与原 `scope_str` **同形状**的 JSON 数组。
//
// 这样 `parseScope` / `InScope` / 缓存快照结构全都不用改（需求 11228：业务代码零改）。
// 关键语义：某凭证 0 行 = 不限接口（`jsonb_agg` 返回 NULL，这里按 '[]' 处理）；
// `method='*'` 原样带出，`InScope` 已支持通配。
const scopeAggregateSQL = `
  LEFT JOIN LATERAL (
        SELECT jsonb_agg(jsonb_build_object('method', r.method, 'path', r.path_pattern)
                         ORDER BY r.idn) AS scope
          FROM open.open_api_credential_scope r
         WHERE r.credential_idn = c.idn
  ) sc ON TRUE`

const selectCredentialSQL = `
SELECT c.idn, c.client_idn, cl.client_code, c.credential_no, c.auth_mode, c.access_key_id,
       COALESCE(c.key_prefix, ''),
       (c.secret_hash IS NOT NULL OR c.secret_cipher IS NOT NULL),
       c.sign_algorithm, COALESCE(sc.scope::text, '[]'), COALESCE(c.allow_cidr::text[], '{}'),
       COALESCE(to_char(c.dt_expire, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
       COALESCE(to_char(c.dt_last_use, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
       c.count_use, c.flag_enable, c.flag_revoke,
       COALESCE(to_char(c.dt_revoke, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
       COALESCE(c.reason_revoke, ''), COALESCE(c.remark, ''),
       to_char(c.gmt_update, 'YYYY-MM-DD"T"HH24:MI:SSOF')
  FROM open.open_api_credential c
  JOIN open.open_api_client cl ON cl.idn = c.client_idn` + scopeAggregateSQL

// CredentialPage 分页查询凭证（可选按调用方、关键字、状态过滤）。
//
// status（需求 11217）：`active`（默认）= 只返回当前可用的（未吊销、启用中、未过期）；
// `all` = 全部，含已吊销/停用/过期。判定口径与鉴权侧的 Credential.Active 保持一致。
func (s *Store) CredentialPage(ctx context.Context, clientIDN int64, keyword, status string,
	page, pageSize int) ([]Credential, int, error) {
	if s.pool == nil {
		return nil, 0, errors.New("数据库连接池未初始化")
	}
	conditions := []string{}
	args := []any{}
	if strings.TrimSpace(status) != CredentialStatusAll {
		conditions = append(conditions,
			`c.flag_revoke = '0' AND c.flag_enable = '1' AND (c.dt_expire IS NULL OR c.dt_expire > now())`)
	}
	if clientIDN > 0 {
		args = append(args, clientIDN)
		conditions = append(conditions, fmt.Sprintf(`c.client_idn = $%d`, len(args)))
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		conditions = append(conditions, fmt.Sprintf(`(c.credential_no ILIKE $%d OR c.access_key_id ILIKE $%d)`, len(args), len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM open.open_api_credential c`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, selectCredentialSQL+where+
		fmt.Sprintf(` ORDER BY c.client_idn, c.credential_no LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Credential, 0, pageSize)
	for rows.Next() {
		item, err := scanCredential(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// CredentialByID 读单个凭证；不存在返回 ErrNotFound。
func (s *Store) CredentialByID(ctx context.Context, idn int64) (Credential, error) {
	if s.pool == nil {
		return Credential{}, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, selectCredentialSQL+` WHERE c.idn = $1`, idn)
	if err != nil {
		return Credential{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Credential{}, ErrNotFound
	}
	item, err := scanCredential(rows)
	if err != nil {
		return Credential{}, err
	}
	return item, rows.Err()
}

func scanCredential(rows pgx.Rows) (Credential, error) {
	item := Credential{}
	var scopeJSON string
	err := rows.Scan(&item.IDN, &item.ClientIDN, &item.ClientCode, &item.CredentialNo, &item.AuthMode,
		&item.AccessKeyID, &item.KeyPrefix, &item.HasSecret, &item.SignAlgorithm, &scopeJSON,
		&item.AllowCIDR, &item.DTExpire, &item.DTLastUse, &item.CountUse, &item.FlagEnable, &item.FlagRevoke,
		&item.DTRevoke, &item.ReasonRevoke, &item.Remark, &item.GMTUpdate)
	if err != nil {
		return item, err
	}
	item.Scope = scopeToText(scopeJSON)
	return item, nil
}

// InsertCredential 插入新凭证（密钥材料由服务层算好后传入）。
//
// 需求 11228：**同一事务内双写**——`scope_str` 保留（兼容/可读快照），
// `open.open_api_credential_scope` 写结构化明细（鉴权读取以它为准）。
// 两者必须原子写入，否则会出现「凭证已建、权限却是全部接口」的窗口。
func (s *Store) InsertCredential(ctx context.Context, req CredentialCreateRequest,
	secretHash, secretCipher, keyPrefix, kid, operator string) (int64, error) {
	if s.pool == nil {
		return 0, errors.New("数据库连接池未初始化")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var idn int64
	if err := tx.QueryRow(ctx, `
INSERT INTO open.open_api_credential
    (client_idn, credential_no, auth_mode, access_key_id, key_prefix, secret_hash, secret_cipher,
     secret_kid, sign_algorithm, scope_str, allow_cidr, dt_expire, flag_enable, flag_revoke, remark,
     create_by, modify_by)
VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''),
        NULLIF($8, ''), $9, $10::jsonb, $11::cidr[], $12::timestamptz, '1', '0', NULLIF($13, ''),
        NULLIF($14, ''), NULLIF($14, ''))
RETURNING idn`,
		req.ClientIDN, req.CredentialNo, req.AuthMode, req.AccessKey, keyPrefix, secretHash, secretCipher,
		kid, req.SignAlgorithm, scopeToJSON(req.Scope), emptyToArray(req.AllowCIDR),
		nilIfEmpty(req.DTExpire), req.Remark, operator).Scan(&idn); err != nil {
		return 0, err
	}
	if err := insertScopeRows(ctx, tx, idn, ParseScopeItems(req.Scope), operator); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return idn, nil
}

// UpdateCredentialExpire 修改已有凭证的有效期（需求 11246）。
//
// `raw` 为空串表示改为长期（`dt_expire = NULL`）。已吊销的凭证不允许改，用 `flag_revoke='0'` 兜住。
func (s *Store) UpdateCredentialExpire(ctx context.Context, credentialIDN int64,
	raw string, operator string) error {
	if s.pool == nil {
		return errors.New("数据库连接池未初始化")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET dt_expire = NULLIF($2, '')::timestamptz, modify_by = NULLIF($3, '')
 WHERE idn = $1 AND flag_revoke = '0'`, credentialIDN, raw, operator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateCredentialScope 修改已有凭证的授权范围（需求 11230）。
//
// **必须事务双写**：`scope_str` 是可读快照、`open_api_credential_scope` 是鉴权读取源，
// 只改一处会出现「页面显示 A、鉴权按 B」的错位。语义：`pairs` 为空 = 不限接口（明细表 0 行）。
//
// 已吊销的凭证不允许改（只能新建），用 `flag_revoke='0'` 兜住并发吊销。
func (s *Store) UpdateCredentialScope(ctx context.Context, credentialIDN int64,
	scopeJSON string, pairs []ScopePair, operator string) error {
	if s.pool == nil {
		return errors.New("数据库连接池未初始化")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
UPDATE open.open_api_credential
   SET scope_str = $2::jsonb, modify_by = NULLIF($3, '')
 WHERE idn = $1 AND flag_revoke = '0'`, credentialIDN, scopeJSON, operator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM open.open_api_credential_scope WHERE credential_idn = $1`, credentialIDN); err != nil {
		return err
	}
	if err := insertScopeRows(ctx, tx, credentialIDN, pairs, operator); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// insertScopeRows 批量写授权范围明细（同一事务内，调用方负责提交）。
func insertScopeRows(ctx context.Context, tx pgx.Tx, credentialIDN int64, pairs []ScopePair,
	operator string) error {
	if len(pairs) == 0 {
		return nil // 0 行 = 不限接口，无需写入
	}
	batch := &pgx.Batch{}
	for _, pair := range pairs {
		batch.Queue(`
INSERT INTO open.open_api_credential_scope (credential_idn, method, path_pattern, create_by)
VALUES ($1, $2, $3, NULLIF($4, ''))
ON CONFLICT (credential_idn, method, path_pattern) DO NOTHING`,
			credentialIDN, pair.Method, pair.Path, operator)
	}
	results := tx.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range pairs {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// RotateCredential 轮换密钥材料。
//
//   - API_KEY：换 `access_key_id`（静态 Key 本身就是公开标识）+ `secret_hash`，旧 Key 立即失效；
//   - AK_SK：换 `secret_cipher`，旧密文挪到 `secret_cipher_prev`（轮换窗口内新旧都能验签）。
func (s *Store) RotateCredential(ctx context.Context, idn int64, authMode, accessKey, secretHash, secretCipher, keyPrefix, kid, operator string) error {
	if s.pool == nil {
		return errors.New("数据库连接池未初始化")
	}
	var (
		tag pgconn.CommandTag
		err error
	)
	switch authMode {
	case AuthModeAPIKey:
		tag, err = s.pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET access_key_id = $2, key_prefix = NULLIF($3, ''), secret_hash = NULLIF($4, ''),
       dt_secret_rotate = now(), dt_last_use = NULL, count_use = 0, modify_by = NULLIF($5, '')
 WHERE idn = $1 AND flag_revoke = '0'`, idn, accessKey, keyPrefix, secretHash, operator)
	default:
		tag, err = s.pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET secret_cipher_prev = secret_cipher, secret_cipher = NULLIF($2, ''), secret_kid = NULLIF($3, ''),
       dt_secret_rotate = now(), dt_last_use = NULL, count_use = 0, modify_by = NULLIF($4, '')
 WHERE idn = $1 AND flag_revoke = '0'`, idn, secretCipher, kid, operator)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetCredentialEnable 启用/停用凭证（已吊销的不可再启用）。
func (s *Store) SetCredentialEnable(ctx context.Context, idn int64, enable bool, operator string) (Credential, error) {
	if s.pool == nil {
		return Credential{}, errors.New("数据库连接池未初始化")
	}
	flag := "0"
	if enable {
		flag = "1"
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET flag_enable = $2, modify_by = NULLIF($3, '')
 WHERE idn = $1 AND flag_revoke = '0'`, idn, flag, operator)
	if err != nil {
		return Credential{}, err
	}
	if tag.RowsAffected() == 0 {
		// 区分「不存在」与「已吊销」
		item, getErr := s.CredentialByID(ctx, idn)
		if getErr != nil {
			return Credential{}, getErr
		}
		return item, fmt.Errorf("%w: 凭证已吊销，不能启用/停用", ErrConflict)
	}
	return s.CredentialByID(ctx, idn)
}

// RevokeCredential 吊销凭证（原因必填，不可恢复）。
func (s *Store) RevokeCredential(ctx context.Context, idn int64, reason, operator string) (Credential, error) {
	if s.pool == nil {
		return Credential{}, errors.New("数据库连接池未初始化")
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE open.open_api_credential
   SET flag_revoke = '1', flag_enable = '0', dt_revoke = now(), reason_revoke = $2,
       modify_by = NULLIF($3, '')
 WHERE idn = $1 AND flag_revoke = '0'`, idn, reason, operator)
	if err != nil {
		return Credential{}, err
	}
	if tag.RowsAffected() == 0 {
		item, getErr := s.CredentialByID(ctx, idn)
		if getErr != nil {
			return Credential{}, getErr
		}
		return item, fmt.Errorf("%w: 凭证已吊销（%s）", ErrConflict, item.ReasonRevoke)
	}
	return s.CredentialByID(ctx, idn)
}

// ---- 调用审计 ----

// selectInvokeLogSQL 审计分页查询。
//
// LEFT JOIN open_api_credential 带出「凭证号」：STS 行的 credential_idn 是**父凭证** idn，
// 页面要能直接看出「这次 STS 调用是基于哪个凭证来的」（需求 11201）。
const selectInvokeLogSQL = `
SELECT l.idn, l.client_idn, l.credential_idn, COALESCE(l.access_key_id, ''), COALESCE(l.kfp, ''), COALESCE(l.tfp, ''),
       COALESCE(l.auth_mode, ''), COALESCE(c.credential_no, ''),
       COALESCE(c.parent_idn, 0), COALESCE(pc.credential_no, ''),
       COALESCE(l.request_id, ''), to_char(l.dt_request, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       COALESCE(l.method, ''), COALESCE(l.path, ''), COALESCE(e.api_code, ''), COALESCE(e.api_name, ''),
       COALESCE(l.query_str, ''),
       l.status_code, l.biz_code, l.flag_pass, COALESCE(l.reason_code, ''),
       COALESCE(host(l.ip), ''), COALESCE(l.user_agent, ''), COALESCE(l.body_sha256, ''),
       l.count_body, l.latency_ms
  FROM open.open_api_invoke_log l
  LEFT JOIN open.open_api_credential c ON c.idn = l.credential_idn
  LEFT JOIN open.open_api_credential pc ON pc.idn = c.parent_idn
  LEFT JOIN open.open_api_endpoint e ON e.http_method = l.method AND e.path = l.path`

// InvokeLogPage 分页查询调用审计（时间倒序）。
func (s *Store) InvokeLogPage(ctx context.Context, filter InvokeLogFilter) ([]InvokeLog, int, error) {
	if s.pool == nil {
		return nil, 0, errors.New("数据库连接池未初始化")
	}
	conditions := []string{}
	args := []any{}
	if filter.ClientIDN > 0 {
		args = append(args, filter.ClientIDN)
		conditions = append(conditions, fmt.Sprintf(`l.client_idn = $%d`, len(args)))
	}
	if filter.CredentialIDN > 0 {
		args = append(args, filter.CredentialIDN)
		conditions = append(conditions, fmt.Sprintf(`l.credential_idn = $%d`, len(args)))
	}
	if key := strings.TrimSpace(filter.AccessKeyID); key != "" {
		args = append(args, "%"+key+"%")
		conditions = append(conditions, fmt.Sprintf(`l.access_key_id ILIKE $%d`, len(args)))
	}
	if fingerprint := strings.TrimSpace(filter.KFP); fingerprint != "" {
		args = append(args, strings.ToUpper(fingerprint))
		conditions = append(conditions, fmt.Sprintf(`l.kfp = $%d`, len(args)))
	}
	if pass := strings.TrimSpace(filter.FlagPass); pass != "" {
		args = append(args, pass)
		conditions = append(conditions, fmt.Sprintf(`l.flag_pass = $%d`, len(args)))
	}
	if reason := strings.TrimSpace(filter.ReasonCode); reason != "" {
		args = append(args, reason)
		conditions = append(conditions, fmt.Sprintf(`l.reason_code = $%d`, len(args)))
	}
	if !filter.DTFrom.IsZero() {
		args = append(args, filter.DTFrom)
		conditions = append(conditions, fmt.Sprintf(`l.dt_request >= $%d`, len(args)))
	}
	if !filter.DTTo.IsZero() {
		args = append(args, filter.DTTo)
		conditions = append(conditions, fmt.Sprintf(`l.dt_request <= $%d`, len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM open.open_api_invoke_log l`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.pool.Query(ctx, selectInvokeLogSQL+where+
		fmt.Sprintf(` ORDER BY l.dt_request DESC, l.idn DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]InvokeLog, 0, filter.PageSize)
	for rows.Next() {
		item := InvokeLog{}
		if err := rows.Scan(&item.IDN, &item.ClientIDN, &item.CredentialIDN, &item.AccessKeyID, &item.KFP, &item.TFP,
			&item.AuthMode, &item.CredentialNo, &item.ParentIDN, &item.ParentCredentialNo,
			&item.RequestID, &item.DTRequest, &item.Method, &item.Path, &item.APICode, &item.APIName, &item.Query,
			&item.StatusCode, &item.BizCode, &item.FlagPass, &item.ReasonCode,
			&item.IP, &item.UserAgent, &item.BodySHA256, &item.CountBody, &item.LatencyMs); err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// ---- 接口清单（open.open_api_endpoint，需求 11220 / 11223）----
//
// 本表是「接口目录 + 元数据」，不是鉴权白名单：允许调哪些接口由代码里的路由注册决定。
// 页面只用它做三件事：列清单、切 flag_public / flag_enable、给授权范围下拉提供可选项。
const selectEndpointSQL = `
SELECT e.idn, e.api_code, e.catalog, e.func, e.alias, e.http_method, e.path, e.api_name, COALESCE(e.api_group, ''),
       e.flag_write, e.flag_public, e.flag_enable,
       COALESCE(to_char(e.dt_deprecate, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
       COALESCE(e.remark, ''),
       to_char(e.gmt_update, 'YYYY-MM-DD"T"HH24:MI:SSOF'),
       (SELECT count(*) FROM open.open_api_credential_scope s
         WHERE s.method = e.http_method AND s.path_pattern = e.path)
  FROM open.open_api_endpoint e`

// EndpointPage 分页查询接口清单（按分组、公开/启用标志、关键字过滤）。
func (s *Store) EndpointPage(ctx context.Context, filter EndpointFilter) ([]Endpoint, int, error) {
	if s.pool == nil {
		return nil, 0, errors.New("数据库连接池未初始化")
	}
	conditions := []string{}
	args := []any{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		conditions = append(conditions, fmt.Sprintf(
			`(e.api_code ILIKE $%d OR e.path ILIKE $%d OR e.api_name ILIKE $%d)`,
			len(args), len(args), len(args)))
	}
	if catalog := strings.TrimSpace(filter.Catalog); catalog != "" {
		args = append(args, catalog)
		conditions = append(conditions, fmt.Sprintf(`e.catalog = $%d`, len(args)))
	}
	if group := strings.TrimSpace(filter.APIGroup); group != "" {
		args = append(args, group)
		conditions = append(conditions, fmt.Sprintf(`COALESCE(e.api_group, '') = $%d`, len(args)))
	}
	if flag := strings.TrimSpace(filter.FlagPublic); flag != "" {
		args = append(args, flag)
		conditions = append(conditions, fmt.Sprintf(`e.flag_public = $%d`, len(args)))
	}
	if flag := strings.TrimSpace(filter.FlagEnable); flag != "" {
		args = append(args, flag)
		conditions = append(conditions, fmt.Sprintf(`e.flag_enable = $%d`, len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM open.open_api_endpoint e`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.pool.Query(ctx, selectEndpointSQL+where+
		fmt.Sprintf(` ORDER BY e.catalog, e.func LIMIT $%d OFFSET $%d`,
			len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Endpoint, 0, filter.PageSize)
	for rows.Next() {
		item := Endpoint{}
		if err := rows.Scan(&item.IDN, &item.APICode, &item.Catalog, &item.Func, &item.Alias, &item.HTTPMethod,
			&item.Path, &item.APIName, &item.APIGroup, &item.FlagWrite, &item.FlagPublic, &item.FlagEnable,
			&item.DTDeprecate, &item.Remark, &item.GMTUpdate, &item.CountGrant); err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	return list, total, rows.Err()
}

// EndpointOptions 授权范围下拉的可选项：只给「对外公开 + 启用中 + 未下线」的接口。
func (s *Store) EndpointOptions(ctx context.Context) ([]Endpoint, error) {
	if s.pool == nil {
		return nil, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, selectEndpointSQL+`
 WHERE e.flag_public = '1' AND e.flag_enable = '1' AND e.dt_deprecate IS NULL
 ORDER BY e.catalog, e.func`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Endpoint, 0, 16)
	for rows.Next() {
		item := Endpoint{}
		if err := rows.Scan(&item.IDN, &item.APICode, &item.Catalog, &item.Func, &item.Alias, &item.HTTPMethod,
			&item.Path, &item.APIName, &item.APIGroup, &item.FlagWrite, &item.FlagPublic, &item.FlagEnable,
			&item.DTDeprecate, &item.Remark, &item.GMTUpdate, &item.CountGrant); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// SetEndpointFlags 切换接口的公开/启用标志（nil 表示该列不动）。
func (s *Store) SetEndpointFlags(ctx context.Context, idn int64, flagPublic, flagEnable *string,
	operator string) (Endpoint, error) {
	if s.pool == nil {
		return Endpoint{}, errors.New("数据库连接池未初始化")
	}
	sets := []string{"modify_by = $1"}
	args := []any{operator}
	if flagPublic != nil {
		args = append(args, *flagPublic)
		sets = append(sets, fmt.Sprintf("flag_public = $%d", len(args)))
	}
	if flagEnable != nil {
		args = append(args, *flagEnable)
		sets = append(sets, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	args = append(args, idn)
	tag, err := s.pool.Exec(ctx, fmt.Sprintf(
		`UPDATE open.open_api_endpoint SET %s WHERE idn = $%d`,
		strings.Join(sets, ", "), len(args)), args...)
	if err != nil {
		return Endpoint{}, err
	}
	if tag.RowsAffected() == 0 {
		return Endpoint{}, ErrNotFound
	}
	return s.EndpointByID(ctx, idn)
}

// EndpointByID 按主键取一行接口清单。
func (s *Store) EndpointByID(ctx context.Context, idn int64) (Endpoint, error) {
	if s.pool == nil {
		return Endpoint{}, errors.New("数据库连接池未初始化")
	}
	item := Endpoint{}
	err := s.pool.QueryRow(ctx, selectEndpointSQL+` WHERE e.idn = $1`, idn).Scan(
		&item.IDN, &item.APICode, &item.Catalog, &item.Func, &item.Alias, &item.HTTPMethod,
		&item.Path, &item.APIName, &item.APIGroup, &item.FlagWrite, &item.FlagPublic, &item.FlagEnable,
		&item.DTDeprecate, &item.Remark, &item.GMTUpdate, &item.CountGrant)
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrNotFound
	}
	if err != nil {
		return Endpoint{}, err
	}
	return item, nil
}

// Begin 开启事务（「重加密 + 写配置」必须原子完成，否则会出现密文已换而配置未换的死局）。
func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	if s.pool == nil {
		return nil, errors.New("数据库连接池未初始化")
	}
	return s.pool.Begin(ctx)
}

// ---- 小工具 ----

// emptyToArray 保证 cidr[] 列拿到空数组而不是 NULL（两列都是 NOT NULL DEFAULT '{}'）。
func emptyToArray(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return items
}

func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// ---- 密钥配置（19004 / 19005 / 19006，需求 11175）----

// SecretState 密钥配置现状（只有布尔与计数，不含任何密钥明文）。
type SecretState struct {
	SecretKID            string
	SecretKeyBase64      string // 仅供服务层内部使用，不出接口
	FallbackKey          string // 同上
	CountCredential      int    // 持有密文的凭证数
	CountCredentialStale int    // secret_kid 与当前 KID 不一致（需重加密）
}

// LoadSecretState 读取密钥配置与凭证统计。
func (s *Store) LoadSecretState(ctx context.Context) (SecretState, error) {
	state := SecretState{SecretKID: "k1"}
	if s.pool == nil {
		return state, errors.New("数据库连接池未初始化")
	}
	rows, err := s.pool.Query(ctx, `
SELECT config_no, COALESCE(str_value, '')
  FROM sys.sys_config WHERE config_no IN (19004, 19005, 19006)`)
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var (
			configNo int
			value    string
		)
		if err := rows.Scan(&configNo, &value); err != nil {
			rows.Close()
			return state, err
		}
		switch configNo {
		case ConfigSecretKID:
			if strings.TrimSpace(value) != "" {
				state.SecretKID = strings.TrimSpace(value)
			}
		case ConfigSecretKey:
			state.SecretKeyBase64 = strings.TrimSpace(value)
		case ConfigFallback:
			state.FallbackKey = strings.TrimSpace(value)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return state, err
	}

	if err := s.pool.QueryRow(ctx, `
SELECT count(*), count(*) FILTER (WHERE COALESCE(secret_kid, '') <> $1)
  FROM open.open_api_credential
 WHERE secret_cipher IS NOT NULL`, state.SecretKID).Scan(&state.CountCredential, &state.CountCredentialStale); err != nil {
		return state, err
	}
	return state, nil
}

// execer 抽象「连接池或事务」，使多条写入能放进同一事务。
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// SaveSecretSQL 写入 19004 / 19005 / 19006（kidKey/secretKey/fallbackKey 指针为 nil 表示不改）。
func (s *Store) SaveSecretSQL(ctx context.Context, db execer, kidKey, secretKey, fallbackKey *string) error {
	batch := &pgx.Batch{}
	if kidKey != nil {
		batch.Queue(`UPDATE sys.sys_config SET str_value = $2 WHERE config_no = $1`, ConfigSecretKID, *kidKey)
	}
	if secretKey != nil {
		batch.Queue(`UPDATE sys.sys_config SET str_value = $2 WHERE config_no = $1`, ConfigSecretKey, *secretKey)
	}
	if fallbackKey != nil {
		batch.Queue(`UPDATE sys.sys_config SET str_value = $2 WHERE config_no = $1`, ConfigFallback, *fallbackKey)
	}
	if len(batch.QueuedQueries) == 0 {
		return nil
	}
	results := db.SendBatch(ctx, batch)
	defer results.Close()
	for range batch.QueuedQueries {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}
	return results.Close()
}

// ReencryptCredentials 用新密钥重加密所有持有密文的凭证，并把 secret_kid 更新为新 KID。
//
// 用旧密钥解不开的行**跳过并计数**（不中断、不改动），由调用方提示人工处理：
// 这类行通常是被更早的密钥加密、而 19005 已被覆盖，需要重新签发凭证。
func (s *Store) ReencryptCredentials(ctx context.Context, db execer, oldKey, newKey []byte, newKID string) (int, int, error) {
	type row struct {
		idn     int64
		current string
		prev    string
	}
	rows, err := db.Query(ctx, `
SELECT idn, COALESCE(secret_cipher, ''), COALESCE(secret_cipher_prev, '')
  FROM open.open_api_credential
 WHERE secret_cipher IS NOT NULL OR secret_cipher_prev IS NOT NULL`)
	if err != nil {
		return 0, 0, err
	}
	var items []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.idn, &item.current, &item.prev); err != nil {
			rows.Close()
			return 0, 0, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	updated, skipped := 0, 0
	for _, item := range items {
		newCurrent, errCurrent := reencryptOne(item.current, oldKey, newKey)
		newPrev, errPrev := reencryptOne(item.prev, oldKey, newKey)
		if errCurrent != nil || errPrev != nil {
			skipped++
			continue
		}
		var currentValue, prevValue *string
		if newCurrent != "" {
			currentValue = &newCurrent
		}
		if newPrev != "" {
			prevValue = &newPrev
		}
		if _, err := db.Exec(ctx, `
UPDATE open.open_api_credential
   SET secret_cipher = $2, secret_cipher_prev = $3, secret_kid = $4
 WHERE idn = $1`, item.idn, currentValue, prevValue, newKID); err != nil {
			return updated, skipped, err
		}
		updated++
	}
	return updated, skipped, nil
}

// reencryptOne 单字段重加密；空密文原样返回空串。这里只做字节搬运，密码学在 secret 包里。
func reencryptOne(cipherText string, oldKey, newKey []byte) (string, error) {
	if strings.TrimSpace(cipherText) == "" {
		return "", nil
	}
	plain, err := secret.Decrypt(cipherText, oldKey)
	if err != nil {
		return "", err
	}
	return secret.Encrypt(plain, newKey)
}
