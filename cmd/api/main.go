package main

import (
	"github.com/koungkub/fw-challenge-notification-service/internal/client"
	"github.com/koungkub/fw-challenge-notification-service/internal/handler"
	"github.com/koungkub/fw-challenge-notification-service/internal/metrics"
	"github.com/koungkub/fw-challenge-notification-service/internal/repository"
	"github.com/koungkub/fw-challenge-notification-service/internal/server"
	"github.com/koungkub/fw-challenge-notification-service/internal/service"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	fx.New(
		fx.Provide(func() *zap.Logger { return logger }),
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		metrics.Module,
		server.Module,
		handler.Module,
		service.Module,
		repository.Module,
		client.Module,
		fx.Invoke(func(*server.HTTPServer) {}),
	).Run()
}
