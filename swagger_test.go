package router

import (
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	swagger "go.lumeweb.com/gswagger"
	"go.lumeweb.com/queryutil/filter"
)

// Define a test schema that implements FieldSchema
type testSchema struct{}

func (s *testSchema) FilterOperators() map[string][]string {
	return map[string][]string{
		"age":  {"gt", "lt", "between"},
		"name": {"contains", "startswith"},
	}
}

func (s *testSchema) SortableFields() []string {
	return []string{"age", "name"}
}

func TestWithSwaggerOptions(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler, WithSwaggerOptions(WithSummary("Test Summary")))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)
}

func TestWithFilterParamsFromSchema(t *testing.T) {

	tests := []struct {
		name          string
		schema        FieldSchema
		wantParams    []string
		wantArrayOps  []string
		wantDescRegex string
	}{
		{
			name:   "basic field operators",
			schema: &testSchema{},
			wantParams: []string{
				"age_gt",
				"age_lt",
				"age_between",
				"name_contains",
				"name_startswith",
				"filters[age][gt]",
				"filters[age][lt]",
				"filters[age][between]",
				"filters[name][contains]",
				"filters[name][startswith]",
			},
			wantArrayOps:  []string{"age_between"},
			wantDescRegex: `Filter by (age|name)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create route with filter params
			def := swagger.Definitions{}
			WithFilterParamsFromSchema(tt.schema)(&def)

			// Verify all expected parameters exist
			for _, param := range tt.wantParams {
				assert.Contains(t, def.Querystring, param, "missing parameter: %s", param)
			}

			// Verify array parameter schemas
			for _, param := range tt.wantArrayOps {
				schema := def.Querystring[param].Schema.Value
				schemaMap, ok := schema.(map[string]interface{})
				require.True(t, ok, "expected map schema for %s", param)

				assert.Equal(t, "array", schemaMap["type"])
				assert.Equal(t, true, schemaMap["x-csv"])
				assert.Equal(t, "multi", schemaMap["x-collectionFormat"])
			}

			// Verify descriptions contain expected text and complex format structure
			for param, paramDef := range def.Querystring {
				assert.Regexp(t, tt.wantDescRegex, paramDef.Description,
					"unexpected description for %s", param)

				// Verify complex format params have correct structure
				if strings.Contains(param, "filters[") {
					assert.Contains(t, param, "[", "complex format param should contain brackets")
					assert.Contains(t, param, "]", "complex format param should contain brackets")
				}

			}
		})
	}
}

func TestOpIsMultiValue(t *testing.T) {
	tests := []struct {
		name string
		op   string
		want bool
	}{
		{"OpIn", "in", true},
		{"OpNin", "nin", true},
		{"OpIna", "ina", true},
		{"OpNina", "nina", true},
		{"OpBetween", "between", true},
		{"OpNbetween", "nbetween", true},
		{"OpEq", "eq", false},
		{"OpContains", "contains", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, opIsMultiValue(filter.Operator(tt.op)))
		})
	}
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
	assert.IsType(t, swagger.Content{}, resp.Content)
	assert.Equal(t, content, map[string]swagger.Schema(resp.Content))
	assert.Equal(t, headers, resp.Headers)
}

func TestResponseHelpers(t *testing.T) {
	t.Run("WithContent", func(t *testing.T) {
		resp := &Response{}
		WithContent("application/json", "test")(resp)
		assert.Equal(t, "application/json", resp.Content.MediaType)
		assert.Equal(t, "test", resp.Content.Schema)
	})

	t.Run("WithHeader", func(t *testing.T) {
		resp := &Response{}
		WithHeader("X-Test", "Test header")(resp)
		assert.Len(t, resp.Headers, 1)
		assert.Equal(t, "X-Test", resp.Headers[0].Name)
		assert.Equal(t, "Test header", resp.Headers[0].Value)
	})

	t.Run("WithJSONContent", func(t *testing.T) {
		resp := &Response{}
		WithJSONContent("test")(resp)
		assert.Equal(t, "application/json", resp.Content.MediaType)
		assert.Equal(t, "test", resp.Content.Schema)
	})

	t.Run("WithTotalCountHeader", func(t *testing.T) {
		resp := &Response{}
		WithTotalCountHeader()(resp)
		assert.Len(t, resp.Headers, 1)
		assert.Equal(t, "X-Total-Count", resp.Headers[0].Name)
	})

	t.Run("WithCacheControl", func(t *testing.T) {
		resp := &Response{}
		WithCacheControl("no-cache")(resp)
		assert.Len(t, resp.Headers, 1)
		assert.Equal(t, "Cache-Control", resp.Headers[0].Name)
		assert.Equal(t, "no-cache", resp.Headers[0].Value)
	})

	t.Run("DefineResponse", func(t *testing.T) {
		resp := DefineResponse(http.StatusOK, "Success",
			WithJSONContent("test"),
			WithTotalCountHeader(),
		)
		assert.Contains(t, resp, http.StatusOK)
		assert.Equal(t, "Success", resp[http.StatusOK].Description)
		assert.Equal(t, "test", resp[http.StatusOK].Content["application/json"].Value)
		assert.Equal(t, "Total number of items", resp[http.StatusOK].Headers["X-Total-Count"])
	})

	t.Run("WithSuccessResponse", func(t *testing.T) {
		def := swagger.Definitions{}
		WithSuccessResponse(http.StatusOK, "Success",
			WithJSONContent("test"),
			WithTotalCountHeader(),
		)(&def)

		assert.Contains(t, def.Responses, http.StatusOK)
		resp := def.Responses[http.StatusOK]
		assert.Equal(t, "Success", resp.Description)
		assert.Equal(t, "test", resp.Content["application/json"].Value)
		assert.Equal(t, "Total number of items", resp.Headers["X-Total-Count"])
	})

	t.Run("WithPaginatedResponse", func(t *testing.T) {
		type Item struct{ Name string }
		type Meta struct{ Total int }

		def := swagger.Definitions{}
		WithPaginatedResponse(Item{}, Meta{})(&def)

		assert.Contains(t, def.Responses, http.StatusOK)
		resp := def.Responses[http.StatusOK]
		assert.Equal(t, "Success", resp.Description)

		content := resp.Content["application/json"].Value.(map[string]interface{})
		assert.NotNil(t, content["items"])
		assert.NotNil(t, content["pagination"])
	})

	t.Run("DefineSwaggerErrorResponse", func(t *testing.T) {
		resp := DefineSwaggerErrorResponse(http.StatusNotFound, "Not Found")
		assert.Contains(t, resp, http.StatusNotFound)
		assert.Equal(t, "Not Found", resp[http.StatusNotFound].Description)
		assert.Equal(t, "Not Found", resp[http.StatusNotFound].Content["application/json"].Value.(ResponseError).Error)
	})

	t.Run("DefineSwaggerErrorResponses", func(t *testing.T) {
		resp1 := DefineSwaggerErrorResponse(http.StatusNotFound, "Not Found")
		resp2 := DefineSwaggerErrorResponse(http.StatusBadRequest, "Bad Request")
		combined := DefineSwaggerErrorResponses(resp1, resp2)

		assert.Contains(t, combined, http.StatusNotFound)
		assert.Contains(t, combined, http.StatusBadRequest)
		assert.Equal(t, "Not Found", combined[http.StatusNotFound].Description)
		assert.Equal(t, "Bad Request", combined[http.StatusBadRequest].Description)
	})

	t.Run("DefaultCoreErrorResponses", func(t *testing.T) {
		resp := DefaultCoreErrorResponses()
		assert.Contains(t, resp, http.StatusBadRequest)
		assert.Contains(t, resp, http.StatusNotFound)
		assert.Contains(t, resp, http.StatusInternalServerError)
	})

	t.Run("DefaultPublicErrorResponses", func(t *testing.T) {
		resp := DefaultPublicErrorResponses()
		assert.Contains(t, resp, http.StatusBadRequest)
		assert.Contains(t, resp, http.StatusNotFound)
		assert.Contains(t, resp, http.StatusInternalServerError)
	})

	t.Run("DefaultAuthErrorResponses", func(t *testing.T) {
		resp := DefaultAuthErrorResponses()
		assert.Contains(t, resp, http.StatusBadRequest)
		assert.Contains(t, resp, http.StatusNotFound)
		assert.Contains(t, resp, http.StatusInternalServerError)
		assert.Contains(t, resp, http.StatusUnauthorized)
		assert.Contains(t, resp, http.StatusForbidden)
	})
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
