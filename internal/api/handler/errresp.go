package handler

import (
	"crypto/subtle"
	"log"
	"net"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu/kline"
)

// ctxRequestID 请求追踪 ID 的上下文键（同一请求的成功/错误响应用同一个值）。
const ctxRequestID = "request_id"

// dsnCredential 匹配连接串中的「用户名:口令@」，日志输出前脱敏。
var dsnCredential = regexp.MustCompile(`://[^@/\s]+@`)

// redact 脱敏日志文本（不改变错误语义，仅掩盖可能的连接串凭据）。
func redact(text string) string {
	return dsnCredential.ReplaceAllString(text, "://***@")
}

// RequestID 返回当前请求的追踪 ID（大写 UUID4），首次调用时生成。
func RequestID(c *gin.Context) string {
	if value, ok := c.Get(ctxRequestID); ok {
		if text, ok := value.(string); ok && text != "" {
			return text
		}
	}
	id := strings.ToUpper(uuid.NewString())
	c.Set(ctxRequestID, id)
	return id
}

// ErrorResponder 统一错误响应：按 DC-06 分级输出（safe 默认 / admin 白名单 debug）。
type ErrorResponder struct {
	mode  string
	token string
	cidrs []*net.IPNet
}

// NewErrorResponder 创建错误响应器（非法 CIDR 会被忽略）。
func NewErrorResponder(cfg *config.Config) *ErrorResponder {
	responder := &ErrorResponder{}
	if cfg != nil {
		responder.mode = strings.ToLower(strings.TrimSpace(cfg.ErrorDetailMode))
		responder.token = cfg.ErrorDebugToken
		for _, raw := range cfg.ErrorDetailAllowedCIDRs {
			if _, network, err := net.ParseCIDR(raw); err == nil {
				responder.cidrs = append(responder.cidrs, network)
			}
		}
	}
	return responder
}

// isPrivileged 判断当前请求是否可获取 debug 详情：
// 需要开启 admin 模式，且（调试令牌匹配 或 来源 IP 命中内网 CIDR）。
func (r *ErrorResponder) isPrivileged(c *gin.Context) bool {
	if r == nil || r.mode != "admin" {
		return false
	}
	if r.token != "" {
		provided := c.GetHeader("X-Debug-Token")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(r.token)) == 1 {
			return true
		}
	}
	if len(r.cidrs) > 0 {
		ip := net.ParseIP(c.ClientIP())
		for _, network := range r.cidrs {
			if ip != nil && network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// Respond 输出错误响应；err 的内部细节只进日志，响应体只放安全文案与白名单 debug 字段。
func (r *ErrorResponder) Respond(c *gin.Context, err error) {
	ke := kline.AsError(err)
	if ke == nil {
		return
	}
	requestID := RequestID(c)
	body := gin.H{
		"code":       ke.Code,
		"message":    ke.SafeMsg,
		"detail":     ke.SafeMsg,
		"request_id": requestID,
		"error": gin.H{
			"code":            ke.Code,
			"catalog_version": kline.CatalogVersion,
			"retryable":       ke.Retryable,
		},
	}
	privileged := r.isPrivileged(c)
	if privileged {
		debug := gin.H{}
		for _, key := range ke.DebugKeys {
			if value, ok := ke.DebugFields()[key]; ok && value != "" {
				debug[key] = value
			}
		}
		if len(debug) > 0 {
			body["debug"] = debug
		}
		// 高权限详情输出留审计痕迹（不记录令牌本身）。
		log.Printf("[kline] error-detail ip=%s method=%s path=%s code=%d symbol=%s request_id=%s internal=%q",
			c.ClientIP(), c.Request.Method, c.Request.URL.Path, ke.Code, ke.Symbol, requestID, redact(ke.Error()))
	} else {
		log.Printf("[kline] error ip=%s method=%s path=%s code=%d symbol=%s request_id=%s internal=%q",
			c.ClientIP(), c.Request.Method, c.Request.URL.Path, ke.Code, ke.Symbol, requestID, redact(ke.Error()))
	}
	c.AbortWithStatusJSON(ke.HTTP, body)
}
