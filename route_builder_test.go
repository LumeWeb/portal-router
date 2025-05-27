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

func TestWithSwagger(t *testing.T) {
	handler := func(c echo.Context) error { return nil }
	swaggerDef := swagger.Definitions{
		Summary: "Test Summary",
	}

	route := NewRoute("GET", "/test", handler, WithSwagger(swaggerDef))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)

	// Test route can be registered with NewRouter
	router, err := NewRouter(APIInfo().
		Title("Test API").
		Version("1.0.0"))
	require.NoError(t, err)

	_, err = router.AddRoute(route.Method, route.Path, route.Handler, route.Swagger)
	assert.NoError(t, err)
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
			responseVal := resp[tt.status].(ResponseError)
			assert.Equal(t, tt.wantError, responseVal.Error)
		})
	}
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
	
	notFound := combined[http.StatusNotFound].(ResponseError)
	assert.Equal(t, "Not found", notFound.Error)
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

	t.Run("router with echo options", func(t *testing.T) {
		info := APIInfo().
			Title("Test API").
			Version("1.0.0")

		// Test with Echo configuration option
		router, err := NewRouter(info, RouterOption(func(e *echo.Echo) {
			e.HideBanner = true
		}))
		assert.NoError(t, err)
		assert.NotNil(t, router)
		assert.True(t, GetRouter(router).HideBanner)
	})

	t.Run("invalid api info", func(t *testing.T) {
		// Empty API info should fail validation
		_, err := NewRouter(APIInfo())
		assert.Error(t, err)
	})
}
