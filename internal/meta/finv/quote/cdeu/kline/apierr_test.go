package kline

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestCatalogIntegrity(t *testing.T) {
	catalog := Catalog()
	if len(catalog) != 15 {
		t.Fatalf("目录条目数应为 15，实际 %d", len(catalog))
	}
	seenSymbols := map[string]bool{}
	for code, def := range catalog {
		if code != def.Code {
			t.Fatalf("map key %d 与 Def.Code %d 不一致", code, def.Code)
		}
		if code < 211001 || code > 211399 {
			t.Fatalf("错误码 %d 不在 211xxx 具体码号段内", code)
		}
		if def.HTTP < 400 || def.HTTP > 599 {
			t.Fatalf("错误码 %d 的 HTTP 状态码非法: %d", code, def.HTTP)
		}
		if def.SafeMsg == "" {
			t.Fatalf("错误码 %d 缺少安全文案", code)
		}
		if seenSymbols[def.Symbol] {
			t.Fatalf("符号重复: %s", def.Symbol)
		}
		seenSymbols[def.Symbol] = true
		// 分类必须与号段一致。
		switch def.Kind {
		case KindValidation:
			if code/100 != 2110 {
				t.Fatalf("validation 码 %d 号段错误", code)
			}
		case KindResource:
			if code/100 != 2111 {
				t.Fatalf("resource 码 %d 号段错误", code)
			}
		case KindCapacity:
			if code/100 != 2112 {
				t.Fatalf("capacity 码 %d 号段错误", code)
			}
		case KindStorage:
			if code/100 != 2113 {
				t.Fatalf("storage 码 %d 号段错误", code)
			}
		default:
			t.Fatalf("未知分类: %s", def.Kind)
		}
	}
}

func TestClassifyDB(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"唯一键冲突", &pgconn.PgError{Code: "23505", ConstraintName: "finv_quote_secu_kline_min_pkey"}, CodeDuplicateKey},
		{"CHECK 约束", &pgconn.PgError{Code: "23514", ConstraintName: "finv_quote_secu_kline_min_time_check"}, CodeDBConstraint},
		{"数值越界", &pgconn.PgError{Code: "22003"}, CodeDBConstraint},
		{"其他 PG 错误", &pgconn.PgError{Code: "42P01"}, CodeDBError},
		{"普通错误", errors.New("boom"), CodeDBError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ke := ClassifyDB(tc.err)
			if ke.Code != tc.code {
				t.Fatalf("期望 %d，实际 %d", tc.code, ke.Code)
			}
			if ke.SafeMsg == "" {
				t.Fatal("缺少安全文案")
			}
			if strings.Contains(ke.SafeMsg, "boom") || strings.Contains(ke.SafeMsg, "SELECT") {
				t.Fatalf("安全文案泄漏内部信息: %s", ke.SafeMsg)
			}
		})
	}
	// 唯一冲突的 debug 字段应包含约束名（仅高权限可见）。
	ke := ClassifyDB(&pgconn.PgError{Code: "23505", ConstraintName: "pk"})
	if ke.DebugFields()["constraint"] != "pk" {
		t.Fatalf("缺少 constraint debug 字段: %v", ke.DebugFields())
	}
}

func TestAsErrorUnknown(t *testing.T) {
	ke := AsError(errors.New("dial tcp 10.0.0.1:5432: connect: refused"))
	if ke.Code != CodeDBError {
		t.Fatalf("未知错误应归并为 %d，实际 %d", CodeDBError, ke.Code)
	}
	if strings.Contains(ke.SafeMsg, "10.0.0.1") {
		t.Fatalf("安全文案不得包含内部地址: %s", ke.SafeMsg)
	}
}
