package router

import (
	"github.com/stretchr/testify/require"
	swagger "go.lumeweb.com/gswagger"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewRoute(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler)

	assert.Equal(t, "GET", route.Method)
	assert.Equal(t, "/test", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Equal(t, ACCESS_USER_ROLE, route.Access)
	assert.NotNil(t, route.Swagger)
	assert.Empty(t, route.Middlewares)

	// Test route can be registered with NewRouter
	eRouter, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = eRouter.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	assert.NoError(t, err)
}

func TestWithAccess(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler, WithAccess(ACCESS_ADMIN_ROLE))

	assert.Equal(t, ACCESS_ADMIN_ROLE, route.Access)

	// Test route can be registered with NewRouter
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	assert.NoError(t, err)
}

func TestWithMiddlewares(t *testing.T) {
	handler := func(c echo.Context) error { return nil }
	mw1 := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}
	mw2 := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}

	route := NewRoute("GET", "/test", handler, WithMiddlewares(mw1, mw2))

	assert.Len(t, route.Middlewares, 2)

	// Test route can be registered with NewRouter
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger, route.Middlewares...)
	assert.NoError(t, err)
}

func TestParameterOptions(t *testing.T) {
	tests := []struct {
		name     string
		option   SwaggerOption
		paramKey string
	}{
		{
			name:     "WithPathParam",
			option:   WithPathParam("id", "Resource ID", "string"),
			paramKey: "id",
		},
		{
			name:     "WithQueryParam",
			option:   WithQueryParam("filter", "Filter criteria", "string"),
			paramKey: "filter",
		},
		{
			name:     "WithHeaderParam",
			option:   WithHeaderParam("X-Custom-Header", "Custom header", "string"),
			paramKey: "X-Custom-Header",
		},
		{
			name:     "WithCookieParam",
			option:   WithCookieParam("session", "Session token", "string"),
			paramKey: "session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := swagger.Definitions{}
			tt.option(&def, "")

			var params swagger.ParameterValue
			switch tt.name {
			case "WithPathParam":
				params = def.PathParams
			case "WithQueryParam":
				params = def.Querystring
			case "WithHeaderParam":
				params = def.Headers
			case "WithCookieParam":
				params = def.Cookies
			}

			assert.NotNil(t, params)
			assert.Contains(t, params, tt.paramKey)
			assert.Equal(t, "string", params[tt.paramKey].Schema.Value)
		})
	}
}

func TestSwaggerParameterFunctions(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(swagger.Definitions, string, string, any) swagger.Definitions
		paramKey string
	}{
		{
			name:     "SwaggerPathParam",
			fn:       SwaggerPathParam,
			paramKey: "id",
		},
		{
			name:     "SwaggerQueryParam",
			fn:       SwaggerQueryParam,
			paramKey: "filter",
		},
		{
			name:     "SwaggerHeaderParam",
			fn:       SwaggerHeaderParam,
			paramKey: "X-Custom-Header",
		},
		{
			name:     "SwaggerCookieParam",
			fn:       SwaggerCookieParam,
			paramKey: "session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := swagger.Definitions{}
			def = tt.fn(def, tt.paramKey, "test description", "string")

			var params swagger.ParameterValue
			switch tt.name {
			case "SwaggerPathParam":
				params = def.PathParams
			case "SwaggerQueryParam":
				params = def.Querystring
			case "SwaggerHeaderParam":
				params = def.Headers
			case "SwaggerCookieParam":
				params = def.Cookies
			}

			assert.NotNil(t, params)
			assert.Contains(t, params, tt.paramKey)
			assert.Equal(t, "string", params[tt.paramKey].Schema.Value)
		})
	}
}

func TestRouteExecution(t *testing.T) {
	called := false
	handler := func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	}

	route := NewRoute("GET", "/test", handler)

	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestNewRouter(t *testing.T) {
	t.Run("basic router creation", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0")

		router, err := NewRouter(info)
		assert.NoError(t, err)
		assert.NotNil(t, router)
	})

	t.Run("router with options", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0")

		// Test with router configuration option
		router, err := NewRouter(info,
			WithRouterBasePath("/v1"),
			WithRouterJSONDocsPath("/api/docs.json"),
		)
		assert.NoError(t, err)
		assert.NotNil(t, router)

		echoRouter := GetRouter(router)
		require.NotNil(t, echoRouter, "router should not be nil")

		req := httptest.NewRequest("GET", "/v1/test", nil)
		rr := httptest.NewRecorder()
		echoRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code) // Should route through prefix
	})

	t.Run("invalid api info", func(t *testing.T) {
		// Empty API info should fail validation
		_, err := NewRouter(APIInfo())
		assert.Error(t, err)
	})

	t.Run("router with servers", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0")

		router, err := NewRouter(info,
			WithServer("https://api.example.com", "Production API"),
			WithServer("https://staging-api.example.com", "Staging API"),
		)
		assert.NoError(t, err)
		assert.NotNil(t, router)
		assert.Len(t, router.GetSwaggerSchema().Servers, 3) // Includes default /api server
	})
}
