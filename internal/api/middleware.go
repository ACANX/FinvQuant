package api

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/auth"
)

// AuthScheme Bearer 认证方案前缀。
const AuthScheme = "Bearer "

// RequireAuth 登录校验中间件。
//
// 校验 Authorization: Bearer <token>；令牌无效或已过期统一返回 401，
// 前端据此跳转登录页。校验通过的会话写入 gin.Context，
// 业务处理器可通过 handler.SessionFrom 取用。
func RequireAuth(service *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := BearerToken(c.GetHeader("Authorization"))
		if token == "" {
			abortUnauthorized(c)
			return
		}
		session, err := service.Resolve(c.Request.Context(), token)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		c.Set(auth.ContextSessionKey, session)
		c.Next()
	}
}

// BearerToken 从 Authorization 头解析令牌；不匹配返回空串。
func BearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) <= len(AuthScheme) || !strings.EqualFold(header[:len(AuthScheme)], AuthScheme) {
		return ""
	}
	return strings.TrimSpace(header[len(AuthScheme):])
}

// abortUnauthorized 统一 401 响应，并终止后续处理器。
func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    401,
		"message": "未登录或登录已过期",
	})
}

// OpenAPIKeyHeader 外部 OpenAPI 密钥头（与 Authorization: Bearer 二选一）。
const OpenAPIKeyHeader = "X-API-Key"

// OpenAPIKeyFromRequest 解析外部 OpenAPI 密钥：优先 X-API-Key，其次 Authorization: Bearer。
func OpenAPIKeyFromRequest(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader(OpenAPIKeyHeader)); key != "" {
		return key
	}
	return BearerToken(c.GetHeader("Authorization"))
}

// RequireOpenAPIKey 校验外部 OpenAPI 静态密钥（需求 10787，过渡方案）。
//
// 仅用于 /OpenAPI/V1 路由组；与登录会话 RequireAuth 完全独立。
// 未配置密钥时路由组不应注册（见 router.go），此处再兜底拒绝。
func RequireOpenAPIKey(expected string) gin.HandlerFunc {
	secret := strings.TrimSpace(expected)
	return func(c *gin.Context) {
		if secret == "" {
			abortUnauthorized(c)
			return
		}
		provided := OpenAPIKeyFromRequest(c)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// responseCapture 缓冲 handler 输出，便于外部 OpenAPI 统一包装响应。
type responseCapture struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	written bool
}

func (w *responseCapture) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
	}
}

func (w *responseCapture) Write(data []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	return w.body.Write(data)
}

func (w *responseCapture) WriteString(text string) (int, error) {
	return w.Write([]byte(text))
}

func (w *responseCapture) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *responseCapture) Size() int { return w.body.Len() }

func (w *responseCapture) Written() bool { return w.written }

// WrapOpenAPIResponse 把外部 OpenAPI 路由的成功响应包装为统一信封（需求 10788）：
//
//	{"code": 0, "data": <handler 原始 JSON>, "message": "OK"}
//
// 成功码默认 0；1 / 200 / 202 同样视为正常响应（兼容），服务端默认输出 0。
// 非 2xx 响应保持 handler 原始错误信封（已含 code/message/detail/request_id）不变。
func WrapOpenAPIResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Writer
		capture := &responseCapture{ResponseWriter: original}
		c.Writer = capture
		c.Next()
		c.Writer = original

		status := capture.Status()
		body := capture.body.Bytes()
		original.Header().Del("Content-Length")

		if status >= http.StatusOK && status < http.StatusMultipleChoices && status != http.StatusNoContent {
			// 内部 handler 若已返回 {code,data,message} 信封（如列表端点），
			// 取内层 data，避免外部信封嵌套。
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(body, &payload); err == nil {
				if _, hasCode := payload["code"]; hasCode {
					if inner, hasData := payload["data"]; hasData {
						body = inner
					}
				}
			}
			data := json.RawMessage(body)
			if len(bytes.TrimSpace(body)) == 0 {
				data = json.RawMessage("null")
			}
			wrapped, err := json.Marshal(gin.H{"code": 0, "data": data, "message": "OK"})
			if err != nil {
				original.Header().Set("Content-Type", "application/json; charset=utf-8")
				original.WriteHeader(http.StatusInternalServerError)
				_, _ = original.Write([]byte(`{"code":211301,"message":"服务内部错误，请稍后重试"}`))
				return
			}
			original.Header().Set("Content-Type", "application/json; charset=utf-8")
			original.WriteHeader(status)
			_, _ = original.Write(wrapped)
			return
		}

		original.WriteHeader(status)
		_, _ = original.Write(body)
	}
}
