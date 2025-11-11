package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
)

const (
	cacheKeyPattern = "notification:preferences:%s"
)

//go:generate mockgen -package mockrepository -destination ./mock/mockcache.go . CacheProvider
type CacheProvider interface {
	Get(key NotificationProvider) ([]NotificationPreference, error)
	Set(key NotificationProvider, values []NotificationPreference) error
}

var _ CacheProvider = (*Cache)(nil)

type Cache struct {
	engine      *ristretto.Cache[string, []NotificationPreference]
	expiredTime time.Duration
}

type CacheParams struct {
	fx.In

	Config CacheConfig
}

func NewCache(lc fx.Lifecycle, params CacheParams) (*Cache, error) {
	engine, err := ristretto.NewCache(&ristretto.Config[string, []NotificationPreference]{
		NumCounters: params.Config.NumCounters,
		MaxCost:     params.Config.MaxCost,
		BufferItems: params.Config.BufferItems,
	})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			engine.Close()
			return nil
		},
	})

	return &Cache{
		engine: engine,
	}, nil
}

type CacheConfig struct {
	ExpiredTime time.Duration `envconfig:"CACHE_EXPIRED_TIME" default:"10m"`
	NumCounters int64         `envconfig:"CACHE_NUM_COUNTERS" default:"10000000"`
	MaxCost     int64         `envconfig:"CACHE_MAX_COST" default:"1073741824"` // 1GB
	BufferItems int64         `envconfig:"CACHE_BUFFER_ITEMS" default:"64"`
}

func NewCacheConfig() CacheConfig {
	var cfg CacheConfig
	envconfig.MustProcess("", &cfg)

	return cfg
}

func (c *Cache) Get(key NotificationProvider) ([]NotificationPreference, error) {
	cacheKey := fmt.Sprintf(cacheKeyPattern, key.String())

	value, found := c.engine.Get(cacheKey)
	if !found {
		return nil, fmt.Errorf("cache key: '%s' not found", cacheKey)
	}
	return value, nil
}

func (c *Cache) Set(key NotificationProvider, values []NotificationPreference) error {
	cacheKey := fmt.Sprintf(cacheKeyPattern, key.String())

	c.engine.SetWithTTL(cacheKey, values, 1, c.expiredTime)
	return nil
}
