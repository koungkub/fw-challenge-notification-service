package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/koungkub/fw-challenge-notification-service/internal/client"
	mockclient "github.com/koungkub/fw-challenge-notification-service/internal/client/mock"
	"github.com/koungkub/fw-challenge-notification-service/internal/repository"
	mockrepository "github.com/koungkub/fw-challenge-notification-service/internal/repository/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewNotificationService(t *testing.T) {
	t.Run("creates service with all dependencies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockCache := mockrepository.NewMockCacheProvider(ctrl)
		mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
		mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

		service := NewNotificationService(NotificationServiceParams{
			CacheProvider:      mockCache,
			PersistentProvider: mockPersistent,
			HTTPclient:         mockHTTPClient,
		})

		assert.NotNil(t, service)
		assert.Equal(t, mockCache, service.cacheProvider)
		assert.Equal(t, mockPersistent, service.persistentProvider)
		assert.Equal(t, mockHTTPClient, service.httpclient)
	})
}

func TestNotificationService_SendToBuyer(t *testing.T) {
	tests := []struct {
		name           string
		to             string
		title          string
		message        string
		setupMocks     func(*mockrepository.MockCacheProvider, *mockrepository.MockPersistentProvider, *mockclient.MockHTTPClientProvider)
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:    "successful send with cache hit",
			to:      "buyer@example.com",
			title:   "Order Confirmation",
			message: "Your order has been confirmed",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "secret1"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(preferences, nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", client.NotificationRequest{
					To:        "buyer@example.com",
					Title:     "Order Confirmation",
					Message:   "Your order has been confirmed",
					SecretKey: "secret1",
				}).Return(nil)
			},
			expectedError: false,
		},
		{
			name:    "successful send with cache miss",
			to:      "buyer@example.com",
			title:   "Order Confirmation",
			message: "Your order has been confirmed",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "secret1"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(preferences, nil)
				cache.EXPECT().Set(repository.EmailProvider, preferences).Return(nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", client.NotificationRequest{
					To:        "buyer@example.com",
					Title:     "Order Confirmation",
					Message:   "Your order has been confirmed",
					SecretKey: "secret1",
				}).Return(nil)
			},
			expectedError: false,
		},
		{
			name:    "fails when preferences not found",
			to:      "buyer@example.com",
			title:   "Order Confirmation",
			message: "Your order has been confirmed",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(nil, errors.New("database error"))
			},
			expectedError:  true,
			expectedErrMsg: "database error",
		},
		{
			name:    "succeeds on first preference",
			to:      "buyer@example.com",
			title:   "Order Confirmation",
			message: "Your order has been confirmed",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "secret1"},
					{Host: "https://email-service2.com", SecretKey: "secret2"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(preferences, nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockCache, mockPersistent, mockHTTPClient)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			err := service.SendToBuyer(context.Background(), tt.to, tt.title, tt.message)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotificationService_SendToSeller(t *testing.T) {
	tests := []struct {
		name           string
		to             string
		title          string
		message        string
		setupMocks     func(*mockrepository.MockCacheProvider, *mockrepository.MockPersistentProvider, *mockclient.MockHTTPClientProvider)
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:    "successful send with both email and push notification",
			to:      "seller@example.com",
			title:   "New Order",
			message: "You have a new order",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				emailPreferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "email-secret"},
				}
				pushPreferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(emailPreferences, nil)
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(pushPreferences, nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", gomock.Any()).Return(nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://push-service.com", gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name:    "fails when email preferences fetch fails",
			to:      "seller@example.com",
			title:   "New Order",
			message: "You have a new order",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				pushPreferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(pushPreferences, nil)
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(nil, errors.New("database error"))
				httpClient.EXPECT().Post(gomock.Any(), "https://push-service.com", gomock.Any()).Return(nil)
			},
			expectedError:  true,
			expectedErrMsg: "database error",
		},
		{
			name:    "fails when push notification preferences fetch fails",
			to:      "seller@example.com",
			title:   "New Order",
			message: "You have a new order",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(nil, errors.New("email db error"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.PushNotificationProvider).Return(nil, errors.New("push db error"))
			},
			expectedError:  true,
			expectedErrMsg: "db error",
		},
		{
			name:    "succeeds when email notification succeeds",
			to:      "seller@example.com",
			title:   "New Order",
			message: "You have a new order",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				emailPreferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "email-secret"},
				}
				pushPreferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(emailPreferences, nil)
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(pushPreferences, nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", gomock.Any()).Return(nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://push-service.com", gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name:    "successful with cache miss and DB fetch",
			to:      "seller@example.com",
			title:   "New Order",
			message: "You have a new order",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				emailPreferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "email-secret"},
				}
				pushPreferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(emailPreferences, nil)
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.PushNotificationProvider).Return(pushPreferences, nil)
				cache.EXPECT().Set(repository.EmailProvider, emailPreferences).Return(nil)
				cache.EXPECT().Set(repository.PushNotificationProvider, pushPreferences).Return(nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://email-service.com", gomock.Any()).Return(nil)
				httpClient.EXPECT().Post(gomock.Any(), "https://push-service.com", gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockCache, mockPersistent, mockHTTPClient)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			err := service.SendToSeller(context.Background(), tt.to, tt.title, tt.message)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotificationService_getNotificationPreferences(t *testing.T) {
	tests := []struct {
		name           string
		providerType   repository.NotificationProvider
		setupMocks     func(*mockrepository.MockCacheProvider, *mockrepository.MockPersistentProvider)
		expectedPrefs  []repository.NotificationPreference
		expectedError  bool
		expectedErrMsg string
		verifyCacheSet bool
	}{
		{
			name:         "returns preferences from cache",
			providerType: repository.EmailProvider,
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "secret1"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(preferences, nil)
			},
			expectedPrefs: []repository.NotificationPreference{
				{Host: "https://email-service.com", SecretKey: "secret1"},
			},
			expectedError:  false,
			verifyCacheSet: false,
		},
		{
			name:         "fetches from database on cache miss and sets cache",
			providerType: repository.PushNotificationProvider,
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.PushNotificationProvider).Return(preferences, nil)
				cache.EXPECT().Set(repository.PushNotificationProvider, preferences).Return(nil)
			},
			expectedPrefs: []repository.NotificationPreference{
				{Host: "https://push-service.com", SecretKey: "push-secret"},
			},
			expectedError:  false,
			verifyCacheSet: true,
		},
		{
			name:         "returns error when database fetch fails",
			providerType: repository.EmailProvider,
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider) {
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(nil, errors.New("database connection error"))
			},
			expectedPrefs:  []repository.NotificationPreference{},
			expectedError:  true,
			expectedErrMsg: "database connection error",
			verifyCacheSet: false,
		},
		{
			name:         "returns empty preferences from database and sets cache",
			providerType: repository.EmailProvider,
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider) {
				preferences := []repository.NotificationPreference{}
				cache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(preferences, nil)
				cache.EXPECT().Set(repository.EmailProvider, preferences).Return(nil)
			},
			expectedPrefs:  []repository.NotificationPreference{},
			expectedError:  false,
			verifyCacheSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockCache, mockPersistent)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			prefs, err := service.getNotificationPreferences(context.Background(), tt.providerType)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPrefs, prefs)
			}
		})
	}
}

func TestNotificationService_sendNotification(t *testing.T) {
	tests := []struct {
		name           string
		preferences    []repository.NotificationPreference
		request        client.NotificationRequest
		setupMocks     func(*mockclient.MockHTTPClientProvider)
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "returns nil on first success",
			preferences: []repository.NotificationPreference{
				{Host: "https://service1.com", SecretKey: "secret1"},
				{Host: "https://service2.com", SecretKey: "secret2"},
			},
			request: client.NotificationRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(httpClient *mockclient.MockHTTPClientProvider) {
				httpClient.EXPECT().Post(gomock.Any(), "https://service1.com", client.NotificationRequest{
					To:        "user@example.com",
					Title:     "Test",
					Message:   "Test message",
					SecretKey: "secret1",
				}).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "tries next preference on error and succeeds",
			preferences: []repository.NotificationPreference{
				{Host: "https://service1.com", SecretKey: "secret1"},
				{Host: "https://service2.com", SecretKey: "secret2"},
			},
			request: client.NotificationRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(httpClient *mockclient.MockHTTPClientProvider) {
				httpClient.EXPECT().Post(gomock.Any(), "https://service1.com", client.NotificationRequest{
					To:        "user@example.com",
					Title:     "Test",
					Message:   "Test message",
					SecretKey: "secret1",
				}).Return(errors.New("connection failed"))
				httpClient.EXPECT().Post(gomock.Any(), "https://service2.com", client.NotificationRequest{
					To:        "user@example.com",
					Title:     "Test",
					Message:   "Test message",
					SecretKey: "secret2",
				}).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "returns error when all HTTP requests fail",
			preferences: []repository.NotificationPreference{
				{Host: "https://service1.com", SecretKey: "secret1"},
				{Host: "https://service2.com", SecretKey: "secret2"},
			},
			request: client.NotificationRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(httpClient *mockclient.MockHTTPClientProvider) {
				httpClient.EXPECT().Post(gomock.Any(), "https://service1.com", client.NotificationRequest{
					To:        "user@example.com",
					Title:     "Test",
					Message:   "Test message",
					SecretKey: "secret1",
				}).Return(errors.New("connection failed"))
				httpClient.EXPECT().Post(gomock.Any(), "https://service2.com", client.NotificationRequest{
					To:        "user@example.com",
					Title:     "Test",
					Message:   "Test message",
					SecretKey: "secret2",
				}).Return(errors.New("connection failed"))
			},
			expectedError:  true,
			expectedErrMsg: "failure to sent the notifications",
		},
		{
			name:        "returns error for empty preferences",
			preferences: []repository.NotificationPreference{},
			request: client.NotificationRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(httpClient *mockclient.MockHTTPClientProvider) {
				// No HTTP calls expected
			},
			expectedError:  true,
			expectedErrMsg: "failure to sent the notifications",
		},
		{
			name: "tries multiple preferences until success",
			preferences: []repository.NotificationPreference{
				{Host: "https://service1.com", SecretKey: "secret1"},
				{Host: "https://service2.com", SecretKey: "secret2"},
				{Host: "https://service3.com", SecretKey: "secret3"},
			},
			request: client.NotificationRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(httpClient *mockclient.MockHTTPClientProvider) {
				httpClient.EXPECT().Post(gomock.Any(), "https://service1.com", gomock.Any()).Return(errors.New("network error"))
				httpClient.EXPECT().Post(gomock.Any(), "https://service2.com", gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockHTTPClient)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			err := service.sendNotification(context.Background(), tt.preferences, tt.request)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotificationService_SendToBuyer_ContextCancellation(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mockrepository.MockCacheProvider, *mockrepository.MockPersistentProvider, *mockclient.MockHTTPClientProvider)
		cancelTiming  string
		expectedError bool
	}{
		{
			name: "context cancelled before getNotificationPreferences",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				cache.EXPECT().Get(repository.EmailProvider).DoAndReturn(func(key repository.NotificationProvider) ([]repository.NotificationPreference, error) {
					return nil, errors.New("cache miss")
				})
				persistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).DoAndReturn(func(ctx context.Context, provider repository.NotificationProvider) ([]repository.NotificationPreference, error) {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return nil, errors.New("context should be cancelled")
				})
			},
			cancelTiming:  "immediate",
			expectedError: true,
		},
		{
			name: "context cancelled during HTTP request",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				preferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "secret1"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(preferences, nil)
				httpClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, u string, reqBody client.NotificationRequest) error {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return nil
				})
			},
			cancelTiming:  "during_http",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockCache, mockPersistent, mockHTTPClient)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelTiming == "immediate" {
				cancel()
			} else {
				defer cancel()
			}

			err := service.SendToBuyer(ctx, "buyer@example.com", "Test", "Test message")

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotificationService_SendToSeller_ContextCancellation(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mockrepository.MockCacheProvider, *mockrepository.MockPersistentProvider, *mockclient.MockHTTPClientProvider)
		cancelAfter   time.Duration
		expectedError bool
	}{
		{
			name: "context cancelled before goroutines start",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				cache.EXPECT().Get(repository.EmailProvider).DoAndReturn(func(key repository.NotificationProvider) ([]repository.NotificationPreference, error) {
					return nil, errors.New("cache miss")
				}).AnyTimes()
				cache.EXPECT().Get(repository.PushNotificationProvider).DoAndReturn(func(key repository.NotificationProvider) ([]repository.NotificationPreference, error) {
					return nil, errors.New("cache miss")
				}).AnyTimes()
				persistent.EXPECT().FindByProviderType(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, provider repository.NotificationProvider) ([]repository.NotificationPreference, error) {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					return nil, errors.New("unexpected call")
				}).AnyTimes()
			},
			cancelAfter:   0,
			expectedError: true,
		},
		{
			name: "context cancelled during concurrent execution",
			setupMocks: func(cache *mockrepository.MockCacheProvider, persistent *mockrepository.MockPersistentProvider, httpClient *mockclient.MockHTTPClientProvider) {
				emailPreferences := []repository.NotificationPreference{
					{Host: "https://email-service.com", SecretKey: "email-secret"},
				}
				pushPreferences := []repository.NotificationPreference{
					{Host: "https://push-service.com", SecretKey: "push-secret"},
				}
				cache.EXPECT().Get(repository.EmailProvider).Return(emailPreferences, nil).AnyTimes()
				cache.EXPECT().Get(repository.PushNotificationProvider).Return(pushPreferences, nil).AnyTimes()
				httpClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, u string, reqBody client.NotificationRequest) error {
					time.Sleep(10 * time.Millisecond)
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return nil
				}).AnyTimes()
			},
			cancelAfter:   5 * time.Millisecond,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCache := mockrepository.NewMockCacheProvider(ctrl)
			mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
			mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

			tt.setupMocks(mockCache, mockPersistent, mockHTTPClient)

			service := NewNotificationService(NotificationServiceParams{
				CacheProvider:      mockCache,
				PersistentProvider: mockPersistent,
				HTTPclient:         mockHTTPClient,
			})

			ctx, cancel := context.WithCancel(context.Background())
			if tt.cancelAfter == 0 {
				cancel()
			} else {
				time.AfterFunc(tt.cancelAfter, cancel)
				defer cancel()
			}

			err := service.SendToSeller(ctx, "seller@example.com", "Test", "Test message")

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotificationService_getNotificationPreferences_ContextCancellation(t *testing.T) {
	t.Run("handles context cancellation during database fetch", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockCache := mockrepository.NewMockCacheProvider(ctrl)
		mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
		mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

		mockCache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
		mockPersistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).DoAndReturn(func(ctx context.Context, provider repository.NotificationProvider) ([]repository.NotificationPreference, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("context should be cancelled")
		})

		service := NewNotificationService(NotificationServiceParams{
			CacheProvider:      mockCache,
			PersistentProvider: mockPersistent,
			HTTPclient:         mockHTTPClient,
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := service.getNotificationPreferences(ctx, repository.EmailProvider)

		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestNotificationService_CacheSetError(t *testing.T) {
	t.Run("continues even if cache.Set fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockCache := mockrepository.NewMockCacheProvider(ctrl)
		mockPersistent := mockrepository.NewMockPersistentProvider(ctrl)
		mockHTTPClient := mockclient.NewMockHTTPClientProvider(ctrl)

		preferences := []repository.NotificationPreference{
			{Host: "https://email-service.com", SecretKey: "secret1"},
		}

		mockCache.EXPECT().Get(repository.EmailProvider).Return(nil, errors.New("cache miss"))
		mockPersistent.EXPECT().FindByProviderType(gomock.Any(), repository.EmailProvider).Return(preferences, nil)
		mockCache.EXPECT().Set(repository.EmailProvider, preferences).Return(errors.New("redis connection error"))
		mockHTTPClient.EXPECT().Post(gomock.Any(), "https://email-service.com", gomock.Any()).Return(nil)

		service := NewNotificationService(NotificationServiceParams{
			CacheProvider:      mockCache,
			PersistentProvider: mockPersistent,
			HTTPclient:         mockHTTPClient,
		})

		err := service.SendToBuyer(context.Background(), "buyer@example.com", "Test", "Test message")

		require.NoError(t, err)
	})
}
