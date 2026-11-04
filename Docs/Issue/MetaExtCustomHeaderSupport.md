# MetaExt `quoteapiv2` 支持自定义请求头（Issue 草案）

> 状态：待评审（待上游 MetaExt 采纳）
> 依据数据：2026-11-04 采集结果与 `cdeu.finv_cdeu_secu_realtime_stock` 3950 行统计；MetaExt v0.1.0/v0.1.1 源码核对
> 关联需求编号：待上游采纳并在 FinvQuant 落地后登记（当前用 10989 的规避实现顶替）
> 用途：提交给上游仓库 `github.com/acnxo/MetaExt` 的 Issue 正文（可直接复制粘贴），落地后回填结论
> 提出方：FinvQuant（`github.com/ACANX/FinvQuant`） · 目标版本：MetaExt v0.1.1（当前依赖版本）
> 类型：功能增强（向后兼容） · 优先级：中高（影响行情采集正确性）

---

## Issue 标题

`quoteapiv2: 允许通过 Config 指定自定义请求头（accept-language 等），零值保持向后兼容`

---

## 1. 问题

`quoteapiv2` 目前**没有任何途径**让调用方指定任意请求头。`Config` 与 header 相关的只有两个专用字段：

```go
CSRF   string // → 请求头 futu-x-csrf-token + Cookie csrfToken
Locale string // → 仅写进 Cookie locale，不是 HTTP 头
```

而请求头在 `client.go` 的 `do()` 里是硬编码的，其中**没有 `accept-language`**：

```go
req.Header.Set("quote-token", token)
req.Header.Set("futu-x-csrf-token", c.cfg.CSRF)
req.Header.Set("cookie", fmt.Sprintf("csrfToken=%s; locale=%s", c.cfg.CSRF, c.cfg.Locale))
req.Header.Set("accept", "application/json")
req.Header.Set("referer", DefaultReferer)
req.Header.Set("user-agent", DefaultUserAgent)
```

**后果**：moomoo 会按「出口 IP + 请求语言」决定返回口径，同一个接口在两种情况下返回的**名称语言、数值单位、甚至数据集合本身**都不一样。

中英文口径对比（`/quote-api/quote-v2/get-stock-list`，同一 `marketType=1&plateType=1&rankType=3&page=1`）：

| 口径 | 名称 | 总市值 | 单位后缀 |
|------|------|--------|----------|
| 中文 | `腾讯控股` | `4.07万亿` | `万亿` / `亿` / `万` |
| 英文 | `TENCENT` | `4.07T` | `T`(1e12) / `B`(1e9) / `M`(1e6) / `K`(1e3) |

调用方（FinvQuant）在海外 IP 部署时，因为没有 `accept-language`，拿到的是英文口径：

- 名称全英文（`TENCENT`、`XIAOMI-W`），按中文关键字检索不到；
- 数值带 `T` 后缀，只按中文单位解析时会导致**万亿级市值整批解析失败**；
- 观测到返回集合也随语言/地区漂移（`pagination.total` 在两次运行中分别为 `3337` 与 `200`）。

这类问题在调用方**无法从外部补救**——只能改请求头。

## 2. 现有规避方案及其缺陷

目前唯一的办法是注入 `Config.HTTPClient`，包一层 `RoundTripper` 补头：

```go
func stockListHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: stockListHeaderTransport{base: http.DefaultTransport},
	}
}

type stockListHeaderTransport struct{ base http.RoundTripper }

func (t stockListHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("accept-language") == "" {
		req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	}
	return t.base.RoundTrip(req)
}
```

它能跑通，但有四个明显缺陷：

1. **与 `Config.Timeout` 冲突**：一旦传入 `HTTPClient`，`New()` 不再用 `cfg.Timeout` 构造客户端（`client.go` 中的 `if client == nil { client = &http.Client{Timeout: cfg.Timeout} }`），调用方被迫自己维护超时，两处超时语义容易分叉；
2. **无法修改/删除内置头**：只能"补空位"，改不了 `referer`、`user-agent` 等已硬编码的值，实测中 `referer` 也需要按市场切换；
3. **测试耦合**：单测本来只想注入 `httptest` 客户端，现在必须同时承担 header 注入职责，两个关注点混在一个 `*http.Client` 里；
4. **每个包各自实现一遍**：这是通用诉求，调用方越多重复代码越多。

## 3. 提案（首选）

在 `Config` 中新增：

```go
// Header 额外请求头；在内置默认头写入之后再逐项 Set，同名覆盖内置值。
//
// 典型用途：
//   - accept-language：决定服务端返回的语言与数值单位口径（zh-CN → 万亿/亿/万，en → T/B/M/K）；
//   - referer：部分接口按市场要求不同来源页。
//
// 安全提示：cookie / quote-token / futu-x-csrf-token 仍建议使用 CSRF、Locale 等专用字段，
// 本字段仅作为兜底扩展点。
Header map[string]string
```

语义定义（请在实现时明确）：

| 项 | 约定 |
|----|------|
| 应用时机 | 内置默认头全部 `Set` 之后、`c.http.Do(req)` 之前 |
| 覆盖规则 | 同名即覆盖（调用方显式值优先） |
| 零值 | `nil` / 空 map → 行为与现在完全一致（向后兼容） |
| 键为空白 | 跳过，不 panic |
| 并发 | `New()` 时对 map 做浅拷贝，避免调用方运行期修改导致 data race |
| 大小写 | 交给 `http.Header.Set` 规范化，无需自查 |

参考实现（`client.go`）：

```go
// withDefaults 之后保持不变

func (c *Client) do(ctx context.Context, method, path string, query []Param, body []byte, token string) ([]byte, error) {
	// ...构造 req、设置内置头（现有代码不动）...

	// 新增：自定义头最后应用，显式值优先
	for key, value := range c.cfg.Header {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := c.http.Do(req)
	// ...
}
```

`New()` 中浅拷贝：

```go
if len(cfg.Header) > 0 {
	header := make(map[string]string, len(cfg.Header))
	for k, v := range cfg.Header {
		header[k] = v
	}
	cfg.Header = header
}
```

## 4. 备选方案（若不愿引入 map）

只加一个专用字段，能解决本 Issue 的原始诉求，扩展性差但改动最小：

```go
// AcceptLanguage 请求头 accept-language；空取默认（当前服务端行为）。
AcceptLanguage string
```

实现方式等同：内置头之后 `req.Header.Set("accept-language", c.cfg.AcceptLanguage)`（非空时）。

两者可先做备选、后续再演进到 `Header`。

## 5. 影响面

改动集中在 `Client.do()` 与 `Config`，对以下所有方法一次性生效（无需逐个改）：

`GetKLine` · `GetStockList` · `GetEtfList` · `GetFuturesList` · `GetOptionQuote` · `GetNewsList` · `GetQuoteMinute` · `GetHistoryCashTrend`

## 6. 测试建议

```go
func TestConfigHeaderApplied(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("accept-language")
		_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL: srv.URL,
		Header:  map[string]string{"accept-language": "zh-CN,zh;q=0.9"},
	})
	_, _ = c.GetStockList(context.Background(), GetStockListRequest{MarketType: "1"})
	if got != "zh-CN,zh;q=0.9" {
		t.Fatalf("accept-language = %q", got)
	}
}

func TestConfigHeaderOverridesBuiltin(t *testing.T) {
	// referer 被显式覆盖
}

func TestConfigHeaderNilKeepsDefaults(t *testing.T) {
	// 不传 Header 时，referer/user-agent/accept 与当前完全一致
}
```

## 7. 验收标准

1. `Config{Header: map[string]string{"accept-language": "zh-CN"}}` 时，实际请求携带该头；
2. 与内置头同名时，`Config.Header` 的值生效；
3. 不传 `Header`（零值）时，请求头与 v0.1.1 完全一致（可用快照测试固化）；
4. `New()` 后修改调用方持有的原 map，不影响已创建 Client（浅拷贝验证）；
5. 并发调用 `do()` 无 data race（`go test -race`）。

## 8. 目标与非目标

**目标**

- 让调用方在上游包内**原生**指定任意请求头（至少 `accept-language`），不必再包 `RoundTripper`；
- 保持零值配置下行为与 v0.1.1 完全一致。

**非目标**

- 不改任何接口路径、参数、响应结构；
- 不引入 Cookie/会话管理、不处理登录态；
- 不在本提案范围内调整签名算法（`quote-token`）逻辑。

## 9. 待确认决策点

| 编号 | 问题 | 选项 | 建议 |
|------|------|------|------|
| D1 | 方案形态 | A. `Config.Header map[string]string`（通用）<br>B. `Config.AcceptLanguage string`（仅解决本诉求） | **A**，B 可作为过渡 |
| D2 | 与内置头同名时谁优先 | A. `Config.Header` 覆盖内置<br>B. 内置优先、`Header` 只补空缺 | **A**（`referer` 也需按市场覆盖） |
| D3 | 是否允许覆盖 `cookie` / `quote-token` / `futu-x-csrf-token` | A. 允许（灵活，风险自负）<br>B. 禁止（文档保留专用字段） | **A** + 文档提示，不在代码里拦截 |
| D4 | `New()` 是否浅拷贝 `Header` | A. 拷贝（防运行期改 map 引发 race）<br>B. 不拷贝 | **A** |

## 10. 风险与红线

- 本提案**不涉及**任何数据库结构变更、迁移或数据删除，不触发 `Docs/DevSpec/DbSchemaSpec.md` §0 红线。
- 若 D3 选择"允许覆盖关键头"，调用方写错 `cookie` 可能导致接口静默返回异常数据；需在 `Config.Header` 注释中显式警示。
- 上游若改变默认 `accept-language` 行为（例如新增默认值），可能影响存量调用方；本提案要求**未设置 `Header` 时不得改变现有请求头**。

## 11. 附：FinvQuant 侧的临时规避（待上游支持后移除）

FinvQuant 当前用 `Config.HTTPClient` + `RoundTripper` 注入 `accept-language: zh-CN,zh;q=0.9,en;q=0.8`，
代码位于 `internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go`（`stockListHTTPClient` /
`stockListHeaderTransport`）。上游支持 `Config.Header` 后，这部分会被替换为一行配置：

```go
quoteapiv2.New(quoteapiv2.Config{
	Header: map[string]string{"accept-language": "zh-CN,zh;q=0.9,en;q=0.8"},
})
```
