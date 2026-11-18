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

	"github.com/acanx/finvquant/internal/meta/component/quote/moomoo/webapi"
)

// sys_config 中 moomoo OpenAPI 凭据的配置编号（V300227）。
const (
	// SysConfigMoomooAppKey 20016：AppKey ID。
	SysConfigMoomooAppKey = 20016
	// SysConfigMoomooPrivateKey 20017：Ed25519 私钥 PEM（敏感）。
	SysConfigMoomooPrivateKey = 20017
)

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

// LoadMoomooConfig 读取 sys.sys_config 20016/20017（缺行按空串处理）。
func (s *Store) LoadMoomooConfig(ctx context.Context) (MoomooConfig, error) {
	var cfg MoomooConfig
	rows, err := s.pool.Query(ctx, `
SELECT config_no, COALESCE(str_value, '')
  FROM sys.sys_config
 WHERE config_no IN ($1, $2)`, SysConfigMoomooAppKey, SysConfigMoomooPrivateKey)
	if err != nil {
		return cfg, fmt.Errorf("读取 moomoo 凭据配置失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var no int
		var value string
		if err := rows.Scan(&no, &value); err != nil {
			return cfg, fmt.Errorf("解析 moomoo 凭据配置失败: %w", err)
		}
		switch no {
		case SysConfigMoomooAppKey:
			cfg.AppKey = strings.TrimSpace(value)
		case SysConfigMoomooPrivateKey:
			cfg.PrivateKeyPEM = value
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, fmt.Errorf("读取 moomoo 凭据配置失败: %w", err)
	}
	return cfg, nil
}

// SaveMoomooConfig 写入 sys.sys_config 20016/20017。
//
// appKey 必填（空串表示清空）；privateKeyPEM 为空串时**保留原值**（前端留空 = 不修改），
// 需要清空私钥时传 `__CLEAR__`。
func (s *Store) SaveMoomooConfig(ctx context.Context, appKey, privateKeyPEM string) error {
	appKey = strings.TrimSpace(appKey)
	if appKey == "" {
		return errors.New("AppKey 不能为空")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("保存 moomoo 凭据配置失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
INSERT INTO sys.sys_config (config_no, config_name, data_type, char_value, str_value, access_level, remark)
VALUES ($1, 'moomoo OpenAPI AppKey', '3', '', $2, 2,
        'moomoo OpenAPI 的 AppKey ID（控制台 AppKey 管理创建）；留空表示未配置（回退环境变量 FINV_MOOMOO_APP_KEY）')
ON CONFLICT (config_no) DO UPDATE SET str_value = EXCLUDED.str_value`,
		SysConfigMoomooAppKey, appKey); err != nil {
		return fmt.Errorf("保存 moomoo AppKey 失败: %w", err)
	}

	key := strings.TrimSpace(privateKeyPEM)
	switch key {
	case "":
		// 留空 = 不修改私钥
	case "__CLEAR__":
		if _, err := tx.Exec(ctx, `
UPDATE sys.sys_config SET str_value = '' WHERE config_no = $1`, SysConfigMoomooPrivateKey); err != nil {
			return fmt.Errorf("清空 moomoo 私钥失败: %w", err)
		}
	default:
		// 先做一次格式校验，避免把非法 PEM 写进系统配置
		if _, err := webapi.ParseEd25519PrivateKey(key); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO sys.sys_config (config_no, config_name, data_type, char_value, str_value, access_level, remark)
VALUES ($1, 'moomoo OpenAPI 私钥（Ed25519 PEM）', '3', '', $2, 3,
        'moomoo OpenAPI 的 Ed25519 PKCS#8 私钥 PEM；留空表示未配置（回退环境变量 FINV_MOOMOO_PRIVATE_KEY）。敏感信息，请勿在日志/前端明文回显')
ON CONFLICT (config_no) DO UPDATE SET str_value = EXCLUDED.str_value`,
			SysConfigMoomooPrivateKey, key); err != nil {
			return fmt.Errorf("保存 moomoo 私钥失败: %w", err)
		}
	}

	return tx.Commit(ctx)
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
