package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mockservice "github.com/koungkub/fw-challenge-notification-service/internal/service/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewNotificationHandler(t *testing.T) {
	t.Run("creates handler with service dependency", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockService := mockservice.NewMockNotificationProvider(ctrl)

		handler := NewNotificationHandler(NotificationParams{
			Services: mockService,
		})

		assert.NotNil(t, handler)
		assert.Equal(t, mockService, handler.services)
	})
}

func TestNotification_NotifyHandler(t *testing.T) {
	tests := []struct {
		name               string
		recipient          string
		requestBody        any
		setupMocks         func(*mockservice.MockNotificationProvider)
		expectedStatusCode int
		expectedResponse   map[string]any
	}{
		{
			name:      "successful notification to buyer",
			recipient: RecipientTypeBuyer,
			requestBody: NotifyRequest{
				To:      "buyer@example.com",
				Title:   "Order Confirmation",
				Message: "Your order has been confirmed",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				mockService.EXPECT().SendToBuyer(
					gomock.Any(),
					"buyer@example.com",
					"Order Confirmation",
					"Your order has been confirmed",
				).Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse: map[string]any{
				"message": "nofitication sent",
			},
		},
		{
			name:      "successful notification to seller",
			recipient: RecipientTypeSeller,
			requestBody: NotifyRequest{
				To:      "seller@example.com",
				Title:   "New Order",
				Message: "You have a new order",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				mockService.EXPECT().SendToSeller(
					gomock.Any(),
					"seller@example.com",
					"New Order",
					"You have a new order",
				).Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedResponse: map[string]any{
				"message": "nofitication sent",
			},
		},
		{
			name:      "invalid JSON body",
			recipient: RecipientTypeBuyer,
			requestBody: map[string]any{
				"invalid": "data",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			expectedResponse: map[string]any{
				"error_code": "E101",
				"message":    "Key: 'NotifyRequest.To' Error:Field validation for 'To' failed on the 'required' tag\nKey: 'NotifyRequest.Title' Error:Field validation for 'Title' failed on the 'required' tag\nKey: 'NotifyRequest.Message' Error:Field validation for 'Message' failed on the 'required' tag",
			},
		},
		{
			name:      "missing required field - to",
			recipient: RecipientTypeBuyer,
			requestBody: map[string]any{
				"title":   "Test Title",
				"message": "Test Message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			expectedResponse: map[string]any{
				"error_code": "E101",
			},
		},
		{
			name:      "missing required field - title",
			recipient: RecipientTypeBuyer,
			requestBody: map[string]any{
				"to":      "test@example.com",
				"message": "Test Message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			expectedResponse: map[string]any{
				"error_code": "E101",
			},
		},
		{
			name:      "missing required field - message",
			recipient: RecipientTypeBuyer,
			requestBody: map[string]any{
				"to":    "test@example.com",
				"title": "Test Title",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected
			},
			expectedStatusCode: http.StatusUnprocessableEntity,
			expectedResponse: map[string]any{
				"error_code": "E101",
			},
		},
		{
			name:      "service error for buyer",
			recipient: RecipientTypeBuyer,
			requestBody: NotifyRequest{
				To:      "buyer@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				mockService.EXPECT().SendToBuyer(
					gomock.Any(),
					"buyer@example.com",
					"Test",
					"Test message",
				).Return(errors.New("service unavailable"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse: map[string]any{
				"error_code": "E102",
				"message":    "service unavailable",
			},
		},
		{
			name:      "service error for seller",
			recipient: RecipientTypeSeller,
			requestBody: NotifyRequest{
				To:      "seller@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				mockService.EXPECT().SendToSeller(
					gomock.Any(),
					"seller@example.com",
					"Test",
					"Test message",
				).Return(errors.New("database connection error"))
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse: map[string]any{
				"error_code": "E102",
				"message":    "database connection error",
			},
		},
		{
			name:      "unsupported recipient type",
			recipient: "admin",
			requestBody: NotifyRequest{
				To:      "admin@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedResponse: map[string]any{
				"error_code": "E102",
				"message":    "not supported recipient type",
			},
		},
		{
			name:      "empty recipient type returns 404",
			recipient: "",
			requestBody: NotifyRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			},
			setupMocks: func(mockService *mockservice.MockNotificationProvider) {
				// No service calls expected - route won't match
			},
			expectedStatusCode: http.StatusNotFound,
			expectedResponse:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := mockservice.NewMockNotificationProvider(ctrl)
			tt.setupMocks(mockService)

			handler := NewNotificationHandler(NotificationParams{
				Services: mockService,
			})

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/notify/:recipient", handler.NotifyHandler)

			bodyBytes, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/notify/"+tt.recipient, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)

			// Verify response if expected response is not empty
			if len(tt.expectedResponse) > 0 {
				var response map[string]any
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				for key, expectedValue := range tt.expectedResponse {
					actualValue, exists := response[key]
					assert.True(t, exists, "Expected key %s to exist in response", key)
					if exists {
						// For error messages, we may want to check if it contains the expected text
						if key == "message" && tt.expectedStatusCode == http.StatusUnprocessableEntity {
							assert.Contains(t, actualValue.(string), "Error:Field validation")
						} else {
							assert.Equal(t, expectedValue, actualValue, "Mismatch for key %s", key)
						}
					}
				}
			}
		})
	}
}

func TestNotification_NotifyHandler_InvalidJSON(t *testing.T) {
	t.Run("malformed JSON body", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockService := mockservice.NewMockNotificationProvider(ctrl)

		handler := NewNotificationHandler(NotificationParams{
			Services: mockService,
		})

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/notify/:recipient", handler.NotifyHandler)

		malformedJSON := []byte(`{"to": "test@example.com", "title": `)

		req := httptest.NewRequest(http.MethodPost, "/notify/buyer", bytes.NewReader(malformedJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Verify status code
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		// Verify error response
		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "E101", response["error_code"])
		assert.NotEmpty(t, response["message"])
	})
}

func TestNotification_NotifyHandler_ContextPropagation(t *testing.T) {
	t.Run("propagates context to service layer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockService := mockservice.NewMockNotificationProvider(ctrl)

		mockService.EXPECT().SendToBuyer(
			gomock.Any(),
			"buyer@example.com",
			"Test",
			"Test message",
		).DoAndReturn(func(ctx context.Context, to, title, message string) error {
			// Verify context is not nil
			assert.NotNil(t, ctx)
			return nil
		})

		handler := NewNotificationHandler(NotificationParams{
			Services: mockService,
		})

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/notify/:recipient", handler.NotifyHandler)

		requestBody := NotifyRequest{
			To:      "buyer@example.com",
			Title:   "Test",
			Message: "Test message",
		}
		bodyBytes, _ := json.Marshal(requestBody)

		req := httptest.NewRequest(http.MethodPost, "/notify/buyer", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestNotification_NotifyHandler_RecipientTypeCaseSensitive(t *testing.T) {
	tests := []struct {
		name               string
		recipient          string
		expectedStatusCode int
		expectServiceCall  bool
	}{
		{
			name:               "lowercase buyer",
			recipient:          "buyer",
			expectedStatusCode: http.StatusOK,
			expectServiceCall:  true,
		},
		{
			name:               "lowercase seller",
			recipient:          "seller",
			expectedStatusCode: http.StatusOK,
			expectServiceCall:  true,
		},
		{
			name:               "uppercase BUYER",
			recipient:          "BUYER",
			expectedStatusCode: http.StatusInternalServerError,
			expectServiceCall:  false,
		},
		{
			name:               "uppercase SELLER",
			recipient:          "SELLER",
			expectedStatusCode: http.StatusInternalServerError,
			expectServiceCall:  false,
		},
		{
			name:               "mixed case Buyer",
			recipient:          "Buyer",
			expectedStatusCode: http.StatusInternalServerError,
			expectServiceCall:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := mockservice.NewMockNotificationProvider(ctrl)

			if tt.expectServiceCall {
				switch tt.recipient {
				case "buyer":
					mockService.EXPECT().SendToBuyer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				case "seller":
					mockService.EXPECT().SendToSeller(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				}
			}

			handler := NewNotificationHandler(NotificationParams{
				Services: mockService,
			})

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.POST("/notify/:recipient", handler.NotifyHandler)

			requestBody := NotifyRequest{
				To:      "user@example.com",
				Title:   "Test",
				Message: "Test message",
			}
			bodyBytes, _ := json.Marshal(requestBody)

			req := httptest.NewRequest(http.MethodPost, "/notify/"+tt.recipient, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code)
		})
	}
}
