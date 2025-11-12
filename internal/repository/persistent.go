package repository

import (
	"context"
	"fmt"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

//go:generate mockgen -package mockrepository -destination ./mock/mockpersistent.go . PersistentProvider
type PersistentProvider interface {
	FindByProviderType(ctx context.Context, provider NotificationProvider) ([]NotificationPreference, error)
}

var _ PersistentProvider = (*Persistent)(nil)

type Persistent struct {
	conn   *gorm.DB
	logger *zap.Logger
}

type PersistentParams struct {
	fx.In

	Config PersistentConfig
	Logger *zap.Logger
}

func NewPersistent(lc fx.Lifecycle, params PersistentParams) (*Persistent, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		params.Config.Host,
		params.Config.Username,
		params.Config.Password,
		params.Config.Name,
		params.Config.Port,
		params.Config.SSLMode,
	)

	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			sqlDB, _ := conn.DB()
			return sqlDB.Close()
		},
	})

	return &Persistent{
		conn:   conn,
		logger: params.Logger,
	}, nil
}

type PersistentConfig struct {
	Host     string `envconfig:"DB_HOST" required:"true"`
	Port     string `envconfig:"DB_PORT" required:"true"`
	Name     string `envconfig:"DB_NAME" required:"true"`
	Username string `envconfig:"DB_USERNAME" required:"true"`
	Password string `envconfig:"DB_PASSWORD" required:"true"`
	SSLMode  string `envconfig:"DB_SSLMODE" default:"disable"`
}

func NewPersistentConfig() PersistentConfig {
	var cfg PersistentConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}

func (p *Persistent) FindByProviderType(ctx context.Context, provider NotificationProvider) ([]NotificationPreference, error) {
	preferences, err := gorm.
		G[NotificationPreference](p.conn).
		Where("provider_type = ?", provider.String()).
		Where("deleted_at IS NULL").
		Order("priority").
		Find(ctx)
	if err != nil {
		p.logger.Error("database query failed",
			zap.String("provider_type", provider.String()),
			zap.Error(err),
		)
		return []NotificationPreference{}, err
	}
	if len(preferences) == 0 {
		p.logger.Warn("no preferences found for provider type",
			zap.String("provider_type", provider.String()),
		)
		return []NotificationPreference{}, gorm.ErrRecordNotFound
	}

	return preferences, nil
}
