package pkg

import (
	"errors"
	"fmt"
)

var (
	ErrClientNotSupport          = errors.New("this feature client not support")
	ErrServerNotSupport          = errors.New("this feature server not support")
	ErrRequestInvalid            = errors.New("request invalid")
	ErrLackResponseChan          = errors.New("lack response chan")
	ErrDuplicateResponseReceived = errors.New("duplicate response received")
	ErrMethodNotSupport          = errors.New("method not support")
	ErrJSONUnmarshal             = errors.New("json unmarshal error")
	ErrSessionHasNotInitialized  = errors.New("the session has not been initialized")
	ErrLackSession               = errors.New("lack session")
	ErrSessionClosed             = errors.New("session closed")
	ErrSendEOF                   = errors.New("send EOF")
	ErrRateLimitExceeded         = errors.New("rate limit exceeded")
)

type ResponseError struct {
	Code    int
	Message string
	Data    interface{}
}

func NewResponseError(code int, message string, data interface{}) *ResponseError {
	return &ResponseError{Code: code, Message: message, Data: data}
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("code=%d message=%s data=%+v", e.Code, e.Message, e.Data)
}
