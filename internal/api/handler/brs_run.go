// BRS 备份执行处理器：调度配置（sys.sys_config 39001）+ 手动触发 + 实时运行状态与日志。
//
// URL 查询参数遵循小驼峰（Docs/DevSpec/ApiSpec.md §3）：`sinceSeq`。
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/brs"
)

// BrsRun BRS 备份执行处理器。
type BrsRun struct {
	runner *brs.Runner
}

// NewBrsRun 创建备份执行处理器。
func NewBrsRun(runner *brs.Runner) *BrsRun {
	return &BrsRun{runner: runner}
}

// GetSchedule 读取每日备份调度配置（GET /API/V1/Meta/BRS/Backup/Finv/Schedule/Get）。
func (h *BrsRun) GetSchedule(c *gin.Context) {
	cfg, err := h.runner.LoadSchedule(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取备份调度配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": cfg})
}

// SaveSchedule 保存调度配置（POST /API/V1/Meta/BRS/Backup/Finv/Schedule/Save）。
//
// 表达式为 6 段 cron（秒 分 时 日 月 周）；保存后调度器最多 60 秒内重新加载。
func (h *BrsRun) SaveSchedule(c *gin.Context) {
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
	cfg, err := h.runner.SaveSchedule(c.Request.Context(), req.CronExpr, operator)
	if err != nil {
		if isBrsInputError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存备份调度配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": cfg})
}

// TriggerManual 手动触发一轮备份（POST /API/V1/Meta/BRS/Backup/Finv/Run/Manual）。
//
// 异步执行，立即返回本次运行的状态快照；已有任务在跑时返回 409。
func (h *BrsRun) TriggerManual(c *gin.Context) {
	state, err := h.runner.TriggerManual()
	if err != nil {
		var busy *brs.BusyError
		if errors.As(err, &busy) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": busy.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "触发备份失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已开始备份", "data": state})
}

// Status 查询运行状态与日志（GET /API/V1/Meta/BRS/Backup/Finv/Run/Status）。
//
// `sinceSeq` 为增量拉取起点：只返回 seq 大于该值的日志；省略或 0 表示返回全部（最多 1000 条）。
func (h *BrsRun) Status(c *gin.Context) {
	sinceSeq, _ := strconv.ParseInt(c.DefaultQuery("sinceSeq", "0"), 10, 64)
	state := h.runner.Status(sinceSeq)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": state})
}
