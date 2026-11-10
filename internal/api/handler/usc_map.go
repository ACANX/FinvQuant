// USC 映射规则维护处理器（规则/条目/预演/应用/审计日志）。
//
// 需求 11072：把 `FinvQuantDesign/Mapping/USCMappingRule.md` 的规则固化为数据表后，
// 提供 Web 维护与预演能力；应用（写结果表 + 回写键表 + 复用归档）走异步运行 + 实时日志。
//
// 路由（用户 2026-11-10 指定前缀 /API/V1/Meta/Finv/Quote/UscMap/*）：
//
//	GET  /Status          只读快照（规则数/条目数/结果表/键表匹配率/cron）
//	GET  /Rule/List       规则层列表（含条目数）
//	GET  /Item/List       条目分页
//	POST /Item/Save       新增或编辑条目（item_desc 必填）
//	POST /Item/Retire     停用条目（保留痕迹）
//	GET  /Diff/Preview    预演差异（不落库，返回 preview_token）
//	POST /Sync/Run        异步应用（需带预演令牌，返回 run_id）
//	GET  /Sync/Run        运行时快照（进度 + 日志）
//	GET  /Log/List        审计日志分页
//
// 约定：URL 查询参数小驼峰（ApiSpec §3），JSON 体 snake_case（与库列名一致）；
// 日志前缀 `[usc]`。
package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/uscrule"
)

// UscMap USC 映射规则维护处理器。
type UscMap struct {
	service *uscrule.Service
	runner  *uscrule.Runner
}

// NewUscMap 创建处理器（运行器由 cmd/server 注入：与每日 cron 共用同一份运行状态）。
func NewUscMap(service *uscrule.Service, runner *uscrule.Runner) *UscMap {
	return &UscMap{service: service, runner: runner}
}

// Status GET /Meta/Finv/Quote/UscMap/Status。
func (h *UscMap) Status(c *gin.Context) {
	snapshot, err := h.service.Status(c.Request.Context(), c.Query("env"))
	if err != nil {
		log.Printf("[usc] api status failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取状态失败: " + err.Error()})
		return
	}
	log.Printf("[usc] api status env=%s rule=%d item=%d map_active=%d key_total=%d key_miss=%d",
		snapshot.Env, snapshot.RuleCount, snapshot.ItemCount, snapshot.MapActive,
		snapshot.KeyTotal, snapshot.KeyMiss)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": snapshot})
}

// RuleList GET /Meta/Finv/Quote/UscMap/Rule/List。
func (h *UscMap) RuleList(c *gin.Context) {
	rules, err := h.service.ListRules(c.Request.Context())
	if err != nil {
		log.Printf("[usc] api rule list failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": gin.H{"list": rules}})
}

// ItemList GET /Meta/Finv/Quote/UscMap/Item/List。
func (h *UscMap) ItemList(c *gin.Context) {
	page, pageSize := parseCamelPager(c)
	items, total, err := h.service.ListItems(c.Request.Context(), uscrule.ItemFilter{
		RuleNo:   c.Query("ruleNo"),
		Market:   c.Query("market"),
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		log.Printf("[usc] api item list failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询条目失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "page": page, "pageSize": pageSize, "list": items},
	})
}

// ItemSave POST /Meta/Finv/Quote/UscMap/Item/Save。
func (h *UscMap) ItemSave(c *gin.Context) {
	var item uscrule.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	idn, err := h.service.SaveItem(c.Request.Context(), item, operator)
	if err != nil {
		log.Printf("[usc] api item save failed idn=%d rule=%s err=%v", item.IDN, item.RuleNo, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	log.Printf("[usc] api item save ok idn=%d rule=%s operator=%s", idn, item.RuleNo, operator)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"idn": idn}})
}

// ItemRetire POST /Meta/Finv/Quote/UscMap/Item/Retire。
func (h *UscMap) ItemRetire(c *gin.Context) {
	var req struct {
		IDN    int64  `json:"idn"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	if err := h.service.RetireItem(c.Request.Context(), req.IDN, req.Reason, operator); err != nil {
		log.Printf("[usc] api item retire failed idn=%d err=%v", req.IDN, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	log.Printf("[usc] api item retire ok idn=%d operator=%s", req.IDN, operator)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已停用", "data": gin.H{"idn": req.IDN}})
}

// DiffPreview GET /Meta/Finv/Quote/UscMap/Diff/Preview。
//
// 不落库：算完差异整体回滚，返回计数 + 明细（最多 200 行）+ preview_token。
func (h *UscMap) DiffPreview(c *gin.Context) {
	preview, err := h.service.Preview(c.Request.Context(), c.Query("env"))
	if err != nil {
		log.Printf("[usc] api preview failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "预演失败: " + err.Error()})
		return
	}
	log.Printf("[usc] api preview env=%s insert=%d update=%d retire=%d reuse=%d escalate=%d token=%s",
		preview.Env, preview.Insert, preview.Update, preview.Retire, preview.Reuse,
		preview.Escalate, preview.PreviewToken)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "预演完成", "data": preview})
}

// ScheduleGet GET /Meta/Finv/Quote/UscMap/Schedule。
func (h *UscMap) ScheduleGet(c *gin.Context) {
	cfg, err := h.service.LoadSchedule(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": cfg})
}

// ScheduleSave POST /Meta/Finv/Quote/UscMap/Schedule。
//
// 请求体：{"cron_expr":"0 30 3 * * *"}；传空串表示**关闭**自动同步（仍可手动执行）。
func (h *UscMap) ScheduleSave(c *gin.Context) {
	var req struct {
		CronExpr string `json:"cron_expr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	cfg, err := h.service.SaveSchedule(c.Request.Context(), req.CronExpr, operator)
	if err != nil {
		log.Printf("[usc] api schedule save failed expr=%q err=%v", req.CronExpr, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	log.Printf("[usc] api schedule save ok expr=%q enable=%v operator=%s", cfg.CronExpr, cfg.Enable, operator)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存", "data": cfg})
}

// DiffPage GET /Meta/Finv/Quote/UscMap/Diff/Page。
//
// 分页浏览最近一次预演的完整明细（预演不落库，明细缓存在内存，30 分钟或重新预演后失效）。
// 参数：token（预演令牌，可选）、diff（insert/update/retire/reuse，空=全部）、page、pageSize。
func (h *UscMap) DiffPage(c *gin.Context) {
	page, pageSize := parseCamelPager(c)
	rows, total, token, err := h.service.DiffPage(c.Query("token"), c.Query("diff"), page, pageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"total": total, "page": page, "pageSize": pageSize,
			"preview_token": token, "list": rows,
		},
	})
}

// StartSyncRun POST /Meta/Finv/Quote/UscMap/Sync/Run。
//
// 请求体：{"env":"*","preview_token":"...","operator":"web","reason":"..."}（都可省略）。
func (h *UscMap) StartSyncRun(c *gin.Context) {
	var req struct {
		Env          string `json:"env"`
		PreviewToken string `json:"preview_token"`
		Reason       string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误: " + err.Error()})
		return
	}
	operator := ""
	if session, ok := SessionFrom(c); ok && session != nil {
		operator = session.UID
	}
	run, err := h.runner.Start(uscrule.RunRequest{
		Env:          req.Env,
		PreviewToken: req.PreviewToken,
		Operator:     operator,
		Reason:       req.Reason,
		Trigger:      uscrule.TriggerManual,
	})
	if err != nil {
		log.Printf("[usc] api sync start rejected err=%v", err)
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	log.Printf("[usc] api sync start run_id=%s env=%s operator=%s", run.RunID, run.Env, run.Operator)
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "同步已启动", "data": run})
}

// SyncRunStatus GET /Meta/Finv/Quote/UscMap/Sync/Run。
func (h *UscMap) SyncRunStatus(c *gin.Context) {
	runID := c.Query("runId")
	run, ok := h.runner.Status(runID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "运行不存在或已过期: " + runID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": run})
}

// LogList GET /Meta/Finv/Quote/UscMap/Log/List。
func (h *UscMap) LogList(c *gin.Context) {
	page, pageSize := parseCamelPager(c)
	logs, total, err := h.service.ListLogs(c.Request.Context(), page, pageSize)
	if err != nil {
		log.Printf("[usc] api log list failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询审计日志失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "page": page, "pageSize": pageSize, "list": logs},
	})
}
