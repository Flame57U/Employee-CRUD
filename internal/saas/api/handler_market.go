package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/itick"
)

// MarketHandler exposes read-only market data sourced from itick.org. The token
// lives server-side; the browser only ever talks to this endpoint.
type MarketHandler struct {
	itick *itick.Client
}

// NewMarketHandler constructs a MarketHandler. client may be nil/disabled when
// ITICK_TOKEN is unset; Quote degrades gracefully in that case.
func NewMarketHandler(client *itick.Client) *MarketHandler {
	return &MarketHandler{itick: client}
}

// RegisterRoutes mounts /market on r (the authenticated /api/v1 group).
func (h *MarketHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/market")
	g.GET("/quote", h.Quote)
}

// Quote godoc
// GET /api/v1/market/quote?asset=forex&region=gb&code=GBPUSD
func (h *MarketHandler) Quote(c *gin.Context) {
	if !h.itick.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "市场数据未配置（服务端未设置 ITICK_TOKEN）",
		})
		return
	}
	asset := c.DefaultQuery("asset", "forex")
	region := c.Query("region")
	code := c.Query("code")
	if region == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "region 与 code 为必填项"})
		return
	}

	q, err := h.itick.Quote(c.Request.Context(), asset, region, code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, q)
}
