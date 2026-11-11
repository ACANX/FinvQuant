package openapi

// STS 换证接口（需求 11191）：POST /OpenAPI/V1/Meta/Auth/Credential/STS/Issue
//
// 端点由用户定稿（缩写 STS 保持全大写，其余段 PascalCase）。
// 该接口本身走 OpenAPI 鉴权：调用方必须用**长期 AK/SK** 签名调用，且父凭证的授权范围
// 必须包含本路径；返回的临时凭据有效期固定 2 小时。

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// StsIssuePath 换证接口路径（用户定稿，需求 11191）。
const StsIssuePath = "/OpenAPI/V1/Meta/Auth/Credential/STS/Issue"

// StsIssueHandler 换证处理器；settings 用于取 SK 加密密钥（19005）。
type StsIssueHandler struct {
	settings SettingsLoader
	sts      *StsStore
}

// NewStsIssueHandler 创建换证处理器。
func NewStsIssueHandler(settings SettingsLoader, sts *StsStore) *StsIssueHandler {
	return &StsIssueHandler{settings: settings, sts: sts}
}

// Issue 签发临时凭据（三要素只返回一次）。
func (h *StsIssueHandler) Issue(c *gin.Context) {
	parent, ok := CredentialFrom(c)
	if !ok || parent == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": CodeKeyMissing, "message": "未携带凭证"})
		return
	}
	// 只有长期 AK/SK 可以换证：STS 不能换 STS（防凭据链无限延长）
	if parent.AuthMode != AuthModeAKSK {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    CodeScopeDeny,
			"message": "只有 AK/SK 长期凭证可以换取 STS 临时凭据",
		})
		return
	}

	settings := h.settings.Load(c.Request.Context())
	key, err := settings.SecretKeyBytes()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": CodeAuthNotReady, "message": err.Error()})
		return
	}

	// 外部换证没有登录会话，签发人记空（审计仍能通过 parent_idn 追溯到调用方）
	issued, err := h.sts.Issue(c.Request.Context(), parent, key, "")
	if err != nil {
		code := CodeAuthNotReady
		if errors.Is(err, ErrCredentialNotFound) {
			code = CodeKeyInvalid
		}
		c.JSON(statusOf(code), gin.H{"code": code, "message": "签发临时凭据失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": gin.H{
		"credential_idn": issued.CredentialIDN,
		"credential_no":  issued.CredentialNo,
		"access_key_id":  issued.AccessKeyID,
		"secret_key":     issued.SecretKey,
		"security_token": issued.SecurityToken,
		"expire_at":      issued.ExpireAt.UTC().Format("2006-01-02T15:04:05Z"),
		"expire_seconds": int(StsTTL.Seconds()),
	}})
}
