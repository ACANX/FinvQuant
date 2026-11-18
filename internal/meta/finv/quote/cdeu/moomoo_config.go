package cdeu

// moomoo OpenAPI 凭据的系统配置读写（sys.sys_config 20016 / 20017，需求 11318）。
//
// 读取顺序：sys_config（本文件）优先 → 环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY 兜底。
// 写入只更新 str_value（data_type='3'），配置项由迁移 V300227 发放空串占位。
//
// 安全约定：
//   - 私钥只在「读 → 构造客户端」链路里使用，接口只回显「是否已配置」与掩码；
//   - 日志不打印私钥；`MoomooConfig.MaskedPrivateKey` 只保留首尾少量字符。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// SysConfigMoomooCredential moomoo OpenAPI 凭据的配置编号（V300227 发放，V300228 合并为单项）。
//
// 值格式：`{AppKey}|{私钥 PEM}`（半角竖线分隔；PEM 内不含竖线）。
// 只配 AppKey 时允许写成 `{AppKey}|`；空串表示未配置。
const SysConfigMoomooCredential = 20016

// MoomooConfig moomoo OpenAPI 凭据（来自 sys.sys_config）。
type MoomooConfig struct {
	// AppKey 20016（去空白）。
	AppKey string
	// PrivateKeyPEM 20017（原样保存，构造客户端时再去空白/还原 \n）。
	PrivateKeyPEM string
}

// Configured 是否两项都已配置。
func (c MoomooConfig) Configured() bool {
	return strings.TrimSpace(c.AppKey) != "" && strings.TrimSpace(c.PrivateKeyPEM) != ""
}

// MaskedPrivateKey 私钥掩码（已配置返回固定 4 个星号，未配置返回空串）。
//
// PEM 的首尾都是固定的 `-----`，截取首尾字符没有辨识价值还会误导，这里干脆不泄露
// 任何字符；页面只需要知道「是否已配置」。
func (c MoomooConfig) MaskedPrivateKey() string {
	if strings.TrimSpace(c.PrivateKeyPEM) == "" {
		return ""
	}
	return "****"
}

// LoadMoomooConfig 读取 sys.sys_config 20016 并解析 `{AppKey}|{私钥 PEM}`（缺行按空串处理）。
func (s *Store) LoadMoomooConfig(ctx context.Context) (MoomooConfig, error) {
	var raw string
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(str_value, '')
  FROM sys.sys_config
 WHERE config_no = $1`, SysConfigMoomooCredential).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return MoomooConfig{}, nil
	}
	if err != nil {
		return MoomooConfig{}, fmt.Errorf("读取 moomoo 凭据配置失败: %w", err)
	}
	return parseMoomooCredential(raw), nil
}

// parseMoomooCredential 解析 `{AppKey}|{私钥 PEM}`；没有竖线时整串按 AppKey 处理。
func parseMoomooCredential(raw string) MoomooConfig {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MoomooConfig{}
	}
	appKey, key, ok := strings.Cut(raw, "|")
	if !ok {
		return MoomooConfig{AppKey: raw}
	}
	return MoomooConfig{AppKey: strings.TrimSpace(appKey), PrivateKeyPEM: key}
}

// marshalMoomooCredential 序列化为 `{AppKey}|{私钥 PEM}`。
func marshalMoomooCredential(appKey, privateKeyPEM string) string {
	return strings.TrimSpace(appKey) + "|" + privateKeyPEM
}

// SaveMoomooConfig 写入 sys.sys_config 20016（`{AppKey}|{私钥 PEM}`）。
//
// appKey 必填（空串返回错误）；privateKeyPEM 为空串时**保留原私钥**（前端留空 = 不修改），
// 需要清空私钥时传 `__CLEAR__`（保留 AppKey，只清私钥）。
func (s *Store) SaveMoomooConfig(ctx context.Context, appKey, privateKeyPEM string) error {
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return errors.New("AppKey 不能为空")
	}

	current, err := s.LoadMoomooConfig(ctx)
	if err != nil {
		return err
	}
	key := current.PrivateKeyPEM
	switch strings.TrimSpace(privateKeyPEM) {
	case "":
		// 留空 = 不修改私钥
	case "__CLEAR__":
		key = ""
	default:
		// 先做一次格式校验，避免把非法 PEM 写进系统配置
		if _, err := webapi.ParseEd25519PrivateKey(privateKeyPEM); err != nil {
			return err
		}
		key = strings.TrimSpace(privateKeyPEM)
	}

	if _, err := s.pool.Exec(ctx, `
INSERT INTO sys.sys_config (config_no, config_name, data_type, char_value, str_value, access_level, remark)
VALUES ($1, 'moomoo OpenAPI 凭据（AppKey|私钥 PEM）', '3', '', $2, 3,
        'moomoo OpenAPI 凭据，格式 {AppKey}|{Ed25519 PKCS#8 私钥 PEM}（半角竖线分隔）；只配 AppKey 时写成 {AppKey}|；留空表示未配置（回退环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）。敏感信息，请勿在日志/前端明文回显')
ON CONFLICT (config_no) DO UPDATE SET str_value = EXCLUDED.str_value`,
		SysConfigMoomooCredential, marshalMoomooCredential(appKey, key)); err != nil {
		return fmt.Errorf("保存 moomoo 凭据失败: %w", err)
	}
	return nil
}

// MoomooClientFromConfig 按「sys_config 优先 → 环境变量兜底」构造客户端。
//
// 返回 (nil, nil) 表示两处都没有配置（调用方按「未配置凭据」报错/提示）。
func (s *Store) MoomooClientFromConfig(ctx context.Context) (*webapi.Client, error) {
	cfg, err := s.LoadMoomooConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.Configured() {
		return webapi.NewClient(webapi.Config{
			AppKey:        cfg.AppKey,
			PrivateKeyPEM: cfg.PrivateKeyPEM,
		})
	}
	return webapi.NewClientFromEnv()
}
