package handler

// 「OpenAPI 调用方管理」接口（需求 11169）。
//
// 全部挂 `/API/V1/Meta/Finv/OpenAPI/*`（内部接口，需登录）：
//
//	GET  /Secret/Status                   读密钥配置现状（19004 / 19005 是否已配置 + 凭证统计）
//	POST /Secret/Save                     保存 19004 / 19005 / 19006（换密钥时可选重加密现有凭证）
//	GET  /Settings                        读鉴权配置（19001～19003 值 + 密钥是否已配置）
//	PUT  /Settings                        保存 19001～19003
//	GET  /Client/Page                     调用方分页
//	POST /Client/Save                     新建/更新调用方（idn=0 新建）
//	POST /Client/Enable                   启用/停用调用方
//	GET  /Credential/Page                 凭证分页（可按 clientId 过滤）
//	POST /Credential/Create               签发凭证（密钥明文只返回这一次）
//	POST /Credential/Enable               启用/停用凭证
//	POST /Credential/Revoke               吊销凭证（原因必填）
//	POST /Credential/Rotate               轮换密钥（新密钥只返回这一次）
//	GET  /InvokeLog/Page                  调用审计分页
//
// 约定：URL 查询参数小驼峰、JSON 体字段 snake_case（ApiSpec §3）；
// 敏感列（secret_hash / secret_cipher）任何接口都不返回，只回 has_secret 布尔。

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	openapisvc "github.com/acanx/finvquant/internal/meta/finv/openapi"
)

// OpenAPIAdmin 「OpenAPI 调用方管理」处理器。
type OpenAPIAdmin struct {
	service *openapisvc.Service
}

// NewOpenAPIAdmin 构造处理器。
func NewOpenAPIAdmin(service *openapisvc.Service) *OpenAPIAdmin {
	return &OpenAPIAdmin{service: service}
}

// operatorOf 取当前登录账号（审计列 create_by / modify_by、调用方负责人默认值）。
//
// 登录会话由 RequireAuth 放在 auth.ContextSessionKey 里；这里不再去 Context 里
// 猜 "uid" 之类的键（那是取不到的，会静默返回空串）。
func operatorOf(c *gin.Context) string {
	if session, ok := SessionFrom(c); ok && session != nil {
		return strings.TrimSpace(session.UID)
	}
	return ""
}

// respondOpenAPIError 统一错误映射：400 / 404 / 409 / 503，其余 500。
func respondOpenAPIError(c *gin.Context, action string, err error) {
	switch {
	case errors.Is(err, openapisvc.ErrInvalidRequest):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
	case errors.Is(err, openapisvc.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
	case errors.Is(err, openapisvc.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
	case errors.Is(err, openapisvc.ErrNotReady):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error()})
	default:
		log.Printf("[openapi][admin] %s failed err=%v", action, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": action + "失败: " + err.Error()})
	}
}

func openAPIQueryInt(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func openAPIQueryInt64(c *gin.Context, key string) int64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// Settings GET /OpenAPI/Settings。
func (h *OpenAPIAdmin) Settings(c *gin.Context) {
	view, err := h.service.Settings(c.Request.Context())
	if err != nil {
		respondOpenAPIError(c, "读取鉴权配置", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": view})
}

// SaveSettings PUT /OpenAPI/Settings。
func (h *OpenAPIAdmin) SaveSettings(c *gin.Context) {
	var req openapisvc.SettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	view, err := h.service.SaveSettings(c.Request.Context(), req)
	if err != nil {
		respondOpenAPIError(c, "保存鉴权配置", err)
		return
	}
	log.Printf("[openapi][admin] settings saved mode=%s window=%d retain=%d by=%s",
		view.Mode, view.SignTimeWindow, view.LogRetainDays, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "鉴权配置已保存（10 秒内生效）", "data": view})
}

// ClientPage GET /OpenAPI/Client/Page。
func (h *OpenAPIAdmin) ClientPage(c *gin.Context) {
	list, total, err := h.service.ClientPage(c.Request.Context(), c.Query("keyword"),
		openAPIQueryInt(c, "page", 1), openAPIQueryInt(c, "pageSize", 20))
	if err != nil {
		respondOpenAPIError(c, "查询调用方", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK",
		"data": gin.H{"total": total, "list": list}})
}

// SaveClient POST /OpenAPI/Client/Save。
func (h *OpenAPIAdmin) SaveClient(c *gin.Context) {
	var req openapisvc.ClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	client, err := h.service.SaveClient(c.Request.Context(), req, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "保存调用方", err)
		return
	}
	log.Printf("[openapi][admin] client saved idn=%d code=%s enable=%s by=%s",
		client.IDN, client.ClientCode, client.FlagEnable, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "调用方已保存", "data": client})
}

// SetClientEnable POST /OpenAPI/Client/Enable。
func (h *OpenAPIAdmin) SetClientEnable(c *gin.Context) {
	var req struct {
		IDN    int64 `json:"idn"`
		Enable bool  `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	client, err := h.service.SetClientEnable(c.Request.Context(), req.IDN, req.Enable, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "启用/停用调用方", err)
		return
	}
	log.Printf("[openapi][admin] client enable=%t idn=%d by=%s", req.Enable, req.IDN, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "调用方状态已更新", "data": client})
}

// CredentialPage GET /OpenAPI/Credential/Page。
func (h *OpenAPIAdmin) CredentialPage(c *gin.Context) {
	list, total, err := h.service.CredentialPage(c.Request.Context(), openAPIQueryInt64(c, "clientId"), c.Query("keyword"),
		openAPIQueryInt(c, "page", 1), openAPIQueryInt(c, "pageSize", 20))
	if err != nil {
		respondOpenAPIError(c, "查询凭证", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK",
		"data": gin.H{"total": total, "list": list}})
}

// CreateCredential POST /OpenAPI/Credential/Create。
func (h *OpenAPIAdmin) CreateCredential(c *gin.Context) {
	var req openapisvc.CredentialCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	issued, err := h.service.CreateCredential(c.Request.Context(), req, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "签发凭证", err)
		return
	}
	log.Printf("[openapi][admin] credential created idn=%d no=%s mode=%s client=%d by=%s",
		issued.Credential.IDN, issued.Credential.CredentialNo, issued.Credential.AuthMode,
		issued.Credential.ClientIDN, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "凭证已签发（密钥只显示一次）", "data": issued})
}

// SetCredentialEnable POST /OpenAPI/Credential/Enable。
func (h *OpenAPIAdmin) SetCredentialEnable(c *gin.Context) {
	var req struct {
		IDN    int64 `json:"idn"`
		Enable bool  `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	credential, err := h.service.SetCredentialEnable(c.Request.Context(), req.IDN, req.Enable, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "启用/停用凭证", err)
		return
	}
	log.Printf("[openapi][admin] credential enable=%t idn=%d by=%s", req.Enable, req.IDN, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "凭证状态已更新", "data": credential})
}

// RevokeCredential POST /OpenAPI/Credential/Revoke。
func (h *OpenAPIAdmin) RevokeCredential(c *gin.Context) {
	var req struct {
		IDN    int64  `json:"idn"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	credential, err := h.service.RevokeCredential(c.Request.Context(), req.IDN, req.Reason, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "吊销凭证", err)
		return
	}
	log.Printf("[openapi][admin] credential revoked idn=%d reason=%q by=%s", req.IDN, req.Reason, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "凭证已吊销（不可恢复）", "data": credential})
}

// RotateCredential POST /OpenAPI/Credential/Rotate。
func (h *OpenAPIAdmin) RotateCredential(c *gin.Context) {
	var req struct {
		IDN int64 `json:"idn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	issued, err := h.service.RotateCredential(c.Request.Context(), req.IDN, operatorOf(c))
	if err != nil {
		respondOpenAPIError(c, "轮换密钥", err)
		return
	}
	log.Printf("[openapi][admin] credential rotated idn=%d mode=%s by=%s",
		issued.Credential.IDN, issued.Credential.AuthMode, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "密钥已轮换（新密钥只显示一次）", "data": issued})
}

// InvokeLogPage GET /OpenAPI/InvokeLog/Page。
func (h *OpenAPIAdmin) InvokeLogPage(c *gin.Context) {
	filter := openapisvc.InvokeLogFilter{
		ClientIDN:     openAPIQueryInt64(c, "clientId"),
		CredentialIDN: openAPIQueryInt64(c, "credentialId"),
		AccessKeyID:   c.Query("accessKey"),
		KFP:           c.Query("kfp"),
		FlagPass:      c.Query("flagPass"),
		ReasonCode:    c.Query("reasonCode"),
		Page:          openAPIQueryInt(c, "page", 1),
		PageSize:      openAPIQueryInt(c, "pageSize", 20),
	}
	if from, ok := parseQueryTime(c.Query("dtFrom")); ok {
		filter.DTFrom = from
	}
	if to, ok := parseQueryTime(c.Query("dtTo")); ok {
		filter.DTTo = to
	}
	list, total, err := h.service.InvokeLogPage(c.Request.Context(), filter)
	if err != nil {
		respondOpenAPIError(c, "查询调用审计", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK",
		"data": gin.H{"total": total, "list": list}})
}

// parseQueryTime 解析查询时间（RFC3339 或 2006-01-02）；空串或非法值返回 false。
func parseQueryTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// SecretStatus GET /OpenAPI/Secret/Status。
func (h *OpenAPIAdmin) SecretStatus(c *gin.Context) {
	view, err := h.service.SecretStatus(c.Request.Context())
	if err != nil {
		respondOpenAPIError(c, "读取密钥配置", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": view})
}

// SaveSecret POST /OpenAPI/Secret/Save。
//
// 请求体里的密钥明文只在本次请求中出现，不写日志、不回显。
func (h *OpenAPIAdmin) SaveSecret(c *gin.Context) {
	var req openapisvc.SecretSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体不是合法 JSON: " + err.Error()})
		return
	}
	view, err := h.service.SaveSecret(c.Request.Context(), req)
	if err != nil {
		respondOpenAPIError(c, "保存密钥配置", err)
		return
	}
	// 日志只记「改了什么」，绝不记密钥内容
	log.Printf("[openapi][admin] secret saved kid=%s key_changed=%t reencrypt=%d skipped=%d fallback_ready=%t by=%s",
		view.SecretKID, strings.TrimSpace(req.SecretKeyBase64) != "", view.UpdatedCredential,
		view.SkippedCredential, view.FallbackKeyReady, operatorOf(c))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "密钥配置已保存（10 秒内生效）", "data": view})
}
