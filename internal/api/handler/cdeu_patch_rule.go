package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// 本文件实现 USC 补丁规则接口（需求 10931，方案见 Docs/Proposal/202611-0002）。
// 路径前缀 /API/V1/Meta/Finv/Quote/Cdeu/Usc/Patch/。
// 约定：URL 查询参数小驼峰，JSON 体字段 snake_case（ApiSpec §3）。

// UscPatchRuleList 分页查询补丁规则（GET .../Usc/Patch/List）。
func (h *ExportArchive) UscPatchRuleList(c *gin.Context) {
	page, err := h.service.Store().ListUscPatchRules(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询补丁规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// UscPatchRuleSave 新增或更新补丁规则（POST .../Usc/Patch/Save）。
func (h *ExportArchive) UscPatchRuleSave(c *gin.Context) {
	var req cdeu.UscPatchRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": "请求体解析失败: " + err.Error()})
		return
	}
	idn, err := h.service.Store().SaveUscPatchRule(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 2007, "message": "保存补丁规则失败: " + err.Error()})
		return
	}
	h.audit(c, "保存 USC 补丁规则 idn="+strconv.FormatInt(idn, 10)+
		" "+req.UscOld+"→"+req.UscNew+" flag_enable="+req.FlagEnable)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"idn": idn}})
}

// UscPatchRuleEnable 启用或停用补丁规则（POST .../Usc/Patch/Enable）。
func (h *ExportArchive) UscPatchRuleEnable(c *gin.Context) {
	var req struct {
		Idn    int64 `json:"idn"`
		Enable bool  `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := h.service.Store().SetUscPatchRuleEnable(c.Request.Context(), req.Idn, req.Enable); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 2007, "message": "更新规则状态失败: " + err.Error()})
		return
	}
	state := "停用"
	if req.Enable {
		state = "启用"
	}
	h.audit(c, state+" USC 补丁规则 idn="+strconv.FormatInt(req.Idn, 10))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": state + "成功"})
}

// UscPatchRuleDelete 删除补丁规则（POST .../Usc/Patch/Delete）。
func (h *ExportArchive) UscPatchRuleDelete(c *gin.Context) {
	var req struct {
		Idn int64 `json:"idn"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := h.service.Store().DeleteUscPatchRule(c.Request.Context(), req.Idn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 2007, "message": "删除规则失败: " + err.Error()})
		return
	}
	h.audit(c, "删除 USC 补丁规则 idn="+strconv.FormatInt(req.Idn, 10))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// UscPatchPreview 试算：返回每条生效规则当前影响的行数（GET .../Usc/Patch/Preview）。
func (h *ExportArchive) UscPatchPreview(c *gin.Context) {
	items, err := h.service.Store().PreviewUscPatch(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "试算失败: " + err.Error()})
		return
	}
	var totalRows, totalConflict int64
	for _, item := range items {
		totalRows += item.Rows
		totalConflict += item.ConflictRows
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "试算完成", "data": gin.H{
		"list":           items,
		"total_rows":     totalRows,
		"total_conflict": totalConflict,
	}})
}

// UscPatchRun 手动触发补丁执行（POST .../Usc/Patch/Run）。
//
// 异步执行：立即返回运行快照，前端按 sinceSeq 轮询 Status 获取增量日志，
// 与 BRS 手动备份（/Meta/BRS/Backup/Finv/Run）的交互一致。
func (h *ExportArchive) UscPatchRun(c *gin.Context) {
	var req struct {
		BatchSize int `json:"batch_size"`
	}
	_ = c.ShouldBindJSON(&req)
	state, err := h.service.UscPatchRunner().TriggerManual(req.BatchSize)
	if err != nil {
		var busy *cdeu.UscPatchBusyError
		if errors.As(err, &busy) {
			c.JSON(http.StatusConflict, gin.H{"code": 2009, "message": busy.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2008, "message": "补丁执行失败: " + err.Error()})
		return
	}
	h.audit(c, "手动执行 USC 补丁 run_id="+state.RunID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已开始执行", "data": state})
}

// UscPatchStatus 查询补丁执行状态与增量日志（GET .../Usc/Patch/Status?sinceSeq=）。
func (h *ExportArchive) UscPatchStatus(c *gin.Context) {
	var sinceSeq int64
	if value, ok := queryInt64Param(c, "sinceSeq"); ok {
		sinceSeq = value
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成",
		"data": h.service.UscPatchRunner().Status(sinceSeq)})
}
