package router

import (
	"encoding/json"
	errors "errors"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	swagger "go.lumeweb.com/gswagger"
	"go.lumeweb.com/queryutil/filter"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const (
	TypeArray  = openapi3.TypeArray
	TypeObject = openapi3.TypeObject
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

func (s *testSchema) FieldEnums() map[string][]string {
	return nil
}

// testSchemaWithEnums implements FieldSchema with enum values
type testSchemaWithEnums struct{}

func (s *testSchemaWithEnums) FilterOperators() map[string][]string {
	return map[string][]string{
		"status": {"eq", "ne"},
		"level":  {"in"},
	}
}

func (s *testSchemaWithEnums) SortableFields() []string {
	return []string{"status"}
}

func (s *testSchemaWithEnums) FieldEnums() map[string][]string {
	return map[string][]string{
		"status": {"pending", "processing", "completed", "failed"},
		"level":  {"info", "warn", "error"},
	}
}

func TestWithSwaggerOptions(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler, WithSwaggerOptions(WithSummary("Test Summary")))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)
}

func TestResponseSchemaGeneration(t *testing.T) {
	type User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name        string
		status      int
		description string
		content     interface{}
		wantSchema  map[string]interface{}
	}{
		{
			name:        "simple object response",
			status:      http.StatusOK,
			description: "User details",
			content:     User{},
			wantSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":    map[string]interface{}{"type": "string"},
					"name":  map[string]interface{}{"type": "string"},
					"email": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			name:        "array response",
			status:      http.StatusOK,
			description: "User list",
			content:     []User{},
			wantSchema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":    map[string]interface{}{"type": "string"},
						"name":  map[string]interface{}{"type": "string"},
						"email": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := NewRoute("GET", "/test", nil,
				WithSwagger(
					WithSuccessResponse(tt.status, tt.description,
						WithJSONContent(tt.content),
					),
				),
			)

			// Verify the response schema was properly generated
			resp := route.Swagger.Responses[tt.status]
			assert.Equal(t, tt.description, resp.Description)

			content := resp.Content[MediaTypeJSON]
			assert.NotNil(t, content)

			// Generate the OpenAPI schema
			router, err := NewRouter(APIInfo().
				Title("Test API").
				Version("1.0.0"))
			require.NoError(t, err)

			_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
			require.NoError(t, err)

			// Generate and resolve references
			err = router.GenerateAndExposeOpenapi()
			require.NoError(t, err)

			// Get the generated OpenAPI spec
			swaggerSchema := router.GetSwaggerSchema()
			require.NotNil(t, swaggerSchema)

			// Find our path in the schema
			pathItem := swaggerSchema.Paths.Find("/test")
			require.NotNil(t, pathItem)

			operation := pathItem.GetOperation("GET")
			require.NotNil(t, operation)

			responseRef, exists := operation.Responses.Map()[strconv.Itoa(tt.status)]
			require.True(t, exists, "expected response for status %d", tt.status)
			require.NotNil(t, responseRef)

			mediaType := responseRef.Value.Content.Get(MediaTypeJSON)
			require.NotNil(t, mediaType, "expected application/json content for status %d", tt.status)

			// Verify schema properties
			schema := mediaType.Schema.Value
			assert.NotNil(t, schema)

			// Compare schema structure
			if tt.wantSchema["type"] == "object" {
				assert.True(t, schema.Type.Is(TypeObject))
				assert.Equal(t, len(tt.wantSchema["properties"].(map[string]interface{})), len(schema.Properties))
				for propName, propSchema := range tt.wantSchema["properties"].(map[string]interface{}) {
					assert.Contains(t, schema.Properties, propName)
					assert.True(t, schema.Properties[propName].Value.Type.Is(propSchema.(map[string]interface{})["type"].(string)))
				}
			} else if tt.wantSchema["type"] == "array" {
				assert.True(t, schema.Type.Is(TypeArray))
				items := schema.Items.Value
				assert.NotNil(t, items)
				assert.Equal(t, len(tt.wantSchema["items"].(map[string]interface{})["properties"].(map[string]interface{})), len(items.Properties))
			}
		})
	}
}

func TestAPIKeyAuthEndpoint(t *testing.T) {
	type Response struct {
		Data string `json:"data"`
	}

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, Response{Data: "test data"})
	}

	route := NewRoute("GET", "/secure/data", handler,
		WithSwagger(
			WithSummary("Get secure data"),
			WithDescription("Requires API key authentication"),
			WithHeaderParam("X-API-Key", "API key for authentication", "string"),
			WithSuccessResponse(http.StatusOK, "Secure data retrieved",
				WithJSONContent(Response{}),
			),
		),
	)

	// Test Swagger config
	assert.Contains(t, route.Swagger.Headers, "X-API-Key")
	assert.Equal(t, "string", route.Swagger.Headers["X-API-Key"].Schema.Value)

	// Test actual response decoding
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/secure/data", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Decode and verify response
	var resp Response
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "test data", resp.Data)
}

func TestComplexRequestBodyEndpoint(t *testing.T) {
	type OrderRequest struct {
		Items    []string `json:"items"`
		Priority int      `json:"priority"`
		Notes    string   `json:"notes,omitempty"`
	}

	type OrderResponse struct {
		OrderID string `json:"order_id"`
	}

	handler := func(c echo.Context) error {
		var req OrderRequest
		if err := c.Bind(&req); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, OrderResponse{OrderID: "123"})
	}

	route := NewRoute("POST", "/orders", handler,
		WithSwagger(
			WithSummary("Create new order"),
			WithRequestBody(OrderRequest{}, "Order details", true),
			WithSuccessResponse(http.StatusCreated, "Order created",
				WithJSONContent(OrderResponse{}),
			),
		),
	)

	// Test Swagger config
	assert.True(t, route.Swagger.RequestBody.Required)
	assert.Contains(t, route.Swagger.RequestBody.Content, MediaTypeJSON)

	// Test actual request/response flow
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	require.NoError(t, err)

	reqBody := `{"items":["item1","item2"],"priority":1}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", MediaTypeJSON)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	// Decode and verify response
	var resp OrderResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "123", resp.OrderID)
}

func TestDeprecatedEndpoint(t *testing.T) {
	type Response struct {
		Message string `json:"message"`
	}

	handler := func(c echo.Context) error {
		c.Response().Header().Set("Deprecation", "true")
		c.Response().Header().Set("Sunset", "2025-12-31")
		return c.JSON(http.StatusOK, Response{Message: "Deprecated"})
	}

	route := NewRoute("GET", "/old-endpoint", handler,
		WithSwagger(
			WithSummary("Deprecated endpoint"),
			WithDescription("This endpoint is deprecated and will be removed"),
			WithResponseHeaders(http.StatusOK, "Success but deprecated",
				map[string]swagger.Schema{
					MediaTypeJSON: {
						Value: Response{},
					},
				},
				map[string]string{
					"Deprecation": "true",
					"Sunset":      "2025-12-31",
				},
			),
		),
	)

	// Test Swagger config
	resp := route.Swagger.Responses[http.StatusOK]
	assert.Contains(t, resp.Headers, "Deprecation")
	assert.Contains(t, resp.Headers, "Sunset")

	// Test actual response
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/old-endpoint", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "true", rr.Header().Get("Deprecation"))
	assert.Equal(t, "2025-12-31", rr.Header().Get("Sunset"))

	// Decode and verify response
	var respBody Response
	err = json.NewDecoder(rr.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, "Deprecated", respBody.Message)
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
		{
			name:   "field operators with enums",
			schema: &testSchemaWithEnums{},
			wantParams: []string{
				"status_eq",
				"status_ne",
				"level_in",
				"filters[status][eq]",
				"filters[status][ne]",
				"filters[level][in]",
			},
			wantArrayOps:  []string{"level_in"},
			wantDescRegex: `Filter by (status|level)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create route with filter params
			def := swagger.Definitions{}
			WithFilterParamsFromSchema(tt.schema)(&def, "")

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

			// Verify enum values appear in schema for enum-bearing fields
			if tt.name == "field operators with enums" {
				// Single-value param (status_eq) should have enum
				statusEqSchema := def.Querystring["status_eq"].Schema.Value
				statusMap, ok := statusEqSchema.(map[string]any)
				require.True(t, ok, "expected map schema for status_eq")
				assert.Equal(t, "string", statusMap["type"])
				assert.ElementsMatch(t, []string{"pending", "processing", "completed", "failed"}, statusMap["enum"])

				// Complex format param (filters[status][eq]) should have enum
				complexSchema := def.Querystring["filters[status][eq]"].Schema.Value
				complexMap, ok := complexSchema.(map[string]any)
				require.True(t, ok, "expected map schema for filters[status][eq]")
				assert.ElementsMatch(t, []string{"pending", "processing", "completed", "failed"}, complexMap["enum"])

				// Array param (level_in) should have enum in items
				levelInSchema := def.Querystring["level_in"].Schema.Value
				levelMap, ok := levelInSchema.(map[string]any)
				require.True(t, ok, "expected map schema for level_in")
				items, ok := levelMap["items"].(map[string]any)
				require.True(t, ok, "expected items map for level_in")
				assert.ElementsMatch(t, []string{"info", "warn", "error"}, items["enum"])
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
	opt(&def, "")

	assert.NotNil(t, def.RequestBody)
	assert.Equal(t, "Test description", def.RequestBody.Description)
	assert.True(t, def.RequestBody.Required)
	assert.Contains(t, def.RequestBody.Content, MediaTypeJSON)
}

func TestWithFileUpload(t *testing.T) {
	opt := WithFileUpload("File upload description", true)
	def := swagger.Definitions{}
	opt(&def, "")

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
	opt(&def, "")

	assert.NotNil(t, def.Responses)
	assert.Contains(t, def.Responses, http.StatusOK)
	resp := def.Responses[http.StatusOK]
	assert.Equal(t, "Test items", resp.Description)
	assert.Contains(t, resp.Content, MediaTypeJSON)
}

func TestWithResponseHeaders(t *testing.T) {
	content := map[string]swagger.Schema{
		MediaTypeJSON: {
			Value: map[string]string{"message": "success"},
		},
	}
	headers := map[string]string{
		"X-RateLimit-Limit": "Rate limit header",
	}

	opt := WithResponseHeaders(http.StatusOK, "Success", content, headers)
	def := swagger.Definitions{}
	opt(&def, "")

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
		WithContent(MediaTypeJSON, "test")(resp)
		assert.Equal(t, MediaTypeJSON, resp.Content.MediaType)
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
		assert.Equal(t, MediaTypeJSON, resp.Content.MediaType)
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
		assert.Equal(t, "test", resp[http.StatusOK].Content[MediaTypeJSON].Value)
		assert.Equal(t, "Total number of items", resp[http.StatusOK].Headers["X-Total-Count"])
	})

	t.Run("WithSuccessResponse", func(t *testing.T) {
		def := swagger.Definitions{}
		WithSuccessResponse(http.StatusOK, "Success",
			WithJSONContent("test"),
			WithTotalCountHeader(),
		)(&def, "")

		assert.Contains(t, def.Responses, http.StatusOK)
		resp := def.Responses[http.StatusOK]
		assert.Equal(t, "Success", resp.Description)
		assert.Equal(t, "test", resp.Content[MediaTypeJSON].Value)
		assert.Equal(t, "Total number of items", resp.Headers["X-Total-Count"])
	})

	t.Run("WithPaginatedResponse", func(t *testing.T) {
		type Item struct{ Name string }
		type Meta struct{ Total int }

		def := swagger.Definitions{}
		WithPaginatedResponse(Item{}, Meta{})(&def, "")

		assert.Contains(t, def.Responses, http.StatusOK)
		resp := def.Responses[http.StatusOK]
		assert.Equal(t, "Success", resp.Description)

		content := resp.Content[MediaTypeJSON].Value.(map[string]interface{})
		assert.NotNil(t, content["items"])
		assert.NotNil(t, content["pagination"])
	})

	t.Run("DefineSwaggerErrorResponse", func(t *testing.T) {
		resp := DefineSwaggerErrorResponse(http.StatusNotFound, "Not Found")
		assert.Contains(t, resp, http.StatusNotFound)
		assert.Equal(t, "Not Found", resp[http.StatusNotFound].Description)
		assert.Equal(t, "Not Found", resp[http.StatusNotFound].Content[MediaTypeJSON].Value.(ResponseError).Error())
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
	opt(&def, "")

	assert.Equal(t, []string{"users", "admin"}, def.Tags)
}

func TestWithSummary(t *testing.T) {
	opt := WithSummary("Test summary")
	def := swagger.Definitions{}
	opt(&def, "")

	assert.Equal(t, "Test summary", def.Summary)
}

func TestWithDescription(t *testing.T) {
	opt := WithDescription("Test description")
	def := swagger.Definitions{}
	opt(&def, "")

	assert.Equal(t, "Test description", def.Description)
}

func TestDefineSwaggerErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		error          interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "string error message",
			status:         http.StatusBadRequest,
			error:          "Bad request details",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Bad request details",
		},
		{
			name:   "ResponseError implementation",
			status: http.StatusForbidden,
			error: &ErrorWrapper{
				Message: "custom forbidden error",
				Status:  http.StatusForbidden,
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "custom forbidden error",
		},
		{
			name:           "non-string error",
			status:         http.StatusInternalServerError,
			error:          errors.New("generic error"),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := DefineSwaggerErrorResponse(tt.status, tt.error)
			assert.Contains(t, resp, tt.expectedStatus)

			content := resp[tt.expectedStatus].Content[MediaTypeJSON]
			assert.NotNil(t, content)

			switch v := content.Value.(type) {
			case ErrorResponse:
				assert.Equal(t, tt.expectedError, v.Error())
			case ResponseError:
				assert.Equal(t, tt.expectedError, v.Error())
			default:
				t.Errorf("unexpected response type: %T", v)
			}
		})
	}
}
func TestMergeResponses(t *testing.T) {
	tests := []struct {
		name     string
		input    []map[int]swagger.ContentValue
		expected map[int]swagger.ContentValue
	}{
		{
			name: "empty inputs",
			input: []map[int]swagger.ContentValue{
				{},
				{},
			},
			expected: map[int]swagger.ContentValue{},
		},
		{
			name: "merge success responses",
			input: []map[int]swagger.ContentValue{
				{
					200: {Description: "Success A"},
					400: {Description: "Error A"},
				},
				{
					201: {Description: "Success B"},
					401: {Description: "Error B"},
				},
			},
			expected: map[int]swagger.ContentValue{
				200: {Description: "Success A"},
				201: {Description: "Success B"},
				400: {Description: "Error A"},
				401: {Description: "Error B"},
			},
		},
		{
			name: "preserve first success response",
			input: []map[int]swagger.ContentValue{
				{
					200: {Description: "First Success"},
				},
				{
					200: {Description: "Second Success"},
				},
			},
			expected: map[int]swagger.ContentValue{
				200: {Description: "First Success"},
			},
		},
		{
			name: "override error responses",
			input: []map[int]swagger.ContentValue{
				{
					400: {Description: "First Error"},
				},
				{
					400: {Description: "Second Error"},
				},
			},
			expected: map[int]swagger.ContentValue{
				400: {Description: "Second Error"},
			},
		},
		{
			name: "mixed success and error responses",
			input: []map[int]swagger.ContentValue{
				{
					200: {Description: "Success"},
					400: {Description: "First Error"},
				},
				{
					201: {Description: "New Success"},
					400: {Description: "New Error"},
				},
			},
			expected: map[int]swagger.ContentValue{
				200: {Description: "Success"},
				201: {Description: "New Success"},
				400: {Description: "New Error"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeResponses(tt.input...)
			assert.Equal(t, tt.expected, result)
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

	content := combined[http.StatusNotFound].Content[MediaTypeJSON]
	// Access the Value field of the swagger.Schema struct and assert its type
	responseError, ok := content.Value.(ResponseError)
	assert.True(t, ok, "Expected content value to be ResponseError")

	assert.Equal(t, "Not found", responseError.Error())
}
func TestWithErrorResponses(t *testing.T) {
	tests := []struct {
		name          string
		accessRole    string
		customErrors  map[int]swagger.ContentValue
		expectedCodes []int
	}{
		{
			name:       "public route with string errors",
			accessRole: "",
			customErrors: map[int]swagger.ContentValue{
				422: {
					Description: "Validation failed",
					Content: map[string]swagger.Schema{
						MediaTypeJSON: {
							Value: ErrorResponse{Message: "Validation failed"},
						},
					},
				},
				429: {
					Description: "Too many requests",
					Content: map[string]swagger.Schema{
						MediaTypeJSON: {
							Value: ErrorResponse{Message: "Too many requests"},
						},
					},
				},
			},
			expectedCodes: []int{200, 400, 404, 422, 429, 500},
		},
		{
			name:       "auth route with string errors",
			accessRole: ACCESS_USER_ROLE,
			customErrors: map[int]swagger.ContentValue{
				409: {
					Description: "Conflict",
					Content: map[string]swagger.Schema{
						MediaTypeJSON: {
							Value: ErrorResponse{Message: "Conflict"},
						},
					},
				},
				503: {
					Description: "Service unavailable",
					Content: map[string]swagger.Schema{
						MediaTypeJSON: {
							Value: ErrorResponse{Message: "Service unavailable"},
						},
					},
				},
			},
			expectedCodes: []int{200, 400, 401, 403, 404, 409, 500, 503},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := NewRoute("GET", "/test", nil, WithAccess(tt.accessRole))
			WithErrorResponses(tt.customErrors)(&route.Swagger, route.Access)

			assert.Len(t, route.Swagger.Responses, len(tt.expectedCodes))
			for _, code := range tt.expectedCodes {
				assert.Contains(t, route.Swagger.Responses, code)
			}
		})
	}
}

func TestSuccessResponsePreservation(t *testing.T) {
	tests := []struct {
		name         string
		successCode  int
		errorCode    int
		successFirst bool
	}{
		{
			name:         "success before errors",
			successCode:  200,
			errorCode:    400,
			successFirst: true,
		},
		{
			name:         "success after errors",
			successCode:  201,
			errorCode:    500,
			successFirst: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create route with success response
			route := NewRoute("GET", "/test", nil)

			// Apply options in specified order
			if tt.successFirst {
				WithSuccessResponse(tt.successCode, "Success", WithJSONContent("success"))(&route.Swagger, "")
				WithErrorResponses(map[int]swagger.ContentValue{
					tt.errorCode: {
						Description: "Error",
						Content: map[string]swagger.Schema{
							MediaTypeJSON: {
								Value: ErrorResponse{Message: "Error"},
							},
						},
					},
				})(&route.Swagger, "")
			} else {
				WithErrorResponses(map[int]swagger.ContentValue{
					tt.errorCode: {
						Description: "Error",
						Content: map[string]swagger.Schema{
							MediaTypeJSON: {
								Value: ErrorResponse{Message: "Error"},
							},
						},
					},
				})(&route.Swagger, "")
				WithSuccessResponse(tt.successCode, "Success", WithJSONContent("success"))(&route.Swagger, "")
			}

			// Verify both responses exist
			assert.Contains(t, route.Swagger.Responses, tt.successCode)
			assert.Contains(t, route.Swagger.Responses, tt.errorCode)

			// Verify success response wasn't overwritten
			successResp := route.Swagger.Responses[tt.successCode]
			assert.Equal(t, "Success", successResp.Description)
			// Now that WithSuccessResponse preserves the original content,
			// we just need to assert that the content exists.
			assert.NotNil(t, successResp.Content[MediaTypeJSON].Value)
		})
	}
}

func TestDefaultResponsesNotOverwritingSuccess(t *testing.T) {
	tests := []struct {
		name        string
		accessRole  string
		successCode int
	}{
		{
			name:        "public route",
			accessRole:  "",
			successCode: 200,
		},
		{
			name:        "auth route",
			accessRole:  ACCESS_USER_ROLE,
			successCode: 201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := NewRoute("GET", "/test", nil, WithAccess(tt.accessRole))

			// Add success response first
			WithSuccessResponse(tt.successCode, "Success", WithJSONContent("success"))(&route.Swagger, tt.accessRole)

			// Then add default error responses via WithSwagger
			WithSwagger()(&route)

			// Verify success response still exists
			assert.Contains(t, route.Swagger.Responses, tt.successCode)
			successResp := route.Swagger.Responses[tt.successCode]
			assert.Equal(t, "Success", successResp.Description)
			assert.Equal(t, "success", successResp.Content[MediaTypeJSON].Value)

			// Verify default error responses were added
			if tt.accessRole == "" {
				assert.Contains(t, route.Swagger.Responses, http.StatusBadRequest)
				assert.Contains(t, route.Swagger.Responses, http.StatusNotFound)
			} else {
				assert.Contains(t, route.Swagger.Responses, http.StatusUnauthorized)
				assert.Contains(t, route.Swagger.Responses, http.StatusForbidden)
			}
		})
	}
}
