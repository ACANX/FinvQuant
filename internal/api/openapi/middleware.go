package openapi

// 鉴权中间件（需求 11168）：`/OpenAPI/V1` 路由组统一入口。

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// 请求上下文键与响应头。
const (
	// ContextCredentialKey 鉴权通过后写入 gin.Context 的凭证（handler 可用于记录调用方）。
	ContextCredentialKey = "openapi_credential"
	// ContextRequestIDKey 本次请求的链路 ID。
	ContextRequestIDKey = "openapi_request_id"
	// HeaderRequestID 响应头，回显链路 ID。
	HeaderRequestID = "X-Request-Id"
)

// maxBodyBytes 参与签名计算的请求体上限（超限一律鉴权失败，避免内存放大）。
const maxBodyBytes = 64 << 20

// Options 构造中间件所需依赖。
type Options struct {
	Pool  *pgxpool.Pool
	Redis *redis.Client
	// FallbackKey 兼容期兜底静态密钥（环境变量 FINV_OPENAPI_KEY）；与 sys_config 19006 任一命中即放行。
	FallbackKey string
	// Source 凭证来源；为空时按 Pool 构造 PGSource。
	Source CredentialSource
	// Settings 配置加载器；为空时按 Pool 构造 SettingsStore。
	Settings SettingsLoader
	// Recorder 审计写入器；为空时按 Pool 构造。
	Recorder *Recorder
	// Now 时钟注入（测试用）。
	Now func() time.Time
	// Logf 应用日志（测试可静音）。
	Logf func(format string, args ...any)
}

// Middleware 外部 OpenAPI 鉴权中间件。
type Middleware struct {
	source   CredentialSource
	settings SettingsLoader
	redis    *RedisStore
	recorder *Recorder
	envKey   string
	now      func() time.Time
	logf     func(format string, args ...any)
}

// NewMiddleware 构造中间件。
func NewMiddleware(opts Options) *Middleware {
	source := opts.Source
	if source == nil {
		source = NewPGSource(opts.Pool)
	}
	settings := opts.Settings
	if settings == nil {
		settings = NewSettingsStore(opts.Pool)
	}
	recorder := opts.Recorder
	if recorder == nil {
		recorder = NewRecorder(opts.Pool)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	logf := opts.Logf
	if logf == nil {
		logf = log.Printf
	}
	return &Middleware{
		source:   source,
		settings: settings,
		redis:    NewRedisStore(opts.Redis),
		recorder: recorder,
		envKey:   strings.TrimSpace(opts.FallbackKey),
		now:      now,
		logf:     logf,
	}
}

// Handler 返回 gin 中间件。
//
// 流程（与 Docs/API/OpenAPI/Auth.md 一致）：
// 读配置 → 关闭则直接放行 → 解析凭证 → 定位凭证（失败可走兼容兜底）→ 解密 SK →
// 状态/时效 → IP 白名单 → 时间窗 → nonce → 签名 → 授权范围 → 限流 → 放行。
// 每次校验（含失败）都写 open.open_api_invoke_log；模式 2（宽松）下失败仍放行。
func (m *Middleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := m.now()
		settings := m.settings.Load(c.Request.Context())
		if !settings.AuthEnabled() {
			// 模式 0（关闭）：完全不做鉴权，行为与早期过渡实现一致，也不写审计（避免日志被灌）。
			c.Next()
			return
		}

		requestID := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Header(HeaderRequestID, requestID)
		c.Set(ContextRequestIDKey, requestID)

		body, err := readBody(c.Request)
		invocation := Invocation{
			RequestID: requestID,
			RequestAt: started,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Query:     c.Request.URL.RawQuery,
			IP:        clientIP(c.Request),
			UserAgent: c.Request.UserAgent(),
			BodySize:  len(body),
			BodyHash:  BodyHash(body),
		}
		if err != nil {
			m.deny(c, settings, invocation, started, CodeKeyInvalid, "读取请求体失败", err)
			return
		}

		credential, code, detail := m.authenticate(c, settings, c.Request, body, &invocation)
		if credential != nil {
			invocation.ClientIDN = credential.ClientIDN
			invocation.CredentialIDN = credential.IDN
			invocation.AccessKeyID = credential.AccessKeyID
			invocation.AuthMode = credential.AuthMode
		}
		if code != 0 {
			m.deny(c, settings, invocation, started, code, detail, nil)
			return
		}

		// 授权范围
		if !credential.InScope(c.Request.Method, c.Request.URL.Path) {
			m.deny(c, settings, invocation, started, CodeScopeDeny, "接口不在授权范围内", nil)
			return
		}

		// 速率与配额
		allowed, rateErr := m.redis.AllowRate(c.Request.Context(), credential.ClientIDN, credential.RateLimitRPM, credential.QuotaDay)
		if rateErr != nil {
			m.logf("[openapi] 限流检查失败（放行）: %v", rateErr)
		}
		if !allowed {
			m.deny(c, settings, invocation, started, CodeRateLimit, "超出速率或配额", nil)
			return
		}

		c.Set(ContextCredentialKey, credential)
		m.recorder.Write(c.Request.Context(), m.finish(invocation, started, true, "", http.StatusOK, 0))
		c.Next()
	}
}

// authenticate 完成「凭证定位 → 解密 → 状态/时效 → IP → 时间窗 → nonce → 签名」。
//
// 返回 (credential, 0, "") 表示通过；返回 code != 0 表示失败（含兼容兜底命中时 credential.ClientIDN=0）。
func (m *Middleware) authenticate(c *gin.Context, settings Settings, req *http.Request, body []byte, invocation *Invocation) (*Credential, int, string) {
	authorization := strings.TrimSpace(req.Header.Get("Authorization"))

	// ---- 模式二：AK/SK 签名 ----
	if strings.HasPrefix(strings.ToUpper(authorization), strings.ToUpper(AuthSchemeSign)) {
		accessKeyID, signature, ok := ParseSignHeader(authorization)
		if !ok {
			return nil, CodeKeyInvalid, "Authorization 头格式非法（应为 FinvAK <ak>:<signature>）"
		}
		credential, err := m.source.Lookup(req.Context(), accessKeyID)
		if errors.Is(err, ErrCredentialNotFound) {
			return nil, CodeKeyInvalid, "AK 未登记"
		}
		if err != nil {
			return nil, CodeAuthNotReady, "查询凭证失败: " + err.Error()
		}
		if credential.AuthMode != AuthModeAKSK {
			return nil, CodeKeyInvalid, "该凭证不是 AK/SK 模式"
		}

		key, err := settings.SecretKeyBytes()
		if err != nil {
			return credential, CodeAuthNotReady, err.Error()
		}
		secret, err := DecryptSecret(credential.SecretCipher, key)
		if err != nil {
			return credential, CodeAuthNotReady, "解密 SK 失败: " + err.Error()
		}
		previous := ""
		if secret == "" || credential.SecretCipherP != "" {
			if prev, prevErr := DecryptSecret(credential.SecretCipherP, key); prevErr == nil {
				previous = prev
			}
		}

		if ok, reason := credential.Active(m.now()); !ok {
			return credential, CodeCredentialStop, "凭证不可用（" + reason + "）"
		}
		if !credential.AllowIP(invocation.IP) {
			return credential, CodeCIDRDeny, "来源 IP 不在白名单"
		}

		timestamp := strings.TrimSpace(req.Header.Get(HeaderTimestamp))
		nonce := strings.TrimSpace(req.Header.Get(HeaderNonce))
		ts, err := ParseTimestamp(timestamp)
		if err != nil || nonce == "" {
			return credential, CodeKeyInvalid, "缺少 " + HeaderTimestamp + " 或 " + HeaderNonce
		}
		skew := m.now().Sub(time.Unix(ts, 0))
		if skew < 0 {
			skew = -skew
		}
		if skew > settings.SignTimeWindow {
			return credential, CodeTimestampSkew, "时间戳偏差 " + skew.String()
		}

		fresh, err := m.redis.AcquireNonce(req.Context(), credential.IDN, nonce, settings.NonceKeepWindow)
		if err != nil {
			m.logf("[openapi] nonce 检查失败（放行）: %v", err)
		} else if !fresh {
			return credential, CodeNonceReplay, "nonce 重复"
		}

		canonical := CanonicalString(SignRequest{
			Method:    req.Method,
			Path:      req.URL.Path,
			RawQuery:  req.URL.RawQuery,
			BodyHash:  BodyHash(body),
			Timestamp: timestamp,
			Nonce:     nonce,
		})
		if !VerifySignature(credential.SignAlgorithm, secret, canonical, signature) &&
			!(previous != "" && VerifySignature(credential.SignAlgorithm, previous, canonical, signature)) {
			return credential, CodeSignMismatch, "签名不匹配"
		}
		return credential, 0, ""
	}

	// ---- 模式一：静态 API Key ----
	provided := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if provided == "" {
		provided = bearerToken(authorization)
	}
	if provided == "" {
		return nil, CodeKeyMissing, "未携带 X-API-Key 或 Authorization"
	}

	credential, err := m.source.Lookup(req.Context(), provided)
	switch {
	case err == nil && credential.AuthMode == AuthModeAPIKey:
		if !MatchStaticKey(credential.SecretHash, provided) {
			return credential, CodeKeyInvalid, "静态 Key 哈希不匹配"
		}
		if ok, reason := credential.Active(m.now()); !ok {
			return credential, CodeCredentialStop, "凭证不可用（" + reason + "）"
		}
		if !credential.AllowIP(invocation.IP) {
			return credential, CodeCIDRDeny, "来源 IP 不在白名单"
		}
		return credential, 0, ""
	case err == nil:
		// 命中了 AK/SK 等其它模式的凭证，但客户端按静态 Key 方式使用
		return credential, CodeKeyInvalid, "该凭证不是静态 API Key 模式"
	case errors.Is(err, ErrCredentialNotFound):
		// 兼容期兜底：环境变量 FINV_OPENAPI_KEY 与 sys_config 19006
		if m.matchFallback(settings, provided) {
			return &Credential{AuthMode: "FALLBACK", AccessKeyID: maskKey(provided)}, 0, ""
		}
		return nil, CodeKeyInvalid, "静态 Key 未登记"
	default:
		return nil, CodeAuthNotReady, "查询凭证失败: " + err.Error()
	}
}

// matchFallback 兼容期兜底密钥比对（恒定时间，任一命中即可）。
func (m *Middleware) matchFallback(settings Settings, provided string) bool {
	matched := false
	for _, candidate := range []string{m.envKey, settings.FallbackKey} {
		if candidate == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(provided)) == 1 {
			matched = true
		}
	}
	return matched
}

// deny 处理鉴权失败：模式 2（宽松）只记审计并放行，模式 1 直接拒绝。
func (m *Middleware) deny(c *gin.Context, settings Settings, invocation Invocation, started time.Time, code int, detail string, cause error) {
	status := statusOf(code)
	message := messageOf(code)
	reason := reasonOf(code)
	if code == CodeCredentialStop {
		if strings.Contains(detail, ReasonRevoke) {
			reason = ReasonRevoke
		} else {
			reason = ReasonExpire
		}
	}

	m.recorder.Write(c.Request.Context(), m.finish(invocation, started, false, reason, status, code))

	if !settings.Enforce() {
		// 宽松模式：照常放行，只留下审计与告警日志。
		m.logf("[openapi][permissive] %s %s 鉴权失败但放行: code=%d reason=%s detail=%s",
			c.Request.Method, c.Request.URL.Path, code, reason, detail)
		c.Next()
		return
	}

	if cause != nil {
		m.logf("[openapi] %s %s 鉴权失败: code=%d reason=%s detail=%s err=%v",
			c.Request.Method, c.Request.URL.Path, code, reason, detail, cause)
	} else {
		m.logf("[openapi] %s %s 鉴权失败: code=%d reason=%s detail=%s",
			c.Request.Method, c.Request.URL.Path, code, reason, detail)
	}
	c.AbortWithStatusJSON(status, gin.H{
		"code":       code,
		"message":    message,
		"request_id": c.GetString(ContextRequestIDKey),
	})
}

// finish 补齐审计要素（状态码/耗时/结果）。
func (m *Middleware) finish(invocation Invocation, started time.Time, pass bool, reason string, status, bizCode int) Invocation {
	invocation.Pass = pass
	invocation.ReasonCode = reason
	invocation.StatusCode = status
	invocation.BizCode = bizCode
	invocation.Latency = m.now().Sub(started)
	return invocation
}

// readBody 读取并复位请求体（签名要覆盖 body，handler 还要继续用）。
func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	if len(body) > maxBodyBytes {
		return nil, errors.New("请求体超过 64 MiB 上限")
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// bearerToken 解析 `Authorization: Bearer <token>`。
func bearerToken(header string) string {
	const scheme = "bearer "
	header = strings.TrimSpace(header)
	if len(header) <= len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return ""
	}
	return strings.TrimSpace(header[len(scheme):])
}

// maskKey 脱敏展示密钥（审计/AK 字段只留前 8 后 4）。
func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "***" + key[len(key)-4:]
}

// CredentialFrom 从上下文取凭证（handler 可用）。
func CredentialFrom(c *gin.Context) (*Credential, bool) {
	value, exists := c.Get(ContextCredentialKey)
	if !exists {
		return nil, false
	}
	credential, ok := value.(*Credential)
	return credential, ok
}
