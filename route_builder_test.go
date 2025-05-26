package router

import (
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
}

func TestWithAccess(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	route := NewRoute("GET", "/test", handler, WithAccess(ACCESS_ADMIN_ROLE))

	assert.Equal(t, ACCESS_ADMIN_ROLE, route.Access)
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
}

func TestWithSwagger(t *testing.T) {
	handler := func(c echo.Context) error { return nil }
	swaggerDef := swagger.Definitions{
		Summary: "Test Summary",
	}

	route := NewRoute("GET", "/test", handler, WithSwagger(swaggerDef))

	assert.Equal(t, "Test Summary", route.Swagger.Summary)
}

func TestRouteExecution(t *testing.T) {
	called := false
	handler := func(c echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	}

	route := NewRoute("GET", "/test", handler)

	router := echo.New()
	router.Add(route.Method, route.Path, route.Handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}
