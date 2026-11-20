package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// 本文件实现「缺省汇率」两个接口（需求 10999）：
//
//	POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync  手动更新（同步执行，一次 get-fx-overview）
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer      整表查询（88 行量级，不分页）
//
// 数据表：public.finv_fxcer（V300124 创建，需求 11002；一行=一个货币对的最新汇率，主键 usc）。
// 约定：URL 查询参数小驼峰、JSON 体字段 snake_case（ApiSpec §3）；
// 统计字段用动词原形 `rows_receive` / `insert` / `update`（ApiSpec §1.1）。
// 日志前缀 `[cdeu][fxcer]`，线上排查：`docker logs <容器> 2>&1 | grep '\[cdeu\]\[fxcer\]'`。

// FxCERSync POST /Meta/Finv/Quote/Cdeu/Fx/CerSync?source=overview|rateList。
//
// 无请求体；`source` 决定采集来源（需求 11001）：
//   - overview（默认）：get-fx-overview，9 币种两两完整矩阵（72 对），只落直采行（需求 11020 起不再派生 CNY）；
//   - rateList：get-fx-rate-list（listId=1、page=0、pageSize=500），币种更多、补齐长尾货币对。
//
// 全量 upsert 后返回本次实际写入统计；source 非法返回 400，上游失败/口径异常返回 500 + code 2006。
func (h *ExportArchive) FxCERSync(c *gin.Context) {
	source := strings.TrimSpace(c.Query("source"))
	if source != "" && source != cdeu.FxCERSourceOverview && source != cdeu.FxCERSourceRateList {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "source 非法（仅支持 overview / rateList）: " + source,
		})
		return
	}
	result, err := h.service.SyncFxCER(c.Request.Context(), source)
	if err != nil {
		log.Printf("[cdeu][fxcer] api sync failed source=%q err=%v", source, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "汇率更新失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][fxcer] api sync ok source=%s rows=%d derive=%d insert=%d update=%d elapsed_ms=%d",
		result.Source, result.RowsReceive, result.Derive, result.Insert, result.Update, result.ElapsedMs)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "汇率已更新",
		"data":    result,
	})
}

// FxCERList GET /Meta/Finv/Quote/Cdeu/Fx/Cer。
//
// 返回整表：data.total 为行数、data.list 为明细（已按源/目标币种排序）。
// 表为空表示从未手动更新过，前端据此提示先点「立即更新」。
func (h *ExportArchive) FxCERList(c *gin.Context) {
	rows, err := h.service.FxCERList(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][fxcer] api list failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询缺省汇率失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data": gin.H{
			"total": len(rows),
			"list":  rows,
			// currency_names 币种简体中文名（展示用，需求 11000/11002）：
			// 先按内置表，再用行内 name（「美元/日元」）反推补齐——rateList 带来的
			// 长尾币种没有内置名，靠这里补上；仍未命中的由前端回落为代码。
			"currency_names": fxCerCurrencyNames(rows),
		},
	})
}

// fxCerCurrencyNames 汇总币种中文名：内置表优先，行内 name 反推补齐。
//
// 行内 name 形如「美元/日元」（来源：get-fx-rate-list 的 name 或服务端合成），
// 按「/」拆成两段分别对应 currency_src / currency_dest；拆不出两段则忽略该行。
func fxCerCurrencyNames(rows []cdeu.FxCERRow) map[string]string {
	names := make(map[string]string, len(cdeu.FxCERCurrencyNames)+len(rows))
	for code, name := range cdeu.FxCERCurrencyNames {
		names[code] = name
	}
	for _, row := range rows {
		parts := strings.SplitN(row.Name, "/", 2)
		if len(parts) != 2 {
			continue
		}
		srcName := strings.TrimSpace(parts[0])
		destName := strings.TrimSpace(parts[1])
		if _, ok := names[row.CurrencySrc]; !ok && srcName != "" {
			names[row.CurrencySrc] = srcName
		}
		if _, ok := names[row.CurrencyDest]; !ok && destName != "" {
			names[row.CurrencyDest] = destName
		}
	}
	return names
}
