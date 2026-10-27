package api

import (
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
