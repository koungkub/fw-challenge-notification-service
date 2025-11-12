package handler

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRequestError(t *testing.T) {
	tests := []struct {
		name              string
		inputError        error
		expectedErrorCode string
		expectedMessage   string
	}{
		{
			name:              "wraps error with E101 code",
			inputError:        errors.New("invalid request format"),
			expectedErrorCode: "E101",
			expectedMessage:   "invalid request format",
		},
		{
			name:              "wraps error with empty message",
			inputError:        errors.New(""),
			expectedErrorCode: "E101",
			expectedMessage:   "",
		},
		{
			name:              "wraps error with long message",
			inputError:        errors.New("Key: 'NotifyRequest.To' Error:Field validation for 'To' failed on the 'required' tag"),
			expectedErrorCode: "E101",
			expectedMessage:   "Key: 'NotifyRequest.To' Error:Field validation for 'To' failed on the 'required' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRequestError(tt.inputError)

			assert.NotNil(t, result)

			// Type assert to ErrorHandler
			errorHandler, ok := result.(*ErrorHandler)
			assert.True(t, ok, "Expected result to be *ErrorHandler")

			// Verify error code
			assert.Equal(t, tt.expectedErrorCode, errorHandler.ErrorCode)

			// Verify message
			assert.Equal(t, tt.expectedMessage, errorHandler.Message)
		})
	}
}

func TestGetInternalError(t *testing.T) {
	tests := []struct {
		name              string
		inputError        error
		expectedErrorCode string
		expectedMessage   string
	}{
		{
			name:              "wraps error with E102 code",
			inputError:        errors.New("database connection error"),
			expectedErrorCode: "E102",
			expectedMessage:   "database connection error",
		},
		{
			name:              "wraps service unavailable error",
			inputError:        errors.New("service unavailable"),
			expectedErrorCode: "E102",
			expectedMessage:   "service unavailable",
		},
		{
			name:              "wraps not supported recipient error",
			inputError:        errors.New("not supported recipient type"),
			expectedErrorCode: "E102",
			expectedMessage:   "not supported recipient type",
		},
		{
			name:              "wraps empty error message",
			inputError:        errors.New(""),
			expectedErrorCode: "E102",
			expectedMessage:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInternalError(tt.inputError)

			assert.NotNil(t, result)

			// Type assert to ErrorHandler
			errorHandler, ok := result.(*ErrorHandler)
			assert.True(t, ok, "Expected result to be *ErrorHandler")

			// Verify error code
			assert.Equal(t, tt.expectedErrorCode, errorHandler.ErrorCode)

			// Verify message
			assert.Equal(t, tt.expectedMessage, errorHandler.Message)
		})
	}
}

func TestErrorHandler_Error(t *testing.T) {
	tests := []struct {
		name           string
		errorCode      string
		message        string
		expectedString string
	}{
		{
			name:           "formats E101 error correctly",
			errorCode:      "E101",
			message:        "invalid request",
			expectedString: "error code: E101, message: invalid request",
		},
		{
			name:           "formats E102 error correctly",
			errorCode:      "E102",
			message:        "internal error",
			expectedString: "error code: E102, message: internal error",
		},
		{
			name:           "formats error with empty message",
			errorCode:      "E101",
			message:        "",
			expectedString: "error code: E101, message: ",
		},
		{
			name:           "formats error with long message",
			errorCode:      "E102",
			message:        "database connection failed: timeout after 30s",
			expectedString: "error code: E102, message: database connection failed: timeout after 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorHandler := &ErrorHandler{
				ErrorCode: tt.errorCode,
				Message:   tt.message,
			}

			result := errorHandler.Error()

			assert.Equal(t, tt.expectedString, result)
		})
	}
}

func TestErrorHandler_ErrorImplementsErrorInterface(t *testing.T) {
	t.Run("ErrorHandler implements error interface", func(t *testing.T) {
		var err error = &ErrorHandler{
			ErrorCode: "E101",
			Message:   "test error",
		}

		assert.NotNil(t, err)
		assert.Equal(t, "error code: E101, message: test error", err.Error())
	})
}

func TestGetRequestError_PreservesOriginalError(t *testing.T) {
	t.Run("preserves original error message exactly", func(t *testing.T) {
		originalErr := errors.New("original error message with special characters: !@#$%^&*()")

		result := GetRequestError(originalErr)
		errorHandler := result.(*ErrorHandler)

		assert.Equal(t, "original error message with special characters: !@#$%^&*()", errorHandler.Message)
	})
}

func TestGetInternalError_PreservesOriginalError(t *testing.T) {
	t.Run("preserves original error message exactly", func(t *testing.T) {
		originalErr := errors.New("internal error with special characters: !@#$%^&*()")

		result := GetInternalError(originalErr)
		errorHandler := result.(*ErrorHandler)

		assert.Equal(t, "internal error with special characters: !@#$%^&*()", errorHandler.Message)
	})
}
