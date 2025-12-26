package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/invopop/jsonschema"
	"go.lumeweb.com/gswagger/apirouter"
	"go.lumeweb.com/portal-middleware/cors"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	swagger "go.lumeweb.com/gswagger"
	es "go.lumeweb.com/gswagger/support/echo"
	"go.lumeweb.com/portal-middleware/auth/jwt"
)

const (
	SwaggerJSONPath = "/swagger.json" // Default path for JSON OpenAPI spec
	SwaggerYAMLPath = "/swagger.yaml" // Default path for YAML OpenAPI spec
)

type Router = *swagger.Router[echo.HandlerFunc, echo.MiddlewareFunc, es.Route]

// GetRouter returns the underlying echo router (either *echo.Echo or *echo.Group)
func GetRouter(r Router) *echo.Echo {
	if r == nil {
		return nil
	}

	// Get the framework router interface
	frameworkRouter := r.Router().Router(false)

	// Try to cast directly to *echo.Echo first
	if e, ok := frameworkRouter.(*echo.Echo); ok {
		return e
	}

	// Fallback for other cases
	return nil
}

// GetGroup returns the underlying *echo.Group instance
func GetGroup(r Router) *echo.Group {
	router := r.Router().Router(true)
	if g, ok := router.(*echo.Group); ok {
		return g
	}
	return nil
}

func GetGroupRouter(r Router) *echo.Group {
	return GetGroup(r)
}

// NewSwaggerRouter creates a new gswagger Router instance from an echo.Echo with default OpenAPI options.
// It initializes the OpenAPI specification with the provided API info and sets up standard documentation paths.
//
// Parameters:
//   - echoRouter: The base Echo router that will handle the actual HTTP requests
//   - info: API metadata including title, version, description etc (use APIInfo() builder)
//
// Returns:
//   - *swagger.Router: Configured router ready for route registration
//   - error: Any initialization error
//
// Example:
//
//	muxRouter := mux.NewRouter()
//	gRouter, err := NewSwaggerRouter(muxRouter, APIInfo().
//	    Title("My API").
//	    Version("1.0.0"))
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewSwaggerRouter(info APIInfoDefinition, opts ...RouterOption) (Router, error) {
	echoFactory := func() *echo.Echo {
		e := echo.New()
		// Trusted proxy assumption: service should be behind reverse proxy like nginx/Cloudflare
		// Do not expose service directly to untrusted networks
		e.IPExtractor = echo.ExtractIPFromXFFHeader()
		return e
	}

	// Initialize config with defaults
	config := &RouterConfig{
		EchoRouter: echoFactory(),
		OpenAPI: &openapi3.T{
			Info: info.toOpenAPI(),
			Servers: []*openapi3.Server{
				{
					URL: "/api",
				},
			},
		},
		Options: swagger.Options[echo.HandlerFunc, echo.MiddlewareFunc, es.Route]{
			JSONDocumentationPath: SwaggerJSONPath,
			YAMLDocumentationPath: SwaggerYAMLPath,
			FrameworkRouterFactory: func() apirouter.Router[echo.HandlerFunc, echo.MiddlewareFunc, es.Route] {
				return es.NewRouter(echoFactory())
			},
		},
	}

	// Apply all options
	applyRouterOptions(config, opts...)

	// Create router with final config
	options := config.Options
	if config.PathPrefix != "" {
		options.PathPrefix = config.PathPrefix
	}
	if config.OpenAPI != nil {
		options.Openapi = config.OpenAPI
	}
	if config.ReflectorOptions != nil {
		options.ReflectorOptions = config.ReflectorOptions
	}

	router, err := swagger.NewRouter(
		es.NewRouter(config.EchoRouter),
		options,
	)
	if err != nil {
		return nil, err
	}

	return router, nil
}

// UpdateRouterInfo updates the OpenAPI info for a router using the provided APIInfoDefinition
func UpdateRouterInfo(r Router, info APIInfoDefinition) {
	// Convert the API info to OpenAPI format
	openapiInfo := info.toOpenAPI()

	// Update the router's info
	r.SetInfo(openapiInfo)
}

// RouterConfig holds configuration for router initialization
type RouterConfig struct {
	EchoRouter       *echo.Echo
	OpenAPI          *openapi3.T
	Options          swagger.Options[echo.HandlerFunc, echo.MiddlewareFunc, es.Route]
	PathPrefix       string
	ReflectorOptions *jsonschema.Reflector
}

// RouterOption defines a function type for configuring router initialization.
type RouterOption func(*RouterConfig)

// applyRouterOptions applies all router configuration options
func applyRouterOptions(c *RouterConfig, opts ...RouterOption) {
	for _, opt := range opts {
		opt(c)
	}
}

// WithRouterJSONDocsPath sets the path for JSON documentation
func WithRouterJSONDocsPath(path string) RouterOption {
	return func(c *RouterConfig) {
		c.Options.JSONDocumentationPath = path
	}
}

// WithRouterYAMLDocsPath sets the path for YAML documentation
func WithRouterYAMLDocsPath(path string) RouterOption {
	return func(c *RouterConfig) {
		c.Options.YAMLDocumentationPath = path
	}
}

// WithRouterBasePath sets the base path prefix for all routes
func WithRouterBasePath(path string) RouterOption {
	return func(c *RouterConfig) {
		c.PathPrefix = path
	}
}

// WithRouterOpenAPI allows custom OpenAPI configuration
func WithRouterOpenAPI(openapi *openapi3.T) RouterOption {
	return func(c *RouterConfig) {
		c.OpenAPI = openapi
	}
}

// WithReflectorOptions configures the jsonschema reflector options
// used for schema generation.
func WithReflectorOptions(opts *jsonschema.Reflector) RouterOption {
	return func(c *RouterConfig) {
		c.ReflectorOptions = opts
	}
}

// WithServer adds a server URL to the OpenAPI spec
func WithServer(url string, description string) RouterOption {
	return func(c *RouterConfig) {
		if c.OpenAPI == nil {
			return
		}
		if c.OpenAPI.Servers == nil {
			c.OpenAPI.Servers = []*openapi3.Server{}
		}
		c.OpenAPI.Servers = append(c.OpenAPI.Servers, &openapi3.Server{
			URL:         url,
			Description: description,
		})
	}
}

// WithServers replaces all server URLs in the OpenAPI spec
func WithServers(servers []*openapi3.Server) RouterOption {
	return func(c *RouterConfig) {
		if c.OpenAPI == nil {
			return
		}
		c.OpenAPI.Servers = servers
	}
}

// NewRouter creates a new Echo router with Swagger integration and sensible defaults.
// This is a convenience wrapper that handles both Echo and Swagger router initialization.
//
// Parameters:
//   - info: API metadata (use APIInfo() builder)
//   - opts: Optional router configuration options
//
// Returns:
//   - Router: The Swagger-wrapped router
//   - error: Any initialization error
//
// Example:
//
//	router, err := NewRouter(APIInfo().
//	    Title("My API").
//	    Version("1.0.0"))
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewRouter(info APIInfoDefinition, opts ...RouterOption) (Router, error) {
	return NewSwaggerRouter(info, opts...)
}

// ListEndpointSwagger generates Swagger definitions for a list endpoint.
func ListEndpointSwagger(
	summary, description string,
	purpose jwt.Purpose,
	itemSchema any,
	paginationSchema any,
	sortableFields []string,
	filterParams []FilterParam,
	errResp map[int]any,
	opts ...SwaggerOption,
) swagger.Definitions {
	// Start with either AuthSwagger or BasicSwagger based on purpose
	var def swagger.Definitions
	if purpose != jwt.PurposeNone {
		def = AuthSwagger(summary, description, purpose, errResp)
	} else {
		def = BasicSwagger(summary, description, errResp)
	}

	// Apply standard list endpoint options
	def = SwaggerPaginationParams(def)
	if len(sortableFields) > 0 {
		def = SwaggerSortParams(def, sortableFields)
	}
	def = SwaggerGlobalSearchParam(def)

	// Add filter parameters
	for _, fp := range filterParams {
		def = SwaggerFilterParam(def, fp.Name, fp.Description, fp.SchemaValue)
	}

	// Create response options based on pagination
	var responseOpt SwaggerOption
	if paginationSchema != nil {
		responseOpt = WithPaginatedResponse(itemSchema, paginationSchema)
	} else {
		responseOpt = WithArrayResponse(http.StatusOK, "Success", itemSchema)
	}

	// Apply all options including the response option
	allOpts := append([]SwaggerOption{responseOpt}, opts...)
	def = applySwaggerOpts(def, "", allOpts)

	return def
}

// Route is an alias for RouteDefinition for backwards compatibility.
type Route = RouteDefinition

// RouteDefinition defines a single HTTP route and its associated documentation and middleware.
type RouteDefinition struct {
	// Path is the URL path for the route (e.g., "/users/{id}").
	Path string
	// Method is the HTTP method for the route (e.g., "GET", "POST").
	Method string
	// Handler is the echo.HandlerFunc that handles the request.
	Handler echo.HandlerFunc
	// Access is the required access role for this route (e.g., core.ACCESS_ADMIN_ROLE).
	Access string
	// Swagger contains the OpenAPI definitions for this route.
	Swagger swagger.Definitions
	// Middlewares contains route-specific middleware chain
	Middlewares []echo.MiddlewareFunc
	// CorsConfig stores the CORS middleware configuration for the route
	CorsConfig *cors.Config
}

// RegisterRoutes registers a slice of RouteDefinitions with the provided Router.
// It handles:
// - Applying common middleware
// - Initializing route definitions
// - Registering routes with the router
// - Setting up access control
func RegisterRoutes(
	gRouter Router,
	accessSvc AccessService,
	subdomain string,
	routes []RouteDefinition,
	commonOpts ...RouteOption,
) error {
	// Create a group with common middleware if any exist
	var group = gRouter

	for _, route := range routes {
		// Apply all route options including common ones
		finalRoute := applyRouteOpts(route, commonOpts...)

		// Register route with router
		_, err := group.AddRoute(
			finalRoute.Method,
			finalRoute.Path,
			finalRoute.Handler,
			finalRoute.Swagger,
			finalRoute.Middlewares...,
		)
		if err != nil {
			return fmt.Errorf("failed to register route %s %s: %w", route.Method, route.Path, err)
		}

		// Register access control if needed
		if finalRoute.Access != "" && accessSvc != nil {
			if err := accessSvc.RegisterRoute(context.Background(), subdomain, finalRoute.Path, finalRoute.Method, finalRoute.Access); err != nil {
				return fmt.Errorf("failed to register access for route %s: %w", finalRoute.Path, err)
			}
		}
	}
	return nil
}

// DefineRoutes is a helper function to create a slice of RouteDefinition
// from a variable number of arguments.
// This allows defining routes without explicitly typing the slice literal.
// Example:
// routes := httputil.DefineRoutes(
//
//	{...}, // RouteDefinition 1
//	{...}, // RouteDefinition 2
//
// )
func DefineRoutes(routes ...RouteDefinition) []RouteDefinition {
	return routes
}

// AuthSwagger generates Swagger definitions for an authenticated endpoint.
// It includes standard security schemes (Bearer token) and common responses
// for authentication/authorization failures (401, 403).
//
// Parameters:
// - summary: A brief summary of the operation.
// - description: A detailed description of the operation.
// - purpose: The JWT purpose required for this endpoint (e.g., jwt.PurposeLogin).
// - reqBody: An instance of the request body DTO for schema generation. Can be nil.
// - respBody: An instance of the success response body DTO for schema generation. Can be nil.
// - errResp: A map of additional error status codes and example response bodies.
func AuthSwagger(
	summary, description string,
	purpose jwt.Purpose,
	errResp map[int]any,
) swagger.Definitions {
	def := baseDefinition()
	def.Summary = summary
	def.Description = description
	def.Tags = []string{"Authenticated"}
	def.Security = swagger.SecurityRequirements{
		{"bearerAuth": []string{string(purpose)}},
	}

	// Add standard responses
	def.Responses[http.StatusOK] = defaultSuccessResponse()
	def.Responses[http.StatusUnauthorized] = unauthorizedResponse()
	def.Responses[http.StatusForbidden] = forbiddenResponse()

	convertedErrors := convertErrorResponses(errResp)

	// Merge with error responses while preserving defaults
	def.Responses = MergeResponses(
		def.Responses,
		convertedErrors,
		map[int]swagger.ContentValue{
			http.StatusOK: defaultSuccessResponse(),
		},
	)

	return def
}

// BasicSwagger generates basic Swagger definitions for an endpoint.
// It does not include authentication security schemes by default.
//
// Parameters:
// - summary: A brief summary of the operation.
// - description: A detailed description of the operation.
// - reqBody: An instance of the request body DTO for schema generation. Can be nil.
// - respBody: An instance of the success response body DTO for schema generation. Can be nil.
// - errResp: A map of additional error status codes and response bodies (use DefineSwaggerErrorResponse(s) helpers).
func BasicSwagger(
	summary, description string,
	errResp map[int]any,
	opts ...SwaggerOption,
) swagger.Definitions {
	def := baseDefinition()
	def.Summary = summary
	def.Description = description
	def.Tags = []string{"Public"}

	// Convert map[int]any to map[int]swagger.ContentValue
	convertedErrors := make(map[int]swagger.ContentValue)
	for code, body := range errResp {
		convertedErrors[code] = swagger.ContentValue{
			Description: http.StatusText(code),
			Content: swagger.Content{
				MediaTypeJSON: {
					Value: body,
				},
			},
		}
	}

	// Merge with error responses while preserving defaults
	def.Responses = MergeResponses(
		def.Responses,
		convertedErrors,
		map[int]swagger.ContentValue{
			http.StatusOK: defaultSuccessResponse(),
		},
	)

	return applySwaggerOpts(def, "", opts)
}

// PaginatedResponseSwagger generates Swagger definitions for an endpoint
// that returns a paginated list of items.
//
// Parameters:
// - summary, description: Standard operation details.
// - purpose: JWT purpose (for authenticated endpoints). Use jwt.PurposeNone for public.
// - reqBody: Request body schema (can be nil).
// - itemSchema: An instance of the schema for a single item in the list.
// - paginationSchema: An instance of the schema for pagination metadata (can be nil).
// - errResp: Additional error responses.
func PaginatedResponseSwagger(summary, description string, purpose jwt.Purpose, itemSchema, paginationSchema any, errResp map[int]any) swagger.Definitions {
	// Start with either AuthSwagger or BasicSwagger based on purpose
	var definitions swagger.Definitions
	if purpose != jwt.PurposeNone {
		definitions = AuthSwagger(summary, description, purpose, errResp)
	} else {
		definitions = BasicSwagger(summary, description, errResp)
	}

	// Define the schema for the paginated response
	// This is typically an object containing the list of items and pagination metadata
	paginatedResponseSchema := map[string]any{
		"items": map[string]any{
			"type":  "array",
			"items": itemSchema, // Schema for a single item
		},
	}

	// Add pagination metadata schema if provided
	if paginationSchema != nil {
		// Assuming paginationSchema is a struct or map that defines the metadata fields
		// We need to merge its schema into the paginatedResponseSchema
		// A simple way is to reflect its fields, but a more robust way might involve
		// using jsonschema.Reflector directly if needed. For simplicity, let's assume
		// paginationSchema is a map or struct that jsonschema can handle.
		// A more advanced approach might involve combining schemas using 'allOf'.
		// For now, let's just add it as a property. This might not be perfect
		// depending on the exact structure you want.
		paginatedResponseSchema["pagination"] = paginationSchema
	}

	// Update the 200 OK response content with the paginated schema
	definitions.Responses[http.StatusOK] = swagger.ContentValue{
		Description: "Success",
		Content:     swagger.Content{MediaTypeJSON: {Value: paginatedResponseSchema}},
	}

	return definitions
}

// FileUploadSwaggerBody generates the RequestBody definition for a file upload endpoint.
//
// Parameters:
// - fileFieldName: The name of the form field for the file.
// - fileDescription: Description for the file field.
// - additionalFields: A map of additional form fields (name -> schema value).
func FileUploadSwaggerBody(
	fileFieldName string,
	fileDescription string,
	additionalFields map[string]any,
) *swagger.ContentValue {
	properties := map[string]any{
		fileFieldName: map[string]any{
			"type":        "string",
			"format":      "binary", // Indicates file upload
			"description": fileDescription,
		},
	}

	// Add additional form fields
	for name, schemaValue := range additionalFields {
		properties[name] = schemaValue
	}

	return &swagger.ContentValue{
		Content: swagger.Content{
			"multipart/form-data": {
				Value: map[string]any{
					"type":       "object",
					"properties": properties,
				},
			},
		},
		//Required: true, // File uploads typically require a body
	}
}

// SwaggerPathParam adds a path parameter definition to existing Swagger definitions.
// This is intended to be chained.
func SwaggerPathParam(d swagger.Definitions, name, description string, schemaValue any) swagger.Definitions {
	if d.PathParams == nil {
		d.PathParams = make(swagger.ParameterValue)
	}
	d.PathParams[name] = swagger.Parameter{
		Description: description,
		Schema:      &swagger.Schema{Value: schemaValue},
	}
	return d
}

// SwaggerQueryParam adds a query parameter definition to existing Swagger definitions.
// This is intended to be chained.
func SwaggerQueryParam(d swagger.Definitions, name, description string, schemaValue any) swagger.Definitions {
	if d.Querystring == nil {
		d.Querystring = make(swagger.ParameterValue)
	}
	d.Querystring[name] = swagger.Parameter{
		Description: description,
		Schema:      &swagger.Schema{Value: schemaValue},
	}
	return d
}

// SwaggerHeaderParam adds a header parameter definition to existing Swagger definitions.
// This is intended to be chained.
func SwaggerHeaderParam(d swagger.Definitions, name, description string, schemaValue any) swagger.Definitions {
	if d.Headers == nil {
		d.Headers = make(swagger.ParameterValue)
	}
	d.Headers[name] = swagger.Parameter{
		Description: description,
		Schema:      &swagger.Schema{Value: schemaValue},
	}
	return d
}

// SwaggerCookieParam adds a cookie parameter definition to existing Swagger definitions.
// This is intended to be chained.
func SwaggerCookieParam(d swagger.Definitions, name, description string, schemaValue any) swagger.Definitions {
	if d.Cookies == nil {
		d.Cookies = make(swagger.ParameterValue)
	}
	d.Cookies[name] = swagger.Parameter{
		Description: description,
		Schema:      &swagger.Schema{Value: schemaValue},
	}
	return d
}

// EmptyResponseSwagger generates the Swagger definition for a 200 OK response with no body.
func EmptyResponseSwagger() swagger.ContentValue {
	return swagger.ContentValue{
		Description: "Success",
		Content:     nil, // Or an empty map: swagger.Content{}
	}
}

// ErrorResponseSwagger generates a map for a single additional error response.
// Useful for adding one-off custom error responses to the errResp map.
func ErrorResponseSwagger(status int, body any) map[int]any {
	return map[int]any{
		status: body,
	}
}

// SwaggerPaginationParams adds standard pagination query parameters (_start, _end) to Swagger definitions.
// It takes the base Swagger definitions and returns the modified definitions for chaining.
func SwaggerPaginationParams(d swagger.Definitions) swagger.Definitions {
	if d.Querystring == nil {
		d.Querystring = make(swagger.ParameterValue)
	}
	d.Querystring["_start"] = swagger.Parameter{
		Description: "Starting index of the items to return (0-based). Defaults to 0.",
		Schema:      &swagger.Schema{Value: 0},
	}
	d.Querystring["_end"] = swagger.Parameter{
		Description: "Ending index of the items to return (exclusive). Defaults to 10.",
		Schema:      &swagger.Schema{Value: 10},
	}
	return d
}

// SwaggerSortParams adds standard sorting query parameters (_sort, _order) to Swagger definitions.
// Parameters:
// - d: The base Swagger definitions
// - sortableFields: A list of fields that can be sorted. Used in the description.
func SwaggerSortParams(d swagger.Definitions, sortableFields []string) swagger.Definitions {
	if d.Querystring == nil {
		d.Querystring = make(swagger.ParameterValue)
	}
	d.Querystring["_sort"] = swagger.Parameter{
		Description: fmt.Sprintf("Comma-separated list of fields to sort by. Available fields: %s", strings.Join(sortableFields, ", ")),
		Schema:      &swagger.Schema{Value: ""},
	}
	d.Querystring["_order"] = swagger.Parameter{
		Description: "Comma-separated list of sort orders ('asc' or 'desc') corresponding to _sort fields. Defaults to 'asc'.",
		Schema:      &swagger.Schema{Value: ""},
	}
	return d
}

// SortParams returns the standard sorting query parameters (_sort, _order).
// Parameters:
// - sortableFields: A list of fields that can be sorted. Used in the description.
func SortParams(sortableFields []string) swagger.ParameterValue {
	return swagger.ParameterValue{
		"_sort": swagger.Parameter{
			Description: fmt.Sprintf("Comma-separated list of fields to sort by. Available fields: %s", strings.Join(sortableFields, ", ")),
			Schema:      &swagger.Schema{Value: ""},
		},
		"_order": swagger.Parameter{
			Description: "Comma-separated list of sort orders ('asc' or 'desc') corresponding to _sort fields. Defaults to 'asc'.",
			Schema:      &swagger.Schema{Value: ""},
		},
	}
}

// SwaggerFilterParam adds a single filter query parameter to Swagger definitions.
// This helper is for simple filters like `fieldName_operator=value`.
//
// Parameters:
// - d: The base Swagger definitions.
// - name: The full query parameter name (e.g., "age_gte", "status_eq").
// - description: Description of the filter parameter.
// - schemaValue: An example value for schema generation (e.g., 30, "active").
func SwaggerFilterParam(d swagger.Definitions, name, description string, schemaValue any) swagger.Definitions {
	if d.Querystring == nil {
		d.Querystring = make(swagger.ParameterValue)
	}
	d.Querystring[name] = swagger.Parameter{
		Description: description,
		Schema:      &swagger.Schema{Value: schemaValue},
	}
	return d
}

// SwaggerGlobalSearchParam adds the standard global search query parameter ('q') to Swagger definitions.
func SwaggerGlobalSearchParam(d swagger.Definitions) swagger.Definitions {
	if d.Querystring == nil {
		d.Querystring = make(swagger.ParameterValue)
	}
	d.Querystring["q"] = swagger.Parameter{
		Description: "Global search query string.",
		Schema:      &swagger.Schema{Value: ""},
	}
	return d
}

// GlobalSearchParam returns the standard global search query parameter ('q').
func GlobalSearchParam() swagger.ParameterValue {
	return swagger.ParameterValue{
		"q": swagger.Parameter{
			Description: "Global search query string.",
			Schema:      &swagger.Schema{Value: ""},
		},
	}
}

// FilterParam defines the details for a single filter query parameter.
type FilterParam struct {
	Name        string // The full query parameter name (e.g., "age_gte", "status_eq")
	Description string
	SchemaValue any // An example value for schema generation
}

// Schema helpers provide safe access to schema fields
func getSchemaDescription(schema *swagger.Schema) string {
	if schema == nil || schema.Value == nil {
		return ""
	}
	if valMap, ok := schema.Value.(map[string]any); ok {
		if desc, ok := valMap["description"].(string); ok {
			return desc
		}
	}
	return ""
}

// Define reusable schemas for common TUS headers
var (
	tusResumableSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"enum":        []string{"1.0.0"},
			"description": "Protocol version",
		},
	}
	tusVersionSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"description": "The Tus-Version response header MUST be a comma-separated list of protocol versions supported by the Server. The list MUST be sorted by Server's preference where the first one is the most preferred one.",
		},
	}
	tusExtensionSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"description": "The Tus-Extension response header MUST be a comma-separated list of the extensions supported by the Server. If no extensions are supported, the Tus-Extension header MUST be omitted.",
		},
	}
	tusMaxSizeSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "integer",
			"description": "The Tus-Max-Size response header MUST be a non-negative integer indicating the maximum allowed size of an entire upload in bytes. The Server SHOULD set this header if there is a known hard limit.",
		},
	}
	uploadLengthSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "integer",
			"description": "The Upload-Length request and response header indicates the size of the entire upload in bytes. The value MUST be a non-negative integer. In the concatenation extension, the Client MUST NOT include the Upload-Length header in the final upload creation",
		},
	}
	uploadOffsetSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "integer",
			"description": "The Upload-Offset request and response header indicates a byte offset within a resource. The value MUST be a non-negative integer.",
		},
	}
	tusChecksumAlgorithmSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"description": "Added by the checksum extension. The Tus-Checksum-Algorithm response header MUST be a comma-separated list of the checksum algorithms supported by the server.",
		},
	}
	uploadChecksumSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"description": "Added by the checksum extension. The Upload-Checksum request header contains information about the checksum of the current body payload. The header MUST consist of the name of the used checksum algorithm and the Base64 encoded checksum separated by a space.",
		},
	}
	uploadExpiresSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"format":      "date-time", // RFC 7231 datetime format
			"description": "Added by the expiration extension. The Upload-Expires response header indicates the time after which the unfinished upload expires. A Server MAY wish to remove incomplete uploads after a given period of time to prevent abandoned uploads from taking up extra storage. The Client SHOULD use this header to determine if an upload is still valid before attempting to resume the upload. This header MUST be included in every PATCH response if the upload is going to expire. If the expiration is known at the creation, the Upload-Expires header MUST be included in the response to the initial POST request. Its value MAY change over time. If a Client does attempt to resume an upload which has since been removed by the Server, the Server SHOULD respond with the 404 Not Found or 410 Gone status. The latter one SHOULD be used if the Server is keeping track of expired uploads. In both cases the Client SHOULD start a new upload. The value of the Upload-Expires header MUST be in RFC 7231 datetime format.",
		},
	}
	locationSchema = &swagger.Schema{
		Value: map[string]any{
			"type":        "string",
			"format":      "url",
			"description": "Url of the created resource.",
		},
	}
	cacheControlSchema = &swagger.Schema{
		Value: map[string]any{
			"type": "string",
			"enum": []string{"no-store"},
		},
	}
)

// TusPostSwagger generates Swagger definitions for the TUS POST /files operation.
func TusPostSwagger(summary, description string, errResp map[int]any) swagger.Definitions {
	definitions := swagger.Definitions{
		Summary:     summary,
		Description: description,
		Tags:        []string{"TUS"},
		Parameters: map[string]swagger.ParameterDefinition{ // Corrected type
			"Content-Length": {
				In:          "header",
				Description: "Must be 0 for creation extension. May be a positive number for Creation With Upload extension.",
				Schema:      &swagger.Schema{Value: 0}, // Example value
			},
			"Upload-Length": {
				In:       "header",
				Required: true, // Required by core protocol for initial creation
				Schema:   uploadLengthSchema,
			},
			"Tus-Resumable": {
				In:       "header",
				Required: true,
				Schema:   tusResumableSchema,
			},
			"Upload-Metadata": {
				In:          "header",
				Description: "Added by the Creation extension. The Upload-Metadata request and response header MUST consist of one or more comma-separated key-value pairs. The key and value MUST be separated by a space. The key MUST NOT contain spaces and commas and MUST NOT be empty. The key SHOULD be ASCII encoded and the value MUST be Base64 encoded. All keys MUST be unique. The value MAY be empty. In these cases, the space, which would normally separate the key and the value, MAY be left out. Since metadata can contain arbitrary binary values, Servers SHOULD carefully validate metadata values or sanitize them before using them as header values to avoid header smuggling.",
				Schema:      &swagger.Schema{Value: "filename bXktZmlsZS50eHQ="}, // Example value
			},
			"Upload-Concat": {
				In:          "header",
				Description: "Added by the Concatenation extension. Indicates whether the upload is either a partial or final upload.",
				Schema:      &swagger.Schema{Value: "partial"}, // Example value
			},
			"Upload-Defer-Length": {
				In:          "header",
				Description: "Added by the Creation Defer Length extension. Indicates that the size of the upload is not known currently.",
				Schema:      &swagger.Schema{Value: 1}, // Example value
			},
			"Upload-Offset": { // Included for Creation With Upload extension
				In:          "header",
				Description: "Added by the Creation With Upload Extension. Indicates the offset of the included data.",
				Schema:      uploadOffsetSchema,
			},
			"Upload-Checksum": { // Added by the Creation With Upload extension in combination with the checksum extension
				In:          "header",
				Description: "Added by the Creation With Upload Extension in combination with the checksum extension. Checksum of the included data.",
				Schema:      uploadChecksumSchema,
			},
		},
		RequestBody: &swagger.ContentValue{
			Description: "Remaining (possibly partial) content of the file. Required if Content-Length > 0.",
			Required:    false, // Only required if Content-Length > 0
			Content: swagger.Content{
				"application/offset+octet-stream": {
					Value: map[string]any{
						"type":   "string",
						"format": "binary",
					},
				},
			},
		},
		Responses: map[int]swagger.ContentValue{
			http.StatusCreated: {
				Description: "Created",
				Headers: map[string]string{
					"Tus-Resumable":  getSchemaDescription(tusResumableSchema),
					"Upload-Offset":  getSchemaDescription(uploadOffsetSchema),
					"Upload-Expires": getSchemaDescription(uploadExpiresSchema),
					"Location":       getSchemaDescription(locationSchema),
				},
			},
			http.StatusBadRequest: { // 400 Bad Request (e.g., Checksum algorithm not supported)
				Description: "Bad Request",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusPreconditionFailed: { // 412 Precondition Failed (e.g., Tus-Version mismatch)
				Description: "Precondition Failed",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
					"Tus-Version":   getSchemaDescription(tusVersionSchema),
				},
			},
			http.StatusRequestEntityTooLarge: { // 413 Request Entity Too Large
				Description: "Request Entity Too Large",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusUnsupportedMediaType: { // 415 Unsupported Media Type (e.g., Content-Type not application/offset+octet-stream)
				Description: "Unsupported Media Type",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			460: { // 460 Checksum Mismatch
				Description: "Checksums mismatch",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
		},
	}

	// Add additional error responses
	for status, v := range errResp { // Iterate over map[int]any
		// Assuming errResp values are schema examples, wrap them in ContentValue
		definitions.Responses[status] = swagger.ContentValue{
			Description: http.StatusText(status),
			Content:     swagger.Content{MediaTypeJSON: {Value: v}},
		}
	}

	return definitions
}

// TusHeadSwagger generates Swagger definitions for the TUS HEAD /files/{id} operation.
func TusHeadSwagger(summary, description string, errResp map[int]any) swagger.Definitions {
	definitions := swagger.Definitions{
		Summary:     summary,
		Description: description,
		Tags:        []string{"TUS"},
		Parameters: map[string]swagger.ParameterDefinition{ // Corrected type
			"id": {
				In:          "path",
				Required:    true,
				Description: "The ID of the upload resource.",
				Schema:      &swagger.Schema{Value: "string"}, // Example value
			},
			"Tus-Resumable": {
				In:       "header",
				Required: true,
				Schema:   tusResumableSchema,
			},
		},
		Responses: map[int]swagger.ContentValue{
			http.StatusOK: {
				Description: "Returns offset",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
					"Cache-Control": getSchemaDescription(cacheControlSchema),
					"Upload-Offset": getSchemaDescription(uploadOffsetSchema),
					"Upload-Length": getSchemaDescription(uploadLengthSchema),
				},
			},
			http.StatusForbidden: { // 403 Forbidden
				Description: "Forbidden",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusNotFound: { // 404 Not Found
				Description: "Not Found",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusGone: { // 410 Gone
				Description: "Gone",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusPreconditionFailed: { // 412 Precondition Failed
				Description: "Precondition Failed",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
					"Tus-Version":   getSchemaDescription(tusVersionSchema),
				},
			},
		},
	}

	// Add additional error responses
	for status, v := range errResp { // Iterate over map[int]any
		// Assuming errResp values are schema examples, wrap them in ContentValue
		definitions.Responses[status] = swagger.ContentValue{
			Description: http.StatusText(status),
			Content:     swagger.Content{MediaTypeJSON: {Value: v}},
		}
	}

	return definitions
}

// TusPatchSwagger generates Swagger definitions for the TUS PATCH /files/{id} operation.
func TusPatchSwagger(summary, description string, errResp map[int]any) swagger.Definitions {
	definitions := swagger.Definitions{
		Summary:     summary,
		Description: description,
		Tags:        []string{"TUS"},
		Parameters: map[string]swagger.ParameterDefinition{ // Corrected type
			"id": {
				In:          "path",
				Required:    true,
				Description: "The ID of the upload resource.",
				Schema:      &swagger.Schema{Value: "string"}, // Example value
			},
			"Tus-Resumable": {
				In:       "header",
				Required: true,
				Schema:   tusResumableSchema,
			},
			"Content-Length": {
				In:          "header",
				Required:    true,
				Description: "Length of the body of this request",
				Schema:      &swagger.Schema{Value: 0}, // Example value
			},
			"Upload-Offset": {
				In:          "header",
				Required:    true,
				Description: "The offset at which the upload should be continued.",
				Schema:      uploadOffsetSchema,
			},
			"Upload-Checksum": {
				In:          "header",
				Description: "Added by the checksum extension. Checksum of the current body payload.",
				Schema:      uploadChecksumSchema,
			},
		},
		RequestBody: &swagger.ContentValue{
			Description: "Remaining (possibly partial) content of the file. Required if Content-Length > 0.",
			Required:    false, // Only required if Content-Length > 0
			Content: swagger.Content{
				"application/offset+octet-stream": {
					Value: map[string]any{
						"type":   "string",
						"format": "binary",
					},
				},
			},
		},
		Responses: map[int]swagger.ContentValue{
			http.StatusNoContent: { // 204 No Content
				Description: "Upload offset was updated",
				Headers: map[string]string{
					"Tus-Resumable":  getSchemaDescription(tusResumableSchema),
					"Upload-Offset":  getSchemaDescription(uploadOffsetSchema),
					"Upload-Expires": getSchemaDescription(uploadExpiresSchema),
				},
			},
			http.StatusBadRequest: { // 400 Bad Request (e.g., Checksum algorithm not supported)
				Description: "Bad Request",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusForbidden: { // 403 Forbidden (e.g., PATCH against a final upload URL)
				Description: "Forbidden",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusNotFound: { // 404 Not Found
				Description: "Not Found",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusConflict: { // 409 Conflict (Upload-Offset mismatch)
				Description: "Conflict",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusGone: { // 410 Gone
				Description: "Gone",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusPreconditionFailed: { // 412 Precondition Failed
				Description: "Precondition Failed",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
					"Tus-Version":   getSchemaDescription(tusVersionSchema),
				},
			},
			http.StatusUnsupportedMediaType: { // 415 Unsupported Media Type
				Description: "Unsupported Media Type",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			460: { // 460 Checksum Mismatch
				Description: "Checksums mismatch",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
		},
	}

	// Add additional error responses
	for status, v := range errResp { // Iterate over map[int]any
		// Assuming errResp values are schema examples, wrap them in ContentValue
		definitions.Responses[status] = swagger.ContentValue{
			Description: http.StatusText(status),
			Content:     swagger.Content{MediaTypeJSON: {Value: v}},
		}
	}

	return definitions
}

// TusDeleteSwagger generates Swagger definitions for the TUS DELETE /files/{id} operation.
func TusDeleteSwagger(summary, description string, errResp map[int]any) swagger.Definitions {
	definitions := swagger.Definitions{
		Summary:     summary,
		Description: description,
		Tags:        []string{"TUS"},
		Parameters: map[string]swagger.ParameterDefinition{ // Corrected type
			"id": {
				In:          "path",
				Required:    true,
				Description: "The ID of the upload resource.",
				Schema:      &swagger.Schema{Value: "string"}, // Example value
			},
			"Tus-Resumable": {
				In:       "header",
				Required: true,
				Schema:   tusResumableSchema,
			},
		},
		Responses: map[int]swagger.ContentValue{
			http.StatusNoContent: { // 204 No Content
				Description: "Upload was terminated",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
				},
			},
			http.StatusPreconditionFailed: { // 412 Precondition Failed
				Description: "Precondition Failed",
				Headers: map[string]string{
					"Tus-Resumable": getSchemaDescription(tusResumableSchema),
					"Tus-Version":   getSchemaDescription(tusVersionSchema),
				},
			},
		},
	}

	// Add additional error responses
	for status, v := range errResp { // Iterate over map[int]any
		// Assuming errResp values are schema examples, wrap them in ContentValue
		definitions.Responses[status] = swagger.ContentValue{
			Description: http.StatusText(status),
			Content:     swagger.Content{MediaTypeJSON: {Value: v}},
		}
	}

	return definitions
}

// TusOptionsSwagger generates Swagger definitions for the TUS OPTIONS /files operation.
func TusOptionsSwagger(summary, description string, errResp map[int]any) swagger.Definitions {
	definitions := swagger.Definitions{
		Summary:     summary,
		Description: description,
		Tags:        []string{"TUS"},
		Parameters:  map[string]swagger.ParameterDefinition{}, // OPTIONS typically has no request parameters defined in the spec
		Responses: map[int]swagger.ContentValue{
			http.StatusOK: { // 200 OK
				Description: "Success",
				Headers: map[string]string{
					"Tus-Resumable":          getSchemaDescription(tusResumableSchema),
					"Tus-Checksum-Algorithm": getSchemaDescription(tusChecksumAlgorithmSchema),
					"Tus-Version":            getSchemaDescription(tusVersionSchema),
					"Tus-Max-Size":           getSchemaDescription(tusMaxSizeSchema),
					"Tus-Extension":          getSchemaDescription(tusExtensionSchema),
				},
			},
			http.StatusNoContent: { // 204 No Content
				Description: "Success",
				Headers: map[string]string{
					"Tus-Resumable":          getSchemaDescription(tusResumableSchema),
					"Tus-Checksum-Algorithm": getSchemaDescription(tusChecksumAlgorithmSchema),
					"Tus-Version":            getSchemaDescription(tusVersionSchema),
					"Tus-Max-Size":           getSchemaDescription(tusMaxSizeSchema),
					"Tus-Extension":          getSchemaDescription(tusExtensionSchema),
				},
			},
		},
	}

	// Add additional error responses
	for status, v := range errResp { // Iterate over map[int]any
		// Assuming errResp values are schema examples, wrap them in ContentValue
		definitions.Responses[status] = swagger.ContentValue{
			Description: http.StatusText(status),
			Content:     swagger.Content{MediaTypeJSON: {Value: v}},
		}
	}

	return definitions
}
