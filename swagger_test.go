package router

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	swagger "go.lumeweb.com/gswagger"
)

func TestWithSwaggerOptions(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler, WithSwaggerOptions(WithSummary("Test Summary")))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)
}

func TestWithRequestBody(t *testing.T) {
	type TestBody struct {
		Name string `json:"name"`
	}

	opt := WithRequestBody(TestBody{}, "Test description", true)
	def := swagger.Definitions{}
	opt(&def)

	assert.NotNil(t, def.RequestBody)
	assert.Equal(t, "Test description", def.RequestBody.Description)
	assert.True(t, def.RequestBody.Required)
	assert.Contains(t, def.RequestBody.Content, "application/json")
}

func TestWithFileUpload(t *testing.T) {
	opt := WithFileUpload("File upload description", true)
	def := swagger.Definitions{}
	opt(&def)

	assert.NotNil(t, def.RequestBody)
	assert.Equal(t, "File upload description", def.RequestBody.Description)
	assert.True(t, def.RequestBody.Required)
	assert.Contains(t, def.RequestBody.Content, "multipart/form-data")
}

func TestWithArrayResponse(t *testing.T) {
	type Item struct {
		ID string `json:"id"`
	}

	opt := WithArrayResponse(http.StatusOK, "Test items", Item{})
	def := swagger.Definitions{}
	opt(&def)

	assert.NotNil(t, def.Responses)
	assert.Contains(t, def.Responses, http.StatusOK)
	resp := def.Responses[http.StatusOK]
	assert.Equal(t, "Test items", resp.Description)
	assert.Contains(t, resp.Content, "application/json")
}

func TestWithResponseHeaders(t *testing.T) {
	content := map[string]swagger.Schema{
		"application/json": {
			Value: map[string]string{"message": "success"},
		},
	}
	headers := map[string]string{
		"X-RateLimit-Limit": "Rate limit header",
	}

	opt := WithResponseHeaders(http.StatusOK, "Success", content, headers)
	def := swagger.Definitions{}
	opt(&def)

	assert.NotNil(t, def.Responses)
	assert.Contains(t, def.Responses, http.StatusOK)
	resp := def.Responses[http.StatusOK]
	assert.Equal(t, "Success", resp.Description)
	// Assert that the Content is of type swagger.Content and has the expected content
	assert.IsType(t, swagger.Content{}, resp.Content)
	assert.Equal(t, content, map[string]swagger.Schema(resp.Content)) // Cast swagger.Content back to map for comparison
	assert.Equal(t, headers, resp.Headers)
}

func TestWithTags(t *testing.T) {
	opt := WithTags("users", "admin")
	def := swagger.Definitions{}
	opt(&def)

	assert.Equal(t, []string{"users", "admin"}, def.Tags)
}

func TestWithSummary(t *testing.T) {
	opt := WithSummary("Test summary")
	def := swagger.Definitions{}
	opt(&def)

	assert.Equal(t, "Test summary", def.Summary)
}

func TestWithDescription(t *testing.T) {
	opt := WithDescription("Test description")
	def := swagger.Definitions{}
	opt(&def)

	assert.Equal(t, "Test description", def.Description)
}

func TestDefineSwaggerErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		errorMsg  string
		wantError string
	}{
		{
			name:      "not found error",
			status:    http.StatusNotFound,
			errorMsg:  "Not found",
			wantError: "Not found",
		},
		{
			name:      "bad request error",
			status:    http.StatusBadRequest,
			errorMsg:  "Invalid input",
			wantError: "Invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := DefineSwaggerErrorResponse(tt.status, tt.errorMsg)

			assert.Contains(t, resp, tt.status)
			contentValue, ok := resp[tt.status].Content["application/json"]
			assert.True(t, ok, "Expected application/json content")

			// Access the Value field of the swagger.Schema struct and assert its type
			responseError, ok := contentValue.Value.(ResponseError)
			assert.True(t, ok, "Expected content value to be ResponseError")

			assert.Equal(t, tt.wantError, responseError.Error)
		})
	}
}

func TestDefaultCoreErrorResponses(t *testing.T) {
	core := DefaultCoreErrorResponses()

	assert.Len(t, core, 3)
	assert.Contains(t, core, http.StatusBadRequest)
	assert.Contains(t, core, http.StatusNotFound)
	assert.Contains(t, core, http.StatusInternalServerError)
	assert.NotContains(t, core, http.StatusUnauthorized)
	assert.NotContains(t, core, http.StatusForbidden)
}

func TestDefaultPublicErrorResponses(t *testing.T) {
	defaults := DefaultPublicErrorResponses()

	assert.Len(t, defaults, 3)
	assert.Contains(t, defaults, http.StatusBadRequest)
	assert.Contains(t, defaults, http.StatusNotFound)
	assert.Contains(t, defaults, http.StatusInternalServerError)
	assert.NotContains(t, defaults, http.StatusUnauthorized)
	assert.NotContains(t, defaults, http.StatusForbidden)
}

func TestDefaultAuthErrorResponses(t *testing.T) {
	defaults := DefaultAuthErrorResponses()

	assert.Len(t, defaults, 5)
	assert.Contains(t, defaults, http.StatusBadRequest)
	assert.Contains(t, defaults, http.StatusUnauthorized)
	assert.Contains(t, defaults, http.StatusForbidden)
	assert.Contains(t, defaults, http.StatusNotFound)
	assert.Contains(t, defaults, http.StatusInternalServerError)
}

func TestDefineSwaggerErrorResponses(t *testing.T) {
	resp1 := DefineSwaggerErrorResponse(http.StatusNotFound, "Not found")
	resp2 := DefineSwaggerErrorResponse(http.StatusBadRequest, "Bad request")
	resp3 := DefineSwaggerErrorResponse(http.StatusInternalServerError, "Server error")

	combined := DefineSwaggerErrorResponses(resp1, resp2, resp3)

	assert.Len(t, combined, 3)
	assert.Contains(t, combined, http.StatusNotFound)
	assert.Contains(t, combined, http.StatusBadRequest)
	assert.Contains(t, combined, http.StatusInternalServerError)

	content := combined[http.StatusNotFound].Content["application/json"]
	// Access the Value field of the swagger.Schema struct and assert its type
	responseError, ok := content.Value.(ResponseError)
	assert.True(t, ok, "Expected content value to be ResponseError")

	assert.Equal(t, "Not found", responseError.Error)
}
