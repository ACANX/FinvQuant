package openapi

// 鉴权契约与行为测试（需求 11168）。全部用假凭证来源 + 假时钟，不连库、不连 Redis。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ---- 错误码契约 ----

// TestErrorCodesAreUniqueAndInRange 号段 + 唯一性（ErrorCodeSpec §1.2 的契约测试）。
func TestErrorCodesAreUniqueAndInRange(t *testing.T) {
	seen := map[int]bool{}
	for _, code := range AllCodes() {
		if code < CodeRange[0] || code > CodeRange[1] {
			t.Fatalf("错误码 %d 不在 190xx 子段内", code)
		}
		if seen[code] {
			t.Fatalf("错误码 %d 重复定义", code)
		}
		seen[code] = true
	}
	if len(seen) != 10 {
		t.Fatalf("错误码应为 10 个，实际 %d", len(seen))
	}
}

// TestErrorCodeHTTPMapping 码 → HTTP 状态映射固定（改映射等于改契约）。
func TestErrorCodeHTTPMapping(t *testing.T) {
	want := map[int]int{
		CodeKeyMissing:     http.StatusUnauthorized,
		CodeKeyInvalid:     http.StatusUnauthorized,
		CodeCredentialStop: http.StatusForbidden,
		CodeCIDRDeny:       http.StatusUnauthorized,
		CodeSignMismatch:   http.StatusUnauthorized,
		CodeTimestampSkew:  http.StatusUnauthorized,
		CodeNonceReplay:    http.StatusUnauthorized,
		CodeRateLimit:      http.StatusTooManyRequests,
		CodeScopeDeny:      http.StatusForbidden,
		CodeAuthNotReady:   http.StatusServiceUnavailable,
	}
	for code, status := range want {
		if got := statusOf(code); got != status {
			t.Fatalf("错误码 %d 的 HTTP 状态应为 %d，实际 %d", code, status, got)
		}
		if messageOf(code) == "" {
			t.Fatalf("错误码 %d 缺少安全文案", code)
		}
		if reasonOf(code) == "" {
			t.Fatalf("错误码 %d 缺少审计 reason_code", code)
		}
	}
}

// ---- 签名 ----

// TestCanonicalStringAndSignature 待签串拼装与验签（含查询串排序）。
func TestCanonicalStringAndSignature(t *testing.T) {
	req := SignRequest{
		Method:    "post",
		Path:      "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert",
		RawQuery:  "b=2&a=1&a=0&c=hello+world",
		BodyHash:  BodyHash([]byte(`{"rows":[]}`)),
		Timestamp: "1737000000",
		Nonce:     "nonce-1",
	}
	canonical := CanonicalString(req)
	want := "POST\n/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert\n" +
		"a=0&a=1&b=2&c=hello%20world\n" +
		req.BodyHash + "\n1737000000\nnonce-1"
	if canonical != want {
		t.Fatalf("待签串不符:\n实际=%q\n期望=%q", canonical, want)
	}

	signature, err := Sign("HMAC-SHA256", "sk-test", canonical)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	if !VerifySignature("HMAC-SHA256", "sk-test", canonical, signature) {
		t.Fatalf("正确签名应校验通过")
	}
	if VerifySignature("HMAC-SHA256", "sk-other", canonical, signature) {
		t.Fatalf("错误 SK 不应校验通过")
	}
	if VerifySignature("HMAC-SHA256", "sk-test", canonical+"x", signature) {
		t.Fatalf("待签串被改动后不应校验通过")
	}
	if _, err := Sign("HMAC-SHA512", "sk-test", canonical); err == nil {
		t.Fatalf("未实现的算法应明确报错，不能静默放行")
	}
}

// TestParseSignHeader Authorization 头解析（META1-<算法> Credential=…, Signature=…）。
func TestParseSignHeader(t *testing.T) {
	cases := []struct {
		header    string
		wantAlg   string
		wantAK    string
		wantSigOK bool
		wantOK    bool
	}{
		{"META1-HMAC-SHA256 Credential=AK_LIVE_1, Signature=c2lnbmF0dXJl", "HMAC-SHA256", "AK_LIVE_1", true, true},
		{"meta1-hmac-sha256 Credential=AK_LIVE_1, Signature=c2lnbmF0dXJl==", "HMAC-SHA256", "AK_LIVE_1", true, true},
		// 带引号 / 逗号后空格 / 参数顺序颠倒都要能解析
		{`META1-HMAC-SHA256 Signature="c2ln", Credential="AK_LIVE_1"`, "HMAC-SHA256", "AK_LIVE_1", true, true},
		// 缺 Signature / 缺 Credential / 缺算法 → 不合法
		{"META1-HMAC-SHA256 Credential=AK_LIVE_1", "", "", false, false},
		{"META1-HMAC-SHA256 Signature=c2ln", "", "", false, false},
		{"META1- Credential=AK_LIVE_1, Signature=c2ln", "", "", false, false},
		// 旧格式（FinvAK / Bearer）一律不再接受
		{"FinvAK AK_LIVE_1:c2lnbmF0dXJl", "", "", false, false},
		{"Bearer token", "", "", false, false},
	}
	for _, c := range cases {
		algorithm, ak, signature, ok := ParseSignHeader(c.header)
		if ok != c.wantOK {
			t.Fatalf("%q 解析结果应为 %t，实际 %t", c.header, c.wantOK, ok)
		}
		if !ok {
			continue
		}
		if algorithm != c.wantAlg || ak != c.wantAK {
			t.Fatalf("%q 解析为 alg=%q ak=%q，期望 alg=%q ak=%q", c.header, algorithm, ak, c.wantAlg, c.wantAK)
		}
		if (signature != "") != c.wantSigOK {
			t.Fatalf("%q 的签名解析异常: %q", c.header, signature)
		}
	}
}

// ---- 加解密 ----

// TestEncryptDecryptSecret SK 密文往返 + 错误密钥必须失败。
func TestEncryptDecryptSecret(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cipherText, err := EncryptSecret("sk-super-secret", key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if strings.Contains(cipherText, "sk-super-secret") {
		t.Fatalf("密文里不得出现明文")
	}

	plain, err := DecryptSecret(cipherText, key)
	if err != nil || plain != "sk-super-secret" {
		t.Fatalf("解密失败: plain=%q err=%v", plain, err)
	}

	// 同明文两次加密的密文必须不同（随机 nonce）
	again, err := EncryptSecret("sk-super-secret", key)
	if err != nil {
		t.Fatalf("二次加密失败: %v", err)
	}
	if again == cipherText {
		t.Fatalf("相同明文两次加密结果不应相同（nonce 未随机化）")
	}

	wrong := make([]byte, 32)
	if _, err := DecryptSecret(cipherText, wrong); err == nil {
		t.Fatalf("错误密钥解密应失败")
	}
	if plain, err := DecryptSecret("", key); err != nil || plain != "" {
		t.Fatalf("空密文应返回空串与 nil，实际 %q/%v", plain, err)
	}
}

// TestBodyHashAndRequestIDUppercase 请求体摘要与请求 ID 都是大写十六进制（需求 11177）。
func TestBodyHashAndRequestIDUppercase(t *testing.T) {
	empty := BodyHash(nil)
	// sha256("") 的十六进制（大写）
	if empty != "E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855" {
		t.Fatalf("空体摘要应为大写 sha256(\"\")，实际 %q", empty)
	}
	if got := BodyHash([]byte("abc")); got != strings.ToUpper(got) || len(got) != 64 {
		t.Fatalf("body_sha256 必须 64 位大写十六进制，实际 %q", got)
	}
	for i := 0; i < 5; i++ {
		id := newRequestID()
		if len(id) != 32 || id != strings.ToUpper(id) {
			t.Fatalf("request_id 必须 32 位大写十六进制，实际 %q", id)
		}
	}
}

// TestMatchStaticKey 静态 Key 哈希比对。
func TestMatchStaticKey(t *testing.T) {
	key := "SA_LIVE_ABCDEF123456"
	sum := sha256.Sum256([]byte(key))
	stored := hex.EncodeToString(sum[:])

	if !MatchStaticKey(stored, key) {
		t.Fatalf("相同 Key 应匹配")
	}
	if MatchStaticKey(stored, "SA_LIVE_OTHER") {
		t.Fatalf("不同 Key 不应匹配")
	}
	if MatchStaticKey("", key) {
		t.Fatalf("空哈希不应匹配")
	}
	if MatchStaticKey(strings.ToUpper(stored), key) != true {
		t.Fatalf("库里哈希若为大写也应匹配")
	}
}

// TestScope 授权范围匹配规则。
func TestScope(t *testing.T) {
	credential := &Credential{Scope: []ScopeRule{
		{Method: "POST", Path: "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert"},
		{Method: "GET", Path: "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/*"},
	}}

	if !credential.InScope("POST", "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert") {
		t.Fatalf("精确匹配应通过")
	}
	if credential.InScope("POST", "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni") {
		t.Fatalf("方法不匹配应拒绝（POST 只授权了 Upsert 一条精确路径）")
	}
	if !credential.InScope("GET", "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni") {
		t.Fatalf("通配前缀应通过")
	}
	if credential.InScope("GET", "/OpenAPI/V1/Other") {
		t.Fatalf("范围外应拒绝")
	}
	if !(&Credential{}).InScope("POST", "/any") {
		t.Fatalf("空范围表示全部接口")
	}
}

// ---- 中间件行为 ----

func newTestEngine(t *testing.T, opts Options, settings Settings) (*gin.Engine, *int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Unix(1737000000, 0) }
	}
	if opts.Settings == nil {
		opts.Settings = FixedSettings(settings)
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...any) {}
	}
	engine := gin.New()
	hits := 0
	engine.Use(NewMiddleware(opts).Handler())
	engine.POST("/OpenAPI/V1/Test", func(c *gin.Context) {
		hits++
		// 鉴权必须不破坏请求体，handler 还要能读
		body, _ := c.GetRawData()
		c.JSON(http.StatusOK, gin.H{"code": 0, "len": len(body)})
	})
	return engine, &hits
}

// fakeSource 假凭证来源。
type fakeSource struct {
	byKey map[string]*Credential
	err   error
}

func (f *fakeSource) Lookup(_ context.Context, accessKeyID string) (*Credential, error) {
	if f.err != nil {
		return nil, f.err
	}
	if credential, ok := f.byKey[accessKeyID]; ok {
		return credential, nil
	}
	return nil, ErrCredentialNotFound
}

// testNow 测试固定时钟（与 newTestEngine 注入的 Now 一致）。
var testNow = time.Unix(1737000000, 0)

// tsURL 给静态 Key 的写接口补上 `_`（毫秒时间戳，需求 11188）。
func tsURL(path string) string {
	return path + "?_=" + strconv.FormatInt(testNow.UnixMilli(), 10)
}

func staticCredential(key string) *Credential {
	return &Credential{
		IDN: 1, ClientIDN: 1, CredentialNo: "cred-1", AuthMode: AuthModeAPIKey,
		AccessKeyID: key, SecretHash: HashStaticKey(key), FlagEnable: true, clientEnable: true,
		// 静态 Key 必须恰好授权 1 个接口（需求 11188）
		Scope: []ScopeRule{{Method: "*", Path: "/OpenAPI/V1/Test"}},
	}
}

func akskCredential(t *testing.T, ak, sk string, key []byte) *Credential {
	t.Helper()
	cipherText, err := EncryptSecret(sk, key)
	if err != nil {
		t.Fatalf("加密 SK 失败: %v", err)
	}
	return &Credential{
		IDN: 2, ClientIDN: 1, CredentialNo: "cred-2", AuthMode: AuthModeAKSK,
		AccessKeyID: ak, SecretCipher: cipherText, SignAlgorithm: "HMAC-SHA256",
		FlagEnable: true, clientEnable: true,
	}
}

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i * 3)
	}
	return key
}

func enabledSettings(key []byte) Settings {
	settings := DefaultSettings
	settings.Mode = ModeOn
	settings.SecretKeyBase64 = encodeBase64(key)
	return settings
}

// TestMiddlewareModeOffPassThrough 模式 0：不鉴权，直接放行（兼容期行为）。
func TestMiddlewareModeOffPassThrough(t *testing.T) {
	engine, hits := newTestEngine(t, Options{Source: &fakeSource{}}, DefaultSettings)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x")))
	if recorder.Code != http.StatusOK || *hits != 1 {
		t.Fatalf("模式 0 应直接放行，实际 status=%d hits=%d", recorder.Code, *hits)
	}
}

// TestMiddlewareStaticKey 模式一：缺 Key / 错 Key / 正确 Key。
func TestMiddlewareStaticKey(t *testing.T) {
	settings := enabledSettings(testKey())
	source := &fakeSource{byKey: map[string]*Credential{"SA_LIVE_OK": staticCredential("SA_LIVE_OK")}}
	engine, hits := newTestEngine(t, Options{Source: source}, settings)

	// 缺 Key → 19001 / 401
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("缺 Key 应 401，实际 %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "19001") {
		t.Fatalf("缺 Key 应返回 19001，实际 %s", recorder.Body.String())
	}
	if *hits != 0 {
		t.Fatalf("鉴权失败不应进入 handler")
	}

	// 错 Key → 19002
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", "SA_LIVE_BAD")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "19002") {
		t.Fatalf("错 Key 应 401/19002，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	// 正确 Key → 放行，且请求体仍可读
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), strings.NewReader("payload"))
	request.Header.Set("X-API-Key", "SA_LIVE_OK")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || *hits != 1 {
		t.Fatalf("正确 Key 应放行，实际 %d hits=%d", recorder.Code, *hits)
	}
	if !strings.Contains(recorder.Body.String(), `"len":7`) {
		t.Fatalf("鉴权后 handler 应能读到完整请求体，实际 %s", recorder.Body.String())
	}
	if recorder.Header().Get(HeaderRequestID) == "" {
		t.Fatalf("响应应回显 X-Request-Id")
	}
}

// TestMiddlewareAKSK 模式二：正确签名通过，改一个字节即失败。
func TestMiddlewareAKSK(t *testing.T) {
	key := testKey()
	settings := enabledSettings(key)
	source := &fakeSource{byKey: map[string]*Credential{"AK_LIVE_1": akskCredential(t, "AK_LIVE_1", "sk-super-secret", key)}}
	now := time.Unix(1737000000, 0)
	engine, hits := newTestEngine(t, Options{Source: source, Now: func() time.Time { return now }}, settings)

	body := `{"rows":[{"usc":"US.AAPL"}]}`
	doRequest := func(alter func(*SignRequest, string) (string, string)) *httptest.ResponseRecorder {
		signReq := SignRequest{
			Method:    http.MethodPost,
			Path:      "/OpenAPI/V1/Test",
			RawQuery:  "b=2&a=1",
			BodyHash:  BodyHash([]byte(body)),
			Timestamp: "1737000000",
			Nonce:     "nonce-abc",
		}
		_, signature := alter(&signReq, "")
		if signature == "" {
			signature, _ = Sign("HMAC-SHA256", "sk-super-secret", CanonicalString(signReq))
		}
		signReq.Nonce = strings.TrimSpace(signReq.Nonce)
		request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test?b=2&a=1", strings.NewReader(body))
		request.Header.Set("Authorization", "META1-HMAC-SHA256 Credential=AK_LIVE_1, Signature="+signature)
		if signReq.Timestamp != "" {
			request.Header.Set(HeaderTimestamp, signReq.Timestamp)
		}
		if signReq.Nonce != "" {
			request.Header.Set(HeaderNonce, signReq.Nonce)
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	if recorder := doRequest(func(*SignRequest, string) (string, string) { return "", "" }); recorder.Code != http.StatusOK {
		t.Fatalf("正确签名应放行，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	if *hits != 1 {
		t.Fatalf("handler 应被调用一次，实际 %d", *hits)
	}

	// 换个 body（摘要不符）→ 19005
	recorder := httptest.NewRecorder()
	signReq := SignRequest{
		Method:    http.MethodPost,
		Path:      "/OpenAPI/V1/Test",
		RawQuery:  "b=2&a=1",
		BodyHash:  BodyHash([]byte(body)),
		Timestamp: "1737000000",
		Nonce:     "nonce-def",
	}
	signature, _ := Sign("HMAC-SHA256", "sk-super-secret", CanonicalString(signReq))
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test?b=2&a=1", strings.NewReader(body+"tampered"))
	request.Header.Set("Authorization", "META1-HMAC-SHA256 Credential=AK_LIVE_1, Signature="+signature)
	request.Header.Set(HeaderTimestamp, "1737000000")
	request.Header.Set(HeaderNonce, "nonce-def")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "19005") {
		t.Fatalf("篡改 body 应 401/19005，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	// 时间戳超窗 → 19006
	recorder = httptest.NewRecorder()
	stale := SignRequest{
		Method:    http.MethodPost,
		Path:      "/OpenAPI/V1/Test",
		RawQuery:  "b=2&a=1",
		BodyHash:  BodyHash([]byte(body)),
		Timestamp: "1736000000",
		Nonce:     "nonce-ghi",
	}
	signature, _ = Sign("HMAC-SHA256", "sk-super-secret", CanonicalString(stale))
	request = httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test?b=2&a=1", strings.NewReader(body))
	request.Header.Set("Authorization", "META1-HMAC-SHA256 Credential=AK_LIVE_1, Signature="+signature)
	request.Header.Set(HeaderTimestamp, "1736000000")
	request.Header.Set(HeaderNonce, "nonce-ghi")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "19006") {
		t.Fatalf("时间戳超窗应 401/19006，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestMiddlewareStateAndScope 停用/过期/白名单/授权范围。
func TestMiddlewareStateAndScope(t *testing.T) {
	settings := enabledSettings(testKey())

	disabled := staticCredential("SA_LIVE_OFF")
	disabled.FlagEnable = false
	engine, _ := newTestEngine(t, Options{Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_OFF": disabled}}}, settings)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), nil)
	request.Header.Set("X-API-Key", "SA_LIVE_OFF")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19003") {
		t.Fatalf("停用凭证应 403/19003，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	limited := staticCredential("SA_LIVE_SCOPE")
	limited.Scope = []ScopeRule{{Method: "GET", Path: "/Other"}}
	engine, _ = newTestEngine(t, Options{Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_SCOPE": limited}}}, settings)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), nil)
	request.Header.Set("X-API-Key", "SA_LIVE_SCOPE")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19009") {
		t.Fatalf("范围外应 403/19009，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestMiddlewarePermissive 模式 2：鉴权失败也放行，只留日志。
func TestMiddlewarePermissive(t *testing.T) {
	settings := enabledSettings(testKey())
	settings.Mode = ModePermissive
	var logs []string
	engine, hits := newTestEngine(t, Options{
		Source: &fakeSource{},
		Logf:   func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
	}, settings)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x")))
	if recorder.Code != http.StatusOK || *hits != 1 {
		t.Fatalf("宽松模式下鉴权失败应放行，实际 %d hits=%d", recorder.Code, *hits)
	}
	if len(logs) == 0 {
		t.Fatalf("宽松模式应留下告警日志")
	}
}

// TestMiddlewareFallbackKey 兼容期兜底静态密钥（环境变量 / sys_config 19006）。
func TestMiddlewareFallbackKey(t *testing.T) {
	settings := enabledSettings(testKey())
	settings.FallbackKey = "fk_config_fallback"
	engine, hits := newTestEngine(t, Options{Source: &fakeSource{}, FallbackKey: "fk_env_fallback"}, settings)

	for _, key := range []string{"fk_env_fallback", "fk_config_fallback"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x"))
		request.Header.Set("X-API-Key", key)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("兜底密钥 %s 应放行，实际 %d %s", key, recorder.Code, recorder.Body.String())
		}
	}
	if *hits != 2 {
		t.Fatalf("兜底密钥请求应进入 handler，实际 %d 次", *hits)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", "fk_not_fallback")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("非兜底密钥应拒绝，实际 %d", recorder.Code)
	}
}

// TestMiddlewareNotReadyWhenSecretKeyMissing 19005 未配置时 AK/SK 请求返回 19010。
func TestMiddlewareNotReadyWhenSecretKeyMissing(t *testing.T) {
	settings := enabledSettings(testKey())
	settings.SecretKeyBase64 = "" // 模拟密钥未配置
	source := &fakeSource{byKey: map[string]*Credential{"AK_LIVE_1": akskCredential(t, "AK_LIVE_1", "sk", testKey())}}
	engine, _ := newTestEngine(t, Options{Source: source}, settings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil)
	request.Header.Set("Authorization", "META1-HMAC-SHA256 Credential=AK_LIVE_1, Signature=c2ln")
	request.Header.Set(HeaderTimestamp, "1737000000")
	request.Header.Set(HeaderNonce, "n-1")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "19010") {
		t.Fatalf("密钥未配置应 503/19010，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestSettingsDefaultsAndParse 配置缺省与归一化。
func TestSettingsDefaultsAndParse(t *testing.T) {
	settings := DefaultSettings
	if settings.AuthEnabled() || settings.Enforce() {
		t.Fatalf("默认应为「关闭」模式")
	}
	settings.Mode = ModePermissive
	if !settings.AuthEnabled() || settings.Enforce() {
		t.Fatalf("模式 2 应 enabled 但不 enforce")
	}
	settings.Mode = ModeOn
	if !settings.Enforce() {
		t.Fatalf("模式 1 应 enforce")
	}
	if _, err := (Settings{}).SecretKeyBytes(); err == nil {
		t.Fatalf("未配置密钥应报错")
	}
	if _, err := (Settings{SecretKeyBase64: "not-base64!!"}).SecretKeyBytes(); err == nil {
		t.Fatalf("非法 Base64 应报错")
	}
	if _, err := (Settings{SecretKeyBase64: encodeBase64([]byte("short"))}).SecretKeyBytes(); err == nil {
		t.Fatalf("非 32 字节密钥应报错")
	}
	if seconds, ok := parseDurationSeconds("120"); !ok || seconds != 2*time.Minute {
		t.Fatalf("parseDurationSeconds(120) 应为 2 分钟")
	}
	if _, ok := parseDurationSeconds("abc"); ok {
		t.Fatalf("非法输入应返回 false")
	}
}

// TestAuditMasksStaticKey 审计里的静态 Key 必须是脱敏形态（它本身就是密钥，不能落明文）。
func TestAuditMasksStaticKey(t *testing.T) {
	key := "SA_LIVE_0123456789ABCDEF0123456789ABCDEF"
	credential := &Credential{
		IDN: 1, ClientIDN: 1, AuthMode: AuthModeAPIKey, AccessKeyID: key,
		SecretHash: HashStaticKey(key), FlagEnable: true, clientEnable: true,
		Scope: []ScopeRule{{Method: "*", Path: "/OpenAPI/V1/Test"}},
	}
	engine, _ := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{key: credential}},
	}, enabledSettings(testKey()))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), strings.NewReader("x"))
	req.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("静态 Key 应放行，实际 %d", recorder.Code)
	}
	// 复用中间件内部逻辑断言：脱敏后既不含完整密钥，也不是空
	masked := maskKey(key)
	if strings.Contains(masked, key) || masked == "" || !strings.Contains(masked, "*") {
		t.Fatalf("静态 Key 脱敏形态不符: %q", masked)
	}
	// 掩码总长与原密钥一致（前缀 + 前 8 + 16 个星号 + 后 8）
	if len(masked) != len(key) {
		t.Fatalf("掩码长度应等于原密钥（%d），实际 %d: %q", len(key), len(masked), masked)
	}
	// 必须跳过固定前缀，否则「前 8 位」全是 SA_LIVE_，只剩后 4 位可区分
	if !strings.HasPrefix(masked, "SA_LIVE_") {
		t.Fatalf("应保留平台前缀: %q", masked)
	}
	if masked == maskKey("SA_LIVE_0123456789ABCDEF0123456789ABCDEE") {
		t.Fatalf("不同 Key 的掩码必须不同: %q", masked)
	}
}

// TestMaskKeyDistinguishes 掩码必须能区分不同密钥（需求 11183、11184）：
// 「跳前缀 + 随机段前 8 + 后 8」= 16 位十六进制区分度，而不是只剩后 4 位。
func TestMaskKeyDistinguishes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SA_LIVE_0123456789ABCDEF0123456789ABCDEF", "SA_LIVE_01234567****************89ABCDEF"},
		{"AK_LIVE_7A01139C5571D46EDEFF2DB63C9F0E45", "AK_LIVE_7A01139C****************3C9F0E45"},
		// 随机段恰为 16 位：前 8+后 8 会覆盖整串，退化为前 4 后 4
		{"0123456789ABCDEF", "0123********CDEF"},
		// 过短直接全掩
		{"SA_LIVE_AB", "SA_LIVE_**"},
	}
	for _, c := range cases {
		if got := maskKey(c.in); got != c.want {
			t.Fatalf("maskKey(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	// 同一凭证轮换前后（仅末段不同）也必须区分得开
	a := maskKey("SA_LIVE_0123456789ABCDEF0123456789ABCDEF")
	b := maskKey("SA_LIVE_0123456789ABCDEF0123456789ABCDEE")
	if a == b {
		t.Fatalf("轮换前后的掩码不应相同: %q", a)
	}
}

// TestResolveRequestID 追踪 ID 透传规则（需求 11182）：
// 只接受 8~64 位十六进制并统一转大写，其它形态一律服务端重新生成。
func TestResolveRequestID(t *testing.T) {
	// 透传合法值：小写被归一为大写
	if got := resolveRequestID("abcdef0123456789"); got != "ABCDEF0123456789" {
		t.Fatalf("小写透传应转大写，实际 %q", got)
	}
	if got := resolveRequestID("  ABCDEF01  "); got != "ABCDEF01" {
		t.Fatalf("应去掉首尾空白，实际 %q", got)
	}
	// 非法值：全部落回服务端生成的 32 位大写十六进制
	for _, bad := range []string{"", "   ", "abc", "中文ID", "ABCD-1234",
		"ZZZZZZZZ", strings.Repeat("A", 65)} {
		got := resolveRequestID(bad)
		if len(got) != 32 || got != strings.ToUpper(got) {
			t.Fatalf("非法透传 %q 应回落到 32 位大写十六进制，实际 %q", bad, got)
		}
	}
	// 生成结果每次不同（不会退化成常量）
	if resolveRequestID("") == resolveRequestID("") {
		t.Fatalf("服务端生成的 ID 应每次不同")
	}
}

// TestMiddlewareEchoesRequestIDUppercase 响应头 X-Request-Id 的值恒为大写十六进制。
func TestMiddlewareEchoesRequestIDUppercase(t *testing.T) {
	// 用宽松模式：鉴权失败也继续，便于观察响应头
	settings := enabledSettings(testKey())
	settings.Mode = ModePermissive
	engine, _ := newTestEngine(t, Options{Source: &fakeSource{}}, settings)

	cases := []struct {
		incoming string
		want     string // 空表示「由服务端生成，只校验形态」
	}{
		{"abcdef0123456789", "ABCDEF0123456789"},
		{"ABCDEF0123456789", "ABCDEF0123456789"},
		{"bad-id-中文", ""},
		{"", ""},
	}
	for _, c := range cases {
		request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x"))
		if c.incoming != "" {
			request.Header.Set(HeaderRequestID, c.incoming)
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)

		got := recorder.Header().Get(HeaderRequestID)
		if c.want != "" && got != c.want {
			t.Fatalf("透传 %q 应回显 %q，实际 %q", c.incoming, c.want, got)
		}
		if len(got) != len(c.incoming) && c.want != "" {
			t.Fatalf("回显长度异常: %q", got)
		}
		if got != strings.ToUpper(got) || !requestIDPattern.MatchString(got) {
			t.Fatalf("响应头值必须是大写十六进制，实际 %q", got)
		}
	}
}

// TestFingerprintStaticKey 密钥指纹必须稳定、可区分、且不等于明文（需求 11186）。
func TestFingerprintStaticKey(t *testing.T) {
	a := "SA_LIVE_0123456789ABCDEF0123456789ABCDEF"
	b := "SA_LIVE_0123456789ABCDEF0123456789ABCDEE" // 只差最后一位

	fa, fb := FingerprintStaticKey(a), FingerprintStaticKey(b)
	if fa != FingerprintStaticKey(a) {
		t.Fatalf("同一密钥的指纹必须稳定: %q", fa)
	}
	if fa == fb {
		t.Fatalf("不同密钥的指纹必须不同: %q", fa)
	}
	if len(fa) != 16 || fa != strings.ToUpper(fa) {
		t.Fatalf("指纹应为 16 位大写十六进制，实际 %q", fa)
	}
	if strings.Contains(a, fa) || strings.Contains(fa, a[len(a)-8:]) {
		t.Fatalf("指纹不能由明文片段推导: %q", fa)
	}
	// 大小写不敏感：与存储哈希同口径（先归一为大写）
	if FingerprintStaticKey(strings.ToLower(a)) != fa {
		t.Fatalf("指纹应对大小写不敏感")
	}
}

// TestNoncePattern nonce 形态限制（需求 11188）：8~64 位字母/数字/_/-。
func TestNoncePattern(t *testing.T) {
	good := []string{"abcdef01", "ABCDEF01", "abc-123_XYZ", strings.Repeat("a", 64)}
	for _, v := range good {
		if !noncePattern.MatchString(v) {
			t.Fatalf("合法 nonce 被判非法: %q", v)
		}
	}
	bad := []string{"abc", strings.Repeat("a", 65), "abc def", "abc.def", "中文中文中文"}
	for _, v := range bad {
		if noncePattern.MatchString(v) {
			t.Fatalf("非法 nonce 被判合法: %q", v)
		}
	}
}

// TestCheckStaticWriteTS 静态 Key 写接口的 `_`（毫秒时间戳）校验（需求 11188）。
func TestCheckStaticWriteTS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	at := func(offset time.Duration) string {
		return strconv.FormatInt(now.Add(offset).UnixMilli(), 10)
	}
	cases := []struct {
		raw  string
		ok   bool
		word string
	}{
		{at(0), true, ""},
		{at(-59 * time.Second), true, ""},
		{at(59 * time.Second), true, ""},
		{"", false, "必须携带"},
		{at(-61 * time.Second), false, "超过 60 秒"},
		{at(61 * time.Second), false, "超过 60 秒"},
		{"17abc000000000", false, "纯数字"},
		{"12345", false, "13 位"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/x?_="+c.raw, nil)
		got := checkStaticWriteTS(req, now)
		if c.ok && got != "" {
			t.Fatalf("_=%q 应通过，实际被拒: %s", c.raw, got)
		}
		if !c.ok && !strings.Contains(got, c.word) {
			t.Fatalf("_=%q 期望拒绝含 %q，实际 %q", c.raw, c.word, got)
		}
	}
	// 无参数
	req := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/x", nil)
	if got := checkStaticWriteTS(req, now); !strings.Contains(got, "必须携带") {
		t.Fatalf("缺少 _ 应被拒，实际 %q", got)
	}
}

// TestMiddlewareStaticKeyWriteGuard 静态 Key 写接口的 `_` 时间戳与单接口约束（需求 11188）。
func TestMiddlewareStaticKeyWriteGuard(t *testing.T) {
	settings := enabledSettings(testKey())
	key := "SA_LIVE_OK"
	source := &fakeSource{byKey: map[string]*Credential{key: staticCredential(key)}}
	engine, hits := newTestEngine(t, Options{Source: source}, settings)

	// 缺 `_` → 19006
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x"))
	request.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "19006") {
		t.Fatalf("写接口缺 `_` 应 401/19006，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	// `_` 超窗（61 秒前）→ 19006
	stale := strconv.FormatInt(testNow.Add(-61*time.Second).UnixMilli(), 10)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test?_="+stale, strings.NewReader("x"))
	request.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "19006") {
		t.Fatalf("`_` 超窗应 401/19006，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	// 带合法 `_` → 放行
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), strings.NewReader("x"))
	request.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || *hits != 1 {
		t.Fatalf("合法 `_` 应放行，实际 %d hits=%d", recorder.Code, *hits)
	}

	// GET 不需要 `_`（读接口不受影响）
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusOK {
		// 测试路由只注册了 POST，GET 落到 404；关键是不能因为缺 `_` 被判 19006
		if strings.Contains(recorder.Body.String(), "19006") {
			t.Fatalf("GET 不应要求 `_`，实际 %s", recorder.Body.String())
		}
	}

	// 存量凭证 scope 为空（放全部）→ 19009 兜底拒绝
	loose := staticCredential("SA_LIVE_LOOSE")
	loose.Scope = nil
	engine2, _ := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_LOOSE": loose}},
	}, settings)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, tsURL("/OpenAPI/V1/Test"), strings.NewReader("x"))
	request.Header.Set("X-API-Key", "SA_LIVE_LOOSE")
	engine2.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19009") {
		t.Fatalf("空 scope 的静态 Key 应 403/19009，实际 %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestParseScopeBroken scope_str 损坏必须按「拒绝」处理，不能放大成「全部接口」（需求 11190）。
func TestParseScopeBroken(t *testing.T) {
	// 约定：空串 / null / 空数组 = 无限制（全部接口）
	for _, raw := range []string{"", "  ", "null", "[]"} {
		rules, broken := parseScope(raw)
		if broken || rules != nil {
			t.Fatalf("%q 应表示「全部接口」且不算损坏，实际 rules=%v broken=%t", raw, rules, broken)
		}
	}
	// 合法对象数组
	rules, broken := parseScope(`[{"method":"POST","path":"/OpenAPI/V1/a"}]`)
	if broken || len(rules) != 1 || rules[0].Path != "/OpenAPI/V1/a" {
		t.Fatalf("合法对象数组应解析成功，实际 %v broken=%t", rules, broken)
	}
	// 损坏：非法 JSON、字符串数组（本次事故形态）、缺 path
	for _, raw := range []string{
		`{"method":"POST"}`,
		`["POST /OpenAPI/V1/a"]`,
		`[{"method":"POST"}]`,
		`not-json`,
		`[1,2,3]`,
	} {
		if _, broken := parseScope(raw); !broken {
			t.Fatalf("%q 应判定为损坏", raw)
		}
	}
}

// TestMiddlewareScopeBrokenDenied scope_str 损坏的凭证调用时 403/19009。
func TestMiddlewareScopeBrokenDenied(t *testing.T) {
	settings := enabledSettings(testKey())
	broken := staticCredential("SA_LIVE_BROKEN")
	broken.ScopeBroken = true
	broken.Scope = nil // 即使 scope 为空，也不能因为「空=全部」而放行
	engine, hits := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_BROKEN": broken}},
	}, settings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", "SA_LIVE_BROKEN")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19009") {
		t.Fatalf("scope 损坏应 403/19009，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	if *hits != 0 {
		t.Fatalf("scope 损坏不应进入 handler")
	}
}

// signHeadersWith 生成 AK/SK 签名头（GET、无查询串），供测试复用。
func signHeadersWith(t *testing.T, sk, ak, method, path, query string) (map[string]string, *strings.Reader) {
	return signHeadersWithToken(t, sk, ak, method, path, query, "")
}

// signHeadersWithToken 同上，但带 STS 的 X-META-Token（参与签名第 7 段）。
func signHeadersWithToken(t *testing.T, sk, ak, method, path, query, token string) (map[string]string, *strings.Reader) {
	t.Helper()
	body := ""
	signReq := SignRequest{
		Method:    method,
		Path:      path,
		RawQuery:  query,
		BodyHash:  BodyHash([]byte(body)),
		Timestamp: strconv.FormatInt(testNow.Unix(), 10),
		Nonce:     "nonce-" + ak[len(ak)-4:],
		Token:     token,
	}
	signature, err := Sign("HMAC-SHA256", sk, CanonicalString(signReq))
	if err != nil {
		t.Fatalf("生成签名失败: %v", err)
	}
	headers := map[string]string{
		"Authorization": "META1-HMAC-SHA256 Credential=" + ak + ", Signature=" + signature,
		HeaderTimestamp: signReq.Timestamp,
		HeaderNonce:     signReq.Nonce,
	}
	if token != "" {
		headers[HeaderToken] = token
	}
	return headers, strings.NewReader(body)
}

// TestMiddlewareRevokedCredential 已吊销凭证必须被拒（19003）且审计记 REVOKE 而不是 EXPIRE。
// 两种模式都覆盖：静态 Key（APIKEY）与长期 AK/SK。
func TestMiddlewareRevokedCredential(t *testing.T) {
	key := testKey()
	settings := enabledSettings(key)

	// ---- 静态 Key：吊销 ----
	revokedStatic := staticCredential("SA_LIVE_REVOKED")
	revokedStatic.FlagRevoke = true
	revokedStatic.FlagEnable = false

	var logs []string
	engine, hits := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_REVOKED": revokedStatic}},
		Logf:   func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
	}, settings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", "SA_LIVE_REVOKED")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19003") {
		t.Fatalf("已吊销静态 Key 应 403/19003，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	if *hits != 0 {
		t.Fatalf("已吊销凭证不应进入 handler")
	}
	// 审计必须记 REVOKE（而不是笼统的 EXPIRE），否则事后追查分不清「吊销」与「过期」
	if len(logs) == 0 || !strings.Contains(logs[0], "reason=REVOKE") {
		t.Fatalf("审计应记 reason=REVOKE，实际 %v", logs)
	}

	// ---- 长期 AK/SK：吊销 ----
	revokedAK := akskCredential(t, "AK_LIVE_REVOKED", "sk-revoked-secret", key)
	revokedAK.FlagRevoke = true
	revokedAK.FlagEnable = false
	engine2, hits2 := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{"AK_LIVE_REVOKED": revokedAK}},
		Logf:   func(string, ...any) {},
	}, settings)

	headers, body := signHeadersWith(t, "sk-revoked-secret", "AK_LIVE_REVOKED", "GET", "/OpenAPI/V1/Test", "")
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/OpenAPI/V1/Test", body)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	engine2.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19003") {
		t.Fatalf("已吊销 AK/SK 应 403/19003，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	if *hits2 != 0 {
		t.Fatalf("已吊销 AK/SK 不应进入 handler")
	}
}

// TestStsParentUsable 父凭证吊销/停用/过期后，派生的临时凭据必须失效（需求 11197）。
func TestStsParentUsable(t *testing.T) {
	now := testNow

	// 传入的是**临时凭据**，父凭证状态由主查询 LEFT JOIN 带出的 Parent* 字段表示
	temp := func(mutate func(*Credential)) *Credential {
		credential := &Credential{
			AuthMode: AuthModeSTS, ParentIDN: 900, ParentFound: true, ParentFlagEnable: true,
		}
		if mutate != nil {
			mutate(credential)
		}
		return credential
	}

	if usable, _ := stsParentUsable(temp(nil), now); !usable {
		t.Fatalf("正常父凭证应可用")
	}
	if usable, reason := stsParentUsable(temp(func(c *Credential) { c.ParentFlagRevoke = true }), now); usable || reason != ReasonRevoke {
		t.Fatalf("吊销父凭证应不可用且 reason=REVOKE，实际 usable=%t reason=%q", usable, reason)
	}
	if usable, reason := stsParentUsable(temp(func(c *Credential) { c.ParentFlagEnable = false }), now); usable || reason != ReasonExpire {
		t.Fatalf("停用父凭证应不可用且 reason=EXPIRE，实际 usable=%t reason=%q", usable, reason)
	}
	past := now.Add(-time.Hour)
	if usable, reason := stsParentUsable(temp(func(c *Credential) { c.ParentExp = &past }), now); usable || reason != ReasonExpire {
		t.Fatalf("过期父凭证应不可用且 reason=EXPIRE，实际 usable=%t reason=%q", usable, reason)
	}
	// parent_idn 指向的行已被物理删除（清理/误删）→ ParentFound=false
	if usable, reason := stsParentUsable(temp(func(c *Credential) { c.ParentFound = false }), now); usable || reason != ReasonExpire {
		t.Fatalf("父凭证缺失应不可用且 reason=EXPIRE，实际 usable=%t reason=%q", usable, reason)
	}
	if usable, _ := stsParentUsable(nil, now); usable {
		t.Fatalf("父凭证不存在应不可用")
	}
}

// TestMiddlewareStsRejectsRevokedParent 父凭证被吊销后，已签发的临时凭据立即失效（需求 11197）。
func TestMiddlewareStsRejectsRevokedParent(t *testing.T) {
	key := testKey()
	settings := enabledSettings(key)

	parentRevoked := staticCredential("AK_LIVE_PARENT")
	parentRevoked.AuthMode = AuthModeAKSK
	parentRevoked.FlagRevoke = true
	parentRevoked.FlagEnable = false

	token := "SST_LIVE_ABCDEF0123456789ABCDEF0123456789"
	// STS 临时凭据是独立凭证行（需求 11202）：父凭证状态由主查询 LEFT JOIN 带出（需求 11211）
	stsCred := &Credential{
		IDN: 900, ClientIDN: 1, AuthMode: AuthModeSTS, ParentIDN: parentRevoked.IDN,
		ParentFound: true, ParentFlagEnable: false, ParentFlagRevoke: true,
		AccessKeyID: "SAK_LIVE_TEMP", SignAlgorithm: "HMAC-SHA256",
		TokenFP: TokenFingerprint(token), SecretCipher: "unused",
		FlagEnable: true, clientEnable: true,
		Scope: []ScopeRule{{Method: "*", Path: "/OpenAPI/V1/Test"}},
	}
	parentRevoked.IDN = 900 + 1
	engine, hits := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{
			"AK_LIVE_PARENT": parentRevoked,
			"SAK_LIVE_TEMP":  stsCred,
		}},
		Logf: func(string, ...any) {},
	}, settings)

	headers, body := signHeadersWithToken(t, "sk-temp", "SAK_LIVE_TEMP", "GET", "/OpenAPI/V1/Test", "", token)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/OpenAPI/V1/Test", body)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19003") {
		t.Fatalf("父凭证吊销后 STS 应 403/19003，实际 %d %s", recorder.Code, recorder.Body.String())
	}
	if *hits != 0 {
		t.Fatalf("父凭证吊销后 STS 不应进入 handler")
	}
}

// TestIsolationDenyReason 三类凭证 × 两条路径的隔离矩阵（需求 11210）。
func TestIsolationDenyReason(t *testing.T) {
	aksk := &Credential{IDN: 1, AuthMode: AuthModeAKSK}
	exchange := &Credential{IDN: 2, AuthMode: AuthModeSTS}
	temp := &Credential{IDN: 3, AuthMode: AuthModeSTS, TokenFP: "ABCDEF0123456789"}
	statics := &Credential{IDN: 4, AuthMode: AuthModeAPIKey}
	bizPath := "/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni"

	cases := []struct {
		name   string
		cred   *Credential
		path   string
		denied bool
	}{
		{"AKSK 调业务接口", aksk, bizPath, false},
		{"AKSK 换临时凭据", aksk, StsIssuePath, true},
		{"兑换凭证 换临时凭据", exchange, StsIssuePath, false},
		{"兑换凭证 调业务接口", exchange, bizPath, true},
		{"临时凭据 调业务接口", temp, bizPath, false},
		{"临时凭据 再换证", temp, StsIssuePath, true},
		{"静态 Key 调业务接口", statics, bizPath, false},
		{"静态 Key 换临时凭据", statics, StsIssuePath, false},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			reason := isolationDenyReason(item.cred, item.path)
			if got := reason != ""; got != item.denied {
				t.Fatalf("denied=%v 期望 %v（reason=%q）", got, item.denied, reason)
			}
		})
	}
}

// TestIsStsExchange 只有「STS 且无 SecurityToken」才是兑换凭证（需求 11210）。
func TestIsStsExchange(t *testing.T) {
	if !IsStsExchange(&Credential{AuthMode: AuthModeSTS}) {
		t.Fatal("STS 且 token_fp 为空应判定为兑换凭证")
	}
	if IsStsExchange(&Credential{AuthMode: AuthModeSTS, TokenFP: "X"}) {
		t.Fatal("STS 临时凭据不应判定为兑换凭证")
	}
	if IsStsExchange(&Credential{AuthMode: AuthModeAKSK}) {
		t.Fatal("AKSK 不应判定为兑换凭证")
	}
	if IsStsExchange(nil) {
		t.Fatal("nil 不应判定为兑换凭证")
	}
}
