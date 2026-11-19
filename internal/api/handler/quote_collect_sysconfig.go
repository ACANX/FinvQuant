package handler

// 「行情采集系统配置」页的接口（需求 11343）。
//
//	GET  /API/V1/Meta/Finv/Quote/Collect/Config/List      列表（敏感项回掩码）
//	POST /API/V1/Meta/Finv/Quote/Collect/Config/Save      保存单条（只改，不新增/删除）
//	GET  /API/V1/Meta/Finv/Quote/Collect/Config/Secret    取敏感项明文（base64，需登录）
//
// 范围只有 20013–20018 中与实时行情采集相关的 5 项，见 collectcfg 包注释。
// 安全：列表与响应体一律不含明文；Secret 接口返回 **base64 编码后**的文本，
// 日志只记编号与长度，不记内容。

import (
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/collectcfg"
)

// QuoteCollectSysConfig 行情采集系统配置接口。
type QuoteCollectSysConfig struct {
	store *collectcfg.Store
}

// NewQuoteCollectSysConfig 创建接口处理器。
func NewQuoteCollectSysConfig(store *collectcfg.Store) *QuoteCollectSysConfig {
	return &QuoteCollectSysConfig{store: store}
}

// List GET /Meta/Finv/Quote/Collect/Config/List。
//
// 返回目录内 5 项的当前值：敏感项（20013 / 20016）回掩码 + masked=true；
// 库里缺行时该项 value 为空、has_value=false（页面提示执行种子迁移），不报错。
func (h *QuoteCollectSysConfig) List(c *gin.Context) {
	started := time.Now()
	views, err := h.store.List(c.Request.Context())
	if err != nil {
		log.Printf("[collectcfg] api list failed err=%v elapsed_ms=%d", err, time.Since(started).Milliseconds())
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取行情采集系统配置失败: " + err.Error()})
		return
	}
	log.Printf("[collectcfg] api list end items=%d elapsed_ms=%d", len(views), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": gin.H{"items": views}})
}

// SaveConfigRequest 保存单条配置的请求体。
type SaveConfigRequest struct {
	// ConfigNo 配置编号（必须在 collectcfg 目录内）。
	ConfigNo int `json:"config_no"`
	// Value 新值；语义随配置项不同：
	//   20013 `{PROJECT_ID}|{APIKEY}`（空串 = 未配置）
	//   20014 表名（空串 = 回退默认）
	//   20015 6 段 cron（空串 = 关闭自动同步）
	//   20016 `{AppKey}|{私钥 PEM}`：私钥留空 = 保留原值，__CLEAR__ = 只清私钥
	//   20018 `{enable}|{base_url}`（空串 = 直连）
	Value string `json:"value"`
}

// Save POST /Meta/Finv/Quote/Collect/Config/Save。
func (h *QuoteCollectSysConfig) Save(c *gin.Context) {
	var req SaveConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	started := time.Now()
	if _, err := h.store.Save(c.Request.Context(), req.ConfigNo, req.Value); err != nil {
		status, code := http.StatusBadRequest, 400
		var inputErr *collectcfg.InputError
		if !errors.As(err, &inputErr) {
			status, code = http.StatusInternalServerError, 2006
		}
		log.Printf("[collectcfg] api save failed config_no=%d err=%v elapsed_ms=%d",
			req.ConfigNo, err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[collectcfg] api save ok config_no=%d elapsed_ms=%d", req.ConfigNo, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存完成", "data": gin.H{"config_no": req.ConfigNo}})
}

// Secret GET /Meta/Finv/Quote/Collect/Config/Secret?configNo=20016。
//
// 只允许 20013 / 20016 两个敏感项；返回
//
//	{"config_no":20016,"value_base64":"<base64(原值)>","length":123,"updated_at":"..."}
//
// 为什么是 base64：前端「👁 查看明文」需要原文，但明文直接进 HTTP 响应体会被
// 浏览器历史、代理日志、复制粘贴等方式留痕；base64 至少不会被肉眼/关键词扫描命中，
// 且必须由本接口显式调用才能拿到（列表接口永远只有掩码）。
func (h *QuoteCollectSysConfig) Secret(c *gin.Context) {
	configNo := parseIntDefault(c.Query("configNo"), 0)
	if configNo <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "configNo 必须是正整数"})
		return
	}
	started := time.Now()
	raw, err := h.store.SecretValue(c.Request.Context(), configNo)
	if err != nil {
		log.Printf("[collectcfg] api secret failed config_no=%d err=%v elapsed_ms=%d",
			configNo, err, time.Since(started).Milliseconds())
		status, code := http.StatusBadRequest, 400
		var inputErr *collectcfg.InputError
		if !errors.As(err, &inputErr) {
			status, code = http.StatusInternalServerError, 2006
		}
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	log.Printf("[collectcfg] api secret ok config_no=%d length=%d elapsed_ms=%d",
		configNo, len(raw), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data": gin.H{
			"config_no":    configNo,
			"value_base64": encoded,
			"length":       len(raw),
		},
	})
}
