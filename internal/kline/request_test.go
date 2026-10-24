package kline

import (
	"strings"
	"testing"
)

func decodeItem(t *testing.T, raw string) ItemIn {
	t.Helper()
	var item ItemIn
	if err := DecodeJSON([]byte(raw), &item); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	return item
}

func TestPrevCloseCompat(t *testing.T) {
	cases := []struct {
		name string
		body string
		want *string
	}{
		{"open 优先", `{"usc":"A","ts":1,"open":10.1,"prev_close":9.9}`, strPtr("10.1")},
		{"open 为空时回填", `{"usc":"A","ts":1,"prev_close":9.9}`, strPtr("9.9")},
		{"open=null 时回填", `{"usc":"A","ts":1,"open":null,"prev_close":9.9}`, strPtr("9.9")},
		{"两者都为空", `{"usc":"A","ts":1}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := decodeItem(t, tc.body).ToRow("s")
			if err != nil {
				t.Fatalf("ToRow 失败: %v", err)
			}
			if tc.want == nil {
				if row.Open != nil {
					t.Fatalf("open 期望 nil，实际 %q", *row.Open)
				}
				return
			}
			if row.Open == nil || *row.Open != *tc.want {
				t.Fatalf("open 期望 %q，实际 %v", *tc.want, row.Open)
			}
		})
	}
}

func TestTSUnit(t *testing.T) {
	// 秒级正常通过。
	if row, err := decodeItem(t, `{"usc":"A","ts":1792373400}`).ToRow("s"); err != nil {
		t.Fatalf("秒级 ts 应通过: %v", err)
	} else if row.Ts != 1792373400 {
		t.Fatalf("ts 归一错误: %d", row.Ts)
	}
	// 13 位毫秒在秒级模式默认拒绝。
	if _, err := decodeItem(t, `{"usc":"A","ts":1792373400000}`).ToRow("s"); err == nil {
		t.Fatal("13 位 ts 应被拒绝")
	} else if err.Code != CodeOutOfRange {
		t.Fatalf("期望 %d，实际 %d", CodeOutOfRange, err.Code)
	}
	// 兼容开关下换算为秒。
	row, err := decodeItem(t, `{"usc":"A","ts":1792373400000}`).ToRow("ms")
	if err != nil {
		t.Fatalf("ms 模式应通过: %v", err)
	}
	if row.Ts != 1792373400 {
		t.Fatalf("ms 换算错误: %d", row.Ts)
	}
}

func TestValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
	}{
		{"缺少 usc", `{"ts":1}`, CodeMissingParam},
		{"缺少 ts", `{"usc":"A"}`, CodeMissingParam},
		{"time 越界", `{"usc":"A","ts":1,"time":240000}`, CodeOutOfRange},
		{"date 越界", `{"usc":"A","ts":1,"date":19000101}`, CodeOutOfRange},
		{"volume 为负", `{"usc":"A","ts":1,"volume":-1}`, CodeOutOfRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeItem(t, tc.body).ToRow("s")
			if err == nil {
				t.Fatal("期望校验失败")
			}
			if err.Code != tc.code {
				t.Fatalf("期望 code=%d，实际 %d", tc.code, err.Code)
			}
		})
	}
}

func TestDecodeUnknownField(t *testing.T) {
	var item ItemIn
	err := DecodeJSON([]byte(`{"usc":"A","ts":1,"unknown":1}`), &item)
	if err == nil {
		t.Fatal("未知字段应报错")
	}
	mapped := MapDecodeError(err)
	if mapped.Code != CodeUnknownField {
		t.Fatalf("期望 %d，实际 %d", CodeUnknownField, mapped.Code)
	}
	if got := mapped.DebugFields()["field"]; got != "unknown" {
		t.Fatalf("debug field 期望 unknown，实际 %q", got)
	}
}

func TestPatchExplicitNull(t *testing.T) {
	item := decodeItem(t, `{"usc":"A","ts":1,"open":null}`)
	patch, err := item.ToPatchItem("s")
	if err != nil {
		t.Fatalf("ToPatchItem 失败: %v", err)
	}
	if len(patch.Columns) != 1 || patch.Columns[0] != ColOpen {
		t.Fatalf("期望仅 open 列，实际 %v", patch.Columns)
	}
	if patch.Values[0] != nil {
		t.Fatalf("显式 null 应写 NULL，实际 %q", *patch.Values[0])
	}
}

func TestPatchOnlyKeys(t *testing.T) {
	item := decodeItem(t, `{"usc":"A","ts":1}`)
	patch, err := item.ToPatchItem("s")
	if err != nil {
		t.Fatalf("ToPatchItem 失败: %v", err)
	}
	if len(patch.Columns) != 0 {
		t.Fatalf("仅键条目应无更新列，实际 %v", patch.Columns)
	}
}

func TestPatchPrevClose(t *testing.T) {
	item := decodeItem(t, `{"usc":"A","ts":1,"prev_close":9.9}`)
	patch, err := item.ToPatchItem("s")
	if err != nil {
		t.Fatalf("ToPatchItem 失败: %v", err)
	}
	if len(patch.Columns) != 1 || patch.Columns[0] != ColOpen {
		t.Fatalf("prev_close 应转为 open，实际 %v", patch.Columns)
	}
	if patch.Values[0] == nil || *patch.Values[0] != "9.9" {
		t.Fatalf("open 值错误: %v", patch.Values[0])
	}
}

func TestDecodeJSONRejectsTrailing(t *testing.T) {
	var item ItemIn
	if err := DecodeJSON([]byte(`{"usc":"A","ts":1}{"x":1}`), &item); err == nil {
		t.Fatal("多余内容应报错")
	}
}

func TestQueryInDecode(t *testing.T) {
	var in QueryIn
	body := `{"keys":[{"usc":"A","ts":1}],"uscs":["B"],"provider":"FTMM","limit":10,"offset":5}`
	if err := DecodeJSON([]byte(body), &in); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(in.Keys) != 1 || in.Keys[0].Usc != "A" || in.Keys[0].Ts != 1 {
		t.Fatalf("keys 解析错误: %+v", in.Keys)
	}
	if *in.Limit != 10 || *in.Offset != 5 {
		t.Fatalf("limit/offset 解析错误")
	}
	if !strings.Contains(strings.Join(in.Uscs, ","), "B") {
		t.Fatalf("uscs 解析错误: %v", in.Uscs)
	}
}
