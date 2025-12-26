package router

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	swagger "go.lumeweb.com/gswagger"
	"go.lumeweb.com/portal-middleware/cors"
)

// AccessService defines the interface for route access control
type AccessService interface {
	// CheckAccess verifies if a user has access to a specific route
	CheckAccess(ctx context.Context, userId uint, fqdn, path, method string) (bool, error)

	// RegisterRoute registers a new route with its access requirements
	RegisterRoute(ctx context.Context, subdomain, path, method, role string) error
}

// Common access role constants
const (
	ACCESS_USER_ROLE  = "user"  // Standard user access role
	ACCESS_ADMIN_ROLE = "admin" // Administrator access role
)

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

// applyRouteOpts applies RouteOptions to a RouteDefinition and ensures proper initialization
func applyRouteOpts(d RouteDefinition, opts ...RouteOption) RouteDefinition {
	// Make shallow copy
	result := d

	// Initialize maps if nil
	if result.Swagger.Responses == nil {
		result.Swagger.Responses = make(map[int]swagger.ContentValue)
	}
	if result.Swagger.PathParams == nil {
		result.Swagger.PathParams = make(swagger.ParameterValue)
	}
	if result.Swagger.Querystring == nil {
		result.Swagger.Querystring = make(swagger.ParameterValue)
	}
	if result.Swagger.Headers == nil {
		result.Swagger.Headers = make(swagger.ParameterValue)
	}
	if result.Swagger.Cookies == nil {
		result.Swagger.Cookies = make(swagger.ParameterValue)
	}

	// Apply route options
	for _, opt := range opts {
		if opt != nil {
			opt(&result)
		}
	}

	// Prepend CORS middleware if configured
	if result.CorsConfig != nil {
		corsHandler := cors.NewWithDefaults(*result.CorsConfig)
		result.Middlewares = append([]echo.MiddlewareFunc{echo.WrapMiddleware(corsHandler)}, result.Middlewares...)
	}

	// Ensure we have at least the default success response, preserving existing ones
	result.Swagger.Responses = MergeResponses(
		result.Swagger.Responses,
		map[int]swagger.ContentValue{
			http.StatusOK: defaultSuccessResponse(),
		},
	)

	return result
}

// NewRoute creates a new RouteDefinition with the given method, path and handler,
// applying any provided RouteOptions.
func NewRoute(method, path string, handler echo.HandlerFunc, opts ...RouteOption) RouteDefinition {
	def := RouteDefinition{
		Method:  method,
		Path:    path,
		Handler: handler,
		// Defaults
		Access: ACCESS_USER_ROLE,
		Swagger: swagger.Definitions{
			Responses: DefineSwaggerErrorResponses(
				map[int]swagger.ContentValue{
					http.StatusOK:                  defaultSuccessResponse(),
					http.StatusBadRequest:          badRequestResponse(),
					http.StatusInternalServerError: internalServerErrorResponse(),
				},
				DefaultCoreErrorResponses(),
			),
		},
		CorsConfig: nil,
	}

	return applyRouteOpts(def, opts...)
}

// Option setters
// WithAccess creates a RouteOption that sets the required access role for a route.
func WithAccess(accessRole string) RouteOption {
	return func(d *RouteDefinition) {
		d.Access = accessRole
	}
}

// WithMiddlewares creates a RouteOption that adds middleware functions to a route.
// Middlewares will be executed in the order they are provided.
func WithMiddlewares(middleware ...echo.MiddlewareFunc) RouteOption {
	return func(d *RouteDefinition) {
		d.Middlewares = append(d.Middlewares, middleware...)
	}
}

// WithCors creates a RouteOption that sets CORS configuration for the route.
// The CORS middleware will be prepended to the middleware chain.
// If no configs are provided, an empty config will be used which will apply defaults.
func WithCors(configs ...cors.Config) RouteOption {
	return func(d *RouteDefinition) {
		if len(configs) > 0 {
			d.CorsConfig = &configs[0]
		} else {
			d.CorsConfig = &cors.Config{}
		}
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
