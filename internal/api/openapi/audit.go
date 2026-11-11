package openapi

// 调用审计（open.open_api_invoke_log，需求 11168）。
//
// 设计取舍：**同步写**（带 1.5 秒超时）+ 失败只打日志。
// 理由：外部调用量小（采集钩子/核对工具），同步写换来「请求结束即落库」的可预期性；
// 用异步 channel 的话必须管理 worker 生命周期与丢日志策略，收益不抵复杂度。

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const insertInvokeLogSQL = `
INSERT INTO open.open_api_invoke_log
    (client_idn, credential_idn, access_key_id, auth_mode, request_id, dt_request,
     method, path, query_str, status_code, biz_code, flag_pass, reason_code,
     ip, user_agent, body_sha256, count_body, latency_ms, kfp)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6,
        $7, $8, $9, $10, $11, $12, NULLIF($13, ''),
        $14, $15, NULLIF($16, ''), $17, $18, NULLIF($19, ''))`

// auditWriteTimeout 单次审计写入超时。
const auditWriteTimeout = 1500 * time.Millisecond

// Recorder 写调用审计；pool 为 nil 时退化为只记应用日志（单测与本地无库场景）。
type Recorder struct {
	pool *pgxpool.Pool
	logf func(format string, args ...any)
}

// NewRecorder 创建审计写入器。
func NewRecorder(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool, logf: log.Printf}
}

// Invocation 一次调用的审计要素。
type Invocation struct {
	ClientIDN     int64
	CredentialIDN int64
	AccessKeyID   string
	KFP           string
	AuthMode      string
	RequestID     string
	RequestAt     time.Time

	Method     string
	Path       string
	Query      string
	StatusCode int
	BizCode    int
	Pass       bool
	ReasonCode string

	IP        net.IP
	UserAgent string
	BodyHash  string
	BodySize  int
	Latency   time.Duration
}

// Write 写一条审计记录；失败只记应用日志，不影响响应。
func (r *Recorder) Write(ctx context.Context, item Invocation) {
	if r == nil {
		return
	}
	// 请求上下文可能已取消（客户端断开），审计仍需尽力落库。
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditWriteTimeout)
	defer cancel()

	if r.pool == nil {
		r.logf("[openapi][audit] pass=%t reason=%s mode=%s ak=%s %s %s status=%d",
			item.Pass, item.ReasonCode, item.AuthMode, item.AccessKeyID, item.Method, item.Path, item.StatusCode)
		return
	}

	var ipText *string
	if item.IP != nil {
		value := item.IP.String()
		ipText = &value
	}
	pass := "0"
	if item.Pass {
		pass = "1"
	}
	_, err := r.pool.Exec(writeCtx, insertInvokeLogSQL,
		item.ClientIDN, item.CredentialIDN, item.AccessKeyID, item.AuthMode, item.RequestID, item.RequestAt,
		item.Method, item.Path, truncate(item.Query, 2000), item.StatusCode, item.BizCode, pass, item.ReasonCode,
		ipText, truncate(item.UserAgent, 512), item.BodyHash, item.BodySize, int(item.Latency/time.Millisecond),
		item.KFP,
	)
	if err != nil {
		r.logf("[openapi][audit] 写入 open.open_api_invoke_log 失败: %v（pass=%t reason=%s ak=%s %s %s）",
			err, item.Pass, item.ReasonCode, item.AccessKeyID, item.Method, item.Path)
	}
}

// truncate 按字符数截断（列宽按字符定义，这里用 rune 计数避免把中文截成半个）。
func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// clientIP 取请求来源 IP（信任 Gin 的受信代理配置）。
func clientIP(req *http.Request) net.IP {
	host := strings.TrimSpace(req.RemoteAddr)
	if index := strings.LastIndex(host, ":"); index > 0 {
		if ip := net.ParseIP(strings.Trim(host[:index], "[]")); ip != nil {
			return ip
		}
	}
	return net.ParseIP(host)
}
