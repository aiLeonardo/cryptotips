package app

import (
	"fmt"
	"strconv"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"

	"github.com/gin-gonic/gin"
)

// FearGreedItem 返回给前端的单条恐慌贪婪指数记录
type FearGreedItem struct {
	Value               int    `json:"value"`
	ValueClassification string `json:"value_classification"`
	Timestamp           int64  `json:"timestamp"`
}

// FearGreedHistoryResp 响应体
type FearGreedHistoryResp struct {
	Current *FearGreedItem  `json:"current"`
	History []FearGreedItem `json:"history"`
}

// getFearGreedHistory 返回恐慌贪婪指数历史
// GET /api/feargreed/history?limit=90
func (a *goapi) getFearGreedHistory(c *gin.Context) {
	limit := 2920
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 5840 {
		limit = 5840
	}

	m := &models.FearGreedIndex{}
	records, err := m.GetHistory(a.db, limit)
	if err != nil {
		lib.JsonError(c, fmt.Errorf("查询贪婪恐慌指数失败: %w", err))
		return
	}

	items := make([]FearGreedItem, 0, len(records))
	for _, r := range records {
		items = append(items, FearGreedItem{
			Value:               r.Value,
			ValueClassification: r.ValueClassification,
			Timestamp:           r.FngTimestamp,
		})
	}

	var current *FearGreedItem
	if len(items) > 0 {
		current = &items[0]
	}

	lib.JsonResponse(c, FearGreedHistoryResp{
		Current: current,
		History: items,
	})
}
