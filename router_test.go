package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	swagger "go.lumeweb.com/gswagger"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
)

func TestRegisterRoutes(t *testing.T) {
	tests := []struct {
		name              string
		routes            []RouteDefinition
		accessSvcErr      error
		wantRegisterErr   bool
		wantSwaggerGenErr bool
		wantAccessReg     bool
	}{
		{
			name: "successful registration",
			routes: []RouteDefinition{
				{
					Path:    "/test",
					Method:  "GET",
					Handler: func(c echo.Context) error { return nil },
				},
			},
			wantRegisterErr:   false,
			wantSwaggerGenErr: false,
			wantAccessReg:     false,
		},
		{
			name: "with access control",
			routes: []RouteDefinition{
				{
					Path:    "/secure",
					Method:  "GET",
					Handler: func(c echo.Context) error { return nil },
					Access:  "admin",
				},
			},
			wantRegisterErr:   false,
			wantSwaggerGenErr: false,
			wantAccessReg:     true,
		},
		{
			name: "access service error",
			routes: []RouteDefinition{
				{
					Path:    "/secure",
					Method:  "GET",
					Handler: func(c echo.Context) error { return nil },
					Access:  "admin",
				},
			},
			accessSvcErr:      assert.AnError,
			wantRegisterErr:   true,
			wantSwaggerGenErr: false,
			wantAccessReg:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRouter, err := NewRouter(APIInfo().
				Title("Test API").
				Version("1.0.0"))
			require.NoError(t, err)
			accessSvc := coreMocks.NewMockAccessService(t)

			if tt.wantAccessReg || tt.accessSvcErr != nil {
				// If access registration is expected OR an access service error is expected,
				// set up the mock to be called and return the specified error.
				accessSvc.On("RegisterRoute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(tt.accessSvcErr).Once()
			} else {
				// Otherwise, assert that RegisterRoute is not called.
				accessSvc.AssertNotCalled(t, "RegisterRoute", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}

			err = RegisterRoutes(eRouter, accessSvc, "test", tt.routes)

			if tt.wantRegisterErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			accessSvc.AssertExpectations(t)

			// Generate OpenAPI spec after routes are registered
			err = eRouter.GenerateAndExposeOpenapi()
			if tt.wantSwaggerGenErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify routes are registered by making test requests
			for _, route := range tt.routes {
				req := httptest.NewRequest(route.Method, route.Path, nil)
				rr := httptest.NewRecorder()
				eRouter.ServeHTTP(rr, req)
				assert.NotEqual(t, http.StatusNotFound, rr.Code)
			}
		})
	}
}

func TestDefineRoutes(t *testing.T) {
	r1 := RouteDefinition{Path: "/one"}
	r2 := RouteDefinition{Path: "/two"}

	routes := DefineRoutes(r1, r2)

	assert.Len(t, routes, 2)
	assert.Equal(t, "/one", routes[0].Path)
	assert.Equal(t, "/two", routes[1].Path)
}

func TestAuthSwagger(t *testing.T) {
	def := AuthSwagger(
		"Test Summary",
		"Test Description",
		jwt.PurposeLogin,
		nil,
	)

	assert.Equal(t, "Test Summary", def.Summary)
	assert.Equal(t, "Test Description", def.Description)
	assert.Contains(t, def.Tags, "Authenticated")
	assert.NotEmpty(t, def.Security)
	assert.Contains(t, def.Responses, http.StatusOK)
	assert.Contains(t, def.Responses, http.StatusUnauthorized)
	assert.Contains(t, def.Responses, http.StatusForbidden)
}

func TestBasicSwagger(t *testing.T) {
	def := BasicSwagger(
		"Test Summary",
		"Test Description",
		nil,
		nil,
		nil,
	)

	assert.Equal(t, "Test Summary", def.Summary)
	assert.Equal(t, "Test Description", def.Description)
	assert.Contains(t, def.Tags, "Public")
	assert.Contains(t, def.Responses, http.StatusOK)
	assert.NotContains(t, def.Security, "bearerAuth")
}

func TestSwaggerPathParam(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerPathParam(def, "id", "Test ID", "string")

	assert.NotNil(t, def.PathParams)
	assert.Equal(t, "Test ID", def.PathParams["id"].Description)
	assert.Equal(t, "string", def.PathParams["id"].Schema.Value)
}

func TestSwaggerQueryParam(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerQueryParam(def, "filter", "Test filter", "string")

	assert.NotNil(t, def.Querystring)
	assert.Equal(t, "Test filter", def.Querystring["filter"].Description)
	assert.Equal(t, "string", def.Querystring["filter"].Schema.Value)
}

func TestSwaggerPaginationParams(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerPaginationParams(def)

	assert.NotNil(t, def.Querystring)
	assert.Contains(t, def.Querystring, "_start")
	assert.Contains(t, def.Querystring, "_end")
	assert.Equal(t, 0, def.Querystring["_start"].Schema.Value)
	assert.Equal(t, 10, def.Querystring["_end"].Schema.Value)
}

func TestNewSwaggerRouter(t *testing.T) {
	eRouter, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))

	assert.NoError(t, err)
	assert.NotNil(t, eRouter)
}

func TestSwaggerDocsServed(t *testing.T) {
	echoRouter := echo.New()
	eRouter, err := NewSwaggerRouter(echoRouter, APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)
	require.NotNil(t, eRouter)

	// Register a test route first
	routes := DefineRoutes(
		RouteDefinition{
			Path:    "/test",
			Method:  "GET",
			Handler: func(c echo.Context) error { return nil },
		},
	)
	err = RegisterRoutes(eRouter, nil, "", routes)
	require.NoError(t, err)

	// Now generate the OpenAPI spec
	err = eRouter.GenerateAndExposeOpenapi()
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{
			name:     "JSON docs",
			path:     SwaggerJSONPath,
			wantCode: http.StatusOK,
		},
		{
			name:     "YAML docs",
			path:     SwaggerYAMLPath,
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			echoRouter.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)
			assert.NotEmpty(t, rr.Body.String())
		})
	}
}

func TestWithSortParams(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerSortParams(def, []string{"name", "date"})

	assert.NotNil(t, def.Querystring)
	assert.Contains(t, def.Querystring, "_sort")
	assert.Contains(t, def.Querystring, "_order")
	assert.Contains(t, def.Querystring["_sort"].Description, "name, date")
	assert.Equal(t, "", def.Querystring["_sort"].Schema.Value)
	assert.Equal(t, "", def.Querystring["_order"].Schema.Value)
}

func TestGetGroupRouter(t *testing.T) {
	t.Run("returns group when router is echo.Group", func(t *testing.T) {
		echoRouter := echo.New()

		// Create a swagger router with the echo router
		gRouter, err := NewSwaggerRouter(echoRouter, APIInfo().
			Title("Test API").
			Version("1.0.0"))
		require.NoError(t, err)

		// Create a subrouter from the group
		subRouter, err := gRouter.Group("/test")
		require.NoError(t, err)

		result := GetGroupRouter(Router(subRouter))
		assert.NotNil(t, result)

		// Verify group functionality by registering a route
		result.GET("/route", func(c echo.Context) error {
			return nil
		})

		req := httptest.NewRequest("GET", "/test/route", nil)
		rr := httptest.NewRecorder()
		echoRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("returns nil when router is not echo.Group", func(t *testing.T) {
		echoRouter := echo.New()

		// Create a swagger router with the echo router (not a group)
		gRouter, err := NewSwaggerRouter(echoRouter, APIInfo().
			Title("Test API").
			Version("1.0.0"))
		require.NoError(t, err)

		result := GetGroupRouter(gRouter)
		assert.Nil(t, result)
	})
}

func TestSwaggerFilterParam(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerFilterParam(def, "age_gt", "Filter ages greater than value", 18)

	assert.NotNil(t, def.Querystring)
	assert.Contains(t, def.Querystring, "age_gt")
	assert.Equal(t, "Filter ages greater than value", def.Querystring["age_gt"].Description)
	assert.Equal(t, 18, def.Querystring["age_gt"].Schema.Value)
}

func TestSwaggerHeaderParam(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerHeaderParam(def, "X-Custom-Header", "Custom header value", "string")

	assert.NotNil(t, def.Headers)
	assert.Contains(t, def.Headers, "X-Custom-Header")
	assert.Equal(t, "Custom header value", def.Headers["X-Custom-Header"].Description)
	assert.Equal(t, "string", def.Headers["X-Custom-Header"].Schema.Value)
}

func TestSwaggerCookieParam(t *testing.T) {
	def := swagger.Definitions{}
	def = SwaggerCookieParam(def, "session", "Session token", "string")

	assert.NotNil(t, def.Cookies)
	assert.Contains(t, def.Cookies, "session")
	assert.Equal(t, "Session token", def.Cookies["session"].Description)
	assert.Equal(t, "string", def.Cookies["session"].Schema.Value)
}

func TestListEndpointSwagger(t *testing.T) {
	def := ListEndpointSwagger(
		"List Users",
		"Returns paginated list of users",
		jwt.PurposeNone,
		map[string]string{"name": "string"},
		nil,
		[]string{"name", "created_at"},
		[]FilterParam{
			{
				Name:        "name_eq",
				Description: "Filter by exact name match",
				SchemaValue: "test",
			},
		},
		nil,
	)

	assert.Equal(t, "List Users", def.Summary)
	assert.Equal(t, "Returns paginated list of users", def.Description)
	assert.Contains(t, def.Tags, "Public")
	assert.Contains(t, def.Querystring, "_sort")
	assert.Contains(t, def.Querystring, "_order")
	assert.Contains(t, def.Querystring, "name_eq")
}

func TestTusPostSwagger(t *testing.T) {
	def := TusPostSwagger(
		"Create Upload",
		"Create a new TUS upload",
		map[int]any{
			401: map[string]string{"error": "Unauthorized"},
		},
	)

	assert.Equal(t, "Create Upload", def.Summary)
	assert.Equal(t, "Create a new TUS upload", def.Description)
	assert.Contains(t, def.Tags, "TUS")
	assert.Contains(t, def.Parameters, "Tus-Resumable")
	assert.Contains(t, def.Responses, http.StatusCreated)
	assert.Contains(t, def.Responses, 401)
}

func TestTusHeadSwagger(t *testing.T) {
	def := TusHeadSwagger(
		"Get Upload Status",
		"Get status of TUS upload",
		nil,
	)

	assert.Equal(t, "Get Upload Status", def.Summary)
	assert.Equal(t, "Get status of TUS upload", def.Description)
	assert.Contains(t, def.Tags, "TUS")
	assert.Contains(t, def.Responses, http.StatusOK)
}

func TestTusPatchSwagger(t *testing.T) {
	def := TusPatchSwagger(
		"Upload Chunk",
		"Upload a chunk of data",
		nil,
	)

	assert.Equal(t, "Upload Chunk", def.Summary)
	assert.Equal(t, "Upload a chunk of data", def.Description)
	assert.Contains(t, def.Tags, "TUS")
	assert.NotNil(t, def.RequestBody)
}

func TestTusDeleteSwagger(t *testing.T) {
	def := TusDeleteSwagger(
		"Delete Upload",
		"Delete a TUS upload",
		nil,
	)

	assert.Equal(t, "Delete Upload", def.Summary)
	assert.Equal(t, "Delete a TUS upload", def.Description)
	assert.Contains(t, def.Tags, "TUS")
	assert.Contains(t, def.Responses, http.StatusNoContent)
}

func TestTusOptionsSwagger(t *testing.T) {
	def := TusOptionsSwagger(
		"Get TUS Options",
		"Get supported TUS options",
		nil,
	)

	assert.Equal(t, "Get TUS Options", def.Summary)
	assert.Equal(t, "Get supported TUS options", def.Description)
	assert.Contains(t, def.Tags, "TUS")
	assert.Contains(t, def.Responses, http.StatusOK)
}

func TestWithSwagger(t *testing.T) {
	handler := func(c echo.Context) error { return nil }
	route := NewRoute("GET", "/test", handler, WithSwagger(WithSummary("Test Summary")))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)

	// Test route can be registered with NewRouter
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	assert.NoError(t, err)
}

func TestWithCustomErrorResponses(t *testing.T) {
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
			WithCustomErrorResponses(tt.customErrors)(&route)

			assert.Len(t, route.Swagger.Responses, len(tt.expectedCodes))
			for _, code := range tt.expectedCodes {
				assert.Contains(t, route.Swagger.Responses, code)
			}
		})
	}
}
