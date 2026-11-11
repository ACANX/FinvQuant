package openapi

// STS 换证接口（需求 11191 / 11210）：POST /OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue
//
// 端点由用户定稿（缩写 STS 保持全大写，其余段 PascalCase）。
// 只能用 `auth_mode='STS'` 的**兑换凭证**（长期 AK/SK，`token_fp` 为空）签名调用；
// 长期 AK/SK 模式（AKSK）调用本接口一律拒绝——两类凭证刻意隔离，避免「一把 AK 既能直连业务
// 又能派生 SST」放大风险。返回的三要素有效期固定 2 小时，授权范围继承兑换凭证。

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// StsIssuePath 换证接口路径（用户定稿，需求 11191）。
const StsIssuePath = "/OpenAPI/V1/Meta/Open/Auth/Credential/STS/Issue"

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
	// 只有 STS 兑换凭证可以换证：AKSK 不允许派生 SST（防权限放大），临时凭据不能换证（防链条延长）
	if parent.AuthMode != AuthModeSTS || parent.TokenFP != "" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    CodeScopeDeny,
			"message": "只有 STS 模式的兑换凭证可以换取临时凭据（AK/SK 长期凭证与 STS 临时凭据都不允许）",
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
