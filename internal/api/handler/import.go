// Package handler 提供 HTTP 处理器。
package handler

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote"
	"github.com/acanx/finvquant/internal/meta/finv/quote/mvsv"
)

// 上传文件大小上限（50 MiB）。
const maxUploadBytes = 50 * 1024 * 1024

// QuoteImport 历史行情导入处理器。
type QuoteImport struct {
	service *quote.Service
}

// NewQuoteImport 创建导入处理器。
func NewQuoteImport(service *quote.Service) *QuoteImport {
	return &QuoteImport{service: service}
}

// Upload 处理 POST /API/V1/Quote/Import/Upload：
// multipart 上传 MVSV-1 文件 → 解析 → 字段级覆盖 upsert。
func (h *QuoteImport) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少上传文件字段 file"})
		return
	}
	source := c.PostForm("source")
	upsertMode := c.PostForm("upsert_mode")
	importedBy := c.PostForm("imported_by")
	remark := c.PostForm("remark")
	// 登记字段（可选）：市场代码 / 证券代码，用于与文件头部一致性校验
	formMarketCode := c.PostForm("market_code")
	formUsc := c.PostForm("usc")
	if source == "" {
		source = "upload"
	}
	if upsertMode == "" {
		upsertMode = string(quote.UpsertModeField)
	}
	if upsertMode != string(quote.UpsertModeField) && upsertMode != string(quote.UpsertModeRow) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": 422, "message": "upsert_mode 必须为 FIELD 或 ROW"})
		return
	}
	if importedBy == "" {
		importedBy = "gui"
	}

	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取上传文件失败"})
		return
	}
	defer opened.Close()

	content, err := io.ReadAll(io.LimitReader(opened, maxUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取上传文件失败"})
		return
	}
	if len(content) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": 422, "message": "上传文件为空"})
		return
	}
	if len(content) > maxUploadBytes {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": 422, "message": "文件超过大小上限 50 MiB"})
		return
	}

	parsed, err := mvsv.Parse(content, file.Filename)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": 422, "message": "MVSV 解析失败: " + err.Error()})
		return
	}

	// 登记字段一致性校验：表单填写了市场/证券代码时，必须与文件头部一致
	if formMarketCode != "" && formMarketCode != parsed.Header.Values["MarketCode"] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code": 422, "message": "表单市场代码与文件头部不一致（表单=" + formMarketCode +
				"，文件=" + parsed.Header.Values["MarketCode"] + "）",
		})
		return
	}
	if formUsc != "" && formUsc != parsed.Header.Values["Code"] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code": 422, "message": "表单证券代码与文件头部不一致（表单=" + formUsc +
				"，文件=" + parsed.Header.Values["Code"] + "）",
		})
		return
	}

	ctx, cancel := contextWithTimeout(c, 120*time.Second)
	defer cancel()

	result, err := h.service.ImportRows(ctx, parsed.Rows, quote.UpsertMode(upsertMode), importedBy, source, remark)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "导入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "行情导入完成",
		"data":    result,
	})
}

func contextWithTimeout(c *gin.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), timeout)
}
