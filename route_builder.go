package router

import (
	"github.com/labstack/echo/v4"
	swagger "go.lumeweb.com/gswagger"
)

// Common access role constants
const (
	ACCESS_USER_ROLE  = "user"  // Standard user access role
	ACCESS_ADMIN_ROLE = "admin" // Administrator access role
)

// ResponseError defines a standard error response structure for Swagger documentation
type ResponseError struct {
	Error string `json:"error"`
}

// DefineSwaggerErrorResponse creates a Swagger-compatible error response definition.
func DefineSwaggerErrorResponse(status int, errorMsg string) map[int]any {
	return map[int]any{
		status: ResponseError{Error: errorMsg},
	}
}

// DefineSwaggerErrorResponses combines multiple error responses for Swagger docs.
func DefineSwaggerErrorResponses(responses ...map[int]any) map[int]any {
	combined := make(map[int]any)
	for _, r := range responses {
		for code, resp := range r {
			combined[code] = resp
		}
	}
	return combined
}

// RouteOption defines a function type for modifying RouteDefinition properties.
type RouteOption func(*RouteDefinition)

// DefineOptions converts variadic RouteOption arguments into a slice of RouteOptions.
// This helper function allows cleaner syntax when defining routes with multiple options.
//
// Example:
//
//	router.NewRoute("GET", "/path", handler,
//	    DefineOptions(
//	        WithAccess(ACCESS_ADMIN_ROLE),
//	        WithMiddlewares(mw1, mw2),
//	    )...,
//	)
func DefineOptions(opts ...RouteOption) []RouteOption {
	return opts
}

// Core route builder
// NewRoute creates a new RouteDefinition with the given method, path and handler,
// applying any provided RouteOptions.
func NewRoute(method, path string, handler echo.HandlerFunc, opts ...RouteOption) RouteDefinition {
	def := RouteDefinition{
		Method:  method,
		Path:    path,
		Handler: handler,
		// Defaults
		Access:  ACCESS_USER_ROLE,
		Swagger: swagger.Definitions{},
	}

	for _, opt := range opts {
		opt(&def)
	}

	return def
}

// Option setters
// WithAccess creates a RouteOption that sets the required access role for a route.
func WithAccess(accessRole string) RouteOption {
	return func(d *RouteDefinition) {
		d.Access = accessRole
	}
}

// WithSwagger creates a RouteOption that sets the Swagger documentation definitions
// for a route.
func WithSwagger(def swagger.Definitions) RouteOption {
	return func(d *RouteDefinition) {
		d.Swagger = def
	}
}

// WithMiddlewares creates a RouteOption that adds middleware functions to a route.
// Middlewares will be executed in the order they are provided.
func WithMiddlewares(middleware ...echo.MiddlewareFunc) RouteOption {
	return func(d *RouteDefinition) {
		d.Middlewares = append(d.Middlewares, middleware...)
	}
}

// Middlewares is a convenience function that returns the provided middleware functions as a slice.
// This can be used to make middleware declarations more readable when passing multiple middlewares.
//
// Example:
//
//	router.NewRoute("GET", "/path", handler,
//	    WithMiddlewares(Middlewares(mw1, mw2, mw3)))
func Middlewares(mw ...echo.MiddlewareFunc) []echo.MiddlewareFunc {
	return mw
}
