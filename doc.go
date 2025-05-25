// Package router provides HTTP routing utilities with integrated OpenAPI/Swagger documentation.
// It combines Echo's routing capabilities with gswagger for API documentation generation.
//
// Key Features:
// - Route definition with built-in OpenAPI documentation
// - Authentication and authorization integration
// - Standardized request/response handling
// - TUS protocol support for file uploads
// - Pagination, sorting and filtering utilities
//
// The package provides a fluent interface for defining routes and their documentation:
//
//	routes := router.DefineRoutes(
//		router.NewRoute("GET", "/users", getUsersHandler,
//			router.WithAccess(router.ACCESS_ADMIN_ROLE),
//			router.WithSwagger(router.ListEndpointSwagger(
//				"List Users",
//				"Returns paginated list of users",
//				router.jwt.PurposeNone,
//				nil, // item schema
//				nil, // pagination schema
//				[]string{"name", "created_at"}, // sortable fields
//				nil, // filter params
//				nil, // error responses
//			)),
//		),
//	)
//
//	err := router.RegisterRoutes(gRouter, accessSvc, "api", routes)
//
// Common Patterns:
// - Use AuthSwagger() for authenticated endpoints
//   Example:
//     swagger := router.AuthSwagger(
//         "Get User Profile",
//         "Returns authenticated user's profile information",
//         router.jwt.PurposeLogin,
//         map[int]any{
//             http.StatusNotFound: map[string]string{"error": "User not found"},
//         },
//     )
//
// - Use BasicSwagger() for public endpoints
//   Example:
//     swagger := router.BasicSwagger(
//         "Public Info",
//         "Returns public system information",
//         map[int]any{
//             http.StatusServiceUnavailable: map[string]string{"error": "Service unavailable"},
//         },
//     )
//
// - Use ListEndpointSwagger() for paginated list endpoints
//   Example:
//     swagger := router.ListEndpointSwagger(
//         "List Users",
//         "Returns paginated list of users",
//         router.jwt.PurposeAdmin,
//         map[string]string{"name": "string", "email": "string"},
//         map[string]string{"total": "int", "pages": "int"},
//         []string{"name", "created_at"},
//         []router.FilterParam{
//             {Name: "name_eq", Description: "Filter by exact name", SchemaValue: "test"},
//         },
//         map[int]any{
//             http.StatusForbidden: map[string]string{"error": "Insufficient permissions"},
//         },
//     )
//
// - Use Tus*Swagger() functions for TUS file upload endpoints
//   Example TUS endpoint setup:
//     routes := router.DefineRoutes(
//         router.NewRoute("POST", "/files", tusHandler,
//             router.WithSwagger(router.TusPostSwagger(
//                 "Create Upload",
//                 "Create new TUS upload session",
//                 map[int]any{
//                     http.StatusForbidden: map[string]string{"error": "Upload quota exceeded"},
//                 },
//             )),
//         ),
//         router.NewRoute("HEAD", "/files/{id}", tusHandler,
//             router.WithSwagger(router.TusHeadSwagger(
//                 "Get Upload Status", 
//                 "Get status of TUS upload",
//                 nil,
//             )),
//         ),
//         // ... other TUS methods
//     )
//
// The package also provides utilities for:
// - Path/query parameter documentation
// - Error response standardization
// - Request validation
// - Middleware chaining
package router
