// 39002 BRS 对象存储配置（sys.sys_config.json_value）的读写。
//
// 为什么单独成文件：备份登记（registry.go）维护 brs.brs_bak_rdb_table_full，
// 而对象存储配置是 sys.sys_config 的 JSON 配置项，两者存储位置与语义都不同。
//
// 密钥处理原则：
//   - 读取**永不**返回 secret_key 明文，只返回 secret_key_set 标记；
//   - 保存时 secret_key 留空表示「保持库里现值」，只有显式传入非空值才覆盖；
//   - 任何日志/错误信息都不带出密钥内容。
package brs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// OssConfigNo 对象存储配置的系统配置编号（JSON）。
const OssConfigNo = 39002

// OSSConfig 39002 的配置视图（对外 JSON 一律新键）。
//
// 字段语义（需求 10912）：
//   - enable：是否启用上传（旧键 enabled 兼容读取，保存只写 enable）；
//   - protocol：传输协议，本期支持 s3（S3 兼容）与 local（本地目录适配器）；
//   - provider：服务厂商标签，如 cloudflare-r2 / aws-s3 / local；仅作标识，
//     S3 兼容厂商统一走 internal/meta/base/oss 的 s3 适配器。
//
// 兼容：保存一律写新键（enable / protocol / provider）；读取兼容
// 旧键 enabled、误拼别名 protocal，以及旧格式 provider="s3"（仅协议、无厂商）。
type OSSConfig struct {
	Enable       bool       `json:"enable"`
	Protocol     string     `json:"protocol"`
	Provider     string     `json:"provider"`
	Endpoint     string     `json:"endpoint"`
	Bucket       string     `json:"bucket"`
	Region       string     `json:"region"`
	AccessKey    string     `json:"access_key"`
	SecretKey    string     `json:"secret_key"` // 读取恒为空；保存留空表示不修改
	Prefix       string     `json:"prefix"`
	SecretKeySet bool       `json:"secret_key_set"` // 只读：库内是否已配置 secret_key
	Remark       string     `json:"remark"`         // 只读：39002 的字段说明（迁移内置）
	GmtModify    *time.Time `json:"gmt_modify"`     // 只读：最近修改时间
}

// UnmarshalJSON 兼容旧调用方：enable 未给出时读 enabled，protocol 未给出时读误拼别名 protocal。
func (c *OSSConfig) UnmarshalJSON(data []byte) error {
	type plain OSSConfig
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*c = OSSConfig(p)

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	if _, ok := keys["enable"]; !ok {
		var legacy struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		if legacy.Enabled != nil {
			c.Enable = *legacy.Enabled
		}
	}
	if strings.TrimSpace(c.Protocol) == "" {
		var legacy struct {
			Protocal string `json:"protocal"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		c.Protocol = legacy.Protocal
	}
	return nil
}

// ossSettings 库内 JSON 的实际结构（凭证键为 ak / sk，见迁移 V300082）。
//
// 兼容：历史配置可能仍是 access_key / secret_key（V300082 之前写入），
// 旧键放在 Legacy* 字段里兜底读取，保存时统一写 ak / sk。
type ossSettings struct {
	Enable   *bool  `json:"enable,omitempty"`
	Protocol string `json:"protocol"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	Region   string `json:"region"`
	AK       string `json:"ak"`
	SK       string `json:"sk"`
	Prefix   string `json:"prefix"`
	// 旧键兜底（V300082 之前的 access_key/secret_key；10912 之前的 enabled 与误拼 protocal）
	LegacyEnabled  *bool  `json:"enabled,omitempty"`
	LegacyProtocal string `json:"protocal,omitempty"`
	LegacyAK       string `json:"access_key,omitempty"`
	LegacySK       string `json:"secret_key,omitempty"`
}

// enabled 返回生效的启用开关（新键 enable 优先，旧键 enabled 兜底）。
func (s ossSettings) enabled() bool {
	if s.Enable != nil {
		return *s.Enable
	}
	if s.LegacyEnabled != nil {
		return *s.LegacyEnabled
	}
	return false
}

// protocolValue 返回生效的协议（protocol 优先，兼容旧误拼 protocal）。
func (s ossSettings) protocolValue() string {
	if v := strings.TrimSpace(s.Protocol); v != "" {
		return v
	}
	return strings.TrimSpace(s.LegacyProtocal)
}

// 协议与厂商的取值约定（需求 10912）。
const (
	// OSSProtocolS3 S3 兼容协议（Cloudflare R2 / AWS S3 / MinIO 等厂商共用）。
	OSSProtocolS3 = "s3"
	// OSSProtocolLocal 本地目录协议（开发/离线环境，对应 <备份目录> 自身）。
	OSSProtocolLocal = "local"
	// OSSProviderCloudflareR2 Cloudflare R2 厂商标签。
	OSSProviderCloudflareR2 = "cloudflare-r2"
	// OSSProviderAWSS3 AWS S3 厂商标签。
	OSSProviderAWSS3 = "aws-s3"
)

// normalizeOSS 归一化协议与厂商，返回 (协议, 厂商, 适配器名, 错误)。
//
// 规则：
//   - protocol 空 → 默认 s3；provider=local 或 protocol=local → 本地目录适配器；
//   - 本期只实现 s3 / local 两种协议，其它值直接拒绝（避免静默回退到 s3）；
//   - provider 只是厂商标签：为空或等于协议名（旧格式 provider="s3"）时按 endpoint 推断
//     （含 r2.cloudflarestorage.com → cloudflare-r2，否则 aws-s3）；
//   - 适配器名（driver）来自内部注册表：s3 → s3 适配器，local → local 适配器。
func normalizeOSS(protocol, provider, endpoint string) (string, string, string, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	provider = strings.ToLower(strings.TrimSpace(provider))
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))

	if protocol == "" {
		if provider == OSSProtocolLocal {
			protocol = OSSProtocolLocal
		} else {
			protocol = OSSProtocolS3
		}
	}
	if protocol != OSSProtocolS3 && protocol != OSSProtocolLocal {
		return "", "", "", inputErrorf("protocol 只支持 %s / %s，当前为 %q", OSSProtocolS3, OSSProtocolLocal, protocol)
	}
	if protocol == OSSProtocolLocal {
		return protocol, OSSProtocolLocal, OSSProtocolLocal, nil
	}

	// 厂商归一：空 / 协议名 → 按 endpoint 推断；其它值原样保留（仅作标签）。
	switch provider {
	case "", OSSProtocolS3:
		if strings.Contains(endpoint, "r2.cloudflarestorage.com") {
			provider = OSSProviderCloudflareR2
		} else {
			provider = OSSProviderAWSS3
		}
	}
	return protocol, provider, OSSProtocolS3, nil
}

// accessKey 返回生效的 Access Key（新键优先，旧键兜底）。
func (s ossSettings) accessKey() string {
	if strings.TrimSpace(s.AK) != "" {
		return s.AK
	}
	return s.LegacyAK
}

// secretKey 返回生效的 Secret Key（新键优先，旧键兜底）。
func (s ossSettings) secretKey() string {
	if strings.TrimSpace(s.SK) != "" {
		return s.SK
	}
	return s.LegacySK
}

// LoadOSSConfig 读取 39002 的配置（不返回 secret_key 明文）。
func (r *Registry) LoadOSSConfig(ctx context.Context) (OSSConfig, error) {
	const query = `
SELECT COALESCE(json_value::text, ''), COALESCE(remark, ''), gmt_modify
  FROM sys.sys_config
 WHERE config_no = $1`
	var raw, remark string
	var gmtModify *time.Time
	err := r.pool.QueryRow(ctx, query, OssConfigNo).Scan(&raw, &remark, &gmtModify)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OSSConfig{}, inputErrorf("系统配置 %d 不存在，请先执行迁移 V300061", OssConfigNo)
		}
		return OSSConfig{}, fmt.Errorf("读取对象存储配置失败: %w", err)
	}

	var s ossSettings
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return OSSConfig{}, fmt.Errorf("解析 %d 的 json_value 失败: %w", OssConfigNo, err)
		}
	}
	protocol, provider, _, err := normalizeOSS(s.protocolValue(), s.Provider, s.Endpoint)
	if err != nil {
		return OSSConfig{}, fmt.Errorf("%d 的配置不合法（protocol=%q provider=%q）: %w",
			OssConfigNo, s.protocolValue(), s.Provider, err)
	}
	if strings.TrimSpace(s.Region) == "" {
		s.Region = "auto"
	}
	if strings.TrimSpace(s.Prefix) == "" {
		s.Prefix = "brs/"
	}
	return OSSConfig{
		Enable:       s.enabled(),
		Protocol:     protocol,
		Provider:     provider,
		Endpoint:     s.Endpoint,
		Bucket:       s.Bucket,
		Region:       s.Region,
		AccessKey:    s.accessKey(),
		SecretKey:    "", // 明文永不回传
		Prefix:       s.Prefix,
		SecretKeySet: strings.TrimSpace(s.secretKey()) != "",
		Remark:       remark,
		GmtModify:    gmtModify,
	}, nil
}

// SaveOSSConfig 保存 39002 的配置，返回保存后的视图（同样不含 secret_key）。
//
// 规则：
//   - enable=true 时 endpoint / bucket / access_key 必填，secret_key 可为空但库里必须已有值；
//   - protocol 空则 s3（本期只支持 s3 / local）；provider 空或等于协议名时按 endpoint 推断厂商；
//   - region 空则 auto；prefix 空则 brs/，并补足结尾 "/"；
//   - secret_key 留空表示保持原值；
//   - **保存一律写新键**（enable / protocol / provider），旧键 enabled / protocal 不再写出。
func (r *Registry) SaveOSSConfig(ctx context.Context, cfg OSSConfig, operator string) (OSSConfig, error) {
	cfg.Protocol = strings.TrimSpace(cfg.Protocol)
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.Prefix = strings.TrimSpace(cfg.Prefix)

	protocol, provider, _, err := normalizeOSS(cfg.Protocol, cfg.Provider, cfg.Endpoint)
	if err != nil {
		return OSSConfig{}, err
	}
	cfg.Protocol, cfg.Provider = protocol, provider
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "brs/"
	}
	if !strings.HasSuffix(cfg.Prefix, "/") {
		cfg.Prefix += "/"
	}

	// 先确认配置行存在（不存在时给出迁移提示）
	if _, err := r.LoadOSSConfig(ctx); err != nil {
		return OSSConfig{}, err
	}

	secret := cfg.SecretKey
	if secret == "" {
		// 留空表示不修改：取库内现值（LoadOSSConfig 不回传明文，这里单独读一次）
		var raw string
		if err := r.pool.QueryRow(ctx,
			`SELECT COALESCE(json_value::text, '') FROM sys.sys_config WHERE config_no = $1`,
			OssConfigNo).Scan(&raw); err != nil {
			return OSSConfig{}, fmt.Errorf("读取对象存储配置失败: %w", err)
		}
		var old ossSettings
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &old); err != nil {
				return OSSConfig{}, fmt.Errorf("解析 %d 的 json_value 失败: %w", OssConfigNo, err)
			}
		}
		secret = old.secretKey()
		if secret == "" {
			if cfg.Enable {
				return OSSConfig{}, inputErrorf("启用上传时必须填写 secret_key（或此前已配置过）")
			}
		}
	}

	if cfg.Endpoint != "" && cfg.Protocol != OSSProtocolLocal {
		// endpoint 只能是 scheme://host[:port]，不允许带路径。
		// 为什么：适配器按 path-style 拼 URL（/<bucket>/<key>），endpoint 里多出来的路径段
		// 会被对象存储当成 key 的一部分（例：endpoint 写 .../finvquant 时，
		// 最终对象 key 变成 finvquant/BRS/Finv/...，凭空多一层目录）。
		u, perr := url.Parse(cfg.Endpoint)
		if perr != nil || u.Scheme == "" || u.Host == "" {
			return OSSConfig{}, inputErrorf("endpoint 必须是完整 URL（如 https://<account_id>.r2.cloudflarestorage.com），当前值无法解析")
		}
		if strings.Trim(u.Path, "/") != "" {
			return OSSConfig{}, inputErrorf("endpoint 不能带路径（当前为 %q）：桶名填到 bucket，目录前缀填到 prefix；endpoint 只保留 scheme://host[:port]", u.Path)
		}
	}

	if cfg.Enable {
		if cfg.Endpoint == "" {
			return OSSConfig{}, inputErrorf("启用上传时 endpoint 不能为空")
		}
		if cfg.Bucket == "" {
			return OSSConfig{}, inputErrorf("启用上传时 bucket 不能为空")
		}
		if cfg.AccessKey == "" {
			return OSSConfig{}, inputErrorf("启用上传时 access_key 不能为空")
		}
		if secret == "" {
			return OSSConfig{}, inputErrorf("启用上传时 secret_key 不能为空")
		}
	}

	enable := cfg.Enable
	payload, err := json.Marshal(ossSettings{
		Enable:   &enable,
		Protocol: cfg.Protocol,
		Provider: cfg.Provider,
		Endpoint: cfg.Endpoint,
		Bucket:   cfg.Bucket,
		Region:   cfg.Region,
		AK:       cfg.AccessKey,
		SK:       secret,
		Prefix:   cfg.Prefix,
	})
	if err != nil {
		return OSSConfig{}, fmt.Errorf("序列化对象存储配置失败: %w", err)
	}

	tag, err := r.pool.Exec(ctx, `
UPDATE sys.sys_config
   SET json_value = $1::json,
       gmt_modify = CURRENT_TIMESTAMP(6),
       modify_by = $2
 WHERE config_no = $3`, string(payload), operator, OssConfigNo)
	if err != nil {
		return OSSConfig{}, fmt.Errorf("保存对象存储配置失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return OSSConfig{}, inputErrorf("系统配置 %d 不存在，请先执行迁移 V300061", OssConfigNo)
	}

	return r.LoadOSSConfig(ctx)
}
