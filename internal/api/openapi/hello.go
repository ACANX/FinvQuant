package openapi

// 握手接口（需求 11242）：GET /OpenAPI/V1/Meta/Region/AZ/Server/Hello
//
// 设计思路照搬 TLS 握手的前两步，只做「连通性 + 身份确认」，不承载业务：
//
//	客户端 → 服务端：ClientHello = 一次带认证信息的请求（APIKEY / AKSK / STS 三要素）
//	                  认证由 openapi.Middleware 完成，失败按 190xx 直接拒绝（握手不成立）
//	服务端 → 客户端：ServerHello = 本文件的响应体
//	                  回显「服务端认到的你是谁」+ 服务端时间（供客户端校时）+ 能力清单 + 可用接口清单
//
// 与 TLS 的对应关系：
//
//	TLS version        → protocol（finv-openapi/1.0）
//	random             → server_time（客户端据此校准签名时间窗偏差）
//	session id         → request_id（审计表同值，可回查）
//	cipher suite       → capabilities.sign_algorithms
//	extensions         → capabilities（时间窗 / 限流 / STS 换证规则）+ endpoints（可调接口清单）
//
// 两条刻意设计：
//   - **不校验授权范围**（见 middleware 的 ScopeFreePaths）：握手只验认证与连通性，
//     若还要求凭证 scope 里显式包含 /Hello，"测连通性"就得先配授权，与握手语义冲突；
//   - **仍受凭证类型隔离**（需求 11210）：STS 兑换凭证只能换证，不能调业务接口，也不能握手。
//
// 响应不含任何密钥材料（只有指纹），可安全用于客户端自检与运维巡检。

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HelloPath 握手接口路径（用户定稿，需求 11242）。
const HelloPath = "/OpenAPI/V1/Meta/Region/AZ/Server/Hello"

// helloProtocol 握手协议版本（响应里回给客户端，便于以后演进）。
const helloProtocol = "finv-openapi/1.0"

// helloStsTTLSeconds STS 临时凭据有效期（与 sts.go 的 StsTTL 保持一致）。
const helloStsTTLSeconds = 7200

// HelloHandler 握手处理器。
//
// pool 只读 open_api_endpoint（可用接口清单）；settings 取时间窗与鉴权模式（19001/19002）；
// server / version / commit 是部署标识，便于客户端确认连到了哪个实例。
type HelloHandler struct {
	pool     *pgxpool.Pool
	settings SettingsLoader
	server   string
	version  string
	commit   string
}

// NewHelloHandler 创建握手处理器。
func NewHelloHandler(pool *pgxpool.Pool, settings SettingsLoader,
	server, version, commit string) *HelloHandler {
	return &HelloHandler{pool: pool, settings: settings, server: server, version: version, commit: commit}
}

// helloServer 服务端标识。
type helloServer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	AZ      string `json:"az"`
}

// helloTime 服务端时间（客户端用它校准签名时间窗）。
type helloTime struct {
	Unix    int64  `json:"unix"`
	RFC3339 string `json:"rfc3339"`
	Offset  string `json:"utc_offset"`
}

// helloClient 服务端「认到的你是谁」——回显让客户端确认凭证配置正确。
type helloClient struct {
	ClientIDN     int64  `json:"client_idn"`
	ClientCode    string `json:"client_code"`
	CredentialIDN int64  `json:"credential_idn"`
	CredentialNo  string `json:"credential_no"`
	AuthMode      string `json:"auth_mode"`
	// AccessKeyID 公开标识，形态与审计表 access_key_id 一致：
	// APIKEY 为脱敏形态（前缀 + 前8 + 等量星号 + 后8）、AKSK 为 AK 明文、STS 为临时 AK。
	AccessKeyID string `json:"access_key_id"`
	// KeyFP 密钥指纹，与审计表 kfp 同值：仅 APIKEY / STS 有值，
	// AKSK 为空（AK 已明文记录，无需指纹区分，见 Admin.md §6.1）。
	KeyFP string `json:"key_fp"`
	// ScopeCount 该凭证被授权的接口条数（0 = 不限接口）。
	ScopeCount int `json:"scope_count"`
	// ExpireAt 凭证到期时间（空 = 随调用方 / 长期）。
	ExpireAt string `json:"expire_at"`
}

// helloCapabilities 服务端能力（对应 TLS 的 cipher suite / extensions）。
type helloCapabilities struct {
	SignAlgorithms     []string     `json:"sign_algorithms"`
	ClockSkewSeconds   int          `json:"clock_skew_seconds"`
	AuthMode           string       `json:"auth_mode"`
	RateLimitRPM       int          `json:"rate_limit_rpm"`
	QuotaDay           int          `json:"quota_day"`
	Sts                helloStsInfo `json:"sts"`
	RequestIDHeader    string       `json:"request_id_header"`
	ScopeRequiredPaths string       `json:"scope_required_note"`
}

// helloStsInfo STS 换证规则。
type helloStsInfo struct {
	ExchangePath string `json:"exchange_path"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

// helloEndpoint 可调接口（客户端据此规划后续请求）。
type helloEndpoint struct {
	APICode    string `json:"api_code"`
	Alias      string `json:"alias"`
	HTTPMethod string `json:"http_method"`
	FlagWrite  string `json:"flag_write"`
}

// helloResponse ServerHello 响应体。
type helloResponse struct {
	Handshake    string            `json:"handshake"`
	Protocol     string            `json:"protocol"`
	Server       helloServer       `json:"server"`
	ServerTime   helloTime         `json:"server_time"`
	Client       helloClient       `json:"client"`
	Capabilities helloCapabilities `json:"capabilities"`
	Endpoints    []helloEndpoint   `json:"endpoints"`
	RequestID    string            `json:"request_id"`
}

// Hello 处理 ClientHello：认证已由中间件完成，这里只回 ServerHello。
func (h *HelloHandler) Hello(c *gin.Context) {
	credential, ok := CredentialFrom(c)
	if !ok || credential == nil {
		// 正常走不到这里（中间件已拦），兜底保持与其它接口一致的错误码
		c.JSON(http.StatusUnauthorized, gin.H{"code": CodeKeyMissing, "message": "未携带 API Key 或签名信息"})
		return
	}

	settings := Settings{Mode: "1"}
	if h.settings != nil {
		settings = h.settings.Load(c.Request.Context())
	}

	now := time.Now()
	requestID := c.GetString(ContextRequestIDKey)

	resp := helloResponse{
		Handshake: "ServerHello",
		Protocol:  helloProtocol,
		Server: helloServer{
			Name:    h.server,
			Version: h.version,
			Commit:  h.commit,
			AZ:      "AZ",
		},
		ServerTime: helloTime{
			Unix:    now.Unix(),
			RFC3339: now.Format(time.RFC3339),
			Offset:  now.Format("-07:00"),
		},
		Client: helloClient{
			ClientIDN:     credential.ClientIDN,
			ClientCode:    credential.ClientCode,
			CredentialIDN: credential.IDN,
			CredentialNo:  credential.CredentialNo,
			AuthMode:      credential.AuthMode,
			AccessKeyID:   helloAccessKeyID(credential),
			KeyFP:         helloKeyFingerprint(credential),
			ScopeCount:    len(credential.Scope),
			ExpireAt:      formatTimePtr(credential.CredentialExp),
		},
		Capabilities: helloCapabilities{
			SignAlgorithms:   []string{"HMAC-SHA256"},
			ClockSkewSeconds: int(settings.SignTimeWindow.Seconds()),
			AuthMode:         settings.Mode,
			RateLimitRPM:     credential.RateLimitRPM,
			QuotaDay:         credential.QuotaDay,
			Sts: helloStsInfo{
				ExchangePath: StsIssuePath,
				TTLSeconds:   helloStsTTLSeconds,
			},
			RequestIDHeader: HeaderRequestID,
			// 如实告诉客户端：握手接口本身不需要授权，业务接口需要
			ScopeRequiredPaths: "除 " + HelloPath + " 外的业务接口都需要凭证授权范围包含它",
		},
		Endpoints: h.endpoints(c.Request.Context()),
		RequestID: requestID,
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ServerHello", "data": resp})
}

// endpoints 读出对客户端可见的接口清单（公开 + 启用 + 未下线）。
//
// 读失败不影响握手：返回空列表（客户端仍能确认「认证已通过、服务在线」）。
func (h *HelloHandler) endpoints(ctx context.Context) []helloEndpoint {
	list := make([]helloEndpoint, 0, 8)
	if h.pool == nil {
		return list
	}
	rows, err := h.pool.Query(ctx, `
SELECT api_code, alias, http_method, flag_write
  FROM open.open_api_endpoint
 WHERE flag_public = '1' AND flag_enable = '1' AND dt_deprecate IS NULL
 ORDER BY catalog, func
 LIMIT 200`)
	if err != nil {
		return list
	}
	defer rows.Close()
	for rows.Next() {
		item := helloEndpoint{}
		if err := rows.Scan(&item.APICode, &item.Alias, &item.HTTPMethod, &item.FlagWrite); err != nil {
			return list
		}
		list = append(list, item)
	}
	return list
}

// helloAccessKeyID 返回与审计表 access_key_id 一致的形态（静态 Key 脱敏，其余明文）。
func helloAccessKeyID(credential *Credential) string {
	if credential == nil {
		return ""
	}
	if credential.AuthMode == AuthModeAPIKey {
		return maskKey(credential.AccessKeyID)
	}
	return credential.AccessKeyID
}

// helloKeyFingerprint 计算密钥指纹（与审计表 kfp 同算法同口径，便于两边对账）。
//
// 与审计一致：APIKEY / STS 写 kfp；AKSK 不写（AK 已明文，见 Admin.md §6.1）。
func helloKeyFingerprint(credential *Credential) string {
	if credential == nil {
		return ""
	}
	switch credential.AuthMode {
	case AuthModeAPIKey, AuthModeSTS:
		return FingerprintStaticKey(credential.AccessKeyID)
	default:
		return ""
	}
}

// formatTimePtr 把可能为 nil 的时间格式化成 RFC3339（空串表示长期）。
func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
