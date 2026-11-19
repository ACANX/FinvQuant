package secuft

import (
	"encoding/json"
	"errors"
	"net/url"
	"testing"
)

// TestEditableColumns 可编辑列口径：排除 usc、时间列、采集统计列（需求 11346）。
func TestEditableColumns(t *testing.T) {
	editable := EditableColumns()
	set := make(map[string]bool, len(editable))
	for _, name := range editable {
		set[name] = true
	}
	// 32 列 - usc(1) - 时间列(5) - 采集统计非时间列(4) = 22 列
	if len(editable) != 22 {
		t.Errorf("可编辑列应为 22 列，实际 %d: %v", len(editable), editable)
	}
	for _, banned := range []string{"usc", "gmt_create", "gmt_update", "dt_update",
		"dt_last_check", "dt_last_fetch", "ts_latest_data", "count_last_fetch",
		"count_fail", "count_stale"} {
		if set[banned] {
			t.Errorf("%s 不应可编辑", banned)
		}
	}
	for _, allowed := range []string{"flag_enable", "remark", "symbol", "market", "provider",
		"weight_priority", "type_secu_sub"} {
		if !set[allowed] {
			t.Errorf("%s 应可编辑", allowed)
		}
	}
}

// TestUpdateRejectsNonEditable 传 usc / 时间列 / 采集统计列必须被拒。
func TestUpdateRejectsNonEditable(t *testing.T) {
	store := NewStore(nil)
	for _, column := range []string{"usc", "gmt_create", "dt_last_fetch", "count_fail"} {
		_, err := store.Update(t.Context(), UpdateInput{
			Usc:    "00700",
			Fields: map[string]json.RawMessage{column: json.RawMessage(`1`)},
		})
		if err == nil {
			t.Fatalf("列 %s 应被拒绝", column)
		}
		var inputErr *InputError
		if !asInputError(err, &inputErr) {
			t.Errorf("%s 应返回 InputError，实际 %T", column, err)
		}
	}
}

// TestUpdateRejectsEmpty 空 usc / 空 fields / 全 null 都要拒绝。
func TestUpdateRejectsEmpty(t *testing.T) {
	store := NewStore(nil)
	cases := []UpdateInput{
		{Usc: ""},
		{Usc: "00700"},
		{Usc: "00700", Fields: map[string]json.RawMessage{"remark": json.RawMessage(`null`)}},
	}
	for i, input := range cases {
		if _, err := store.Update(t.Context(), input); err == nil {
			t.Errorf("第 %d 个用例应报错", i+1)
		}
	}
}

// TestBuildUscFilter 过滤条件必须是 usc=eq.xxx（不接受无条件更新）。
func TestBuildUscFilter(t *testing.T) {
	filter := buildUscFilter("00700")
	if got := filter.Get("usc"); got != "eq.00700" {
		t.Errorf("过滤条件应为 eq.00700，实际 %q", got)
	}
	parsed, err := url.ParseQuery(filter.Encode())
	if err != nil {
		t.Fatalf("过滤条件编码异常: %v", err)
	}
	if parsed.Get("usc") != "eq.00700" {
		t.Errorf("编码往返后丢失语义: %q", parsed.Get("usc"))
	}
}

// TestErrNotFoundIsSentinel 不存在错误必须可 errors.Is 判定（handler 据此回 404）。
func TestErrNotFoundIsSentinel(t *testing.T) {
	wrapped := errors.Join(ErrNotFound, errors.New("usc x"))
	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("ErrNotFound 应可被 errors.Is 识别")
	}
}
