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

// RouteOption defines a function type for modifying RouteDefinition properties.
type RouteOption func(*RouteDefinition)

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
