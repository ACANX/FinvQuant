package handler

// 内部管理接口：签发 STS 临时凭据（需求 11200）。
//
//	POST /API/V1/Meta/Finv/OpenAPI/Credential/StsIssue
//
// 与外部换证接口（`/OpenAPI/V1/Meta/Auth/Credential/STS/Issue`）的区别：
// 外部换证是「调用方拿自己的长期 AK/SK 换」，这里是「管理员在页面上直接发」，
// 因此没有父凭证：授权范围 / IP 白名单 / 调用方限流由管理员在表单里给定，期限固定 2 小时，
// 到期自动失效（不落库、不可续期，也无法在凭证列表里管理——列表只显示落库凭证）。

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	openapi "github.com/acanx/finvquant/internal/api/openapi"
	openapisvc "github.com/acanx/finvquant/internal/meta/finv/openapi"
)

// OpenAPIStsAdmin STS 内部签发处理器。
type OpenAPIStsAdmin struct {
	service  *openapisvc.Service
	settings openapi.SettingsLoader
	sts      *openapi.StsStore
}

// NewOpenAPIStsAdmin 构造处理器。
func NewOpenAPIStsAdmin(service *openapisvc.Service, settings openapi.SettingsLoader, sts *openapi.StsStore) *OpenAPIStsAdmin {
	return &OpenAPIStsAdmin{service: service, settings: settings, sts: sts}
}

// stsIssueRequest 内部签发入参。
type stsIssueRequest struct {
	ClientIDN int64    `json:"client_idn"`
	Scope     []string `json:"scope"`
	AllowCIDR []string `json:"allow_cidr"`
	Remark    string   `json:"remark"`
}

// Issue POST /Credential/StsIssue：签发一套 2 小时有效的 STS 三要素。
func (h *OpenAPIStsAdmin) Issue(c *gin.Context) {
	var req stsIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "入参非法: " + err.Error()})
		return
	}
	if req.ClientIDN <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "client_idn 必须为正整数"})
		return
	}

	client, err := h.service.ClientDetail(c.Request.Context(), req.ClientIDN)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "调用方不存在或不可用: " + err.Error()})
		return
	}
	if client.FlagEnable != "1" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "调用方已停用，不能签发临时凭据"})
		return
	}

	scope, err := openapisvc.NormalizeScope(req.Scope)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if len(scope) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "必须指定授权范围（形如 `POST /OpenAPI/V1/...`）；STS 不允许放开全部接口"})
		return
	}
	cidrs, err := openapisvc.NormalizeCIDR(req.AllowCIDR)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	settings := h.settings.Load(c.Request.Context())
	key, err := settings.SecretKeyBytes()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": openapi.CodeAuthNotReady, "message": err.Error()})
		return
	}

	// 页面直发：parent 只承载「调用方 + 授权范围 + 白名单 + 限流」，IDN=0 表示无父凭证
	parent := &openapi.Credential{
		IDN:           0,
		ClientIDN:     client.IDN,
		AuthMode:      openapi.AuthModeSTS,
		SignAlgorithm: "HMAC-SHA256",
		Scope:         toScopeRules(scope),
		CredentialIPs: nil,
		RateLimitRPM:  client.RateLimitRPM,
		QuotaDay:      client.QuotaDay,
		FlagEnable:    true,
	}
	if len(cidrs) > 0 {
		parent.CredentialIPs = mustParseCIDR(cidrs)
	}

	issued, err := h.sts.Issue(c.Request.Context(), parent, key, operatorOf(c))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": openapi.CodeAuthNotReady, "message": "签发临时凭据失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": gin.H{
		"credential_idn": issued.CredentialIDN,
		"credential_no":  issued.CredentialNo,
		"access_key_id":  issued.AccessKeyID,
		"secret_key":     issued.SecretKey,
		"security_token": issued.SecurityToken,
		"expire_at":      issued.ExpireAt.UTC().Format("2006-01-02T15:04:05Z"),
		"expire_seconds": int(openapi.StsTTL.Seconds()),
		"note":           strings.TrimSpace(req.Remark),
	}})
}

// toScopeRules 把「方法 路径」文本转成 ScopeRule 列表。
func toScopeRules(items []string) []openapi.ScopeRule {
	out := make([]openapi.ScopeRule, 0, len(items))
	for _, item := range items {
		parts := strings.Fields(item)
		if len(parts) != 2 {
			continue
		}
		out = append(out, openapi.ScopeRule{Method: parts[0], Path: parts[1]})
	}
	return out
}

// mustParseCIDR 把 CIDR 文本转成 *net.IPNet（入参已由 NormalizeCIDR 校验过）。
func mustParseCIDR(values []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		if _, network, err := net.ParseCIDR(value); err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}
