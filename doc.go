// Package router provides HTTP routing utilities with integrated OpenAPI/Swagger documentation.
// It combines Echo's routing capabilities with gswagger for API documentation generation.
//
// # Key Features
//
// - Route definition with built-in OpenAPI documentation
// - Authentication and authorization integration
// - Standardized request/response handling
// - TUS protocol support for file uploads
// - Pagination, sorting and filtering utilities
// - Automatic error response documentation
//
// # Getting Started
//
// Basic router setup:
//
//	router, err := router.NewRouter(router.APIInfo().
//	    Title("My API").
//	    Version("1.0.0").
//	    Description("API for managing resources"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Route Definition
//
// The package provides a fluent interface for defining routes:
//
//	routes := router.DefineRoutes(
//	    router.NewRoute("GET", "/users", getUsersHandler,
//	        router.WithAccess(router.ACCESS_ADMIN_ROLE),
//	        router.WithSwagger(
//	            router.WithSummary("List Users"),
//	            router.WithDescription("Returns paginated list of users"),
//	            router.WithTags("Users"),
//	            router.WithArrayResponse(http.StatusOK, "User list", User{}),
//	        ),
//	    ),
//	)
//
//	err := router.RegisterRoutes(router, accessSvc, "api", routes)
//
// # Swagger Documentation
//
// The package provides several helpers for common API patterns:
//
// ## Basic Endpoint
//
//	swagger := router.BasicSwagger(
//	    "Public Info",
//	    "Returns public information",
//	    map[int]any{
//	        http.StatusServiceUnavailable: map[string]string{"error": "Service unavailable"},
//	    },
//	)
//
// ## Authenticated Endpoint  
//
//	swagger := router.AuthSwagger(
//	    "Get Profile", 
//	    "Returns user profile",
//	    jwt.PurposeLogin,
//	    map[int]any{
//	        http.StatusNotFound: map[string]string{"error": "User not found"},
//	    },
//	)
//
// ## List Endpoint with Pagination
//
//	swagger := router.ListEndpointSwagger(
//	    "List Users",
//	    "Paginated user list",
//	    jwt.PurposeNone,
//	    User{},
//	    PaginationMeta{},
//	    []string{"name", "created_at"},
//	    []router.FilterParam{
//	        {Name: "name_contains", Description: "Filter by name contains", SchemaValue: ""},
//	    },
//	    nil,
//	)
//
// ## TUS Upload Endpoints
//
//	routes := router.DefineRoutes(
//	    router.NewRoute("POST", "/files", tusHandler,
//	        router.WithSwagger(router.TusPostSwagger(
//	            "Create Upload",
//	            "Create new TUS upload session",
//	            nil,
//	        )),
//	    ),
//	    // Other TUS methods...
//	)
//
// # Core Types
//
// - Router: Main router instance (*swagger.Router wrapper)
// - RouteDefinition: Defines a route and its documentation
// - APIInfoDefinition: API metadata builder
// - FieldSchema: Interface for sort/filter field definitions
//
// # Utilities
//
// - Parameter helpers:
//   - WithPathParam()
//   - WithQueryParam() 
//   - WithHeaderParam()
//   - WithCookieParam()
//
// - Pagination helpers:
//   - WithPaginationParams()
//   - WithSortParams()
//   - WithFilterParam()
//
// - Error handling:
//   - DefaultPublicErrorResponses()
//   - DefaultAuthErrorResponses()
//   - WithCustomErrorResponses()
//
// See package examples for more detailed usage.
package router
