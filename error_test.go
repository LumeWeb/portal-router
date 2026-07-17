package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseErrorInterface(t *testing.T) {
	t.Run("ErrorResponse implements ResponseError", func(t *testing.T) {
		err := ErrorResponse{
			Detail: ErrorDetail{Reason: "test error"},
		}

		assert.Implements(t, (*ResponseError)(nil), err)
		assert.Equal(t, "test error", err.Error())
	})
}

// customError implements ErrorResponder with custom headers
type customError struct {
	ErrorResponse
	headers map[string]string
}

func (e *customError) Headers() map[string]string {
	return e.headers
}

func TestErrorResponderInterface(t *testing.T) {
	t.Run("custom error with headers", func(t *testing.T) {
		err := &customError{
			ErrorResponse: ErrorResponse{
				Detail: ErrorDetail{Reason: "custom error"},
			},
			headers: map[string]string{
				"X-Custom": "value",
			},
		}

		var responder ErrorResponder = err
		assert.Equal(t, "custom error", responder.Error())
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
			input: ErrorResponse{
				Detail: ErrorDetail{Reason: "custom error"},
			},
			expected: ErrorResponse{
				Detail: ErrorDetail{Reason: "custom error"},
			},
		},
		{
			name:     "standard error",
			input:    assert.AnError,
			expected: ErrorResponse{Detail: ErrorDetail{Reason: assert.AnError.Error()}},
		},
		{
			name:     "nil error",
			input:    nil,
			expected: ErrorResponse{Detail: ErrorDetail{Reason: ""}},
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
	assert.Equal(t, "Bad Request", resp.Content[MediaTypeJSON].Value.(ErrorResponse).Detail.Reason)
}

func TestUnauthorizedResponse(t *testing.T) {
	resp := unauthorizedResponse()
	assert.Equal(t, "Unauthorized", resp.Description)
	assert.NotNil(t, resp.Content[MediaTypeJSON])
	assert.Equal(t, "Unauthorized", resp.Content[MediaTypeJSON].Value.(ErrorResponse).Detail.Reason)
}
