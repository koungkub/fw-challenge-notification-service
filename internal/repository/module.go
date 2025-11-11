package repository

import "go.uber.org/fx"

var Module = fx.Module("repository",
	persistentModule,
	cacheModule,
)

var (
	persistentModule = fx.Provide(
		fx.Annotate(
			NewPersistent,
			fx.As(new(PersistentProvider)),
		),
		NewPersistentConfig,
	)

	cacheModule = fx.Provide(
		fx.Annotate(
			NewCache,
			fx.As(new(CacheProvider)),
		),
		NewCacheConfig,
	)
)
