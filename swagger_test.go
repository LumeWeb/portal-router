package router

import (
	"encoding/json"
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
	TypeArray   = openapi3.TypeArray
	TypeObject  = openapi3.TypeObject
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

			content := resp.Content["application/json"]
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

			mediaType := responseRef.Value.Content.Get("application/json")
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
	assert.Contains(t, route.Swagger.RequestBody.Content, "application/json")

	// Test actual request/response flow
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	require.NoError(t, err)

	reqBody := `{"items":["item1","item2"],"priority":1}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
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
					"application/json": {
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
	assert.Contains(t, def.RequestBody.Content, "application/json")
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
		)(&def, "")

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
		WithPaginatedResponse(Item{}, Meta{})(&def, "")

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
						"application/json": {
							Value: ResponseError{Error: "Validation failed"},
						},
					},
				},
				429: {
					Description: "Too many requests",
					Content: map[string]swagger.Schema{
						"application/json": {
							Value: ResponseError{Error: "Too many requests"},
						},
					},
				},
			},
			expectedCodes: []int{400, 404, 500, 422, 429},
		},
		{
			name:       "auth route with string errors",
			accessRole: ACCESS_USER_ROLE,
			customErrors: map[int]swagger.ContentValue{
				409: {
					Description: "Conflict",
					Content: map[string]swagger.Schema{
						"application/json": {
							Value: ResponseError{Error: "Conflict"},
						},
					},
				},
				503: {
					Description: "Service unavailable",
					Content: map[string]swagger.Schema{
						"application/json": {
							Value: ResponseError{Error: "Service unavailable"},
						},
					},
				},
			},
			expectedCodes: []int{400, 401, 403, 404, 500, 409, 503},
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
