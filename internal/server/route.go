package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (h *HTTPServer) setupRoutes() {
	h.router.Use(h.httpMetrics.Middleware())

	h.router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "server is running",
		})
	})
	h.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	h.router.POST("/api/v1.0/recipient/:recipient/notify", h.handler.NotifyHandler)
}
