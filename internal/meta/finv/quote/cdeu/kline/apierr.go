// Package kline 提供分钟行情 K 线 API 的领域实现（融合 FinvQuoteDataDeDup）。
package kline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// CatalogVersion 错误目录版本（随目录条目变化递增）。
const CatalogVersion = "finvquant-1"

// Kind 错误分类，与号段一一对应：
//   - validation：2110xx
//   - resource：2111xx
//   - capacity：2112xx
//   - storage：2113xx
type Kind string

// 错误分类常量。
const (
	KindValidation Kind = "validation"
	KindResource   Kind = "resource"
	KindCapacity   Kind = "capacity"
	KindStorage    Kind = "storage"
)

// Def 错误目录条目：编号、符号、分类、HTTP、安全文案、可重试性与允许出现的 debug 字段。
type Def struct {
	Code      int
	Symbol    string
	Kind      Kind
	HTTP      int
	SafeMsg   string
	Retryable bool
	DebugKeys []string
}

// Error 带目录信息的业务错误。
//
// Error() 返回的是内部技术描述（仅用于日志），绝不直接写入 API 响应；
// 对外输出只使用 SafeMsg 与白名单化的 DebugFields（见 handler.errresp）。
type Error struct {
	Def
	debugFields map[string]string
	internal    error
}

// Error 实现 error 接口（内部描述，仅日志可见）。
func (e *Error) Error() string {
	if e.internal != nil {
		return fmt.Sprintf("%s(%d): %v", e.Symbol, e.Code, e.internal)
	}
	return fmt.Sprintf("%s(%d): %s", e.Symbol, e.Code, e.SafeMsg)
}

// Unwrap 暴露底层错误，便于 errors.Is/As。
func (e *Error) Unwrap() error { return e.internal }

// WithDebug 追加一个 debug 字段（仅白名单键会被输出）。
func (e *Error) WithDebug(key, value string) *Error {
	if e.debugFields == nil {
		e.debugFields = make(map[string]string, 4)
	}
	e.debugFields[key] = value
	return e
}

// DebugFields 返回已设置的 debug 字段（调用方需按 Def.DebugKeys 过滤）。
func (e *Error) DebugFields() map[string]string { return e.debugFields }

// 具体错误码（与 Docs 附录 F.1 逐行一致）。
const (
	CodeMissingParam       = 211001
	CodeMalformedJSON      = 211002
	CodeTypeMismatch       = 211003
	CodeOutOfRange         = 211004
	CodeUnknownField       = 211005
	CodeInvalidEnum        = 211006
	CodeDuplicateKey       = 211101
	CodeDuplicateInRequest = 211102
	CodeNotFound           = 211103
	CodeStateConflict      = 211104
	CodeBatchTooLarge      = 211201
	CodeListTooLarge       = 211202
	CodeDBError            = 211301
	CodeDBUnavailable      = 211302
	CodeDBConstraint       = 211303
)

// catalog 错误目录（编号全局唯一，语义一旦发布不得复用）。
var catalog = map[int]Def{
	CodeMissingParam: {CodeMissingParam, "KLineMissingParam", KindValidation, 400,
		"缺少必填参数", false, []string{"field"}},
	CodeMalformedJSON: {CodeMalformedJSON, "KLineMalformedJSON", KindValidation, 400,
		"请求体格式错误", false, []string{"err_class"}},
	CodeTypeMismatch: {CodeTypeMismatch, "KLineTypeMismatch", KindValidation, 422,
		"字段类型不匹配", false, []string{"field", "want", "got"}},
	CodeOutOfRange: {CodeOutOfRange, "KLineOutOfRange", KindValidation, 422,
		"字段取值超出允许范围", false, []string{"field", "min", "max"}},
	CodeUnknownField: {CodeUnknownField, "KLineUnknownField", KindValidation, 422,
		"请求包含未知字段", false, []string{"field"}},
	CodeInvalidEnum: {CodeInvalidEnum, "KLineInvalidEnum", KindValidation, 400,
		"参数取值不在允许范围内", false, []string{"field", "allowed"}},
	CodeDuplicateKey: {CodeDuplicateKey, "KLineDuplicateKey", KindResource, 409,
		"记录已存在（主键冲突）", false, []string{"pg_code", "sqlstate", "constraint"}},
	CodeDuplicateInRequest: {CodeDuplicateInRequest, "KLineDuplicateInRequest", KindResource, 409,
		"请求内存在重复主键", false, []string{"count", "first_dup"}},
	CodeNotFound: {CodeNotFound, "KLineNotFound", KindResource, 404,
		"记录不存在", false, []string{"usc", "ts"}},
	CodeStateConflict: {CodeStateConflict, "KLineStateConflict", KindResource, 409,
		"当前状态不允许该操作", false, []string{"state", "reason_class"}},
	CodeBatchTooLarge: {CodeBatchTooLarge, "KLineBatchTooLarge", KindCapacity, 413,
		"批量条数超过上限", false, []string{"limit", "got"}},
	CodeListTooLarge: {CodeListTooLarge, "KLineListTooLarge", KindCapacity, 413,
		"列表长度超过上限", false, []string{"limit", "got"}},
	CodeDBError: {CodeDBError, "KLineDBError", KindStorage, 500,
		"服务内部错误，请稍后重试", true, []string{"pg_code", "sqlstate", "err_class"}},
	CodeDBUnavailable: {CodeDBUnavailable, "KLineDBUnavailable", KindStorage, 503,
		"数据服务暂时不可用，请稍后重试", true, []string{"pool_state"}},
	CodeDBConstraint: {CodeDBConstraint, "KLineDBConstraint", KindStorage, 422,
		"数据不满足业务约束", false, []string{"pg_code", "constraint"}},
}

// Catalog 返回错误目录副本（供文档生成与契约测试使用）。
func Catalog() map[int]Def {
	out := make(map[int]Def, len(catalog))
	for code, def := range catalog {
		keys := append([]string(nil), def.DebugKeys...)
		def.DebugKeys = keys
		out[code] = def
	}
	return out
}

// New 按目录编号构造错误。
func New(code int) *Error {
	def, ok := catalog[code]
	if !ok {
		def = catalog[CodeDBError]
		def.Code = code
	}
	return &Error{Def: def}
}

// Wrap 按目录编号包装底层错误（底层信息仅用于日志）。
func Wrap(code int, err error) *Error {
	ke := New(code)
	ke.internal = err
	return ke
}

// AsError 把任意 error 归一为目录错误；未知错误统一归并为 211301。
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var ke *Error
	if errors.As(err, &ke) {
		return ke
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Wrap(CodeDBUnavailable, err).WithDebug("pool_state", "context_done")
	}
	return Wrap(CodeDBError, err).WithDebug("err_class", fmt.Sprintf("%T", err))
}

// ClassifyDB 把 PostgreSQL / 连接池错误映射为目录错误码。
func ClassifyDB(err error) *Error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return Wrap(CodeDuplicateKey, err).
				WithDebug("pg_code", pgErr.Code).
				WithDebug("sqlstate", pgErr.Code).
				WithDebug("constraint", pgErr.ConstraintName)
		case "23514", "22003", "22001", "22P02":
			return Wrap(CodeDBConstraint, err).
				WithDebug("pg_code", pgErr.Code).
				WithDebug("constraint", pgErr.ConstraintName)
		}
		return Wrap(CodeDBError, err).
			WithDebug("pg_code", pgErr.Code).
			WithDebug("sqlstate", pgErr.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return Wrap(CodeDBUnavailable, err).WithDebug("pool_state", "context_done")
	}
	if strings.Contains(strings.ToLower(err.Error()), "pool") {
		return Wrap(CodeDBUnavailable, err).WithDebug("pool_state", "exhausted")
	}
	return Wrap(CodeDBError, err).WithDebug("err_class", fmt.Sprintf("%T", err))
}
