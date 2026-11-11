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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertInvokeLogSQL = `
INSERT INTO open.open_api_invoke_log
    (client_idn, credential_idn, access_key_id, auth_mode, request_id, dt_request,
     method, path, query_str, status_code, biz_code, flag_pass, reason_code,
     ip, user_agent, body_sha256, count_body, latency_ms, kfp, tfp)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6,
        $7, $8, $9, $10, $11, $12, NULLIF($13, ''),
        $14, $15, NULLIF($16, ''), $17, $18, NULLIF($19, ''), NULLIF($20, ''))`

// updateCredentialUsageSQL 更新凭证「最近使用」与「累计调用」（需求 11199）。
//
// 只在**放行**时更新；STS 临时凭据记的是父凭证 idn，因此派生调用也累计到父凭证上。
const updateCredentialUsageSQL = `
UPDATE open.open_api_credential
   SET dt_last_use = now(), count_use = count_use + 1
 WHERE idn = $1`

// countUsageOf 是否给该请求累计凭证使用次数：只算「放行 + 能定位到凭证」。
func countUsageOf(pass bool, credentialIDN int64) bool {
	return pass && credentialIDN > 0
}

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
	// TFP STS 的 SecurityToken 指纹（需求 11191/11196）；长期凭据为空。
	TFP       string
	AuthMode  string
	RequestID string
	RequestAt time.Time

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
	// 审计与「凭证使用统计」放在同一个 batch 里发，少一次往返；
	// 统计只算放行且能定位到凭证的请求（兜底密钥 credential_idn=0 不统计）。
	batch := &pgx.Batch{}
	batch.Queue(insertInvokeLogSQL,
		item.ClientIDN, item.CredentialIDN, item.AccessKeyID, item.AuthMode, item.RequestID, item.RequestAt,
		item.Method, item.Path, truncate(item.Query, 2000), item.StatusCode, item.BizCode, pass, item.ReasonCode,
		ipText, truncate(item.UserAgent, 512), item.BodyHash, item.BodySize, int(item.Latency/time.Millisecond),
		item.KFP, item.TFP,
	)
	countUsage := countUsageOf(item.Pass, item.CredentialIDN)
	if countUsage {
		batch.Queue(updateCredentialUsageSQL, item.CredentialIDN)
	}

	results := r.pool.SendBatch(writeCtx, batch)
	for index := 0; index < batch.Len(); index++ {
		if _, err := results.Exec(); err != nil {
			if index == 0 {
				r.logf("[openapi][audit] 写入 open.open_api_invoke_log 失败: %v（pass=%t reason=%s ak=%s %s %s）",
					err, item.Pass, item.ReasonCode, item.AccessKeyID, item.Method, item.Path)
			} else {
				// 统计失败不影响业务，只记日志（下次调用会继续累计）
				r.logf("[openapi][audit] 更新凭证使用统计失败: %v（credential_idn=%d）", err, item.CredentialIDN)
			}
		}
	}
	if err := results.Close(); err != nil {
		r.logf("[openapi][audit] 关闭审计 batch 失败: %v", err)
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
