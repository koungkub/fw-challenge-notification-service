package server

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"github.com/koungkub/fw-challenge-notification-service/internal/handler"
	"github.com/koungkub/fw-challenge-notification-service/internal/metrics"
	"go.uber.org/fx"
)

var Module = fx.Module("http_server",
	fx.Provide(
		NewHTTP,
		NewConfig,
	),
)

type HTTPParams struct {
	fx.In

	Config      HTTPConfig
	Handler     *handler.Notification
	HTTPMetrics *metrics.HTTPServerCollector
}

type HTTPServer struct {
	router *gin.Engine
	srv    *http.Server

	handler     *handler.Notification
	httpMetrics *metrics.HTTPServerCollector
}

func NewHTTP(lc fx.Lifecycle, params HTTPParams) *HTTPServer {
	router := gin.New()
	router.Use(gin.Recovery())

	httpServer := &HTTPServer{
		router: router,
		srv: &http.Server{
			Addr:    params.Config.Port,
			Handler: router,
		},
		httpMetrics: params.HTTPMetrics,
		handler:     params.Handler,
	}

	httpServer.setupRoutes()

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", httpServer.srv.Addr)
			if err != nil {
				return err
			}
			// log.Info("Starting HTTP server", zap.String("addr", srv.Addr))
			go httpServer.srv.Serve(ln)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return httpServer.srv.Shutdown(ctx)
		},
	})

	return httpServer
}

type HTTPConfig struct {
	Port string `envconfig:"HTTP_SERVER_PORT" default:":8080"`
}

func NewConfig() HTTPConfig {
	var cfg HTTPConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}
