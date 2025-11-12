package service

import (
	"context"
	"errors"

	"github.com/koungkub/fw-challenge-notification-service/internal/client"
	"github.com/koungkub/fw-challenge-notification-service/internal/repository"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"
)

var Module = fx.Module("service",
	fx.Provide(
		fx.Annotate(
			NewNotificationService,
			fx.As(new(NotificationProvider)),
		),
	),
)

//go:generate mockgen -package mockservice -destination ./mock/mockservice.go . NotificationProvider
type NotificationProvider interface {
	SendToSeller(ctx context.Context, to string, title string, message string) error
	SendToBuyer(ctx context.Context, to string, title string, message string) error
}

var _ NotificationProvider = (*NotificationService)(nil)

type NotificationService struct {
	cacheProvider      repository.CacheProvider
	persistentProvider repository.PersistentProvider
	httpclient         client.HTTPClientProvider
}

type NotificationServiceParams struct {
	fx.In

	CacheProvider      repository.CacheProvider
	PersistentProvider repository.PersistentProvider
	HTTPclient         client.HTTPClientProvider
}

func NewNotificationService(params NotificationServiceParams) *NotificationService {
	return &NotificationService{
		cacheProvider:      params.CacheProvider,
		persistentProvider: params.PersistentProvider,
		httpclient:         params.HTTPclient,
	}
}

func (s *NotificationService) SendToSeller(ctx context.Context, to string, title string, message string) error {
	req := client.NotificationRequest{
		To:      to,
		Title:   title,
		Message: message,
	}
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		preferences, err := s.getNotificationPreferences(ctx, repository.EmailProvider)
		if err != nil {
			return err
		}

		if err := s.sendNotification(ctx, preferences, req); err != nil {
			return err
		}
		return nil
	})

	g.Go(func() error {
		preferences, err := s.getNotificationPreferences(ctx, repository.PushNotificationProvider)
		if err != nil {
			return err
		}

		if err := s.sendNotification(ctx, preferences, req); err != nil {
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (s *NotificationService) SendToBuyer(ctx context.Context, to string, title string, message string) error {
	req := client.NotificationRequest{
		To:      to,
		Title:   title,
		Message: message,
	}

	preferences, err := s.getNotificationPreferences(ctx, repository.EmailProvider)
	if err != nil {
		return err
	}

	if err := s.sendNotification(ctx, preferences, req); err != nil {
		return err
	}

	return nil
}

func (s *NotificationService) getNotificationPreferences(
	ctx context.Context,
	providerType repository.NotificationProvider,
) ([]repository.NotificationPreference, error) {
	var (
		preferences []repository.NotificationPreference
		err         error
	)

	preferences, err = s.cacheProvider.Get(providerType)
	if err == nil {
		return preferences, nil
	}

	preferences, err = s.persistentProvider.FindByProviderType(ctx, providerType)
	if err != nil {
		return []repository.NotificationPreference{}, err
	}

	s.cacheProvider.Set(providerType, preferences)
	return preferences, nil
}

func (s *NotificationService) sendNotification(
	ctx context.Context,
	preferences []repository.NotificationPreference,
	req client.NotificationRequest,
) error {
	for _, preference := range preferences {
		req.SecretKey = preference.SecretKey
		if err := s.httpclient.Post(ctx, preference.Host, req); err != nil {
			continue
		}
		return nil
	}
	return errors.New("failure to sent the notifications")
}
