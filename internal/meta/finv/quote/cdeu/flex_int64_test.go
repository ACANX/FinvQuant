package cdeu

import (
	"encoding/json"
	"testing"
)

// TestFlexInt64JSON 请求体的 stock_id 同时接受数字与数字字符串（需求 11304）。
//
// 快照表 stock_id 是 TEXT，列表接口按文本返回，前端会原样回传字符串；
// 此前只收 int64，绑定阶段直接报
// `json: cannot unmarshal string into Go struct field SecuParamCompleteRequest.stock_id of type int64`。
func TestFlexInt64JSON(t *testing.T) {
	const id int64 = 57084410333914

	// 三个对外请求体都要兼容字符串
	for _, raw := range []string{`{"stock_id":57084410333914}`, `{"stock_id":"57084410333914"}`} {
		var complete SecuParamCompleteRequest
		if err := json.Unmarshal([]byte(raw), &complete); err != nil {
			t.Fatalf("SecuParamCompleteRequest %s 解析失败: %v", raw, err)
		}
		if int64(complete.StockID) != id {
			t.Fatalf("SecuParamCompleteRequest %s stock_id = %d", raw, complete.StockID)
		}

		var enroll PendingSecuEnrollRequest
		if err := json.Unmarshal([]byte(raw), &enroll); err != nil {
			t.Fatalf("PendingSecuEnrollRequest %s 解析失败: %v", raw, err)
		}
		if int64(enroll.StockID) != id {
			t.Fatalf("PendingSecuEnrollRequest %s stock_id = %d", raw, enroll.StockID)
		}

		var probe PendingSecuKeyProbeRequest
		if err := json.Unmarshal([]byte(raw), &probe); err != nil {
			t.Fatalf("PendingSecuKeyProbeRequest %s 解析失败: %v", raw, err)
		}
		if int64(probe.StockID) != id {
			t.Fatalf("PendingSecuKeyProbeRequest %s stock_id = %d", raw, probe.StockID)
		}
	}

	// 缺省 / null / 空串 → 0，由业务层按「必须为正整数」拒绝
	for _, raw := range []string{`{}`, `{"stock_id":null}`, `{"stock_id":""}`} {
		var complete SecuParamCompleteRequest
		if err := json.Unmarshal([]byte(raw), &complete); err != nil {
			t.Fatalf("%s 解析失败: %v", raw, err)
		}
		if complete.StockID != 0 {
			t.Fatalf("%s stock_id 应为 0，实际 %d", raw, complete.StockID)
		}
	}

	// 非数字字符串 → 报错（不静默变 0）
	var bad SecuParamCompleteRequest
	if err := json.Unmarshal([]byte(`{"stock_id":"abc"}`), &bad); err == nil {
		t.Fatal("非数字 stock_id 应解析失败")
	}
}
