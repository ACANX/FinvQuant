package secukey

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	syscfg "github.com/acanx/finvquant/internal/meta/finv/sys"
)

// TestNormalizeStatus 校验 status 归一化：合法值原样保留，空值与未知值落 PENDING。
func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		raw       string
		want      string
		wantReset bool
	}{
		{"PENDING", StatusPending, false},
		{"queued", StatusQueued, false},
		{" Done ", StatusDone, false},
		{"FAILED", StatusFailed, false},
		{"SKIP", StatusSkip, false},
		{"", StatusPending, true},
		{"BOGUS", StatusPending, true},
	}
	for _, c := range cases {
		got, reset := NormalizeStatus(c.raw)
		if got != c.want || reset != c.wantReset {
			t.Fatalf("NormalizeStatus(%q) = (%q, %v), want (%q, %v)", c.raw, got, reset, c.want, c.wantReset)
		}
	}
}

// TestNormalizePriorityAndFlagEnable 校验 priority 收敛与 flag_enable 映射。
func TestNormalizePriorityAndFlagEnable(t *testing.T) {
	zero, neg, big, four := 0, -5, 2000000, 4
	if NormalizeFlagEnable(nil) != 1 || NormalizeFlagEnable(&zero) != 0 || NormalizeFlagEnable(&four) != 1 {
		t.Fatalf("NormalizeFlagEnable 映射错误")
	}
	if NormalizePriority(nil) != 10 {
		t.Fatalf("priority NULL 应为 10")
	}
	if NormalizePriority(&neg) != 1 || NormalizePriority(&big) != 1000000 || NormalizePriority(&four) != 4 {
		t.Fatalf("NormalizePriority 收敛错误")
	}
}

// TestNormalizeBatchSize 校验批量收敛：<=0 取默认，越界夹到 100~5000。
func TestNormalizeBatchSize(t *testing.T) {
	cases := map[int]int{0: 1000, -1: 1000, 50: 100, 1000: 1000, 9999: 5000}
	for in, want := range cases {
		if got := NormalizeBatchSize(in); got != want {
			t.Fatalf("NormalizeBatchSize(%d) = %d, want %d", in, got, want)
		}
	}
}

// TestPrepareRows 校验一批内的归一化、跳过与同键去重。
func TestPrepareRows(t *testing.T) {
	status := "bogus"
	url := "  "
	rows := []SourceRow{
		{FtmmSymbol: "US.AAPL", Market: "US", Code: "AAPL", Instrument: "stock", Status: &status, URL: &url},
		{FtmmSymbol: "US.AAPL", Market: "US", Code: "AAPL", Instrument: "stock"}, // 同批同键：后者覆盖前者
		{FtmmSymbol: "  ", Market: "US", Code: "X", Instrument: "stock"},         // 跳过：主键为空
		{FtmmSymbol: "HK.00700", Market: "HK", Code: "00700", Instrument: "stock"},
	}
	prepared, skipped, statusReset := prepareRows(rows)
	if len(prepared) != 2 {
		t.Fatalf("prepared = %d 行, want 2（去重后）", len(prepared))
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
	// status_reset 按源行计：3 行（US.AAPL 两行 + HK.00700）的 status 为空/非法，均归一为 PENDING。
	if statusReset != 3 {
		t.Fatalf("status_reset = %d, want 3", statusReset)
	}
	if prepared[0].FtmmSymbol != "US.AAPL" || prepared[0].Status != StatusPending || prepared[0].URL != nil {
		t.Fatalf("去重后应保留最后一条且 URL 空白归一为 NULL: %+v", prepared[0])
	}
	if prepared[0].Priority != 10 || prepared[0].FlagEnable != 1 {
		t.Fatalf("NULL priority/enable 应归一为 10/1: %+v", prepared[0])
	}
	if prepared[1].FtmmSymbol != "HK.00700" {
		t.Fatalf("第二行应为 HK.00700: %+v", prepared[1])
	}
}

// TestParseContentRangeTotal 校验 Content-Range 总行数解析。
func TestParseContentRangeTotal(t *testing.T) {
	cases := map[string]int64{
		"0-999/73212": 73212,
		"*/0":         0,
		"0-0/1":       1,
		"":            0,
		"garbage":     0,
	}
	for in, want := range cases {
		if got := parseContentRangeTotal(in); got != want {
			t.Fatalf("parseContentRangeTotal(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestFetchBatch 用假 PostgREST 服务校验：请求头、分页参数与 Content-Range 解析。
func TestFetchBatch(t *testing.T) {
	var gotPath, gotAPIKey, gotAuth, gotPrefer, gotProfile string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotAPIKey = r.Header.Get("apikey")
		gotAuth = r.Header.Get("Authorization")
		gotPrefer = r.Header.Get("Prefer")
		gotProfile = r.Header.Get("Accept-Profile")
		w.Header().Set("Content-Range", "1000-1999/2500")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]SourceRow{
			{FtmmSymbol: "US.AAPL", Market: "US", Code: "AAPL", Instrument: "stock"},
		})
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "testproj", APIKey: "sb_publishable_TEST"})
	client.baseURL = server.URL // 指向假服务
	client.httpClient = server.Client()

	rows, total, err := client.FetchBatch(context.Background(), "finv_quote_secu_key", 1000, 1000)
	if err != nil {
		t.Fatalf("FetchBatch 失败: %v", err)
	}
	if total != 2500 {
		t.Fatalf("total = %d, want 2500", total)
	}
	if len(rows) != 1 || rows[0].FtmmSymbol != "US.AAPL" {
		t.Fatalf("rows = %+v", rows)
	}
	if !strings.Contains(gotPath, "/rest/v1/finv_quote_secu_key?") ||
		!strings.Contains(gotPath, "limit=1000") || !strings.Contains(gotPath, "offset=1000") {
		t.Fatalf("请求路径不符合预期: %s", gotPath)
	}
	if gotAPIKey != "sb_publishable_TEST" || gotAuth != "Bearer sb_publishable_TEST" || gotPrefer != "count=exact" {
		t.Fatalf("请求头不符合预期: apikey=%q auth=%q prefer=%q", gotAPIKey, gotAuth, gotPrefer)
	}
	// schema 走 Accept-Profile 头（放到路径里会被 PostgREST 当成 public.public.<table>）。
	if gotProfile != "public" {
		t.Fatalf("Accept-Profile = %q, want public", gotProfile)
	}
}

// TestFetchBatchNotFound 校验 404 直接失败（不重试）并给出可读提示。
func TestFetchBatchNotFound(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, `{"message":"relation does not exist"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "testproj", APIKey: "k"})
	client.baseURL = server.URL
	client.httpClient = server.Client()

	_, _, err := client.FetchBatch(context.Background(), "finv_quote_secu_key", 0, 100)
	if err == nil || !strings.Contains(err.Error(), "源表不存在或不可读") {
		t.Fatalf("期望 404 可读错误，实际: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("404 不应重试，实际请求 %d 次", attempts)
	}
}

// TestNormalizeSupabaseTableName 校验源表名校验规则（防止表名注入 URL）。
func TestNormalizeSupabaseTableName(t *testing.T) {
	ok := map[string]string{
		"":                        syscfg.SupabaseSecuKeyTableDefault,
		"finv_quote_secu_key":     "finv_quote_secu_key",
		"public.finv_quote_secu":  "public.finv_quote_secu",
		"  finv_quote_secu_key  ": "finv_quote_secu_key",
	}
	for in, want := range ok {
		got, err := syscfg.NormalizeSupabaseTableName(in)
		if err != nil || got != want {
			t.Fatalf("NormalizeSupabaseTableName(%q) = (%q, %v), want %q", in, got, err, want)
		}
	}
	bad := []string{"a.b.c", "1abc", "abc;drop", "abc def", "a..b", "abc?limit=1"}
	for _, in := range bad {
		if _, err := syscfg.NormalizeSupabaseTableName(in); err == nil {
			t.Fatalf("NormalizeSupabaseTableName(%q) 应报错", in)
		}
	}
}

// TestFetchByTextKeysEncodesSpecialKeys 校验 `in.(...)` 里含 `&` / `*` / `,` / 引号 / 空格
// 的键不会把 URL 打坏。
//
// 背景（实测事故）：`ftmm_symbol` 里真实存在 `AU.quote-etf-au-index-etfs-s&amp` 这类带 `&`
// 的键。未做百分号编码时 `&` 被当成参数分隔符，上游只收到被截断的过滤器，报
// `PGRST100 unexpected end of input expecting "," or ")"`，回推任务整批失败。
func TestFetchByTextKeysEncodesSpecialKeys(t *testing.T) {
	var queries []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "testproj", APIKey: "sb_publishable_TEST"})
	client.baseURL = server.URL
	client.httpClient = server.Client()

	keys := []string{
		"US.AAPL",
		"AU.quote-etf-au-index-etfs-s&amp",
		"US.ASAI*",
		"US.SBT,OLD",
		`US.ODD"Q`,
		"US.HAS SPACE",
	}
	if _, err := client.FetchByTextKeys(context.Background(), "finv_quote_secu_key", "ftmm_symbol,usc", "ftmm_symbol", keys); err != nil {
		t.Fatalf("FetchByTextKeys 失败: %v", err)
	}
	if len(queries) != 1 {
		t.Fatalf("期望 1 次请求，实际 %d 次", len(queries))
	}
	q := queries[0]
	// `&` 一旦没编码，这里会多出参数（filter 被截断），长度必然大于 3。
	if len(q) != 3 {
		t.Fatalf("查询参数个数 = %d（%v），说明过滤值里的 & 未编码", len(q), q)
	}
	filter := q.Get("ftmm_symbol")
	// `*` 不是 PostgREST 语法字符（`in` 里按字面量），不做引号；`&` 必须编码，
	// 否则 URL 层会把它当参数分隔符 —— 这两个点正是本次事故的分界线。
	want := `in.(US.AAPL,"AU.quote-etf-au-index-etfs-s&amp",US.ASAI*,"US.SBT,OLD","US.ODD\"Q","US.HAS SPACE")`
	if filter != want {
		t.Fatalf("过滤器 = %q，期望 %q", filter, want)
	}
}

// TestFetchByTextKeysSplitsByFilterLen 校验超长键按字符预算分片，避免 URL 被网关截断。
func TestFetchByTextKeysSplitsByFilterLen(t *testing.T) {
	var filters []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters = append(filters, r.URL.Query().Get("ftmm_symbol"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewClient(syscfg.SupabaseConn{ProjectID: "testproj", APIKey: "sb_publishable_TEST"})
	client.baseURL = server.URL
	client.httpClient = server.Client()

	keys := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		keys = append(keys, fmt.Sprintf("AU.quote-etf-au-index-etfs-with-a-really-long-name-%03d", i))
	}
	if _, err := client.FetchByTextKeys(context.Background(), "finv_quote_secu_key", "ftmm_symbol,usc", "ftmm_symbol", keys); err != nil {
		t.Fatalf("FetchByTextKeys 失败: %v", err)
	}
	if len(filters) < 2 {
		t.Fatalf("超长键未分片，实际请求 %d 次", len(filters))
	}
	for i, f := range filters {
		if len(f) > maxFilterLen+120 { // 允许最后一片略微超出（至少放一个键）
			t.Fatalf("第 %d 片过滤器长度 %d 超过预算 %d", i+1, len(f), maxFilterLen)
		}
	}
}
