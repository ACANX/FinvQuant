package api

import (
	"crypto/subtle"
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
