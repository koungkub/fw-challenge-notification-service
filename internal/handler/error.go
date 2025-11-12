package handler

import "fmt"

type ErrorHandler struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func (e *ErrorHandler) Error() string {
	return fmt.Sprintf("error code: %s, message: %s", e.ErrorCode, e.Message)
}

func GetRequestError(err error) error {
	return &ErrorHandler{
		ErrorCode: "E101",
		Message:   err.Error(),
	}
}

func GetInternalError(err error) error {
	return &ErrorHandler{
		ErrorCode: "E102",
		Message:   err.Error(),
	}
}
