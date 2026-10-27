package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/auth"
)

// Auth 登录认证处理器。
//
// 路径前缀：/API/V1/Auth
type Auth struct {
	service *auth.Service
}

// NewAuth 创建认证处理器。
func NewAuth(service *auth.Service) *Auth {
	return &Auth{service: service}
}

// SessionFrom 从 gin.Context 取出登录会话。
//
// 键与 internal/api 中 RequireAuth 写入时一致（auth.ContextSessionKey）。
func SessionFrom(c *gin.Context) (*auth.Session, bool) {
	value, ok := c.Get(auth.ContextSessionKey)
	if !ok {
		return nil, false
	}
	session, ok := value.(*auth.Session)
	return session, ok
}

// loginRequest 登录请求体。
type loginRequest struct {
	UID      string `json:"uid"`
	Password string `json:"password"`
}

// Login 校验用户名密码并签发会话（POST /API/V1/Auth/Login）。
//
// 失败统一返回「用户名或密码错误」，不区分账号是否存在。
func (h *Auth) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式非法"})
		return
	}

	session, err := h.service.Login(c.Request.Context(), request.UID, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
		case errors.Is(err, auth.ErrUserDisabled):
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "登录失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "登录成功",
		"data": gin.H{
			"token":       session.Token,
			"expires_at":  session.ExpiresAt,
			"ttl_seconds": int64(time.Until(session.ExpiresAt).Seconds()),
			"user": gin.H{
				"uid":       session.UID,
				"nick_name": session.NickName,
			},
		},
	})
}

// Logout 吊销当前会话（POST /API/V1/Auth/Logout）。
func (h *Auth) Logout(c *gin.Context) {
	session, ok := SessionFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录或登录已过期"})
		return
	}
	if err := h.service.Logout(c.Request.Context(), session.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "退出登录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已退出登录"})
}

// Profile 返回当前登录用户资料（GET /API/V1/Auth/Profile）。
func (h *Auth) Profile(c *gin.Context) {
	session, ok := SessionFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录或登录已过期"})
		return
	}
	user, err := h.service.Profile(c.Request.Context(), session.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询用户资料失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"uid":         user.UID,
			"nick_name":   user.NickName,
			"email":       user.Email,
			"authority":   user.AuthorityStr,
			"enable":      user.FlagEnable,
			"expires_at":  session.ExpiresAt,
			"ttl_seconds": int64(time.Until(session.ExpiresAt).Seconds()),
		},
	})
}
