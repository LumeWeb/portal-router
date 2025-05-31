package router

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseErrorInterface(t *testing.T) {
	t.Run("ErrorWrapper implements ResponseError", func(t *testing.T) {
		err := &ErrorWrapper{
			Message: "test error",
			Status:  http.StatusBadRequest,
		}

		assert.Implements(t, (*ResponseError)(nil), err)
		assert.Equal(t, "test error", err.Error())
		assert.Equal(t, http.StatusBadRequest, err.HttpStatus())
	})

	t.Run("ErrorWrapper implements error", func(t *testing.T) {
		err := &ErrorWrapper{
			Message: "test error",
			Status:  http.StatusBadRequest,
		}

		var e error = err
		assert.Equal(t, "test error", e.Error())
	})
}

// customError implements ErrorResponder with custom headers
type customError struct {
	ErrorWrapper
	headers map[string]string
}

func (e *customError) Headers() map[string]string {
	return e.headers
}

func TestErrorResponderInterface(t *testing.T) {
	t.Run("custom error with headers", func(t *testing.T) {
		err := &customError{
			ErrorWrapper: ErrorWrapper{
				Message: "custom error",
				Status:  http.StatusConflict,
			},
			headers: map[string]string{
				"X-Custom": "value",
			},
		}

		// Test ErrorResponder interface implementation
		var responder ErrorResponder = err
		assert.Equal(t, "custom error", responder.Error())
		assert.Equal(t, http.StatusConflict, responder.HttpStatus())
		assert.Equal(t, map[string]string{"X-Custom": "value"}, responder.Headers())
	})
}

func TestAsErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected interface{}
	}{
		{
			name: "ResponseError implementation",
			input: &ErrorWrapper{
				Message: "custom error",
				Status:  http.StatusBadRequest,
			},
			expected: &ErrorWrapper{
				Message: "custom error",
				Status:  http.StatusBadRequest,
			},
		},
		{
			name:     "standard error",
			input:    assert.AnError,
			expected: ErrorResponse{Message: assert.AnError.Error()},
		},
		{
			name:     "nil error",
			input:    nil,
			expected: ErrorResponse{Message: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AsErrorResponse(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBadRequestResponse(t *testing.T) {
	resp := badRequestResponse()
	assert.Equal(t, "Bad Request", resp.Description)
	assert.NotNil(t, resp.Content[MediaTypeJSON])
	assert.Equal(t, "Bad Request", resp.Content[MediaTypeJSON].Value.(ErrorResponse).Message)
}

func TestUnauthorizedResponse(t *testing.T) {
	resp := unauthorizedResponse()
	assert.Equal(t, "Unauthorized", resp.Description)
	assert.NotNil(t, resp.Content[MediaTypeJSON])
	assert.Equal(t, "Unauthorized", resp.Content[MediaTypeJSON].Value.(ErrorResponse).Message)
}
