package openapi

// 鉴权契约与行为测试（需求 11168）。全部用假凭证来源 + 假时钟，不连库、不连 Redis。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
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

func staticCredential(key string) *Credential {
	return &Credential{
		IDN: 1, ClientIDN: 1, CredentialNo: "cred-1", AuthMode: AuthModeAPIKey,
		AccessKeyID: key, SecretHash: HashStaticKey(key), FlagEnable: true, clientEnable: true,
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
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil))
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
	request = httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("payload"))
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
	request := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil)
	request.Header.Set("X-API-Key", "SA_LIVE_OFF")
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "19003") {
		t.Fatalf("停用凭证应 403/19003，实际 %d %s", recorder.Code, recorder.Body.String())
	}

	limited := staticCredential("SA_LIVE_SCOPE")
	limited.Scope = []ScopeRule{{Method: "GET", Path: "/Other"}}
	engine, _ = newTestEngine(t, Options{Source: &fakeSource{byKey: map[string]*Credential{"SA_LIVE_SCOPE": limited}}}, settings)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", nil)
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
		Logf:   func(format string, args ...any) { logs = append(logs, format) },
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
	}
	engine, _ := newTestEngine(t, Options{
		Source: &fakeSource{byKey: map[string]*Credential{key: credential}},
	}, enabledSettings(testKey()))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/OpenAPI/V1/Test", strings.NewReader("x"))
	req.Header.Set("X-API-Key", key)
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("静态 Key 应放行，实际 %d", recorder.Code)
	}
	// 复用中间件内部逻辑断言：脱敏后既不含完整密钥，也不是空
	masked := maskKey(key)
	if strings.Contains(masked, key) || masked == "" || !strings.Contains(masked, "***") {
		t.Fatalf("静态 Key 脱敏形态不符: %q", masked)
	}
	if len(masked) >= len(key) {
		t.Fatalf("脱敏后长度应小于原值: %q", masked)
	}
}
