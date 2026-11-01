// BRS 备份登记维护处理器（brs.brs_bak_rdb_table 的查询 / 新增 / 编辑 / 启用禁用）。
//
// URL 查询参数遵循小驼峰（Docs/DevSpec/ApiSpec.md §3）：page / pageSize / keyword / flagEnable；
// JSON 请求与响应体为 snake_case（与库列名一致）。
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/acanx/finvquant/internal/meta/finv/brs"
)

// BrsBackupTable BRS 备份登记维护处理器。
type BrsBackupTable struct {
	registry *brs.Registry
}

// NewBrsBackupTable 创建备份登记维护处理器。
func NewBrsBackupTable(registry *brs.Registry) *BrsBackupTable {
	return &BrsBackupTable{registry: registry}
}

// List 分页查询备份登记（GET /API/V1/Meta/BRS/Backup/Finv/Table/List）。
func (h *BrsBackupTable) List(c *gin.Context) {
	page, pageSize := parseCamelPager(c)
	keyword := c.Query("keyword")
	flagEnable := c.Query("flagEnable")
	list, total, err := h.registry.List(c.Request.Context(), page, pageSize, keyword, flagEnable)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询备份登记失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "page": page, "pageSize": pageSize, "list": list},
	})
}

// Save 新增或更新备份登记（POST /API/V1/Meta/BRS/Backup/Finv/Table/Save）。
//
// idn 为 0 表示新增，否则更新该行；字段校验失败与唯一键冲突返回 400，行不存在返回 404。
func (h *BrsBackupTable) Save(c *gin.Context) {
	var row brs.RegistryRow
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	idn, err := h.registry.Save(c.Request.Context(), row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "备份登记不存在: idn=" + strconv.FormatInt(row.IDN, 10)})
			return
		}
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存备份登记失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"idn": idn}})
}

// Toggle 切换启用状态（POST /API/V1/Meta/BRS/Backup/Finv/Table/Toggle）。
func (h *BrsBackupTable) Toggle(c *gin.Context) {
	var req struct {
		IDN        int64  `json:"idn"`
		FlagEnable string `json:"flag_enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	if err := h.registry.Toggle(c.Request.Context(), req.IDN, req.FlagEnable); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "备份登记不存在: idn=" + strconv.FormatInt(req.IDN, 10)})
			return
		}
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "切换启用状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "切换成功",
		"data":    gin.H{"idn": req.IDN, "flag_enable": req.FlagEnable},
	})
}

// isBrsInputError 判断是否为字段校验失败或唯一键冲突（对应 400）。
func isBrsInputError(err error) bool {
	var target *brs.InputError
	return errors.As(err, &target)
}

// GetOSSConfig 读取对象存储配置（GET /API/V1/Meta/BRS/Backup/Finv/OSSConfig/Get）。
//
// 响应不含 secret_key 明文，只给 secret_key_set 标记与 39002 的字段说明。
func (h *BrsBackupTable) GetOSSConfig(c *gin.Context) {
	cfg, err := h.registry.LoadOSSConfig(c.Request.Context())
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取对象存储配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": cfg})
}

// SaveOSSConfig 保存对象存储配置（POST /API/V1/Meta/BRS/Backup/Finv/OSSConfig/Save）。
//
// secret_key 留空表示保持库内现值；保存后下一轮备份生效（无需重启）。
func (h *BrsBackupTable) SaveOSSConfig(c *gin.Context) {
	var cfg brs.OSSConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	saved, err := h.registry.SaveOSSConfig(c.Request.Context(), cfg, operator)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存对象存储配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": saved})
}

// parseCamelPager 解析分页参数 page / pageSize（小驼峰，ApiSpec §3）。
func parseCamelPager(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return page, pageSize
}
