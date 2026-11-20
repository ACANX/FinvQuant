package handler

// 证券指标 Upsert 的对外入口（需求 11248）。
//
// 路径：POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert
// 数据表：cdeu.finv_quote_secu_indicator + futu.finv_futu_quote_collect + futu.finv_futu_secu_key
//
// 与内部接口的区别：本接口只注册在 openGroup（对外 OpenAPI），没有对应的 /API/V1 内部路由 ——
// 它的调用方只有采集端，人不需要从这里写数据（内部写入口是「待录入证券」「证券参数补全」）。
//
// 日志前缀 [cdeu][secuindicator]，线上排查用
// `docker logs <容器> 2>&1 | grep '\[cdeu\]\[secuindicator\]'`。

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// FTSecuIndicatorUpsert POST /Meta/Finv/Quote/Cdeu/Realtime/FTSecuIndicatorUpsert。
//
// 采集端推一只证券的完整指标过来，服务端按 usc 关联写三张表并把证券键置 DONE。
// 幂等：同一只证券重复推送只会覆盖同一行（indicator / collect 主键都是 usc）。
func (h *ExportArchive) FTSecuIndicatorUpsert(c *gin.Context) {
	started := time.Now()

	body := http.MaxBytesReader(c.Writer, c.Request.Body, realtimeStockMaxBodyBytes)
	decoder := json.NewDecoder(body)
	// 采集端可能在体里带 $workers / $metadata 等额外节点，这里**不**用 DisallowUnknownFields：
	// 上游加字段不应让整批数据被拒（与内部接口的严格模式不同）。
	var req cdeu.SecuIndicatorUpsertRequest
	if err := decoder.Decode(&req); err != nil {
		log.Printf("[cdeu][secuindicator] api upsert bad body err=%v client=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体只能包含一个 JSON 对象"})
		return
	}

	if err := cdeu.ValidateSecuIndicatorUpsert(&req); err != nil {
		log.Printf("[cdeu][secuindicator] api upsert invalid err=%v client=%s", err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	log.Printf("[cdeu][secuindicator] api upsert begin symbol=%q run_id=%q client=%s",
		req.Symbol, req.RunID, c.ClientIP())

	result, err := h.service.Store().UpsertSecuIndicator(c.Request.Context(), &req)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][secuindicator] api upsert failed symbol=%q err=%v elapsed_ms=%d",
			req.Symbol, err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}

	log.Printf("[cdeu][secuindicator] api upsert end usc=%s ftmm_symbol=%s usc_source=%s "+
		"indicator=%s collect=%s key=%s elapsed_ms=%d",
		result.USC, result.FtmmSymbol, result.USCSource,
		result.Indicator, result.Collect, result.Key, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "写入完成", "data": result})
}
