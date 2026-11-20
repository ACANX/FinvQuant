// BRS 大表增量（分片）备份处理器：
// 登记维护（List / Save / Toggle）、分片计划查询、手动触发与实时运行状态。
//
// URL 查询参数遵循小驼峰（Docs/DevSpec/ApiSpec.md §3）：`ruleId`、`sinceSeq`。
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/brs"
)

// BrsSlice 大表分片备份处理器。
type BrsSlice struct {
	runner *brs.SliceRunner
}

// NewBrsSlice 创建大表分片备份处理器。
func NewBrsSlice(runner *brs.SliceRunner) *BrsSlice {
	return &BrsSlice{runner: runner}
}

// ListRules 查询全部大表分片备份登记及其分片完成度
// （GET /API/V1/Meta/BRS/Backup/Finv/Slice/Rule/List）。
//
// 每个登记附带 plan（分片总数 / 已备份 / 待备份 / 明细前 50 条）。
func (h *BrsSlice) ListRules(c *gin.Context) {
	rules, err := brs.ListSliceRulesWithPlan(c.Request.Context(), h.runner.Pool(), h.runner.Options(), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询大表分片备份登记失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": gin.H{"list": rules}})
}

// SaveRule 新增或编辑一条大表分片备份登记
// （POST /API/V1/Meta/BRS/Backup/Finv/Slice/Rule/Save）。
func (h *BrsSlice) SaveRule(c *gin.Context) {
	var req brs.SliceRuleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	rule, err := brs.SaveSliceRule(c.Request.Context(), h.runner.Pool(), req, operator)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存大表分片备份登记失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": rule})
}

// ToggleRule 启用/停用一条大表分片备份登记
// （POST /API/V1/Meta/BRS/Backup/Finv/Slice/Rule/Toggle）。
func (h *BrsSlice) ToggleRule(c *gin.Context) {
	var req struct {
		IDN    int64 `json:"idn"`
		Enable *bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	if req.IDN <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "idn 必须为正整数"})
		return
	}
	enable := true
	if req.Enable != nil {
		enable = *req.Enable
	}
	rule, err := brs.ToggleSliceRule(c.Request.Context(), h.runner.Pool(), req.IDN, enable)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "更新启用状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": rule})
}

// Plan 查询指定登记的分片计划（GET /API/V1/Meta/BRS/Backup/Finv/Slice/Plan）。
//
// 只读查询，不产生备份文件；用于页面刷新「已备份 / 待备份」计数。
func (h *BrsSlice) Plan(c *gin.Context) {
	ruleID, err := strconv.ParseInt(c.DefaultQuery("ruleId", "0"), 10, 64)
	if err != nil || ruleID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ruleId 必须为正整数"})
		return
	}
	ctx := c.Request.Context()
	rule, err := brs.GetSliceRule(ctx, h.runner.Pool(), ruleID)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取登记失败: " + err.Error()})
		return
	}
	plan, err := brs.BuildSlicePlan(ctx, h.runner.Pool(), h.runner.Options(), rule, 200)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "生成分片计划失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": plan})
}

// TriggerManual 手动触发一轮大表分片备份
// （POST /API/V1/Meta/BRS/Backup/Finv/Slice/Run/Manual）。
//
// 异步执行，立即返回运行状态快照；已有任务在跑时返回 409。
func (h *BrsSlice) TriggerManual(c *gin.Context) {
	state, err := h.runner.TriggerManual()
	if err != nil {
		var busy *brs.BusyError
		if errors.As(err, &busy) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": busy.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "触发分片备份失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已开始分片备份", "data": state})
}

// Status 查询分片备份运行状态与日志
// （GET /API/V1/Meta/BRS/Backup/Finv/Slice/Run/Status）。
func (h *BrsSlice) Status(c *gin.Context) {
	sinceSeq, _ := strconv.ParseInt(c.DefaultQuery("sinceSeq", "0"), 10, 64)
	state := h.runner.Status(sinceSeq)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": state})
}
