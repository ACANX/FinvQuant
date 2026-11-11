package openapi

// 鉴权中间件（需求 11168）：`/OpenAPI/V1` 路由组统一入口。

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/openapi/secret"
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
	// RedisFailClose Redis 故障时是否 fail-close（拒绝）：true=拒绝（FINV_OPENAPI_REDIS_FAIL_MODE=close），
	// false=放行并告警（open）。重放/限流依赖 Redis，故障时放行等于这两项失效。
	RedisFailClose bool
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
	// redisFailClose Redis 故障时是否拒绝请求（FINV_OPENAPI_REDIS_FAIL_MODE=close）。
	redisFailClose bool
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

		redisFailClose: opts.RedisFailClose,
		recorder:       recorder,
		envKey:         strings.TrimSpace(opts.FallbackKey),
		now:            now,
		logf:           logf,
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

		// 客户端可透传，但只接受 8~64 位十六进制并统一转大写；否则服务端生成（需求 11182）
		requestID := resolveRequestID(c.GetHeader(HeaderRequestID))
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
			// 静态 Key 的「公开标识」本身就是密钥：authenticate 里已写入脱敏形态，
			// 这里**只在该字段为空时**才用凭证的 access_key_id 兜底（AK/SK 模式是公开值，可以落明文）。
			if invocation.AccessKeyID == "" {
				invocation.AccessKeyID = credential.AccessKeyID
			}
			invocation.AuthMode = credential.AuthMode
		}
		if code != 0 {
			m.deny(c, settings, invocation, started, code, detail, nil)
			return
		}

		// 授权范围：scope_str 数据损坏时**拒绝**（不能当成「全部接口」放行，需求 11190）
		if credential.ScopeBroken {
			m.deny(c, settings, invocation, started, CodeScopeDeny,
				"凭据授权范围数据损坏（scope_str 不是合法对象数组），请联系平台方修复", nil)
			return
		}
		// 三类凭证互相隔离（需求 11210）：都在中间件拦下，保证审计记 pass=0，
		// 而不是等 handler 返回 403（那样审计里会留下一条「放行」记录）。
		//   - 兑换凭证（STS 且 token_fp 为空）：只能调 StsIssuePath 换临时凭据，不能直接调业务接口；
		//   - 长期 AK/SK（AKSK）：只能直接签名调业务接口，**不允许**派生临时凭据；
		//   - 临时凭据（STS 且 token_fp 非空）：只能调业务接口，不能再换证。
		if reason := isolationDenyReason(credential, c.Request.URL.Path); reason != "" {
			m.deny(c, settings, invocation, started, CodeScopeDeny, reason, nil)
			return
		}
		// 兑换凭证的 scope_str 描述的是「派生出的临时凭据」的权限，不用来限制它自己——
		// 它只能访问 StsIssuePath（上面已判定），若再按 scope 卡会与「授权范围给临时凭据用」的语义冲突。
		if !IsStsExchange(credential) && !credential.InScope(c.Request.Method, c.Request.URL.Path) {
			m.deny(c, settings, invocation, started, CodeScopeDeny, "接口不在授权范围内", nil)
			return
		}

		// 静态 Key 没有签名，写接口用查询参数 `_`（毫秒时间戳）做防重放挡板（需求 11188）：
		// 必须存在、为纯数字毫秒、且与服务器时间相差 ≤ 60 秒。放在授权范围校验之后，
		// 让「没有权限」优先于「时间戳不对」，避免越权探测被时间戳错误掩盖。
		if credential.AuthMode == AuthModeAPIKey && writeMethods[c.Request.Method] {
			if reason := checkStaticWriteTS(c.Request, m.now()); reason != "" {
				m.deny(c, settings, invocation, started, CodeTimestampSkew, reason, nil)
				return
			}
		}

		// 速率与配额
		allowed, rateErr := m.redis.AllowRate(c.Request.Context(), credential.ClientIDN, credential.RateLimitRPM, credential.QuotaDay)
		if rateErr != nil {
			m.logf("[openapi] 限流检查失败（fail-close=%t）: %v", m.redisFailClose, rateErr)
			if m.redisFailClose {
				m.deny(c, settings, invocation, started, CodeAuthNotReady, "限流状态不可用（Redis 故障），请稍后重试", rateErr)
				return
			}
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
	if strings.HasPrefix(strings.ToUpper(authorization), AuthSchemePrefix) {
		algorithm, accessKeyID, signature, ok := ParseSignHeader(authorization)
		if !ok {
			return nil, CodeKeyInvalid,
				"Authorization 头格式非法（应为 META1-<算法> Credential=<AK>, Signature=<签名>）"
		}
		// AK 是公开标识，即使后面定位失败也记进审计，便于排查「谁在用什么 AK 撞门」
		invocation.AccessKeyID = accessKeyID
		invocation.AuthMode = AuthModeAKSK
		// STS 临时凭据（需求 11191/11194）：SAK_LIVE_ 前缀走独立分支（Redis 取回、三要素校验）。
		// 与静态 Key 的 SA_LIVE_、长期 AK 的 AK_LIVE_ 前缀均不重叠，看前缀即可区分。
		if strings.HasPrefix(strings.ToUpper(accessKeyID), secret.PrefixTempAK) {
			return m.authenticateSTS(settings, req, body, invocation, algorithm, accessKeyID, signature)
		}
		credential, err := m.source.Lookup(req.Context(), accessKeyID)
		if errors.Is(err, ErrCredentialNotFound) {
			return nil, CodeKeyInvalid, "AK 未登记"
		}
		if err != nil {
			return nil, CodeAuthNotReady, "查询凭证失败: " + err.Error()
		}
		// AK/SK 验签分支同时容纳两类凭证（需求 11210）：长期 AK/SK 与 STS 兑换凭证（token_fp 为空）。
		// 两者的差别不在验签方式，而在**允许访问的路径**：兑换凭证只能调 StsIssuePath（见 isolationDenyReason）。
		if credential.AuthMode != AuthModeAKSK &&
			!(credential.AuthMode == AuthModeSTS && credential.TokenFP == "") {
			return nil, CodeKeyInvalid, "该凭证不是 AK/SK 模式"
		}
		// 请求里的算法必须与凭证登记的一致（防止调用方用别的算法绕过验签口径）
		if !strings.EqualFold(strings.TrimSpace(credential.SignAlgorithm), algorithm) {
			return credential, CodeSignMismatch,
				"签名算法不一致（请求 " + algorithm + "，凭证 " + credential.SignAlgorithm + "）"
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
		// nonce 形态校验（需求 11188）：8~64 位、仅字母/数字/_/-，避免超长或包含
		// 特殊字符的 nonce 撑大 Redis 键空间（重放键按凭证明细存储）。
		if !noncePattern.MatchString(nonce) {
			return credential, CodeKeyInvalid, "nonce 必须为 8~64 位字母/数字/_/-"
		}
		skew := m.now().Sub(time.Unix(ts, 0))
		if skew < 0 {
			skew = -skew
		}
		if skew > settings.SignTimeWindow {
			return credential, CodeTimestampSkew, "时间戳偏差 " + skew.String()
		}

		fresh, err := m.redis.AcquireNonce(req.Context(), strconv.FormatInt(credential.IDN, 10), nonce, settings.NonceKeepWindow)
		if err != nil {
			m.logf("[openapi] nonce 检查失败（fail-close=%t）: %v", m.redisFailClose, err)
			if m.redisFailClose {
				return credential, CodeAuthNotReady, "防重放状态不可用（Redis 故障），请稍后重试"
			}
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
	// 静态 Key 本身就是密钥：审计里只记脱敏形态，绝不落明文；
	// 同时写指纹（sha256 前 16 位）用于严格区分「哪一把 Key」（需求 11186）。
	invocation.AccessKeyID = maskKey(provided)
	invocation.KFP = FingerprintStaticKey(provided)
	invocation.AuthMode = AuthModeAPIKey

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
		// 静态 Key 的授权范围必须恰好 1 个接口（需求 11188）：签发时已强制，
		// 这里兜底存量凭证——空范围（放全部）或多接口都拒绝，需重新签发。
		if len(credential.Scope) != 1 {
			return credential, CodeScopeDeny,
				"静态 Key 必须且只能授权 1 个接口（当前 " + strconv.Itoa(len(credential.Scope)) + " 个），请重新签发"
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

// noncePattern nonce 形态：8~64 位字母/数字/_/-（需求 11188）。
var noncePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// writeMethods 需要校验「毫秒时间戳查询参数 `_`」的写方法（需求 11188，仅静态 Key 模式）。
var writeMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// staticKeyWriteWindow 静态 Key 写接口的时间戳允许偏差。
const staticKeyWriteWindow = time.Minute

// keyPrefixes 是平台密钥的固定前缀；掩码时必须跳过，否则「前 8 位」全是前缀、
// 只剩后 4 位可区分（SA_LIVE_ + 32 hex 场景下所有 Key 前 8 位完全相同）。
var keyPrefixes = []string{"SA_LIVE_", "AK_LIVE_", "SK_LIVE_", "SAK_LIVE_", "SSK_LIVE_", "SST_LIVE_", "FK_LIVE_"}

// maskKey 脱敏展示密钥，供审计用人眼核对（需求 11183、11184）。
//
// 形态：<前缀><随机段前 8><16 个 *><后 8>，如 SA_LIVE_C48AABCD****************EADE84AF。
// 星号个数等于被遮蔽的位数，掩码总长与原密钥一致。
// 跳过固定前缀后保留 16 位十六进制（16^16 量级），既能区分不同 Key、也能区分
// 同一凭证轮换前后的两把 Key；完整密钥不可反推，日志/备份导出不含明文。
// 随机段 ≤16 位时退化为「前 4 + 等量星号 + 后 4」，避免整串（前 8+后 8 即全长）外露。
func maskKey(key string) string {
	key = strings.TrimSpace(key)
	prefix := ""
	for _, candidate := range keyPrefixes {
		if strings.HasPrefix(strings.ToUpper(key), candidate) {
			prefix, key = key[:len(candidate)], key[len(candidate):]
			break
		}
	}
	// 星号个数 = 被遮蔽的字符数，掩码长度与原密钥一致（看得出原长、可核对位数）
	switch {
	case len(key) <= 8:
		return prefix + strings.Repeat("*", len(key))
	case len(key) <= 16:
		// 太短：前 8 + 后 8 会覆盖整串，退化为前 4 后 4
		return prefix + key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
	default:
		return prefix + key[:8] + strings.Repeat("*", len(key)-16) + key[len(key)-8:]
	}
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

// checkStaticWriteTS 校验静态 Key 写接口的 `_`（毫秒时间戳）参数（需求 11188）。
//
// 返回空串表示通过；否则返回拒绝原因。静态 Key 无法像 AK/SK 那样对请求签名，
// 用「调用方必须带当前时间附近的毫秒时间戳」抬高重放门槛（重放窗口 ≤ 60 秒）。
func checkStaticWriteTS(req *http.Request, now time.Time) string {
	raw := strings.TrimSpace(req.URL.Query().Get("_"))
	if raw == "" {
		return "写接口必须携带查询参数 `_`（毫秒时间戳）"
	}
	if len(raw) < 13 || len(raw) > 19 {
		return "查询参数 `_` 必须是毫秒时间戳（13 位数字）"
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return "查询参数 `_` 必须是毫秒时间戳（纯数字）"
		}
	}
	millis, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return "查询参数 `_` 不是合法的毫秒时间戳"
	}
	skew := now.Sub(time.UnixMilli(millis))
	if skew < 0 {
		skew = -skew
	}
	if skew > staticKeyWriteWindow {
		return "查询参数 `_` 与服务器时间相差超过 60 秒"
	}
	return ""
}

// IsStsExchange 是否为 STS 兑换凭证（`auth_mode='STS'` 且没有 SecurityToken：长期 AK/SK）。
//
// 兑换凭证只能调 StsIssuePath 换临时凭据；它自己的 scope_str 是派生出临时凭据的权限。
func IsStsExchange(c *Credential) bool {
	return c != nil && c.AuthMode == AuthModeSTS && c.TokenFP == ""
}

// isolationDenyReason 判定凭证类型与访问路径是否冲突（需求 11210）。
//
// 三类凭证的允许动作被刻意做成互斥，降低「一次泄露能直连业务」的风险：
//   - AKSK（长期 AK/SK）：直接签名调业务接口；调 StsIssuePath 拒绝（不允许派生 SST，防权限放大）；
//   - STS 兑换凭证（auth_mode=STS 且 token_fp 为空）：只能调 StsIssuePath；调业务接口拒绝；
//   - STS 临时凭据（auth_mode=STS 且 token_fp 非空）：只能调业务接口；再换证拒绝。
//
// 返回空串表示允许。
func isolationDenyReason(credential *Credential, path string) string {
	if credential == nil {
		return ""
	}
	atStsIssue := path == StsIssuePath
	switch {
	case IsStsExchange(credential):
		if !atStsIssue {
			return "STS 兑换凭证只能用于换取临时凭据（POST " + StsIssuePath + "），不能直接调用业务接口；请先用它换 SAK/SSK/SST"
		}
	case credential.AuthMode == AuthModeAKSK:
		if atStsIssue {
			return "AK/SK 长期凭证不能派生 STS 临时凭据；需要临时凭据请单独签发 STS 模式的凭证"
		}
	case credential.AuthMode == AuthModeSTS:
		if atStsIssue {
			return "STS 临时凭据不能再换取临时凭据，请用 STS 兑换凭证"
		}
	}
	return ""
}
